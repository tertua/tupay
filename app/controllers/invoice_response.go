package controllers

import (
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
)

// invoiceDetail loads the full invoice response for the frontend, scoped to the caller's org.
func invoiceDetail(db database.Queries, orgID, id uuid.UUID) (invoiceDetailResponse, error) {
	var out invoiceDetailResponse
	invoice, err := db.GetInvoice(orgID, id)
	if err != nil {
		return out, err
	}

	items, err := db.GetInvoiceItems(id)
	if err != nil {
		return out, err
	}
	out.Items = make([]invoiceItemResponse, 0, len(items))
	for _, item := range items {
		out.Items = append(out.Items, invoiceItemResponse{
			Description: item.Description,
			Quantity:    item.Quantity,
			Rate:        item.Rate,
			Amount:      item.Amount,
		})
	}

	payments, err := db.GetInvoicePayments(id)
	if err != nil {
		return out, err
	}
	out.Payments = make([]invoicePaymentResponse, 0, len(payments))
	var paid models.Money
	for _, payment := range payments {
		paid = paid.Add(payment.Amount)
		out.Payments = append(out.Payments, invoicePaymentResponse{
			ID:      payment.ID,
			Amount:  payment.Amount,
			PaidOn:  utils.FormatDate(payment.PaidOn),
			Method:  payment.Method,
			TxnID:   payment.TxnID,
			CanVoid: payment.CanVoid(),
		})
	}

	var clientName, clientCompany, clientEmail string
	if invoice.ClientID != nil {
		if client, err := db.GetClient(orgID, *invoice.ClientID); err == nil {
			clientName = client.Name
			clientCompany = client.Company
			clientEmail = client.Email
		}
	}

	// Expose the existing public payment link (if any) so the MPA can render it persistently instead of transient state.
	var paymentLink *paymentLinkResponse
	if link, err := db.GetPaymentLinkForInvoice(id, orgID); err == nil {
		paymentLink = &paymentLinkResponse{Token: link.Token, URL: "/pay/" + link.Token}
	}

	return invoiceDetailResponse{
		ID:              invoice.ID,
		InvoiceNumber:   invoice.InvoiceNumber,
		Status:          invoice.Status,
		EffectiveStatus: models.ResolveEffectiveStatus(invoice.Status, invoice.DueDate, invoice.Total, paid, db.PendingInvoiceIDs(orgID)[id]),
		PaymentLink:     paymentLink,
		ClientID:        invoice.ClientID,
		ClientName:      clientName,
		ClientCompany:   clientCompany,
		ClientEmail:     clientEmail,
		IssueDate:       utils.FormatDate(invoice.IssueDate),
		DueDate:         utils.FormatDate(invoice.DueDate),
		Currency:        invoice.Currency,
		Subtotal:        invoice.Subtotal,
		Discount:        invoice.Discount,
		TaxRate:         invoice.TaxRate,
		TaxAmount:       invoice.TaxAmount,
		Total:           invoice.Total,
		Notes:           invoice.Notes,
		Terms:           invoice.Terms,
		PaymentMethod:   invoice.PaymentMethod,
		Items:           out.Items,
		Payments:        out.Payments,
		PaidAmount:      paid,
		Balance:         invoice.Total.Sub(paid),
	}, nil
}

// invoiceDetailResponse is the internal invoice detail payload. Field order
// mirrors the historical hand-built map; ExternalID is populated only by the
// gateway invoice routes and omitted elsewhere.
type invoiceDetailResponse struct {
	ID              uuid.UUID                `json:"id"`
	InvoiceNumber   string                   `json:"invoice_number"`
	Status          string                   `json:"status"`
	EffectiveStatus string                   `json:"effective_status"`
	PaymentLink     *paymentLinkResponse     `json:"payment_link"`
	ClientID        *uuid.UUID               `json:"client_id"`
	ClientName      string                   `json:"client_name"`
	ClientCompany   string                   `json:"client_company"`
	ClientEmail     string                   `json:"client_email"`
	IssueDate       string                   `json:"issue_date"`
	DueDate         string                   `json:"due_date"`
	Currency        string                   `json:"currency"`
	Subtotal        models.Money             `json:"subtotal"`
	Discount        models.Money             `json:"discount"`
	TaxRate         float64                  `json:"tax_rate"`
	TaxAmount       models.Money             `json:"tax_amount"`
	Total           models.Money             `json:"total"`
	Notes           string                   `json:"notes"`
	Terms           string                   `json:"terms"`
	PaymentMethod   string                   `json:"payment_method"`
	Items           []invoiceItemResponse    `json:"items"`
	Payments        []invoicePaymentResponse `json:"payments"`
	PaidAmount      models.Money             `json:"paid_amount"`
	Balance         models.Money             `json:"balance"`
	ExternalID      *string                  `json:"external_id,omitempty"`
}

type invoiceItemResponse struct {
	Description string       `json:"description"`
	Quantity    float64      `json:"quantity"`
	Rate        models.Money `json:"rate"`
	Amount      models.Money `json:"amount"`
}

type invoicePaymentResponse struct {
	ID      uuid.UUID    `json:"id"`
	Amount  models.Money `json:"amount"`
	PaidOn  string       `json:"paid_on"`
	Method  string       `json:"method"`
	TxnID   string       `json:"txn_id"`
	CanVoid bool         `json:"can_void"`
}

type paymentLinkResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}
