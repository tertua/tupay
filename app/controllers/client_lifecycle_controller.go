package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/outbox"
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

// DeleteClient deletes a client, refusing while any open receivable (sent,
// overdue, or pending) still exists so a client with unpaid invoices cannot be
// lost. Drafts and paid invoices do not block.
// @Description Delete a client.
// @Summary delete a client
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Success 204 {string} status "ok"
// @Failure 409 {object} map[string]interface{} "client has open invoices"
// @Security SessionCookie
// @Router /clients/{id} [delete]
func DeleteClient(c fiber.Ctx) error {
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

	if _, err := db.GetClient(orgID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}

	rows, err := db.ClientInvoices(orgID, id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client invoices", nil)
	}
	if n := openInvoiceCount(rows, db.PendingInvoiceIDs(orgID)); n > 0 {
		return utils.Fail(c, fiber.StatusConflict, "client has open invoices", fiber.Map{"open_invoices": n})
	}

	if err := db.DeleteClient(orgID, id); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete client", nil)
	}
	recordAudit(c, db, utils.CurrentActorID(c), "client.delete", "client", id.String(), "")
	invalidateAggregates(c, orgID)

	return c.SendStatus(fiber.StatusNoContent)
}

// SendClientReminder queues a one-off payment reminder for every open invoice of
// a client, reusing the scheduled sweep's render/enqueue/event path. Each
// (invoice, manual) leg is claimed before sending, so a double-click is
// idempotent and the scheduled before/after legs are left untouched.
// @Description Send a payment reminder for a client's open invoices.
// @Summary send a client reminder
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Success 202 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id}/reminder [post]
func SendClientReminder(c fiber.Ctx) error {
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

	if _, err := db.GetClient(orgID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}

	rows, err := db.ClientOpenInvoiceReminderRows(orgID, id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client invoices", nil)
	}

	queued, skipped := 0, 0
	for _, row := range rows {
		sent, err := outbox.EnqueueInvoiceReminder(orgID, row, models.InvoiceReminderKindManual)
		if err != nil {
			return utils.Fail(c, fiber.StatusInternalServerError, "failed to queue reminder", nil)
		}
		if sent {
			queued++
		} else {
			skipped++
		}
	}

	recordAudit(c, db, utils.CurrentActorID(c), "client.reminder", "client", id.String(), "")
	return utils.OK(c, fiber.StatusAccepted, fiber.Map{"queued": queued, "skipped": skipped})
}
