package models

import (
	"time"

	"github.com/google/uuid"
)

// Subscription lifecycle. Two values only (Q1): a saved subscription is always
// complete, and "active" is the sole gate on generation. Pausing replaces the
// draft state a three-value model would have offered.
const (
	SubscriptionStatusActive = "active"
	SubscriptionStatusPaused = "paused"
)

// Subscription cadences in scope (weekly = every 7 days, monthly = same
// day-of-month, clamped to the target month's last day).
const (
	SubscriptionCadenceWeekly  = "weekly"
	SubscriptionCadenceMonthly = "monthly"
)

// Subscription is a reusable schedule that the outbox worker expands into
// real invoices. It holds the header + cadence only; line items live in
// SubscriptionItem so this file stays under the model size budget.
type Subscription struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" db:"id" json:"id" validate:"required,uuid"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
	UserID    uuid.UUID  `gorm:"type:uuid" db:"user_id" json:"user_id" validate:"required,uuid"`
	OrgID     uuid.UUID  `gorm:"type:uuid;index" db:"org_id" json:"org_id"`
	// ClientID is nullable so a subscription can exist before a client is chosen;
	// a "sent" subscription requires one at generation time (validateInvoice).
	ClientID *uuid.UUID `gorm:"type:uuid" db:"client_id" json:"client_id"`
	Name     string     `db:"name" json:"name" validate:"required,lte=120"`
	Status   string     `db:"status" json:"status" validate:"required,oneof=active paused"`
	Cadence  string     `db:"cadence" json:"cadence" validate:"required,oneof=weekly monthly"`
	// NextRunDate is the claim cursor (date-only, UTC midnight). Nullable so a
	// freshly-created subscription generates nothing until StartDate seeds it.
	NextRunDate *time.Time `db:"next_run_date" json:"next_run_date"`
	// LeadDays generates the invoice N days before NextRunDate (0 = on the day).
	LeadDays int `db:"lead_days" json:"lead_days" validate:"gte=0,lte=90"`
	// LastRunDate mirrors the last claimed occurrence for observability.
	LastRunDate *time.Time `db:"last_run_date" json:"last_run_date"`

	Currency string  `db:"currency" json:"currency" validate:"required,lte=3"`
	TaxRate  float64 `db:"tax_rate" json:"tax_rate" validate:"gte=0"`
	Discount Money   `gorm:"type:decimal(19,4)" db:"discount" json:"discount"`
	Notes    string  `db:"notes" json:"notes"`
	Terms    string  `db:"terms" json:"terms"`
	// PaymentMethod mirrors Invoice.PaymentMethod (informational, not a gateway
	// routing key); keep the oneof list in sync with webui/src/lib/paymentMethods.js.
	PaymentMethod string `db:"payment_method" json:"payment_method" gorm:"size:32;default:''" validate:"omitempty,lte=32,oneof=Cash 'Bank transfer' Online"`
	// InvoiceStatus is the status the generated invoice takes; never "paid".
	InvoiceStatus string `db:"invoice_status" json:"invoice_status" validate:"required,oneof=draft sent"`
	// DueDays offsets the generated invoice's due date from its issue date.
	DueDays int `db:"due_days" json:"due_days" validate:"gte=0,lte=365"`
}

// SubscriptionInput describes create/update payloads. Its shared fields
// carry the same validator tags as InvoiceInput; the cadence keys follow §5.2
// (cadence, invoice_status).
type SubscriptionInput struct {
	ClientID  *string `json:"client_id"`
	Name      string  `json:"name" validate:"required,lte=120"`
	Status    string  `json:"status" validate:"required,oneof=active paused"`
	Cadence   string  `json:"cadence" validate:"required,oneof=weekly monthly"`
	StartDate string  `json:"start_date"`
	LeadDays  int     `json:"lead_days" validate:"gte=0,lte=90"`
	Currency  string  `json:"currency" validate:"required,lte=3"`
	TaxRate   float64 `json:"tax_rate" validate:"gte=0"`
	Discount  Money   `json:"discount"`
	Notes     string  `json:"notes"`
	Terms     string  `json:"terms"`
	// PaymentMethod mirrors the Invoice column; see the comment there.
	PaymentMethod string             `json:"payment_method" validate:"omitempty,lte=32,oneof=Cash 'Bank transfer' Online"`
	InvoiceStatus string             `json:"invoice_status" validate:"required,oneof=draft sent"`
	DueDays       int                `json:"due_days" validate:"gte=0,lte=365"`
	Items         []InvoiceItemInput `json:"items" validate:"required,min=1,dive"`
}

// SubscriptionStatusInput describes the dedicated status patch payload.
type SubscriptionStatusInput struct {
	Status string `json:"status" validate:"required,oneof=active paused"`
}

// SubscriptionScanRow is one due subscription with the fields the sweep needs.
// It lives in models (not queries) so the outbox worker can consume scan
// results without importing app/queries, exactly like ReminderScanRow.
type SubscriptionScanRow struct {
	SubscriptionID uuid.UUID
	OrgID          uuid.UUID
	UserID         uuid.UUID
	ClientID       *uuid.UUID
	Name           string
	Currency       string
	TaxRate        float64
	Discount       Money
	Notes          string
	Terms          string
	PaymentMethod  string
	InvoiceStatus  string
	Cadence        string
	NextRunDate    *time.Time
	LeadDays       int
	DueDays        int
	Paused         bool
	// RunDate is the concrete occurrence date for a row read back from the run
	// ledger (orphan recovery); nil for rows scanned from the subscription
	// itself.
	RunDate *time.Time
}
