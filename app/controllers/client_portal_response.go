package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
)

// publicClientInfo is the payer-invisible slice of a client the portal shows.
// Email, phone, address, notes, user_id and org_id are deliberately omitted.
type publicClientInfo struct {
	Name    string `json:"name"`
	Company string `json:"company"`
}

// publicClientInvoiceRow is one invoice row on the public portal. public_pay_token
// carries the invoice's existing payment-link token (omitted when none exists)
// so the page can route into the untouched /pay/:token flow.
type publicClientInvoiceRow struct {
	ID              uuid.UUID    `json:"id"`
	InvoiceNumber   string       `json:"invoice_number"`
	Status          string       `json:"status"`
	EffectiveStatus string       `json:"effective_status"`
	IssueDate       string       `json:"issue_date"`
	DueDate         string       `json:"due_date"`
	Currency        string       `json:"currency"`
	Total           models.Money `json:"total"`
	PaidAmount      models.Money `json:"paid_amount"`
	Balance         models.Money `json:"balance"`
	PublicPayToken  string       `json:"public_pay_token,omitempty"`
}

// publicClientPaymentRow is one payment-history row, relabeled so no provider
// name leaks to the payer.
type publicClientPaymentRow struct {
	ID     uuid.UUID    `json:"id"`
	Amount models.Money `json:"amount"`
	PaidOn string       `json:"paid_on"`
	Method string       `json:"method"`
}

// publicClientPortalResponse is the whole public payload.
type publicClientPortalResponse struct {
	Client   publicClientInfo         `json:"client"`
	Invoices []publicClientInvoiceRow `json:"invoices"`
	Payments []publicClientPaymentRow `json:"payments"`
	Branding fiber.Map                `json:"branding"`
}

// clientPortalResponse wraps the owner-facing link payload. Token is present
// only on a fresh mint/regenerate; url_path is always returned so the FE can
// build the absolute URL, and active reports whether a live link exists (so
// the card can show copy/regenerate/revoke without re-exposing the hash).
type clientPortalResponse struct {
	ClientPortal clientPortalLinkInfo `json:"client_portal"`
}

type clientPortalLinkInfo struct {
	URLPath string `json:"url_path"`
	Token   string `json:"token,omitempty"`
	Active  bool   `json:"active"`
}
