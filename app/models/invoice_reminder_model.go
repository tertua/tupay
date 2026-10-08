package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Invoice reminder legs. A reminder is sent once per (invoice, kind): the
// unique index is the idempotency key and the atomic claim.
const (
	InvoiceReminderKindBefore = "before" // sent before_days ahead of due date
	InvoiceReminderKindAfter  = "after"  // sent after_days past the due date
	// InvoiceReminderKindManual is a one-off, user-triggered reminder. It uses
	// its own leg so it can never satisfy (and thus suppress) the automatic
	// before/after legs' unique index.
	InvoiceReminderKindManual = "manual"
)

// NotifEventInvoiceReminder is the user-notification/SSE event type published
// when a reminder leg fires; it lives here to keep notification_model.go within
// its size baseline. It is listed in NotifAllowedEvents so webhook endpoints
// can subscribe to it.
const NotifEventInvoiceReminder = "invoice.reminder"

// InvoiceReminderLog records that one reminder leg was claimed for an invoice.
// Written by the outbox worker before the email/SSE side effects; the unique
// (invoice_id, kind) index makes a second claim for the same leg impossible.
type InvoiceReminderLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" db:"id" json:"id"`
	OrgID     uuid.UUID `gorm:"type:uuid;index" db:"org_id" json:"org_id"`
	InvoiceID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_invoice_reminder_leg" db:"invoice_id" json:"invoice_id"`
	Kind      string    `gorm:"size:16;uniqueIndex:idx_invoice_reminder_leg" db:"kind" json:"kind"`
	SentAt    time.Time `db:"sent_at" json:"sent_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// TableName keeps the singular history table name (one row per sent reminder leg).
func (InvoiceReminderLog) TableName() string { return "invoice_reminder_log" }

// ReminderScanRow is one invoice candidate with the fields the reminder sweep
// needs. It lives in models (not queries) so the outbox worker can consume it
// without importing app/queries.
type ReminderScanRow struct {
	InvoiceID     uuid.UUID
	OrgID         uuid.UUID
	InvoiceNumber string
	Currency      string
	Total         decimal.Decimal
	DueDate       *time.Time
	BeforeDays    int
	AfterDays     int
	Enabled       bool
	BillingEmail  string
	Language      string
}
