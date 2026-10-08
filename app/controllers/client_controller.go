package controllers

import (
	"strings"
	"time"

	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ListClients returns one page of clients for the current user.
// @Description Get all clients of current user.
// @Summary get all clients of current user
// @Tags Clients
// @Accept json
// @Produce json
// @Param q query string false "Case-insensitive search over name, company, email"
// @Param status query string false "Lifecycle status: active or archived (empty = all)"
// @Param sort query string false "Sort column: name, created_at, total_billed, outstanding (default created_at)"
// @Param order query string false "Sort direction: asc or desc (default desc)"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients [get]
func ListClients(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	search := strings.TrimSpace(c.Query("q"))
	status := strings.TrimSpace(c.Query("status"))
	sort := strings.TrimSpace(c.Query("sort"))
	order := strings.TrimSpace(c.Query("order"))

	paging := utils.ParsePagination(c)
	clients, err := db.ListClients(orgID, search, status, sort, order, paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load clients", nil)
	}
	total, err := db.CountClients(orgID, status, search)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count clients", nil)
	}

	return utils.OK(c, fiber.StatusOK, fiber.Map{"clients": clients, "meta": paging.Meta(total)})
}

// @Description Get client by ID with invoices and stats.
// @Summary get client by ID with invoices and stats
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id} [get]
func GetClient(c fiber.Ctx) error {
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

	rows, err := db.ClientInvoices(orgID, id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client invoices", nil)
	}

	invoices := make([]clientInvoiceRow, 0, len(rows))
	var totalBilled, paidTotal decimal.Decimal
	pending := db.PendingInvoiceIDs(orgID)
	for _, row := range rows {
		paid := row.PaidAmount
		// Anything not still a draft is billed: sent, overdue, paid, pending.
		if row.EffectiveStatus(pending[row.ID]) != models.InvoiceStatusDraft {
			totalBilled = totalBilled.Add(row.Total)
			paidTotal = paidTotal.Add(paid)
		}
		invoices = append(invoices, clientInvoiceRow{
			ID:              row.ID,
			InvoiceNumber:   row.InvoiceNumber,
			IssueDate:       utils.FormatDate(row.IssueDate),
			DueDate:         utils.FormatDate(row.DueDate),
			Total:           row.Total,
			Currency:        row.Currency,
			Status:          row.Status,
			EffectiveStatus: row.EffectiveStatus(pending[row.ID]),
			PaidAmount:      paid,
			Balance:         row.Total.Sub(paid),
		})
	}

	stats := models.ClientStats{
		Count:       len(rows),
		TotalBilled: totalBilled,
		Outstanding: totalBilled.Sub(paidTotal),
	}

	return utils.OK(c, fiber.StatusOK, clientDetailResponse{
		Client:   client,
		Invoices: invoices,
		Stats:    stats,
	})
}

// CreateClient creates a new client.
// @Description Create a new client.
// @Summary create a new client
// @Tags Clients
// @Accept json
// @Produce json
// @Param request body models.ClientInput true "Create client payload"
// @Success 201 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients [post]
func CreateClient(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	input := &models.ClientInput{}
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
	now := time.Now()
	client := &models.Client{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: &now,
		OrgID:     orgID,
		UserID:    utils.CurrentActorID(c),
		Name:      input.Name,
		Email:     input.Email,
		Company:   input.Company,
		Phone:     input.Phone,
		Address:   input.Address,
		Notes:     input.Notes,
	}
	if err := db.CreateClient(client); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create client", nil)
	}
	invalidateAggregates(c, orgID)

	return utils.OK(c, fiber.StatusCreated, fiber.Map{"client": client})
}

// UpdateClient updates a client.
// @Description Update a client.
// @Summary update a client
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Param request body models.ClientInput true "Update client payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /clients/{id} [patch]
func UpdateClient(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid client id", nil)
	}

	input := &models.ClientInput{}
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

	client, err := db.GetClient(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}

	now := time.Now()
	client.UpdatedAt = &now
	client.Name = input.Name
	client.Email = input.Email
	client.Company = input.Company
	client.Phone = input.Phone
	client.Address = input.Address
	client.Notes = input.Notes

	if err := db.UpdateClient(&client); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update client", nil)
	}
	invalidateAggregates(c, orgID)

	return utils.OK(c, fiber.StatusOK, fiber.Map{"client": client})
}

// DeleteClient deletes a client.
// @Description Delete a client.
// @Summary delete a client
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Success 204 {string} status "ok"
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

	if err := db.DeleteClient(orgID, id); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete client", nil)
	}
	recordAudit(c, db, utils.CurrentActorID(c), "client.delete", "client", id.String(), "")
	invalidateAggregates(c, orgID)

	return c.SendStatus(fiber.StatusNoContent)
}
