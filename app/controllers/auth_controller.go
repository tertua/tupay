package controllers

import (
	"database/sql"
	"errors"

	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/middleware"
	"github.com/tertua/tupay/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// publicUser returns the user fields exposed to the frontend.
func publicUser(u models.User) publicUserRow {
	return publicUserRow{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
		Role:  u.UserRole,
	}
}

type publicUserRow struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

// Login authenticates a user and starts a session.
// @Description Auth user and start session.
// @Summary auth user and start session
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.Login true "Login payload"
// @Param X-Captcha-Token header string false "Turnstile token (required when CAPTCHA is enabled)"
// @Success 200 {object} map[string]interface{}
// @Router /auth/login [post]
func Login(c fiber.Ctx) error {
	if !checkCaptcha(c) {
		return nil
	}
	payload := &models.Login{}

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

	payload.Email = normalizeEmail(payload.Email)
	// Refuse before touching the password: a locked account must not keep
	// validating guesses, even ones the hash would accept.
	if remaining := loginLockRemaining(c.Context(), payload.Email); remaining > 0 {
		return loginLocked(c, remaining)
	}
	user, err := db.GetUserByEmail(payload.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordBadCredential(c, db, payload.Email, "not_found", uuid.Nil)
			return utils.Fail(c, fiber.StatusUnauthorized, "wrong email address or password", nil)
		}
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to query user account", nil) // live DB error, not a bad credential
	}
	if !utils.ComparePasswords(user.PasswordHash, payload.Password) {
		recordBadCredential(c, db, payload.Email, "bad_password", user.ID)
		return utils.Fail(c, fiber.StatusUnauthorized, "wrong email address or password", nil)
	}
	if gateAccountStatus(c, db, user) {
		return nil
	}

	if _, err := startSession(c, db, user.ID, loginOrgHint(db, user.ID)); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to start session", nil)
	}
	clearLoginFailures(c.Context(), payload.Email)
	recordAudit(c, db, user.ID, "auth.login.success", "user", user.ID.String(), "")

	user.PasswordHash = ""
	return utils.OK(c, fiber.StatusOK, fiber.Map{"user": publicUser(user)})
}

// Logout ends the session.
// @Description De-authorize user and delete session.
// @Summary de-authorize user and delete session
// @Tags Auth
// @Accept json
// @Produce json
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /auth/logout [post]
func Logout(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		clearStaleSession(c)
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	// Audit is best-effort (never fails logout): no handle, no trail.
	if db, ok := openDB(c); ok {
		recordAudit(c, db, userID, "auth.logout", "user", userID.String(), "")
	}
	if err := deleteRefreshToken(c.Context(), userID); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete session", nil)
	}
	utils.ClearSession(c)
	middleware.ClearCSRFCookie(c)

	return c.SendStatus(fiber.StatusNoContent)
}

// Me returns the current session user.
// @Description Get current session user.
// @Summary get current session user
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /auth/me [get]
func Me(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		clearStaleSession(c)
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "user not found", nil)
	}

	user.PasswordHash = ""
	return utils.OK(c, fiber.StatusOK, fiber.Map{"user": publicUser(user), "org": orgPayload(db, userID, activeOrgHint(c))})
}

// UpdateProfile updates the current user display name.
// @Description Update current user profile.
// @Summary update current user profile
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.UpdateProfile true "Update profile payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /auth/profile [patch]
func UpdateProfile(c fiber.Ctx) error {
	userID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	payload := &models.UpdateProfile{}
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

	if err := db.UpdateUserProfile(userID, payload.Name); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update profile", nil)
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "user not found", nil)
	}

	user.PasswordHash = ""
	return utils.OK(c, fiber.StatusOK, fiber.Map{"user": publicUser(user)})
}
