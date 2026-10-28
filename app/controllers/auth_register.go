package controllers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/app/queries"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/middleware"
	"github.com/tertua/tupay/pkg/repository"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/captcha"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/mail"
	"github.com/tertua/tupay/platform/relay"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// checkCaptcha enforces Turnstile verification when configured (disabled/no secret in dev/test, fails closed otherwise) and writes the 403 response itself: callers must return nil when it reports false.
func checkCaptcha(c fiber.Ctx) bool {
	if !captcha.Required() {
		return true
	}
	if err := captcha.Verify(c.Context(), c.Get(captcha.TokenHeader), c.IP()); err != nil {
		_ = utils.Fail(c, fiber.StatusForbidden, "captcha verification failed", nil)
		return false
	}
	return true
}

// inviteErrorMessage maps invite query failures to stable English: a missing or expired token reports "invite expired", revoked/accepted keep the query's own wording.
func inviteErrorMessage(err error) string {
	if errors.Is(err, queries.ErrInviteRevoked) || errors.Is(err, queries.ErrInviteAccepted) {
		return err.Error()
	}
	return "invite expired"
}

// registrationOpen reports whether a register request may proceed: always when registration is allowed or the install has no users yet (the first account bootstraps, and no invite can exist before it), otherwise only when an invite token is present. Presence alone is not validity — a bad token fails later in invite resolution.
func registrationOpen(allow bool, userCount int64, inviteToken string) bool {
	return allow || userCount == 0 || inviteToken != ""
}

// Register creates a new user and starts a session.
// @Description Register a new user.
// @Summary register a new user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.Register true "Register payload"
// @Param X-Captcha-Token header string false "Turnstile token (required when CAPTCHA is enabled)"
// @Success 201 {object} map[string]interface{}
// @Router /auth/register [post]
func Register(c fiber.Ctx) error {
	if !checkCaptcha(c) {
		return nil
	}
	payload := &models.Register{}

	if err := c.Bind().Body(payload); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	// Canonicalize before validation so the stored address matches every later
	// lookup (the validator's `email` rule still accepts a lowercase address).
	payload.Email = normalizeEmail(payload.Email)
	if err := utils.NewValidator().Struct(payload); err != nil {
		return utils.ValidationFailed(c, err)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	inviteToken := strings.TrimSpace(payload.InviteToken)
	count, err := db.CountUsers()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to determine account role", nil)
	}
	// Invite-only gate: with registration disabled only a valid org invite may register, and the first account is always exempt (no invite can exist before the first admin). The closed check runs before the email lookup so a token-less register cannot probe which emails exist via 409 vs 403.
	if !registrationOpen(configs.Get().Auth.AllowRegistration, count, inviteToken) {
		return utils.Fail(c, fiber.StatusForbidden, "registration is disabled", nil)
	}

	if _, err := db.GetUserByEmail(payload.Email); err == nil {
		return utils.Fail(c, fiber.StatusConflict, "email is already registered", nil)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return utils.Fail(c, fiber.StatusInternalServerError, "database query error", nil)
	}

	// The invite is resolved before any write so a bad token never leaves an orphaned user row.
	var invite *models.OrgInvite
	if inviteToken != "" {
		row, ierr := db.GetValidByToken(inviteToken)
		if ierr != nil {
			return utils.Fail(c, fiber.StatusBadRequest, inviteErrorMessage(ierr), nil)
		}
		invite = &row
	}

	role := repository.UserRoleName
	if count == 0 {
		// First-install bootstrap: on a fresh platform the first account becomes the admin. A tie between two concurrent first registers can yield two admins, which the admin console can demote.
		role = repository.AdminRoleName
	}

	user := &models.User{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Name:         payload.Name,
		Email:        payload.Email,
		PasswordHash: utils.GeneratePassword(payload.Password),
		UserStatus:   verifyEmailGate(count), // 0 == blocked, 1 == active, 2 == pending verification
		UserRole:     role,
	}
	if err := utils.NewValidator().Struct(user); err != nil {
		return utils.ValidationFailed(c, err)
	}
	if err := db.CreateUser(user); err != nil {
		// The earlier GetUserByEmail check is a fast path, not a lock: a
		// concurrent register for the same email slips past it and only the
		// unique index stops the second insert, which is a 409, not a 500.
		if isUniqueViolation(err) {
			return utils.Fail(c, fiber.StatusConflict, "email is already registered", nil)
		}
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create user", nil)
	}

	// An invite joins the inviting org (no personal org is provisioned); a token-less register creates or reuses the personal tenant.
	var orgID uuid.UUID
	if invite != nil {
		// Claim before joining: single-use holds under concurrent redemption, and a lost race joins nothing.
		if err := db.MarkAccepted(invite.ID, user.ID); err != nil {
			return utils.Fail(c, fiber.StatusBadRequest, inviteErrorMessage(err), nil)
		}
		orgID = invite.OrgID
		if err := db.CreateIfAbsent(orgID, user.ID, inviteRole(*invite)); err != nil {
			return utils.Fail(c, fiber.StatusInternalServerError, "failed to create user", nil)
		}
	} else {
		orgID, err = db.EnsurePersonalOrg(user.ID)
		if err != nil {
			return utils.Fail(c, fiber.StatusInternalServerError, "failed to create user", nil)
		}
	}

	// Create default settings row (ignored when it already exists).
	settings := models.DefaultSettings(orgID)
	if err := db.CreateSettings(settings); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create user settings", nil)
	}

	// A pending account gets a verification link instead of a session: the email
	// is proof of ownership, and the response never carries cookies or a user.
	if user.UserStatus == models.UserStatusPending {
		// Enqueue is best-effort (the outbox retries deliver): a failure here must
		// not strand the account, which the user can re-trigger via resend.
		_ = enqueueVerification(c, db, *user)
		recordAudit(c, db, user.ID, "auth.register", "user", user.ID.String(), "")
		return utils.OK(c, fiber.StatusCreated, fiber.Map{"status": "verification_required", "message": "verification email sent"})
	}

	if _, err := startSession(c, db, user.ID, orgID.String()); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to start session", nil)
	}
	recordAudit(c, db, user.ID, "auth.register", "user", user.ID.String(), "")

	user.PasswordHash = ""
	return utils.OK(c, fiber.StatusCreated, fiber.Map{"user": publicUser(*user)})
}

