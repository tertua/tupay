package mail

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderInvoiceSend covers the invoice_send template: the number, the
// amount/currency summary, the due date and the CTA link all render.
func TestRenderInvoiceSend(t *testing.T) {
	out, err := Render("invoice_send", TemplateData{
		AppName:       "Invoiceman",
		InvoiceNumber: "INV-1",
		Total:         "100000",
		Currency:      "IDR",
		DueDate:       "2026-10-30",
		URL:           "https://app.example.com/pay/tok",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "INV-1")
	assert.Contains(t, out, "100000 IDR")
	assert.Contains(t, out, "2026-10-30")
	assert.Contains(t, out, "https://app.example.com/pay/tok")
}

// TestRenderInvoiceSendWithoutDueDate keeps the template zero-value safe: the
// optional due-date clause is dropped when the invoice has none.
func TestRenderInvoiceSendWithoutDueDate(t *testing.T) {
	out, err := Render("invoice_send", TemplateData{
		AppName:       "Invoiceman",
		InvoiceNumber: "INV-2",
		Total:         "5000",
		Currency:      "USD",
		URL:           "https://app.example.com/",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "5000 USD")
	assert.NotContains(t, out, "due on")
}
