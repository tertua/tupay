package controllers

import (
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/mail"
)

// SendInvoiceEmail emails the invoice to the client's billing email. Owner-only
// (route guard). The recipient is always the client record's email — never a
// caller-supplied address. A draft must be sent first (409). Resending is
// legitimate, so this endpoint is NOT idempotency-keyed: every call enqueues a
// new mail_outbox row (contrast the reminder sweep's unique (invoice, kind) leg).
// No cached aggregate depends on outbound mail, so invalidateAggregates is
// deliberately omitted.
// @Description Email an invoice to its client.
// @Summary send invoice via email
// @Tags Invoices
// @Accept json
// @Produce json
// @Param id path string true "Invoice ID"
// @Success 202 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{} "invoice not found"
// @Failure 409 {object} map[string]interface{} "invoice is still a draft"
// @Failure 409 {object} map[string]interface{} "client has no email"
// @Failure 501 {object} map[string]interface{} "email provider is not configured"
// @Security SessionCookie
// @Router /invoices/{id}/send-email [post]
func SendInvoiceEmail(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid invoice id", nil)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}

	invoice, err := db.GetInvoice(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "invoice")
	}

	// Emailing is a deliver-the-bill action: a draft has not been sent yet, so
	// there is nothing to deliver. Sent/overdue/paid all pass (a paid resend is
	// a legitimate receipt).
	if effectiveInvoiceStatus(*db, invoice) == models.InvoiceStatusDraft {
		return utils.Fail(c, fiber.StatusConflict, "invoice is still a draft", nil)
	}

	to, status, message, details := invoiceRecipient(*db, orgID, invoice)
	if status != 0 {
		return utils.Fail(c, status, message, details)
	}

	// Fail fast when mail is not configured (501 contract), then queue for async delivery by the worker.
	if _, err := mail.NewFromEnv(); errors.Is(err, mail.ErrNotConfigured) {
		return utils.Fail(c, fiber.StatusNotImplemented, "email provider is not configured", nil)
	} else if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "email provider configuration is invalid", nil)
	}

	htmlBody, terr := mail.Render("invoice_send", mail.TemplateData{
		AppName:       configs.Get().AppName,
		InvoiceNumber: invoice.InvoiceNumber,
		Total:         invoice.Total.String(),
		Currency:      invoice.Currency,
		DueDate:       utils.FormatDate(invoice.DueDate),
		URL:           invoiceEmailURL(*db, orgID, invoice),
	})
	if terr != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to render invoice email", nil)
	}

	subject := "Invoice " + invoice.InvoiceNumber
	if err := db.EnqueueMail(&models.MailOutbox{
		To:       to,
		Subject:  subject,
		Body:     subject,
		HtmlBody: htmlBody,
	}); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to queue invoice email", nil)
	}

	recordAudit(c, db, utils.CurrentActorID(c), "invoice.sendEmail", "invoice", id.String(), "")

	return utils.OK(c, fiber.StatusAccepted, fiber.Map{"queued": true})
}

// invoiceRecipient resolves the client email for an invoice. It returns
// ("", 0, "", nil) when the client has a usable address; otherwise the status,
// message and details for utils.Fail: 409 when the client has no email, 404
// when the client row vanished, 500 on a load error.
func invoiceRecipient(db database.Queries, orgID uuid.UUID, inv models.Invoice) (string, int, string, fiber.Map) {
	if inv.ClientID == nil {
		return "", fiber.StatusConflict, "client has no email", fiber.Map{"client_id": nil}
	}
	client, err := db.GetClient(orgID, *inv.ClientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fiber.StatusNotFound, "client not found", nil
		}
		return "", fiber.StatusInternalServerError, "failed to load client", nil
	}
	if client.Email == "" {
		return "", fiber.StatusConflict, "client has no email", fiber.Map{"client_id": inv.ClientID}
	}
	return client.Email, 0, "", nil
}

// invoiceEmailURL builds the CTA link: the invoice's existing public pay link
// when one exists, otherwise the app root. It never mints a link (that is
// ensurePaymentLink's job) so emailing a non-Online invoice stays honest.
func invoiceEmailURL(db database.Queries, orgID uuid.UUID, inv models.Invoice) string {
	link, err := db.GetPaymentLinkForInvoice(inv.ID, orgID)
	if err != nil || link.Token == "" {
		return publicURL("/")
	}
	return publicURL("/pay/" + link.Token)
}