// ForgotPassword creates a password reset token for the given email.
// @Description Request password reset token.
// @Summary request password reset token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.ForgotPassword true "Forgot password payload"
// @Param X-Captcha-Token header string false "Turnstile token (required when CAPTCHA is enabled)"
// @Success 200 {object} map[string]interface{}
// @Router /auth/forgot-password [post]
func ForgotPassword(c fiber.Ctx) error {
	if !checkCaptcha(c) {
		return nil
	}
	payload := &models.ForgotPassword{}

	if err := c.Bind().Body(payload); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(payload); err != nil {
		return utils.ValidationFailed(c, err)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	// Always respond generically to avoid email enumeration; delivery is async (the worker sends the email, retries included).
	if user, err := db.GetUserByEmail(normalizeEmail(payload.Email)); err == nil {
		enqueuePasswordReset(c, db, user)
	}

	return utils.OK(c, fiber.StatusOK, fiber.Map{"message": "if the email exists, a reset link was sent"})
}

// enqueuePasswordReset mints a fresh single-use password reset token for user,
// stores only its hash, and queues the email. Best-effort on delivery: any error
// is logged but not surfaced, since the caller must always respond generically
// to avoid email enumeration. Callers must be inside an enumeration-safe path.
func enqueuePasswordReset(c fiber.Ctx, db *database.Queries, user models.User) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		utils.RequestLogger(c).Warn("password reset token gen failed", "err", err)
		return
	}
	token := hex.EncodeToString(raw)
	if err := db.DeletePasswordResetsByUser(user.ID); err != nil {
		utils.RequestLogger(c).Warn("password reset prior-token delete failed", "err", err)
		return
	}
	// Store only the hash: a DB leak must not hand out live reset links (same rule as service API keys, relay.HashKey).
	if err := db.CreatePasswordReset(user.ID, relay.HashKey(token), time.Now().Add(time.Hour)); err != nil {
		utils.RequestLogger(c).Warn("password reset token store failed", "err", err)
		return
	}
	resetURL := strings.TrimRight(configs.Get().Mail.AppPublicURL, "/") + "/reset-password?token=" + token
	body := fmt.Sprintf("Hello %s,\n\nReset your password using this link:\n%s\n\nThis link expires in one hour.", user.Name, resetURL)
	htmlBody, terr := mail.Render("reset_password", mail.TemplateData{
		AppName: configs.Get().AppName, Name: user.Name, URL: resetURL,
	})
	if terr != nil {
		utils.RequestLogger(c).Warn("password reset email template failed", "err", terr)
	}
	if err := db.EnqueueMail(&models.MailOutbox{
		To:       user.Email,
		Subject:  "Reset your " + configs.Get().AppName + " password",
		Body:     body,
		HtmlBody: htmlBody,
	}); err != nil {
		utils.RequestLogger(c).Warn("password reset email queue failed", "email", user.Email, "err", err)
	}
}

// ResetPassword resets the password using a reset token.
// @Description Reset password with token.
// @Summary reset password with token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.ResetPassword true "Reset password payload"
// @Param X-Captcha-Token header string false "Turnstile token (required when CAPTCHA is enabled)"
// @Success 200 {object} map[string]interface{}
// @Router /auth/reset-password [post]
func ResetPassword(c fiber.Ctx) error {
	if !checkCaptcha(c) {
		return nil
	}
	payload := &models.ResetPassword{}

	if err := c.Bind().Body(payload); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(payload); err != nil {
		return utils.ValidationFailed(c, err)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	reset, err := db.GetPasswordReset(relay.HashKey(strings.TrimSpace(payload.Token)))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid or expired token", nil)
	}
	if time.Now().After(reset.ExpiresAt) {
		_ = db.DeletePasswordResetsByUser(reset.UserID)
		return utils.Fail(c, fiber.StatusBadRequest, "invalid or expired token", nil)
	}

	if err := db.UpdateUserPassword(reset.UserID, utils.GeneratePassword(payload.NewPassword)); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update password", nil)
	}
	_ = db.DeletePasswordResetsByUser(reset.UserID)
	// Inbox control outranks a stale lockout counter: the reset proves ownership,
	// so the next sign-in must not walk into a lock armed by an attacker.
	if user, err := db.GetUserByID(reset.UserID); err == nil {
		clearLoginFailures(c.Context(), user.Email)
	}
	// The password changed out-of-band: the old session (if any) dies here, and any stale cookies lingering in this browser are cleared too.
	_ = deleteRefreshToken(c.Context(), reset.UserID)
	utils.ClearSession(c)
	middleware.ClearCSRFCookie(c)
	recordAudit(c, db, reset.UserID, "auth.password.reset", "user", reset.UserID.String(), "")

	return utils.OK(c, fiber.StatusOK, fiber.Map{"message": "password updated"})
}
