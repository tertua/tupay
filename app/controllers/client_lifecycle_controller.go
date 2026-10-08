package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
)

// ArchiveClient marks a client as archived. Owner-only at the route level;
// archiving an already-archived client is a stable 409.
// @Description Archive a client.
// @Summary archive a client
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id}/archive [patch]
func ArchiveClient(c fiber.Ctx) error {
	return setClientStatus(c, true)
}

// UnarchiveClient returns an archived client to the active list. Owner-only at
// the route level; unarchiving an already-active client is a stable 409.
// @Description Unarchive a client.
// @Summary unarchive a client
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id}/unarchive [patch]
func UnarchiveClient(c fiber.Ctx) error {
	return setClientStatus(c, false)
}

// setClientStatus is the shared archive/unarchive body: resolve scope, load the
// client for 404/current-state, reject a no-op flip with 409, persist, audit and
// invalidate aggregates.
func setClientStatus(c fiber.Ctx, archived bool) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid client id", nil)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	client, err := db.GetClient(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}

	current := client.Status
	if current == "" {
		current = models.ClientStatusActive
	}
	target := models.ClientStatusActive
	action := "client.unarchive"
	conflict := "client not archived"
	if archived {
		target = models.ClientStatusArchived
		action = "client.archive"
		conflict = "client already archived"
	}
	if current == target {
		return utils.Fail(c, fiber.StatusConflict, conflict, nil)
	}

	if _, err := db.ArchiveClient(orgID, id, archived); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update client", nil)
	}
	recordAudit(c, db, utils.CurrentActorID(c), action, "client", id.String(), "")
	invalidateAggregates(c, orgID)

	client.Status = target
	return utils.OK(c, fiber.StatusOK, fiber.Map{"client": client})
}
