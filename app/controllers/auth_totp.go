package controllers

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/relay"
)

var (
	// errTOTPNotEnrolled means the account has no activated second factor.
	errTOTPNotEnrolled = errors.New("totp is not enabled")
	// errTOTPInvalid covers a wrong code and a replayed one: the caller must
	// not be able to tell them apart.
	errTOTPInvalid = errors.New("invalid authenticator code")
)

// totpOpts is the only TOTP profile the app accepts: 6 digits, SHA1, 30s step,
// and one step of skew on either side for clock drift.
var totpOpts = totp.ValidateOpts{Period: models.TOTPPeriod, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1}

// totpStepAt returns the RFC 6238 counter of an instant.
func totpStepAt(t time.Time) int64 { return t.Unix() / models.TOTPPeriod }

// matchTOTPStep finds which of the accepted steps produced code, so the exact
// step can be recorded and a later replay of the same code rejected. Steps at
// or below lastStep are refused (single use per window).
func matchTOTPStep(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	current := totpStepAt(now)
	for _, step := range []int64{current - 1, current, current + 1} {
		if step <= lastStep {
			continue
		}
		at := time.Unix(step*models.TOTPPeriod, 0)
		expected, err := totp.GenerateCodeCustom(secret, at, totpOpts)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// openTOTPSecret unseals the stored secret of a row.
func openTOTPSecret(row models.UserTOTP) (string, error) {
	secret, err := relay.OpenSecret(row.Secret)
	if err != nil || secret == "" {
		return "", errTOTPInvalid
	}
	return secret, nil
}

// consumeTOTP verifies a code against the stored secret and records the step
// it was spent on. requireActive gates activation (pending row) from login.
func consumeTOTP(db *database.Queries, userID uuid.UUID, code string, requireActive bool) error {
	row, err := db.GetTOTP(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errTOTPNotEnrolled
		}
		return err
	}
	if requireActive && row.ActivatedAt == nil {
		return errTOTPNotEnrolled
	}
	secret, err := openTOTPSecret(row)
	if err != nil {
		return err
	}
	step, ok := matchTOTPStep(secret, code, time.Now(), row.LastUsedStep)
	if !ok {
		return errTOTPInvalid
	}
	return db.SetTOTPLastStep(userID, step)
}

// TOTPStatus reports whether the session account has an active second factor.
// @Description Read the second-factor status of the session account.
// @Summary get TOTP status
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /auth/totp [get]
func TOTPStatus(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"totp": models.TOTPStatus{Enabled: db.TOTPEnrolled(userID)}})
}

// EnrollTOTP mints a fresh TOTP secret for the session account and returns the
// otpauth URL to scan. The factor stays inactive until a first code is
// confirmed, so a lost secret can never lock the account out.
// @Description Start TOTP enrollment.
// @Summary enroll TOTP
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /auth/totp/enroll [post]
func EnrollTOTP(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if db.TOTPEnrolled(userID) {
		return utils.Fail(c, fiber.StatusConflict, "two-factor authentication is already enabled", nil)
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "user")
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: configs.Get().AppName, AccountName: user.Email})
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to generate authenticator secret", nil)
	}
	sealed, err := relay.SealSecret(key.Secret())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to protect authenticator secret", nil)
	}
	if err := db.UpsertTOTP(userID, sealed); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to store authenticator secret", nil)
	}
	recordAudit(c, db, userID, "auth.totp.enroll", "user", userID.String(), "")
	return utils.OK(c, fiber.StatusOK, fiber.Map{"totp": models.TOTPEnrollResponse{Secret: key.Secret(), OtpauthURL: key.URL()}})
}

// ActivateTOTP confirms a pending enrollment with the first valid code.
// @Description Confirm TOTP enrollment.
// @Summary activate TOTP
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.TOTPActivateInput true "Authenticator code"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /auth/totp/activate [post]
func ActivateTOTP(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &models.TOTPActivateInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if err := consumeTOTP(db, userID, input.Code, false); err != nil {
		return totpFailure(c, err)
	}
	if err := db.ActivateTOTP(userID); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to activate two-factor authentication", nil)
	}
	recordAudit(c, db, userID, "auth.totp.activate", "user", userID.String(), "")
	return utils.OK(c, fiber.StatusOK, fiber.Map{"totp": models.TOTPStatus{Enabled: true}})
}

// DisableTOTP turns the second factor off; both the password and a live code
// are required, so a stolen session alone cannot weaken the account.
// @Description Disable TOTP.
// @Summary disable TOTP
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.TOTPDisableInput true "Password plus authenticator code"
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /auth/totp/disable [post]
func DisableTOTP(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &models.TOTPDisableInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "user")
	}
	if user.PasswordHash == "" || !utils.ComparePasswords(user.PasswordHash, input.Password) {
		return utils.Fail(c, fiber.StatusUnauthorized, "wrong password", nil)
	}
	if err := consumeTOTP(db, userID, input.Code, true); err != nil {
		return totpFailure(c, err)
	}
	if err := db.DeleteTOTP(userID); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to disable two-factor authentication", nil)
	}
	recordAudit(c, db, userID, "auth.totp.disable", "user", userID.String(), "")
	return c.SendStatus(fiber.StatusNoContent)
}

// totpFailure maps a second-factor error onto the API envelope.
func totpFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, errTOTPNotEnrolled):
		return utils.Fail(c, fiber.StatusConflict, "two-factor authentication is not enabled", fiber.Map{"code": "totp_not_enrolled"})
	case errors.Is(err, errTOTPInvalid):
		return utils.Fail(c, fiber.StatusUnauthorized, "invalid authenticator code", fiber.Map{"code": "totp_invalid"})
	default:
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to verify authenticator code", nil)
	}
}
