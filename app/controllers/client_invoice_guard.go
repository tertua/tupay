package controllers

import (
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/app/queries"
)

// openInvoiceCount counts a client's invoices that block a hard delete: any
// invoice whose effective status is sent (including overdue) or pending (a live
// gateway intent). Drafts and paid invoices do not block.
func openInvoiceCount(rows []queries.ClientInvoiceRow, pending map[uuid.UUID]bool) int {
	n := 0
	for _, row := range rows {
		switch row.EffectiveStatus(pending[row.ID]) {
		case models.InvoiceStatusSent, models.InvoiceEffectiveOverdue, models.InvoiceEffectivePending:
			n++
		}
	}
	return n
}

// filterOverdue keeps only rows whose effective status is overdue, backing the
// client-detail overdue-only toggle.
func filterOverdue(rows []queries.ClientInvoiceRow, pending map[uuid.UUID]bool) []queries.ClientInvoiceRow {
	out := make([]queries.ClientInvoiceRow, 0, len(rows))
	for _, row := range rows {
		if row.EffectiveStatus(pending[row.ID]) == models.InvoiceEffectiveOverdue {
			out = append(out, row)
		}
	}
	return out
}
