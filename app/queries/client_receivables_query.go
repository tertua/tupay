package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// ClientReceivablesQueries reads a client's open receivables for the reminder
// and statement paths.
type ClientReceivablesQueries struct{ *gorm.DB }

// ClientOpenInvoiceReminderRows returns the client's open invoices (sent, not
// fully paid, with a due date) joined to the org billing settings a manual
// reminder needs. Paid/draft invoices are excluded; the outbox helper resolves
// the public pay link itself (never minting one).
func (q *ClientReceivablesQueries) ClientOpenInvoiceReminderRows(orgID, clientID uuid.UUID) ([]models.ClientReminderRow, error) {
	out := []models.ClientReminderRow{}

	paidSubquery := q.Model(&models.Payment{}).
		Select("invoice_id, SUM(amount) AS paid").
		Where("voided_at IS NULL").
		Group("invoice_id")

	err := q.Table("invoices").
		Select("invoices.id AS invoice_id, invoices.org_id AS org_id, invoices.invoice_number AS invoice_number, "+
			"invoices.currency AS currency, invoices.total AS total, invoices.due_date AS due_date, "+
			"settings.email AS billing_email, settings.language AS language").
		Joins("LEFT JOIN settings ON settings.org_id = invoices.org_id").
		Joins("LEFT JOIN (?) AS pay ON pay.invoice_id = invoices.id", paidSubquery).
		Where("invoices.org_id = ? AND invoices.client_id = ?", orgID, clientID).
		Where("invoices.status = ?", models.InvoiceStatusSent).
		Where("invoices.due_date IS NOT NULL").
		Where("invoices.total > 0 AND COALESCE(pay.paid, 0) < invoices.total").
		Order("invoices.due_date ASC").
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClientStatementRows returns a client's issued invoices (anything but a
// draft) with their paid amount, for the CSV statement. The optional from/to
// bounds filter issue_date inclusively; both are date-only UTC midnights so the
// bounds resolve the same on SQLite (text compare) and PostgreSQL (instant
// compare).
func (q *ClientReceivablesQueries) ClientStatementRows(orgID, clientID uuid.UUID, from, to *time.Time) ([]ClientInvoiceRow, error) {
	rows := []ClientInvoiceRow{}

	paidSubquery := q.Model(&models.Payment{}).
		Select("invoice_id, SUM(amount) AS paid").
		Where("voided_at IS NULL").
		Group("invoice_id")

	tx := q.Table("invoices").
		Select(`invoices.id, invoices.invoice_number, invoices.issue_date, invoices.due_date,
			invoices.total, invoices.currency, invoices.status,
			COALESCE(pay.paid, 0) AS paid_amount`).
		Joins("LEFT JOIN (?) AS pay ON pay.invoice_id = invoices.id", paidSubquery).
		Where("invoices.org_id = ? AND invoices.client_id = ?", orgID, clientID).
		Where("invoices.status <> ?", models.InvoiceStatusDraft)
	if from != nil {
		tx = tx.Where("invoices.issue_date >= ?", *from)
	}
	if to != nil {
		tx = tx.Where("invoices.issue_date <= ?", *to)
	}

	if err := tx.Order("invoices.issue_date ASC, invoices.created_at ASC").Scan(&rows).Error; err != nil {
		return rows, err
	}
	return rows, nil
}
