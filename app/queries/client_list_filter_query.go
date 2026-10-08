package queries

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// filteredClients builds the shared filter chain for client listing and
// counting so both stay in sync: billing aggregates, lifecycle status and
// free-text search.
func (q *ClientQueries) filteredClients(orgID uuid.UUID, status, search string) *gorm.DB {
	billedSubquery, paidSubquery := q.clientAggregateJoins(orgID)

	tx := q.Table("clients AS c").
		Select(`c.*, COALESCE(inv.total_billed, 0) AS total_billed,
			COALESCE(inv.total_billed, 0) - COALESCE(pay.paid, 0) AS outstanding`).
		Joins("LEFT JOIN (?) AS inv ON inv.client_id = c.id", billedSubquery).
		Joins("LEFT JOIN (?) AS pay ON pay.client_id = c.id", paidSubquery).
		Where("c.org_id = ?", orgID)

	// Unknown status values fall back to "all", matching ListInvoices leniency.
	if status == models.ClientStatusActive || status == models.ClientStatusArchived {
		tx = tx.Where("c.status = ?", status)
	}

	if search != "" {
		// LOWER(...) LIKE lets SQLite and PostgreSQL match case-insensitively
		// for the same input (SQLite LIKE is already case-insensitive for ASCII,
		// PostgreSQL LIKE is not, so both sides are lowered explicitly).
		like := "%" + strings.ToLower(search) + "%"
		tx = tx.Where("LOWER(c.name) LIKE ? OR LOWER(c.company) LIKE ? OR LOWER(c.email) LIKE ?", like, like, like)
	}

	return tx
}

// clientAggregateJoins builds the LEFT JOIN subqueries that compute each
// client's billed total and paid amount, so both stay consistent with
// dashboard/reports.
func (q *ClientQueries) clientAggregateJoins(orgID uuid.UUID) (*gorm.DB, *gorm.DB) {
	// Billed invoices are sent + paid; drafts are not billed yet. A draft
	// with money in flight (pending) still owes, so it is billed too,
	// matching dashboard/reports. Paid must stay in total_billed so
	// auto-marking paid never shrinks client totals.
	pending := pendingInvoiceIDs(q.DB, orgID)

	billedSubquery := q.Model(&models.Invoice{}).
		Select("client_id, SUM(total) AS total_billed").
		Where("org_id = ? AND status IN ?", orgID, []string{models.InvoiceStatusSent, models.InvoiceStatusPaid})
	paidSubquery := q.Model(&models.Payment{}).
		Select("invoices.client_id AS client_id, SUM(payments.amount) AS paid").
		Joins("JOIN invoices ON invoices.id = payments.invoice_id").
		Where("payments.voided_at IS NULL AND invoices.org_id = ? AND invoices.status IN ?", orgID, []string{models.InvoiceStatusSent, models.InvoiceStatusPaid})
	if len(pending) > 0 {
		billedSubquery = billedSubquery.Or("org_id = ? AND id IN ?", orgID, pending)
		paidSubquery = paidSubquery.Or("invoices.org_id = ? AND invoices.id IN ?", orgID, pending)
	}
	billedSubquery = billedSubquery.Group("client_id")
	paidSubquery = paidSubquery.Group("invoices.client_id")
	return billedSubquery, paidSubquery
}
