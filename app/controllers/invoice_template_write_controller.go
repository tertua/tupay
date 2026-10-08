package controllers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
)

// templateBuildError maps a template payload that fails to build into a 400.
type templateBuildError struct{ msg string }

func (e templateBuildError) Error() string { return e.msg }

// buildTemplateInput parses and validates a create/update payload into a
// template header plus item rows. It keeps the write handlers thin and is the
// single place the JSON→model mapping lives.
func buildTemplateInput(input *models.InvoiceTemplateInput, orgID, userID uuid.UUID) (*models.InvoiceTemplate, []models.InvoiceTemplateItem, error) {
	clientID, err := parseTemplateClient(input)
	if err != nil {
		return nil, nil, err
	}
	// Reuse the invoice send rule: a sent template must name a client.
	if err := validateInvoice(input.InvoiceStatus, clientID); err != nil {
		return nil, nil, templateBuildError{msg: err.Error()}
	}
	nextRun, err := parseTemplateStart(input.StartDate)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tpl := &models.InvoiceTemplate{
		ID:            uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     &now,
		UserID:        userID,
		OrgID:         orgID,
		ClientID:      clientID,
		Name:          input.Name,
		Status:        input.Status,
		Cadence:       input.Cadence,
		NextRunDate:   nextRun,
		LeadDays:      input.LeadDays,
		Currency:      input.Currency,
		TaxRate:       input.TaxRate,
		Discount:      input.Discount,
		Notes:         input.Notes,
		Terms:         input.Terms,
		PaymentMethod: input.PaymentMethod,
		InvoiceStatus: input.InvoiceStatus,
		DueDays:       input.DueDays,
	}
	// Build item rows through the shared money math so a negative rate/discount
	// is rejected with the same error as an invoice.
	items, err := templateItemRows(tpl.ID, input.Items)
	if err != nil {
		return nil, nil, err
	}
	return tpl, items, nil
}

// parseTemplateClient resolves the optional client id and rejects a bad one.
func parseTemplateClient(input *models.InvoiceTemplateInput) (*uuid.UUID, error) {
	if input.ClientID == nil || *input.ClientID == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(*input.ClientID)
	if err != nil {
		return nil, templateBuildError{msg: "invalid client_id"}
	}
	return &parsed, nil
}

// parseTemplateStart parses the date-only start_date into the first run cursor.
// An empty start date leaves the template unscheduled (NextRunDate nil).
func parseTemplateStart(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := utils.ParseRequiredDate(value)
	if err != nil {
		return nil, templateBuildError{msg: "invalid start_date, expected YYYY-MM-DD"}
	}
	day := utils.DateOnly(parsed)
	return &day, nil
}

// templateItemRows builds template line rows, reusing the same negative-money
// rejection the invoice path uses.
func templateItemRows(templateID uuid.UUID, entries []models.InvoiceItemInput) ([]models.InvoiceTemplateItem, error) {
	items := make([]models.InvoiceTemplateItem, 0, len(entries))
	for position, entry := range entries {
		if entry.Rate.IsNegative() {
			return nil, models.ErrNegativeMoney
		}
		items = append(items, models.InvoiceTemplateItem{
			ID:          uuid.New(),
			TemplateID:  templateID,
			Description: entry.Description,
			Quantity:    entry.Quantity,
			Rate:        entry.Rate,
			Position:    position,
		})
	}
	return items, nil
}

// rebindTemplateItems points a rebuilt item set at an existing template id.
func rebindTemplateItems(templateID uuid.UUID, items []models.InvoiceTemplateItem) []models.InvoiceTemplateItem {
	for i := range items {
		items[i].TemplateID = templateID
	}
	return items
}

// templateInputError writes the 400 envelope for a bad payload/build. Every
// build failure (bad client id, bad date, negative money, send-without-client)
// is a malformed request, so it always maps to 400.
func templateInputError(c fiber.Ctx, err error) error {
	return utils.Fail(c, fiber.StatusBadRequest, err.Error(), nil)
}

