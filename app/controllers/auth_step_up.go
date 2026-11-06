package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/middleware"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/cache"
	"github.com/tertua/tupay/platform/database"
)

// StepUpInput is the re-authentication payload: the password for password
// accounts, plus the authenticator code whenever the account has one.
type StepUpInput struct {
	Password string `json:"password,omitempty" validate:"omitempty,lte=255"`
	Code     string `json:"code,omitempty" validate:"omitempty,len=6,numeric"`
}

// StepUpResponse hands the client the single-use token and its lifetime.
type StepUpResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// errStepUpUnavailable means the account has no usable factor (password-less
// SSO without TOTP), so the destructive action must stay refused.
var errStepUpUnavailable = errors.New("no second factor available")

// errStepUpCodeRequired means a password account has TOTP enrolled but the
// step-up request did not supply a code.
var errStepUpCodeRequired = errors.New("authenticator code required")

// errStepUpInvalid covers a wrong password or a wrong/replayed code; the
// caller must not learn which one failed.
var errStepUpInvalid = errors.New("wrong step-up credential")

// StepUp re-authenticates the session account and mints a single-use token for
// one destructive request. A password account may confirm with its password;
// when TOTP is enabled the code is required too, and a password-less SSO
// account can only confirm with TOTP.
// @Description Re-authenticate and mint a step-up token.
// @Summary mint step-up token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body controllers.StepUpInput true "Password and/or authenticator code"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /auth/step-up [post]
func StepUp(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &StepUpInput{}
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
	// Step-up verifies a password, so it shares the sign-in lockout: a stolen
	// session cannot brute-force the password past the same threshold as login.
	if remaining := loginLockRemaining(c.Context(), user.Email); remaining > 0 {
		return loginLocked(c, remaining)
	}
	if err := confirmIdentity(db, user, input); err != nil {
		return stepUpFailure(c, db, user, err)
	}
	sid, err := liveSessionID(c, userID)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	token, err := mintStepUp(c, userID, sid)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to start step-up confirmation", nil)
	}
	recordAudit(c, db, userID, "auth.step_up", "user", userID.String(), "")
	return utils.OK(c, fiber.StatusOK, fiber.Map{
		"step_up": StepUpResponse{Token: token, ExpiresIn: int(cache.StepUpTTL().Seconds())},
	})
}

// confirmIdentity verifies whichever factors the account actually has.  It
// fails closed: a DB error propagates so the caller can refuse the request.
func confirmIdentity(db *database.Queries, user models.User, input *StepUpInput) error {
	if user.PasswordHash == "" {
		if input.Code == "" {
			return errStepUpUnavailable
		}
		return stepUpFactorError(consumeTOTP(db, user.ID, input.Code, true))
	}
	if !utils.ComparePasswords(user.PasswordHash, input.Password) {
		return errStepUpInvalid
	}
	enrolled, err := db.TOTPEnrolled(user.ID)
	if err != nil {
		return err
	}
	if input.Code == "" {
		if enrolled {
			return errStepUpCodeRequired
		}
		return nil
	}
	return stepUpFactorError(consumeTOTP(db, user.ID, input.Code, true))
}

// stepUpFactorError keeps a missing factor from leaking as a code failure: an
// account without an active second factor has nothing to confirm with.
func stepUpFactorError(err error) error {
	if errors.Is(err, errTOTPNotEnrolled) {
		return errStepUpUnavailable
	}
	return err
}

// stepUpFailure maps a confirmation error onto the API envelope, counting a
// wrong factor against the sign-in lockout like a failed login.
func stepUpFailure(c fiber.Ctx, db *database.Queries, user models.User, err error) error {
	switch {
	case errors.Is(err, errStepUpUnavailable):
		return utils.Fail(c, fiber.StatusForbidden, "this account has no second factor configured", fiber.Map{"code": "step_up_unavailable"})
	case errors.Is(err, errStepUpCodeRequired):
		return utils.Fail(c, fiber.StatusUnauthorized, "authenticator code required", fiber.Map{"code": "totp_required"})
	case errors.Is(err, errTOTPInvalid):
		recordBadCredential(c, db, user.Email, "step_up", user.ID)
		return utils.Fail(c, fiber.StatusUnauthorized, "wrong password or authenticator code", fiber.Map{"code": "step_up_invalid"})
	case errors.Is(err, errStepUpInvalid):
		recordBadCredential(c, db, user.Email, "step_up", user.ID)
		return utils.Fail(c, fiber.StatusUnauthorized, "wrong password or authenticator code", fiber.Map{"code": "step_up_invalid"})
	default:
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to verify credentials", nil)
	}
}

// liveSessionID returns the sid of the current session so the minted token dies
// with it.
func liveSessionID(c fiber.Ctx, userID uuid.UUID) (string, error) {
	store, err := cache.Sessions()
	if err != nil {
		return "", err
	}
	stored, err := store.Get(c.Context(), userID.String())
	if err != nil {
		return "", err
	}
	sid, _, _, _, ok := cache.DecodeSessionValue(stored)
	if !ok {
		return "", cache.ErrSessionNotFound
	}
	return sid, nil
}

// mintStepUp creates and stores one token for the session.
func mintStepUp(c fiber.Ctx, userID uuid.UUID, sid string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	ttl := configs.Get().Auth.StepUp()
	value := cache.EncodeStepUpValue(userID, sid, time.Now().Add(ttl))
	if err := cache.StoreStepUp(c.Context(), token, value, ttl); err != nil {
		return "", err
	}
	return token, nil
}

// requireLoginTOTP enforces the second factor at sign-in: an enrolled account
// must present a live code, and a missing or wrong one counts as a bad
// credential so guessing codes hits the same lockout as guessing passwords.
func requireLoginTOTP(c fiber.Ctx, db *database.Queries, user models.User, code string) bool {
	enrolled, err := db.TOTPEnrolled(user.ID)
	if err != nil {
		_ = utils.Fail(c, fiber.StatusInternalServerError, "failed to verify second factor", nil)
		return true
	}
	if !enrolled {
		return false
	}
	if code == "" {
		recordBadCredential(c, db, user.Email, "totp_missing", user.ID)
		_ = utils.Fail(c, fiber.StatusUnauthorized, "authenticator code required", fiber.Map{"code": "totp_required"})
		return true
	}
	if err := consumeTOTP(db, user.ID, code, true); err != nil {
		recordBadCredential(c, db, user.Email, "totp_invalid", user.ID)
		_ = utils.Fail(c, fiber.StatusUnauthorized, "invalid authenticator code", fiber.Map{"code": "totp_invalid"})
		return true
	}
	return false
}

// StepUpHeaderName is re-exported for tests and the OpenAPI docs.
const StepUpHeaderName = middleware.StepUpHeader
