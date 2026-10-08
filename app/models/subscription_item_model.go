package models

import (
	"github.com/google/uuid"
)

// SubscriptionItem is one subscription line. It mirrors InvoiceItem field-for-
// field except it points at SubscriptionID and carries no Amount: the invoice
// path recomputes amounts from quantity × rate when the subscription generates
// an invoice, so storing one here would only drift.
type SubscriptionItem struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" db:"id" json:"id" validate:"required,uuid"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;index" db:"subscription_id" json:"subscription_id" validate:"required,uuid"`
	Description    string    `db:"description" json:"description" validate:"required"`
	Quantity       float64   `db:"quantity" json:"quantity" validate:"gte=0"`
	Rate           Money     `gorm:"type:decimal(19,4)" db:"rate" json:"rate"`
	Position       int       `db:"position" json:"position"`
}
