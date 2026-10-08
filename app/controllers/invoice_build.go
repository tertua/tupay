package controllers

import (
	"errors"
	"time"

	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// The item/subtotal/tax math lives in models (AddInvoiceItems /
// ApplyInvoiceTotals) so the outbox recurring sweep shares one source of truth
// without importing this package; buildInvoice is the HTTP-request wrapper.

// buildInvoice computes invoice and item rows.
func buildInvoice(orgID, userID uuid.UUID, input *models.InvoiceInput) (*models.Invoice, []models.InvoiceItem, error) {
	issueDate, err := utils.ParseDate(input.IssueDate)
	if err != nil {
		return nil, nil, errors.New("invalid issue_date, expected YYYY-MM-DD")
	}
	dueDate, err := utils.ParseDate(input.DueDate)
	if err != nil {
		return nil, nil, errors.New("invalid due_date, expected YYYY-MM-DD")
	}

	clientID, err := resolveClient(input)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	invoice := &models.Invoice{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: &now,
		OrgID:     orgID,
		UserID:    userID,
		ClientID:  clientID,
		Status:    input.Status,
		IssueDate: issueDate,
		DueDate:   dueDate,
		Currency:  input.Currency,
		TaxRate:   input.TaxRate,
		Discount:  input.Discount,
		Notes:     input.Notes,
		Terms:     input.Terms,
		// Informational "how to pay" choice, not a gateway routing key.
		PaymentMethod: input.PaymentMethod,
	}

	items, err := models.AddInvoiceItems(invoice, input.Items)
	if err != nil {
		return nil, nil, err
	}
	models.ApplyInvoiceTotals(invoice)

	return invoice, items, nil
}

// isPaidLocked reports whether an invoice is effectively paid (stored paid or payments covering the total) and must be treated as immutable.
func isPaidLocked(status string, dueDate *time.Time, total, paid decimal.Decimal) bool {
	return models.ResolveEffectiveStatus(status, dueDate, total, paid, false) == models.InvoiceStatusPaid
}
