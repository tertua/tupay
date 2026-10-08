package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"gorm.io/gorm"
)

// InvoiceTemplateSweepQueries scans due templates and claims occurrences. The
// structure mirrors InvoiceReminderQueries: the SELECT stays dialect-neutral and
// the per-row decision is computed in Go, while the unique index on
// invoice_template_run is the cross-replica claim arbiter.
type InvoiceTemplateSweepQueries struct {
	*gorm.DB
}

// templateScanWindow bounds the broad NextRunDate scan around today. LeadDays is
// capped at 90, so ±100 days always contains a run whose window has opened.
const templateScanWindow = 100

// DueInvoiceTemplates returns active templates whose NextRunDate may be due
// within the broad scan window. Paused templates are excluded here as a query
// optimization; the exact window and pause check are re-derived in Go.
func (q *InvoiceTemplateSweepQueries) DueInvoiceTemplates(now time.Time, limit int) ([]models.TemplateScanRow, error) {
	today := utils.DateOnly(now)
	out := []models.TemplateScanRow{}
	err := q.Table("invoice_templates").
		Select("invoice_templates.id AS template_id, invoice_templates.org_id AS org_id, "+
			"invoice_templates.user_id AS user_id, invoice_templates.client_id AS client_id, "+
			"invoice_templates.name AS name, invoice_templates.currency AS currency, "+
			"invoice_templates.tax_rate AS tax_rate, invoice_templates.discount AS discount, "+
			"invoice_templates.notes AS notes, invoice_templates.terms AS terms, "+
			"invoice_templates.payment_method AS payment_method, invoice_templates.invoice_status AS invoice_status, "+
			"invoice_templates.cadence AS cadence, invoice_templates.next_run_date AS next_run_date, "+
			"invoice_templates.lead_days AS lead_days, invoice_templates.due_days AS due_days").
		Where("invoice_templates.status = ?", models.InvoiceTemplateStatusActive).
		Where("invoice_templates.next_run_date IS NOT NULL").
		Where("invoice_templates.next_run_date >= ? AND invoice_templates.next_run_date <= ?",
			today.AddDate(0, 0, -templateScanWindow), today.AddDate(0, 0, templateScanWindow)).
		Order("invoice_templates.next_run_date ASC").
		Limit(limit).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClaimInvoiceTemplateRun atomically claims one occurrence by inserting its run
// row. A unique (template_id, run_date) violation means the occurrence was
// already claimed (by this worker or another replica) and reports (false, nil).
//
// The existence check is a fast path, not the arbiter: an already-claimed
// occurrence is reported without executing — and failing — the unique insert,
// so a repeated sweep tick does not log a duplicate-key error every time. The
// insert keeps its role as the race arbiter across replicas.
func (q *InvoiceTemplateSweepQueries) ClaimInvoiceTemplateRun(orgID, templateID uuid.UUID, runDate, now time.Time) (bool, error) {
	var already int64
	if err := q.Model(&models.InvoiceTemplateRun{}).
		Where("template_id = ? AND run_date = ?", templateID, runDate).
		Count(&already).Error; err != nil {
		return false, err
	}
	if already > 0 {
		return false, nil
	}
	return DoRetryValue(func() (bool, error) {
		err := q.Create(&models.InvoiceTemplateRun{
			ID:         uuid.New(),
			OrgID:      orgID,
			TemplateID: templateID,
			RunDate:    runDate,
			CreatedAt:  now,
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

// SetTemplateRunInvoice writes the generated-invoice linkage onto a claimed run
// row (claim-then-generate: the row exists before the invoice does).
func (q *InvoiceTemplateSweepQueries) SetTemplateRunInvoice(templateID uuid.UUID, runDate time.Time, invoiceID uuid.UUID) error {
	return q.Model(&models.InvoiceTemplateRun{}).
		Where("template_id = ? AND run_date = ?", templateID, runDate).
		Update("invoice_id", invoiceID).Error
}

// AdvanceTemplateNextRun moves the claim cursor and records the last run.
func (q *InvoiceTemplateSweepQueries) AdvanceTemplateNextRun(orgID, templateID uuid.UUID, nextDate, lastRun time.Time) error {
	return q.Model(&models.InvoiceTemplate{}).Where("id = ? AND org_id = ?", templateID, orgID).
		Updates(map[string]any{
			"next_run_date": nextDate,
			"last_run_date": lastRun,
		}).Error
}

// OrphanTemplateRuns returns claimed runs with no invoice that are older than
// the grace window, so the sweep can re-attempt generation (content-deterministic
// and therefore safe to retry). A crash between claim and invoice commit leaves
// InvoiceID NULL; this is the recovery path (§3.2 step 5).
func (q *InvoiceTemplateSweepQueries) OrphanTemplateRuns(before time.Time, limit int) ([]models.TemplateScanRow, error) {
	out := []models.TemplateScanRow{}
	err := q.Table("invoice_template_run").
		Select("invoice_template_run.template_id AS template_id, invoice_template_run.org_id AS org_id, "+
			"invoice_template_run.run_date AS run_date, invoice_templates.user_id AS user_id, "+
			"invoice_templates.client_id AS client_id, invoice_templates.name AS name, "+
			"invoice_templates.currency AS currency, invoice_templates.tax_rate AS tax_rate, "+
			"invoice_templates.discount AS discount, invoice_templates.notes AS notes, "+
			"invoice_templates.terms AS terms, invoice_templates.payment_method AS payment_method, "+
			"invoice_templates.invoice_status AS invoice_status, invoice_templates.cadence AS cadence, "+
			"invoice_templates.due_days AS due_days").
		Joins("JOIN invoice_templates ON invoice_templates.id = invoice_template_run.template_id").
		Where("invoice_template_run.invoice_id IS NULL").
		Where("invoice_template_run.created_at < ?", before).
		Order("invoice_template_run.created_at ASC").
		Limit(limit).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}
