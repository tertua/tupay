package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/app/queries"
	"github.com/tertua/tupay/pkg/utils"
)

// GetPublicClient returns a client's invoices and payment history for a public
// portal token. The token is hashed and looked up; a miss (missing or revoked)
// is the same 404 so there is no account enumeration. Every child read is
// scoped by the link's OrgID.
// @Description Get a public client portal.
// @Summary get public client portal
// @Tags Public Client
// @Produce json
// @Param token path string true "Client portal token"
// @Success 200 {object} map[string]interface{}
// @Router /public/client/{token} [get]
func GetPublicClient(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	link, err := db.GetClientLinkByHash(hashClientPortalToken(c.Params("token")))
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "client link not found", nil)
	}
	client, err := db.GetClient(link.OrgID, link.ClientID)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "client link not found", nil)
	}

	cap := queries.ClientPortalPaymentCap()
	rows, err := db.ClientPortalInvoices(link.OrgID, link.ClientID, cap)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client invoices", nil)
	}
	payTokens, err := db.ClientPortalPayTokens(link.OrgID, link.ClientID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client invoices", nil)
	}
	pending := db.PendingInvoiceIDs(link.OrgID)

	invoices := make([]publicClientInvoiceRow, 0, len(rows))
	for _, row := range rows {
		paid := row.PaidAmount
		invoices = append(invoices, publicClientInvoiceRow{
			ID:              row.ID,
			InvoiceNumber:   row.InvoiceNumber,
			Status:          row.Status,
			EffectiveStatus: row.EffectiveStatus(pending[row.ID]),
			IssueDate:       utils.FormatDate(row.IssueDate),
			DueDate:         utils.FormatDate(row.DueDate),
			Currency:        row.Currency,
			Total:           row.Total,
			PaidAmount:      paid,
			Balance:         row.Total.Sub(paid),
			PublicPayToken:  payTokens[row.ID],
		})
	}

	paymentRows, err := db.ClientPortalPayments(link.OrgID, link.ClientID, cap)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load client payments", nil)
	}
	payments := make([]publicClientPaymentRow, 0, len(paymentRows))
	for _, p := range paymentRows {
		payments = append(payments, publicClientPaymentRow{
			ID:     p.PaymentID,
			Amount: p.Amount,
			PaidOn: utils.FormatDate(p.PaidOn),
			Method: publicPayMethodLabel(*db, paymentRowToModel(p)),
		})
	}

	settings, err := db.GetSettings(link.OrgID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load branding", nil)
	}

	return utils.OK(c, fiber.StatusOK, publicClientPortalResponse{
		Client:   publicClientInfo{Name: client.Name, Company: client.Company},
		Invoices: invoices,
		Payments: payments,
		Branding: fiber.Map{
			"company_name": settings.CompanyName,
			"logo_url":     settings.LogoURL,
		},
	})
}

// GetPublicClientInvoice returns payer-safe invoice detail for one invoice that
// belongs to the portal's client and org (else 404). The shape matches the pay
// page's publicInvoiceDetailResponse so the FE can render the PDF without a new
// pipeline.
// @Description Get a public client invoice detail.
// @Summary get public client invoice
// @Tags Public Client
// @Produce json
// @Param token path string true "Client portal token"
// @Param id path string true "Invoice ID"
// @Success 200 {object} map[string]interface{}
// @Router /public/client/{token}/invoice/{id} [get]
func GetPublicClientInvoice(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	link, err := db.GetClientLinkByHash(hashClientPortalToken(c.Params("token")))
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "client link not found", nil)
	}
	invoiceID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "invoice not found", nil)
	}
	invoice, err := db.GetInvoice(link.OrgID, invoiceID)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "invoice not found", nil)
	}
	// The invoice must belong to the portal's client, not merely the org.
	if invoice.ClientID == nil || *invoice.ClientID != link.ClientID {
		return utils.Fail(c, fiber.StatusNotFound, "invoice not found", nil)
	}
	detail, err := invoiceDetail(*db, link.OrgID, invoiceID)
	if err != nil {
		return utils.Fail(c, fiber.StatusNotFound, "invoice not found", nil)
	}
	payments, err := db.GetInvoicePayments(invoiceID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load invoice payments", nil)
	}
	clean := make([]publicPaymentResponse, 0, len(payments))
	for _, p := range payments {
		clean = append(clean, publicPaymentResponse{
			ID:     p.ID,
			Amount: p.Amount,
			PaidOn: utils.FormatDate(p.PaidOn),
			Method: publicPayMethodLabel(*db, p),
		})
	}
	return utils.OK(c, fiber.StatusOK, publicInvoiceDetailResponse{
		invoiceDetailResponse: detail,
		Payments:              clean,
	})
}

// paymentRowToModel adapts a list row back to the Payment shape the method
// relabeler reads (gateway_order_id + method), so no provider name leaks.
func paymentRowToModel(row models.PaymentListRow) models.Payment {
	return models.Payment{
		ID:             row.PaymentID,
		InvoiceID:      row.InvoiceID,
		Amount:         row.Amount,
		Method:         row.Method,
		GatewayOrderID: row.GatewayOrderID,
	}
}
