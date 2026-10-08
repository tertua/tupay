package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/logger"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
)

// recurringGraceWindow is how long a claimed-but-unlinked run row may sit before
// the sweep treats it as an orphan and re-attempts generation. Two polls of
// headroom is enough for an in-flight tick to commit.
const recurringOrphanGrace = 2 * time.Minute

// generateRecurringInvoices sweeps active invoice templates due for an
// occurrence once per tick. It is safe on several replicas: each occurrence is
// claimed atomically through the unique (template_id, run_date) index on
// invoice_template_run before any invoice is created, so overlapping ticks,
// restarts, or a second replica never double-bill.
func (w *Worker) generateRecurringInvoices(ctx context.Context) {
	db, err := database.OpenDBConnection()
	if err != nil {
		logger.L().Warn("outbox recurring tick skipped, database unavailable", "err", err)
		return
	}
	now := time.Now()
	today := utils.DateOnly(now)

	// Recover orphaned claims first so a crashed tick's period is not lost.
	w.recoverTemplateRuns(ctx, db, now)

	rows, err := db.DueInvoiceTemplates(now, w.batch)
	if err != nil {
		logger.L().Warn("outbox recurring tick failed", "err", err)
		return
	}
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.generateOne(ctx, db, row, today, now)
	}
}

// generateOne claims the current occurrence for a template row and, only on a
// successful claim, builds and persists the invoice, links it back, and advances
// the cursor.
func (w *Worker) generateOne(ctx context.Context, db *database.Queries, row models.TemplateScanRow, today, now time.Time) {
	if row.NextRunDate == nil {
		return
	}
	occurrence, due := templateRunDue(row, today)
	if !due {
		return
	}
	// A sent invoice must name a client (same rule as validateInvoice); a
	// template missing one is skipped, never half-generated.
	if row.InvoiceStatus == models.InvoiceStatusSent && row.ClientID == nil {
		logger.L().Warn("outbox recurring template skipped, client required to send", "template_id", row.TemplateID.String())
		return
	}
	claimed, err := db.ClaimInvoiceTemplateRun(row.OrgID, row.TemplateID, occurrence, now)
	if err != nil {
		logger.L().Warn("outbox recurring claim failed", "template_id", row.TemplateID.String(), "err", err)
		return
	}
	if !claimed {
		return
	}
	invoiceID, err := w.buildTemplateInvoice(db, row, occurrence, now)
	if err != nil {
		// The claim row survives with InvoiceID NULL; the orphan recovery path
		// re-attempts it on a later tick, so no period is silently lost.
		logger.L().Warn("outbox recurring generate failed", "template_id", row.TemplateID.String(), "run_date", utils.FormatTime(occurrence), "err", err)
		return
	}
	recordErr("link template run", db.SetTemplateRunInvoice(row.TemplateID, occurrence, invoiceID), "template_id", row.TemplateID.String())
	next := nextTemplateRun(occurrence, row.Cadence, occurrence.Day())
	recordErr("advance template cursor", db.AdvanceTemplateNextRun(row.OrgID, row.TemplateID, next, occurrence), "template_id", row.TemplateID.String())
	logger.L().Info("outbox recurring invoice generated", "template_id", row.TemplateID.String(), "invoice_id", invoiceID.String(), "run_date", utils.FormatTime(occurrence))
}

// buildTemplateInvoice maps a template row to a real invoice and persists it
// (number reserved inside the same transaction), returning the new invoice id.
// It reuses the shared money math so a generated invoice totals exactly like a
// hand-made one.
func (w *Worker) buildTemplateInvoice(db *database.Queries, row models.TemplateScanRow, occurrence, now time.Time) (uuid.UUID, error) {
	issue := occurrence
	due := occurrence.AddDate(0, 0, row.DueDays)
	invoice := &models.Invoice{
		ID:            uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     &now,
		UserID:        row.UserID,
		OrgID:         row.OrgID,
		ClientID:      row.ClientID,
		Status:        row.InvoiceStatus,
		IssueDate:     &issue,
		DueDate:       &due,
		Currency:      row.Currency,
		TaxRate:       row.TaxRate,
		Discount:      row.Discount,
		Notes:         row.Notes,
		Terms:         row.Terms,
		PaymentMethod: row.PaymentMethod,
	}
	items, err := models.AddInvoiceItems(invoice, templateItemInputs(db, row.TemplateID))
	if err != nil {
		return uuid.Nil, err
	}
	models.ApplyInvoiceTotals(invoice)
	if err := db.CreateInvoice(row.OrgID, invoice, items); err != nil {
		return uuid.Nil, err
	}
	return invoice.ID, nil
}

