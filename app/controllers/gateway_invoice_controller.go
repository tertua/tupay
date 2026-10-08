package controllers

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"gorm.io/gorm"
)

// CreateGatewayInvoice creates a client and invoice atomically for a service project.
// @Description Create an invoice using project-scoped external identifiers.
// @Summary create integration invoice
// @Tags Gateway
// @Accept json
// @Produce json
// @Param request body models.GatewayInvoiceInput true "Invoice payload"
// @Param Idempotency-Key header string true "Replay protection key"
// @Success 201 {object} map[string]interface{}
// @Router /gateway/invoices [post]
func CreateGatewayInvoice(c fiber.Ctx) error {
	project, err := utils.CurrentServiceProject(c)
	if err != nil || project.OwnerUserID == nil {
		return utils.Fail(c, fiber.StatusForbidden, "gateway project has no owner", nil)
	}
	if strings.TrimSpace(c.Get("Idempotency-Key")) == "" {
		return utils.Fail(c, fiber.StatusBadRequest, "Idempotency-Key header is required", nil)
	}
	input := &models.GatewayInvoiceInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Customer.ExternalID = strings.TrimSpace(input.Customer.ExternalID)
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}

	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, lookupErr := db.GetGatewayInvoice(project.Slug, input.ExternalID); lookupErr == nil {
		return utils.Fail(c, fiber.StatusConflict, "external_id already exists", nil)
	}

	issueDate, err := utils.ParseDate(input.IssueDate)
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid issue_date, expected YYYY-MM-DD", nil)
	}
	dueDate, err := utils.ParseDate(input.DueDate)
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid due_date, expected YYYY-MM-DD", nil)
	}
	if input.Discount.IsNegative() {
		return utils.Fail(c, fiber.StatusBadRequest, "money values cannot be negative", nil)
	}

	now := time.Now()
	invoice := &models.Invoice{ID: uuid.New(), CreatedAt: now, UpdatedAt: &now, UserID: *project.OwnerUserID, OrgID: project.OrgID,
		Status: input.Status, IssueDate: issueDate, DueDate: dueDate, Currency: input.Currency,
		TaxRate: input.TaxRate, Discount: input.Discount, Notes: input.Notes, Terms: input.Terms}
	projectSlug, externalID := project.Slug, input.ExternalID
	invoice.GatewayProjectSlug, invoice.ExternalID = &projectSlug, &externalID
	items, itemErr := models.AddInvoiceItems(invoice, input.Items)
	if itemErr != nil {
		return utils.Fail(c, fiber.StatusBadRequest, itemErr.Error(), nil)
	}
	models.ApplyInvoiceTotals(invoice)
	err = db.InvoiceQueries.Transaction(func(tx *gorm.DB) error {
		client := models.Client{}
		if lookupErr := tx.Where("gateway_project_slug = ? AND external_id = ?", project.Slug, input.Customer.ExternalID).First(&client).Error; errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			customerID := input.Customer.ExternalID
			client = models.Client{ID: uuid.New(), CreatedAt: now, UserID: *project.OwnerUserID, OrgID: project.OrgID, GatewayProjectSlug: &projectSlug, ExternalID: &customerID,
				Name: input.Customer.Name, Email: input.Customer.Email, Company: input.Customer.Company, Phone: input.Customer.Phone, Address: input.Customer.Address}
			if err := tx.Create(&client).Error; err != nil {
				return err
			}
		} else if lookupErr != nil {
			return lookupErr
		}
		invoice.ClientID = &client.ID
		number, nerr := db.ReserveInvoiceNumber(tx, project.OrgID)
		if nerr != nil {
			return nerr
		}
		invoice.InvoiceNumber = number
		if err := tx.Create(invoice).Error; err != nil {
			return err
		}
		return tx.Create(&items).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return utils.Fail(c, fiber.StatusConflict, "external_id already exists", nil)
		}
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create invoice", nil)
	}
	return gatewayInvoiceResponse(c, *db, project.OrgID, *invoice, fiber.StatusCreated)
}

func gatewayInvoiceResponse(c fiber.Ctx, db database.Queries, orgID uuid.UUID, invoice models.Invoice, status int) error {
	detail, err := invoiceDetail(db, orgID, invoice.ID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load invoice", nil)
	}
	detail.ExternalID = invoice.ExternalID
	return utils.OK(c, status, fiber.Map{"invoice": detail})
}
