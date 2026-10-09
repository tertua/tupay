package controllers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
)

// UpdateUserStatus blocks or unblocks another account by setting user_status
// (0 == blocked, 1 == active). Self-change is refused, like role changes, so
// an admin cannot lock themselves out. Blocking also revokes the target's live
// session, which is what actually logs a currently-online user out; the login
// gate (gateAccountStatus) refuses any non-active account afterwards.
// @Description Block or unblock a user account.
// @Summary update user status
// @Tags Admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body models.StatusInput true "Status payload (0=blocked, 1=active)"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /admin/users/{id}/status [patch]
func UpdateUserStatus(c fiber.Ctx) error {
	adminID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid user id", nil)
	}
	// Blocking yourself would lock out the only session that can undo it.
	if adminID == userID {
		return utils.Fail(c, fiber.StatusBadRequest, "you cannot change your own status", nil)
	}
	input := &models.StatusInput{}
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
	if err := db.UpdateUserStatus(userID, *input.Status); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update user status", nil)
	}
	// Blocking must also drop the live session; refreshSession only checks the
	// account exists, not its status, so without this a blocked user stays
	// logged in until their next refresh is rejected.
	if *input.Status == models.UserStatusBlocked {
		if err := deleteRefreshToken(c.Context(), userID); err != nil {
			utils.RequestLogger(c).Warn("session revoke on block failed", "user_id", userID.String(), "err", err)
		}
	}
	recordAudit(c, db, adminID, "user.status.update", "user", userID.String(),
		`{"status":`+strconv.Itoa(*input.Status)+`}`)
	user.UserStatus = *input.Status
	return utils.OK(c, fiber.StatusOK, fiber.Map{"user": adminUserResponse(user)})
}

// DeleteUser removes an account that has no invoice records yet, with every
// row keyed to it (memberships, external identities, verifications, resets).
// Self-delete is refused like role/status changes, and a user with any invoice
// record — even a draft — is refused with 409 so the books keep their creator
// reference. The target's live session is revoked like on block.
// @Description Delete a user account without invoice records.
// @Summary delete a user
// @Tags Admin
// @Param id path string true "User ID"
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /admin/users/{id} [delete]
func DeleteUser(c fiber.Ctx) error {
	adminID, err := utils.CurrentUserID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid user id", nil)
	}
	// Deleting yourself would orphan the session mid-request with no admin left to undo it.
	if adminID == userID {
		return utils.Fail(c, fiber.StatusBadRequest, "you cannot delete your own account", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetUserByID(userID); err != nil {
		return utils.NotFoundOrFailed(c, err, "user")
	}
	invoices, err := db.CountInvoicesByUser(userID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count user invoices", nil)
	}
	if invoices > 0 {
		return utils.Fail(c, fiber.StatusConflict, "user has invoice records", fiber.Map{"invoices": invoices})
	}
	if err := db.DeleteUserAccount(userID); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete user", nil)
	}
	if err := deleteRefreshToken(c.Context(), userID); err != nil {
		utils.RequestLogger(c).Warn("session revoke on delete failed", "user_id", userID.String(), "err", err)
	}
	recordAudit(c, db, adminID, "user.delete", "user", userID.String(), "")
	return c.SendStatus(fiber.StatusNoContent)
}