// templateItemInputs loads a template's lines in the shape the shared money
// math expects.
func templateItemInputs(db *database.Queries, templateID uuid.UUID) []models.InvoiceItemInput {
	items, err := db.GetInvoiceTemplateItems(templateID)
	if err != nil {
		return nil
	}
	out := make([]models.InvoiceItemInput, 0, len(items))
	for _, it := range items {
		out = append(out, models.InvoiceItemInput{Description: it.Description, Quantity: it.Quantity, Rate: it.Rate})
	}
	return out
}

// recoverTemplateRuns re-generates occurrences whose claim row has no invoice
// past the grace window (a crash between claim and commit). Generation is
// content-deterministic, so re-running it is safe: the invoice is created and
// linked exactly once.
func (w *Worker) recoverTemplateRuns(ctx context.Context, db *database.Queries, now time.Time) {
	rows, err := db.OrphanTemplateRuns(now.Add(-recurringOrphanGrace), w.batch)
	if err != nil {
		logger.L().Warn("outbox recurring orphan scan failed", "err", err)
		return
	}
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if row.RunDate == nil {
			continue
		}
		if row.InvoiceStatus == models.InvoiceStatusSent && row.ClientID == nil {
			continue
		}
		invoiceID, err := w.buildTemplateInvoice(db, row, *row.RunDate, now)
		if err != nil {
			logger.L().Warn("outbox recurring orphan generate failed", "template_id", row.TemplateID.String(), "err", err)
			continue
		}
		recordErr("link recovered run", db.SetTemplateRunInvoice(row.TemplateID, *row.RunDate, invoiceID), "template_id", row.TemplateID.String())
		logger.L().Info("outbox recurring orphan recovered", "template_id", row.TemplateID.String(), "invoice_id", invoiceID.String())
	}
}

// templateRunDue decides whether today is inside [NextRunDate - LeadDays,
// NextRunDate] and returns the occurrence date to claim. The occurrence is the
// target NextRunDate (not the generation day), so re-running within the lead
// window still claims exactly once (Q9).
func templateRunDue(row models.TemplateScanRow, today time.Time) (time.Time, bool) {
	if row.NextRunDate == nil || row.Paused {
		return time.Time{}, false
	}
	target := utils.DateOnly(*row.NextRunDate)
	start := target.AddDate(0, 0, -row.LeadDays)
	if today.Before(start) || today.After(target) {
		return time.Time{}, false
	}
	return target, true
}

// nextTemplateRun returns the occurrence after from for a cadence, using anchor
// as the monthly day-of-month. Monthly clamps to the last day of the target
// month (Jan 31 -> Feb 28/29) instead of letting time.AddDate overflow into the
// next month; weekly simply adds 7 days. The result is UTC midnight via DateOnly
// so SQLite text and PostgreSQL instant comparisons agree.
func nextTemplateRun(from time.Time, cadence string, anchor int) time.Time {
	base := utils.DateOnly(from)
	if cadence == models.InvoiceTemplateCadenceWeekly {
		return utils.DateOnly(base.AddDate(0, 0, 7))
	}
	// Monthly: first day of the next month, clamped to the anchor day.
	firstOfNext := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	lastDay := firstOfNext.AddDate(0, 1, -1).Day()
	day := anchor
	if day > lastDay {
		day = lastDay
	}
	if day < 1 {
		day = 1
	}
	return time.Date(firstOfNext.Year(), firstOfNext.Month(), day, 0, 0, 0, 0, time.UTC)
}
