package models

import (
	"time"

	"github.com/google/uuid"
)

// SubscriptionRun is the claim/ledger row that makes generation idempotent.
// The unique (subscription_id, run_date) index is the idempotency key: a second
// worker (or the same worker after a restart) that tries to claim the same
// occurrence hits a unique violation and reports "not claimed", exactly like
// InvoiceReminderLog. InvoiceID is written after the invoice commits, so it is
// nullable; a row stuck at NULL past the grace window is an orphan the sweep
// re-attempts (§3.2 step 5).
type SubscriptionRun struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" db:"id" json:"id"`
	OrgID          uuid.UUID `gorm:"type:uuid;index" db:"org_id" json:"org_id"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_subscription_run" db:"subscription_id" json:"subscription_id"`
	// RunDate is the occurrence date (date-only, UTC midnight), not the day the
	// sweep ran, so a lead-days window still claims exactly once (Q9).
	RunDate   time.Time  `gorm:"uniqueIndex:idx_subscription_run" db:"run_date" json:"run_date"`
	InvoiceID *uuid.UUID `gorm:"type:uuid" db:"invoice_id" json:"invoice_id"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}
