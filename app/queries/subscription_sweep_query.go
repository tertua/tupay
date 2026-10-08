package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"gorm.io/gorm"
)

// SubscriptionSweepQueries scans due subscriptions and claims occurrences. The
// structure mirrors InvoiceReminderQueries: the SELECT stays dialect-neutral and
// the per-row decision is computed in Go, while the unique index on
// subscription_runs is the cross-replica claim arbiter.
type SubscriptionSweepQueries struct {
	*gorm.DB
}

// subscriptionScanWindow bounds the broad NextRunDate scan around today.
// LeadDays is capped at 90, so ±100 days always contains a run whose window
// has opened.
const subscriptionScanWindow = 100

// DueSubscriptions returns active subscriptions whose NextRunDate may be due
// within the broad scan window. Paused subscriptions are excluded here as a
// query optimization; the exact window and pause check are re-derived in Go.
func (q *SubscriptionSweepQueries) DueSubscriptions(now time.Time, limit int) ([]models.SubscriptionScanRow, error) {
	today := utils.DateOnly(now)
	out := []models.SubscriptionScanRow{}
	err := q.Table("subscriptions").
		Select("subscriptions.id AS subscription_id, subscriptions.org_id AS org_id, "+
			"subscriptions.user_id AS user_id, subscriptions.client_id AS client_id, "+
			"subscriptions.name AS name, subscriptions.currency AS currency, "+
			"subscriptions.tax_rate AS tax_rate, subscriptions.discount AS discount, "+
			"subscriptions.notes AS notes, subscriptions.terms AS terms, "+
			"subscriptions.payment_method AS payment_method, subscriptions.invoice_status AS invoice_status, "+
			"subscriptions.cadence AS cadence, subscriptions.next_run_date AS next_run_date, "+
			"subscriptions.lead_days AS lead_days, subscriptions.due_days AS due_days").
		Where("subscriptions.status = ?", models.SubscriptionStatusActive).
		Where("subscriptions.next_run_date IS NOT NULL").
		Where("subscriptions.next_run_date >= ? AND subscriptions.next_run_date <= ?",
			today.AddDate(0, 0, -subscriptionScanWindow), today.AddDate(0, 0, subscriptionScanWindow)).
		Order("subscriptions.next_run_date ASC").
		Limit(limit).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClaimSubscriptionRun atomically claims one occurrence by inserting its run
// row. A unique (subscription_id, run_date) violation means the occurrence was
// already claimed (by this worker or another replica) and reports (false, nil).
//
// The existence check is a fast path, not the arbiter: an already-claimed
// occurrence is reported without executing — and failing — the unique insert,
// so a repeated sweep tick does not log a duplicate-key error every time. The
// insert keeps its role as the race arbiter across replicas.
func (q *SubscriptionSweepQueries) ClaimSubscriptionRun(orgID, subscriptionID uuid.UUID, runDate, now time.Time) (bool, error) {
	var already int64
	if err := q.Model(&models.SubscriptionRun{}).
		Where("subscription_id = ? AND run_date = ?", subscriptionID, runDate).
		Count(&already).Error; err != nil {
		return false, err
	}
	if already > 0 {
		return false, nil
	}
	return DoRetryValue(func() (bool, error) {
		err := q.Create(&models.SubscriptionRun{
			ID:             uuid.New(),
			OrgID:          orgID,
			SubscriptionID: subscriptionID,
			RunDate:        runDate,
			CreatedAt:      now,
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

// SetSubscriptionRunInvoice writes the generated-invoice linkage onto a claimed
// run row (claim-then-generate: the row exists before the invoice does).
func (q *SubscriptionSweepQueries) SetSubscriptionRunInvoice(subscriptionID uuid.UUID, runDate time.Time, invoiceID uuid.UUID) error {
	return q.Model(&models.SubscriptionRun{}).
		Where("subscription_id = ? AND run_date = ?", subscriptionID, runDate).
		Update("invoice_id", invoiceID).Error
}

// AdvanceSubscriptionNextRun moves the claim cursor and records the last run.
func (q *SubscriptionSweepQueries) AdvanceSubscriptionNextRun(orgID, subscriptionID uuid.UUID, nextDate, lastRun time.Time) error {
	return q.Model(&models.Subscription{}).Where("id = ? AND org_id = ?", subscriptionID, orgID).
		Updates(map[string]any{
			"next_run_date": nextDate,
			"last_run_date": lastRun,
		}).Error
}

// OrphanSubscriptionRuns returns claimed runs with no invoice that are older
// than the grace window, so the sweep can re-attempt generation
// (content-deterministic and therefore safe to retry). A crash between claim and
// invoice commit leaves InvoiceID NULL; this is the recovery path (§3.2 step 5).
func (q *SubscriptionSweepQueries) OrphanSubscriptionRuns(before time.Time, limit int) ([]models.SubscriptionScanRow, error) {
	out := []models.SubscriptionScanRow{}
	err := q.Table("subscription_runs").
		Select("subscription_runs.subscription_id AS subscription_id, subscription_runs.org_id AS org_id, "+
			"subscription_runs.run_date AS run_date, subscriptions.user_id AS user_id, "+
			"subscriptions.client_id AS client_id, subscriptions.name AS name, "+
			"subscriptions.currency AS currency, subscriptions.tax_rate AS tax_rate, "+
			"subscriptions.discount AS discount, subscriptions.notes AS notes, subscriptions.terms AS terms, "+
			"subscriptions.payment_method AS payment_method, subscriptions.invoice_status AS invoice_status, "+
			"subscriptions.cadence AS cadence, subscriptions.due_days AS due_days").
		Joins("JOIN subscriptions ON subscriptions.id = subscription_runs.subscription_id").
		Where("subscription_runs.invoice_id IS NULL").
		Where("subscription_runs.created_at < ?", before).
		Order("subscription_runs.created_at ASC").
		Limit(limit).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}
