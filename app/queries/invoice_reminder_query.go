package queries

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"gorm.io/gorm"
)

// InvoiceReminderQueries reads invoices due for a reminder and claims legs.
type InvoiceReminderQueries struct{ *gorm.DB }

// DueInvoiceReminders returns sent invoices that may be due for a reminder leg
// within a broad calendar window around today, joined to their org settings.
//
// The exact leg (before/after) depends on each org's before_days/after_days, so
// it is computed in Go by the caller; the final per-leg decision is enforced at
// claim time through the unique index on invoice_reminder_log. This keeps the
// SELECT dialect-neutral (both SQLite and PostgreSQL) instead of encoding
// per-org day arithmetic in SQL.
func (q *InvoiceReminderQueries) DueInvoiceReminders(now time.Time, limit int) ([]models.ReminderScanRow, error) {
	today := utils.DateOnly(now)
	out := []models.ReminderScanRow{}
	err := q.Table("invoices").
		Select("invoices.id AS invoice_id, invoices.org_id AS org_id, invoices.invoice_number AS invoice_number, "+
			"invoices.currency AS currency, invoices.total AS total, invoices.due_date AS due_date, "+
			"settings.reminder_before_days AS before_days, settings.reminder_after_days AS after_days, "+
			"settings.reminder_enabled AS enabled, settings.email AS billing_email, settings.language AS language").
		Joins("JOIN settings ON settings.org_id = invoices.org_id").
		Where("invoices.status = ?", models.InvoiceStatusSent).
		Where("invoices.due_date IS NOT NULL").
		Where("settings.reminder_enabled = ?", true).
		Where("settings.reminder_before_days > 0 OR settings.reminder_after_days > 0").
		Where("invoices.due_date >= ? AND invoices.due_date <= ?",
			today.AddDate(0, 0, -370), today.AddDate(0, 0, 370)).
		Order("invoices.due_date ASC").
		Limit(limit).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClaimInvoiceReminder atomically claims one reminder leg by inserting its log
// row. A unique (invoice_id, kind) violation means the leg was already sent
// (by this worker or another replica) and reports (false, nil).
func (q *InvoiceReminderQueries) ClaimInvoiceReminder(orgID, invoiceID uuid.UUID, kind string, now time.Time) (bool, error) {
	return DoRetryValue(func() (bool, error) {
		err := q.Create(&models.InvoiceReminderLog{
			ID:        uuid.New(),
			OrgID:     orgID,
			InvoiceID: invoiceID,
			Kind:      kind,
			SentAt:    now,
			CreatedAt: now,
		}).Error
		if err == nil {
			return true, nil
		}
		if isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	})
}

// isUniqueViolation reports whether err is a unique-constraint violation.
// GORM translates driver errors to gorm.ErrDuplicatedKey when TranslateError is
// on; the string fallback covers backends where it is not.
func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "23505")
}
