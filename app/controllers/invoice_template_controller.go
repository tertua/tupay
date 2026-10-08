package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/pkg/utils"
)

// ListInvoiceTemplates returns one page of recurring invoice templates owned by
// the current org.
// @Description Get recurring invoice templates of current org.
// @Summary list invoice templates
// @Tags InvoiceTemplates
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /invoice-templates [get]
func ListInvoiceTemplates(c fiber.Ctx) error {
	orgID, err := utils.CurrentOrgID(c)
	if err != nil {
		return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
	}
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	paging := utils.ParsePagination(c)
	templates, err := db.ListInvoiceTemplates(orgID, paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load invoice templates", nil)
	}
	total, err := db.CountInvoiceTemplates(orgID)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count invoice templates", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"invoice_templates": templates, "meta": paging.Meta(total)})
}

// GetInvoiceTemplate returns one recurring invoice template with its items.
// @Description Get a recurring invoice template by ID.
// @Summary get invoice template
// @Tags InvoiceTemplates
// @Produce json
// @Param id path string true "Invoice template ID"
// @Success 200 {object} map[string]interface{}
// @Security SessionCookie
// @Router /invoice-templates/{id} [get]
func GetInvoiceTemplate(c fiber.Ctx) error {
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
	tpl, err := db.GetInvoiceTemplate(orgID, id)
	if err != nil {
		return utils.NotFoundOrFailed(c, err, "invoice template")
	}
	items, err := db.GetInvoiceTemplateItems(id)
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load invoice template items", nil)
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"invoice_template": tpl, "items": items})
}
