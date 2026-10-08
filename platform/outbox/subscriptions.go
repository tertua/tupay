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

// subscriptionOrphanGrace is how long a claimed-but-unlinked run row may sit
// before the sweep treats it as an orphan and re-attempts generation. Two polls
// of headroom is enough for an in-flight tick to commit.
const subscriptionOrphanGrace = 2 * time.Minute

// generateSubscriptionInvoices sweeps active subscriptions due for an
// occurrence once per tick. It is safe on several replicas: each occurrence is
// claimed atomically through the unique (subscription_id, run_date) index on
// subscription_runs before any invoice is created, so overlapping ticks,
// restarts, or a second replica never double-bill.
func (w *Worker) generateSubscriptionInvoices(ctx context.Context) {
	db, err := database.OpenDBConnection()
	if err != nil {
		logger.L().Warn("outbox subscription tick skipped, database unavailable", "err", err)
		return
	}
	now := time.Now()
	today := utils.DateOnly(now)

	// Recover orphaned claims first so a crashed tick's period is not lost.
	w.recoverSubscriptionRuns(ctx, db, now)

	rows, err := db.DueSubscriptions(now, w.batch)
	if err != nil {
		logger.L().Warn("outbox subscription tick failed", "err", err)
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

// generateOne claims the current occurrence for a subscription row and, only on
// a successful claim, builds and persists the invoice, links it back, and
// advances the cursor.
func (w *Worker) generateOne(ctx context.Context, db *database.Queries, row models.SubscriptionScanRow, today, now time.Time) {
	if row.NextRunDate == nil {
		return
	}
	occurrence, due := subscriptionRunDue(row, today)
	if !due {
		return
	}
	// A sent invoice must name a client (same rule as validateInvoice); a
	// subscription missing one is skipped, never half-generated.
	if row.InvoiceStatus == models.InvoiceStatusSent && row.ClientID == nil {
		logger.L().Warn("outbox subscription skipped, client required to send", "subscription_id", row.SubscriptionID.String())
		return
	}
	claimed, err := db.ClaimSubscriptionRun(row.OrgID, row.SubscriptionID, occurrence, now)
	if err != nil {
		logger.L().Warn("outbox subscription claim failed", "subscription_id", row.SubscriptionID.String(), "err", err)
		return
	}
	if !claimed {
		return
	}
	invoiceID, err := w.buildSubscriptionInvoice(db, row, occurrence, now)
	if err != nil {
		// The claim row survives with InvoiceID NULL; the orphan recovery path
		// re-attempts it on a later tick, so no period is silently lost.
		logger.L().Warn("outbox subscription generate failed", "subscription_id", row.SubscriptionID.String(), "run_date", utils.FormatTime(occurrence), "err", err)
		return
	}
	recordErr("link subscription run", db.SetSubscriptionRunInvoice(row.SubscriptionID, occurrence, invoiceID), "subscription_id", row.SubscriptionID.String())
	next := nextSubscriptionRun(occurrence, row.Cadence, occurrence.Day())
	recordErr("advance subscription cursor", db.AdvanceSubscriptionNextRun(row.OrgID, row.SubscriptionID, next, occurrence), "subscription_id", row.SubscriptionID.String())
	logger.L().Info("outbox subscription invoice generated", "subscription_id", row.SubscriptionID.String(), "invoice_id", invoiceID.String(), "run_date", utils.FormatTime(occurrence))
}

// buildSubscriptionInvoice maps a subscription row to a real invoice and
// persists it (number reserved inside the same transaction), returning the new
// invoice id. It reuses the shared money math so a generated invoice totals
// exactly like a hand-made one.
func (w *Worker) buildSubscriptionInvoice(db *database.Queries, row models.SubscriptionScanRow, occurrence, now time.Time) (uuid.UUID, error) {
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
	items, err := models.AddInvoiceItems(invoice, subscriptionItemInputs(db, row.SubscriptionID))
	if err != nil {
		return uuid.Nil, err
	}
	models.ApplyInvoiceTotals(invoice)
	if err := db.CreateInvoice(row.OrgID, invoice, items); err != nil {
		return uuid.Nil, err
	}
	return invoice.ID, nil
}

// subscriptionItemInputs loads a subscription's lines in the shape the shared
// money math expects.
func subscriptionItemInputs(db *database.Queries, subscriptionID uuid.UUID) []models.InvoiceItemInput {
	items, err := db.GetSubscriptionItems(subscriptionID)
	if err != nil {
		return nil
	}
	out := make([]models.InvoiceItemInput, 0, len(items))
	for _, it := range items {
		out = append(out, models.InvoiceItemInput{Description: it.Description, Quantity: it.Quantity, Rate: it.Rate})
	}
	return out
}

// recoverSubscriptionRuns re-generates occurrences whose claim row has no
// invoice past the grace window (a crash between claim and commit). Generation
// is content-deterministic, so re-running it is safe: the invoice is created and
// linked exactly once.
func (w *Worker) recoverSubscriptionRuns(ctx context.Context, db *database.Queries, now time.Time) {
	rows, err := db.OrphanSubscriptionRuns(now.Add(-subscriptionOrphanGrace), w.batch)
	if err != nil {
		logger.L().Warn("outbox subscription orphan scan failed", "err", err)
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
		invoiceID, err := w.buildSubscriptionInvoice(db, row, *row.RunDate, now)
		if err != nil {
			logger.L().Warn("outbox subscription orphan generate failed", "subscription_id", row.SubscriptionID.String(), "err", err)
			continue
		}
		recordErr("link recovered run", db.SetSubscriptionRunInvoice(row.SubscriptionID, *row.RunDate, invoiceID), "subscription_id", row.SubscriptionID.String())
		logger.L().Info("outbox subscription orphan recovered", "subscription_id", row.SubscriptionID.String(), "invoice_id", invoiceID.String())
	}
}

// subscriptionRunDue decides whether today is inside [NextRunDate - LeadDays,
// NextRunDate] and returns the occurrence date to claim. The occurrence is the
// target NextRunDate (not the generation day), so re-running within the lead
// window still claims exactly once (Q9).
func subscriptionRunDue(row models.SubscriptionScanRow, today time.Time) (time.Time, bool) {
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

// nextSubscriptionRun returns the occurrence after from for a cadence, using
// anchor as the monthly day-of-month. Monthly clamps to the last day of the
// target month (Jan 31 -> Feb 28/29) instead of letting time.AddDate overflow
// into the next month; weekly simply adds 7 days. The result is UTC midnight via
// DateOnly so SQLite text and PostgreSQL instant comparisons agree.
func nextSubscriptionRun(from time.Time, cadence string, anchor int) time.Time {
	base := utils.DateOnly(from)
	if cadence == models.SubscriptionCadenceWeekly {
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
