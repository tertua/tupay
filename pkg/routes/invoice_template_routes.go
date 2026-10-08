package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerInvoiceTemplateRoutes wires the recurring invoice template CRUD
// endpoints. It is called from registerInvoiceRoutes (and thus indirectly from
// private_routes.go) so the literal /invoice-templates paths never collide with
// the /invoices/:id param route, and private_routes.go stays within its
// ratchet. Reads are open to any member; every mutation is owner-only.
func registerInvoiceTemplateRoutes(route fiber.Router) {
	route.Get("/invoice-templates", controllers.ListInvoiceTemplates)   // list recurring templates
	route.Get("/invoice-templates/:id", controllers.GetInvoiceTemplate) // get one template + items
	route.Post("/invoice-templates",
		middleware.RequireOrgRole("owner"), controllers.CreateInvoiceTemplate) // create a template
	route.Patch("/invoice-templates/:id",
		middleware.RequireOrgRole("owner"), controllers.UpdateInvoiceTemplate) // replace a template
	route.Patch("/invoice-templates/:id/status",
		middleware.RequireOrgRole("owner"), controllers.UpdateInvoiceTemplateStatus) // active<->paused
	route.Delete("/invoice-templates/:id",
		middleware.RequireOrgRole("owner"), controllers.DeleteInvoiceTemplate) // delete an unused template
}