// CreateInvoiceTemplate creates a recurring invoice template for the org.
// @Description Create a recurring invoice template.
// @Summary create invoice template
// @Tags InvoiceTemplates
// @Accept json
// @Produce json
// @Param request body models.InvoiceTemplateInput true "Invoice template payload"
// @Success 201 {object} map[string]interface{}
// @Security SessionCookie
// @Router /invoice-templates [post]
func CreateInvoiceTemplate(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	input := &models.InvoiceTemplateInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	tpl, items, err := buildTemplateInput(input, orgID, utils.CurrentActorID(c))
	if err != nil {
		return templateInputError(c, err)
	}
	if err := db.CreateInvoiceTemplate(tpl, items); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to create invoice template", nil)
	}
	return utils.OK(c, fiber.StatusCreated, fiber.Map{"invoice_template": tpl, "items": items})
}

// UpdateInvoiceTemplate replaces a recurring invoice template and its items.
// @Description Update a recurring invoice template.
// @Summary update invoice template
// @Tags InvoiceTemplates
// @Accept json
// @Produce json
// @Param id path string true "Invoice template ID"
// @Param request body models.InvoiceTemplateInput true "Invoice template payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /invoice-templates/{id} [patch]
func UpdateInvoiceTemplate(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid invoice template id", nil)
	}
	input := &models.InvoiceTemplateInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	existing, err := db.GetInvoiceTemplate(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "invoice template")
	}
	tpl, items, err := buildTemplateInput(input, orgID, existing.UserID)
	if err != nil {
		return templateInputError(c, err)
	}
	tpl.ID = existing.ID
	tpl.CreatedAt = existing.CreatedAt
	// A blank start date keeps the existing cursor so an edit never unschedules
	// or re-fires an in-flight occurrence.
	if tpl.NextRunDate == nil {
		tpl.NextRunDate = existing.NextRunDate
	}
	tpl.LastRunDate = existing.LastRunDate
	if err := db.UpdateInvoiceTemplate(orgID, tpl, rebindTemplateItems(existing.ID, items)); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update invoice template", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"invoice_template": tpl, "items": items})
}

// DeleteInvoiceTemplate deletes a recurring invoice template. It refuses (409)
// when the template already generated invoices; the owner pauses it instead.
// @Description Delete a recurring invoice template.
// @Summary delete invoice template
// @Tags InvoiceTemplates
// @Produce json
// @Param id path string true "Invoice template ID"
// @Success 204 {string} status "ok"
// @Security SessionCookie
// @Router /invoice-templates/{id} [delete]
func DeleteInvoiceTemplate(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid invoice template id", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetInvoiceTemplate(orgID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "invoice template")
	}
	runs, err := db.CountTemplateRuns(id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load invoice template runs", nil)
	}
	if runs > 0 {
		return utils.Fail(c, fiber.StatusConflict, "invoice template has generated invoices; pause it instead", nil)
	}
	if err := db.DeleteInvoiceTemplate(orgID, id); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to delete invoice template", nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateInvoiceTemplateStatus activates or pauses a recurring invoice template.
// @Description Update a recurring invoice template's status.
// @Summary update invoice template status
// @Tags InvoiceTemplates
// @Accept json
// @Produce json
// @Param id path string true "Invoice template ID"
// @Param request body models.InvoiceTemplateStatusInput true "Status payload"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /invoice-templates/{id}/status [patch]
func UpdateInvoiceTemplateStatus(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid invoice template id", nil)
	}
	input := &models.InvoiceTemplateStatusInput{}
	if err := c.Bind().Body(input); err != nil {
		return utils.Fail(c, fiber.StatusBadRequest, "invalid request body", nil)
	}
	if err := utils.NewValidator().Struct(input); err != nil {
		return utils.ValidationFailed(c, err)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	if _, err := db.GetInvoiceTemplate(orgID, id); err != nil {
		return utils.NotFoundOrFailed(c, err, "invoice template")
	}
	if err := db.UpdateTemplateStatus(orgID, id, input.Status); err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to update invoice template status", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"status": input.Status})
}
