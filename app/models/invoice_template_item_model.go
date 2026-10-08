package models

import (
	"github.com/google/uuid"
)

// InvoiceTemplateItem is one template line. It mirrors InvoiceItem field-for-
// field except it points at TemplateID and carries no Amount: the invoice path
// recomputes amounts from quantity × rate when the template generates an
// invoice, so storing one here would only drift.
type InvoiceTemplateItem struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" db:"id" json:"id" validate:"required,uuid"`
	TemplateID  uuid.UUID `gorm:"type:uuid;index" db:"template_id" json:"template_id" validate:"required,uuid"`
	Description string    `db:"description" json:"description" validate:"required"`
	Quantity    float64   `db:"quantity" json:"quantity" validate:"gte=0"`
	Rate        Money     `gorm:"type:decimal(19,4)" db:"rate" json:"rate"`
	Position    int       `db:"position" json:"position"`
}
