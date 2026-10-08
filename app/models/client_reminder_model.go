package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ClientReminderRow is one open invoice of a client with the fields a manual
// reminder needs. It lives in models (not queries) so the outbox helper can
// consume it without importing app/queries, mirroring ReminderScanRow.
type ClientReminderRow struct {
	InvoiceID     uuid.UUID
	OrgID         uuid.UUID
	InvoiceNumber string
	Currency      string
	Total         decimal.Decimal
	DueDate       *time.Time
	BillingEmail  string
	Language      string
}
