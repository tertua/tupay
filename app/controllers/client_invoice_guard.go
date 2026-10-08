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
