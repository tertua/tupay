package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
)

// @Description Get client by ID with invoices and stats.
// @Summary get client by ID with invoices and stats
// @Tags Clients
// @Accept json
// @Produce json
// @Param id path string true "Client ID"
// @Param overdue query string false "Set to 1 to return only overdue invoices (stats stay over all invoices)"
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

	var totalBilled, paidTotal decimal.Decimal
	pending := db.PendingInvoiceIDs(orgID)
	for _, row := range rows {
		// Anything not still a draft is billed: sent, overdue, paid, pending.
		if row.EffectiveStatus(pending[row.ID]) != models.InvoiceStatusDraft {
			totalBilled = totalBilled.Add(row.Total)
			paidTotal = paidTotal.Add(row.PaidAmount)
		}
	}

	// The optional overdue toggle narrows the returned list only; stats stay
	// over every invoice so the summary never changes with the filter.
	visible := rows
	if c.Query("overdue") == "1" {
		visible = filterOverdue(rows, pending)
	}

	invoices := make([]clientInvoiceRow, 0, len(visible))
	for _, row := range visible {
		paid := row.PaidAmount
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
