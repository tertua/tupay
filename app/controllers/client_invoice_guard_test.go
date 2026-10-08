package controllers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/app/queries"
)

// TestOpenInvoiceCount covers the delete guard predicate: sent (including
// overdue) and pending block a hard delete, while drafts and paid invoices do
// not.
func TestOpenInvoiceCount(t *testing.T) {
	past := time.Now().AddDate(0, 0, -5)
	future := time.Now().AddDate(0, 0, 5)
	total := decimal.NewFromInt(1000)
	zero := decimal.Zero

	draftID := uuid.New()
	sentID := uuid.New()
	paidID := uuid.New()
	overdueID := uuid.New()

	rows := []queries.ClientInvoiceRow{
		{ID: draftID, Status: models.InvoiceStatusDraft, Total: total, PaidAmount: zero},
		{ID: sentID, Status: models.InvoiceStatusSent, DueDate: &future, Total: total, PaidAmount: zero},
		{ID: paidID, Status: models.InvoiceStatusPaid, Total: total, PaidAmount: total},
		{ID: overdueID, Status: models.InvoiceStatusSent, DueDate: &past, Total: total, PaidAmount: zero},
	}

	assert.Equal(t, 0, openInvoiceCount(nil, nil))
	assert.Equal(t, 0, openInvoiceCount([]queries.ClientInvoiceRow{rows[0]}, nil))
	assert.Equal(t, 0, openInvoiceCount([]queries.ClientInvoiceRow{rows[2]}, nil))
	// sent + overdue block; draft and paid do not.
	assert.Equal(t, 2, openInvoiceCount(rows, nil))

	// A draft with a live gateway intent counts as pending and blocks.
	assert.Equal(t, 1, openInvoiceCount(
		[]queries.ClientInvoiceRow{rows[0]},
		map[uuid.UUID]bool{draftID: true},
	))
}

// TestFilterOverdue keeps only rows whose effective status is overdue.
func TestFilterOverdue(t *testing.T) {
	past := time.Now().AddDate(0, 0, -5)
	future := time.Now().AddDate(0, 0, 5)
	total := decimal.NewFromInt(1000)

	overdue := queries.ClientInvoiceRow{ID: uuid.New(), Status: models.InvoiceStatusSent, DueDate: &past, Total: total, PaidAmount: decimal.Zero}
	sent := queries.ClientInvoiceRow{ID: uuid.New(), Status: models.InvoiceStatusSent, DueDate: &future, Total: total, PaidAmount: decimal.Zero}
	draft := queries.ClientInvoiceRow{ID: uuid.New(), Status: models.InvoiceStatusDraft, Total: total, PaidAmount: decimal.Zero}

	out := filterOverdue([]queries.ClientInvoiceRow{sent, overdue, draft}, nil)
	assert.Len(t, out, 1)
	assert.Equal(t, overdue.ID, out[0].ID)

	assert.Empty(t, filterOverdue(nil, nil))
}
