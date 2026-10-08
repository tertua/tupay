package models

import (
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// This file holds the invoice money arithmetic shared by every invoice build
// path (the controller builder, the gateway relay, and the recurring-template
// sweep). It lives in models — imported by both controllers and
// platform/outbox — so the outbox worker can build an invoice without pulling
// in the HTTP layer, and so the single source of truth for money math is one
// package.

// ErrNegativeMoney rejects negative money values on any invoice build path.
var ErrNegativeMoney = errors.New("money values cannot be negative")

// AddInvoiceItems builds the item rows and accumulates the invoice subtotal,
// rejecting negative money values. Both create paths run it.
func AddInvoiceItems(invoice *Invoice, entries []InvoiceItemInput) ([]InvoiceItem, error) {
	items := make([]InvoiceItem, 0, len(entries))
	for position, entry := range entries {
		if entry.Rate.IsNegative() || invoice.Discount.IsNegative() {
			return nil, ErrNegativeMoney
		}
		amount := DecimalFromFloat(entry.Quantity).Mul(entry.Rate)
		invoice.Subtotal = invoice.Subtotal.Add(amount)
		items = append(items, InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoice.ID,
			Description: entry.Description,
			Quantity:    entry.Quantity,
			Rate:        entry.Rate,
			Amount:      amount,
			Position:    position,
		})
	}
	return items, nil
}

// ApplyInvoiceTotals derives taxable, tax and total from the subtotal, the
// discount and the tax rate.
func ApplyInvoiceTotals(invoice *Invoice) {
	taxable := invoice.Subtotal.Sub(invoice.Discount)
	if taxable.IsNegative() {
		taxable = decimal.Zero
	}
	invoice.TaxAmount = taxable.Mul(DecimalFromFloat(invoice.TaxRate)).Div(decimal.NewFromInt(100))
	invoice.Total = taxable.Add(invoice.TaxAmount)
}
