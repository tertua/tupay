package controllers

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/queries"
	"github.com/tertua/tupay/pkg/utils"
)

// ExportClientStatement streams a client's issued-invoice statement as a CSV
// attachment (invoice_number, issue_date, due_date, status, total, paid_amount,
// balance, currency). It deliberately bypasses the JSON envelope: the body is
// raw text/csv so the browser downloads it. Optional from/to filter the
// issue_date inclusively.
// @Description Download a client's invoice statement as CSV.
// @Summary export a client statement
// @Tags Clients
// @Produce text/csv
// @Param id path string true "Client ID"
// @Param from query string false "Earliest issue_date (YYYY-MM-DD)"
// @Param to query string false "Latest issue_date (YYYY-MM-DD)"
// @Success 200 {string} string "CSV"
// @Security SessionCookie
// @Router /clients/{id}/statement.csv [get]
func ExportClientStatement(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid client id", nil)
	}

	from, err := utils.ParseDate(strings.TrimSpace(c.Query("from")))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid from date", nil)
	}
	to, err := utils.ParseDate(strings.TrimSpace(c.Query("to")))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid to date", nil)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	client, err := db.GetClient(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "client")
	}

	rows, err := db.ClientStatementRows(orgID, id, from, to)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client invoices", nil)
	}

	csv := clientStatementCSV(rows, db.PendingInvoiceIDs(orgID))
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="client-`+statementSlug(client.Name)+`-statement.csv"`)
	return c.SendString(csv)
}

// clientStatementCSV renders statement rows as CSV with a header row. Money is
// a decimal string, dates are YYYY-MM-DD, and every cell is quoted with inner
// `"` doubled (matching the Reports page's CSV escaping).
func clientStatementCSV(rows []queries.ClientInvoiceRow, pending map[uuid.UUID]bool) string {
	var b strings.Builder
	writeCSVRow(&b, []string{"invoice_number", "issue_date", "due_date", "status", "total", "paid_amount", "balance", "currency"})
	for _, row := range rows {
		balance := row.Total.Sub(row.PaidAmount)
		writeCSVRow(&b, []string{
			row.InvoiceNumber,
			utils.FormatDate(row.IssueDate),
			utils.FormatDate(row.DueDate),
			row.EffectiveStatus(pending[row.ID]),
			row.Total.String(),
			row.PaidAmount.String(),
			balance.String(),
			row.Currency,
		})
	}
	return b.String()
}

// writeCSVRow appends one CSV line, quoting every cell and doubling inner `"`.
func writeCSVRow(b *strings.Builder, cells []string) {
	for i, cell := range cells {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(cell, `"`, `""`))
		b.WriteByte('"')
	}
	b.WriteByte('\n')
}

// statementSlug reduces a client name to a filename-safe slug (lowercase
// alphanumerics, runs of anything else collapsed to a single dash).
func statementSlug(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "client"
	}
	return out
}
