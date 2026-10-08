package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerInvoiceRoutes wires the invoice CRUD routes plus the local Snap
// intent endpoint. Kept in its own file so private_routes.go stays readable
// as the list grows. Order matters: the literal /invoices/status-counts is
// registered before the /invoices/:id param route so it can never be shadowed.
func registerInvoiceRoutes(route fiber.Router) {
	route.Get("/invoices", controllers.ListInvoices)                                                               // get invoices with filters
	route.Get("/invoices/status-counts", controllers.InvoiceStatusCounts)                                          // counts per status for tab badges
	route.Post("/invoices", middleware.Idempotency(middleware.SessionIdempotencyScope), controllers.CreateInvoice) // create a new invoice
	route.Get("/invoices/:id", controllers.GetInvoice)                                                             // get invoice with items and payments
	route.Patch("/invoices/:id", controllers.UpdateInvoice)                                                        // update an invoice
	route.Patch("/invoices/:id/status", controllers.UpdateInvoiceStatus)                                           // update invoice status
	route.Delete("/invoices/:id", controllers.DeleteInvoice)                                                       // delete an invoice
	// Local Snap intent for one invoice (session user, no service key); kept under /invoices so the /gateway API-key group cannot shadow it.
	// Owner-only unconditionally, so it is gated here by middleware; state-dependent rules (draft-only edits, transition matrix) stay in invoice_rules.go because they must read the row.
	route.Post("/invoices/:id/intents",
		middleware.RequireOrgRole("owner"),
		middleware.Idempotency(middleware.SessionIdempotencyScope),
		middleware.WithAITimeout(controllers.CreateInvoiceIntent))
	// Subscriptions (literal paths, so they can never be shadowed by
	// /invoices/:id) are registered here to keep private_routes.go flat.
	registerSubscriptionRoutes(route)
}
