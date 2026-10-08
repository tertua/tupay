package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerPublicClientRoutes wires the public client-portal endpoints onto the
// public group, reusing the shared abuse limiter passed in by the caller.
func registerPublicClientRoutes(route fiber.Router, limiter fiber.Handler) {
	route.Get("/public/client/:token", limiter, controllers.GetPublicClient)
	route.Get("/public/client/:token/invoice/:id", limiter, controllers.GetPublicClientInvoice)
}

// registerClientRoutes wires the tenant's client CRUD, the owner-only
// client-portal link lifecycle, the owner-only archive lifecycle, and the
// member-level receivables reminder onto the private session group. Split out
// of private_routes.go so the router index stays within its size ratchet.
func registerClientRoutes(route fiber.Router) {
	route.Get("/clients", controllers.ListClients)         // get all clients
	route.Post("/clients", controllers.CreateClient)       // create a new client
	route.Get("/clients/:id", controllers.GetClient)       // get client with invoices and stats
	route.Patch("/clients/:id", controllers.UpdateClient)  // update a client
	route.Delete("/clients/:id", controllers.DeleteClient) // delete a client

	// Client lifecycle: archive/unarchive flip status. The more specific
	// :id/archive paths do not collide with :id in Fiber, but they sit after
	// the CRUD lines so the generic routes stay grouped.
	route.Patch("/clients/:id/archive", middleware.RequireOrgRole("owner"), controllers.ArchiveClient)
	route.Patch("/clients/:id/unarchive", middleware.RequireOrgRole("owner"), controllers.UnarchiveClient)

	// One-off receivables reminder: queues a manual reminder leg per open
	// invoice; idempotent via the (invoice, manual) claim.
	route.Post("/clients/:id/reminder", controllers.SendClientReminder)

	// Statement CSV export: intentionally bypasses the JSON envelope (text/csv
	// attachment), hence the distinct .csv path segment.
	route.Get("/clients/:id/statement.csv", controllers.ExportClientStatement)

	// Owner-only portal link lifecycle: PATCH ensures/mints, POST regenerates,
	// DELETE revokes. RequireOrgRole("owner") gates every mutation.
	route.Patch("/clients/:id/portal", middleware.RequireOrgRole("owner"), controllers.EnsureClientPortalLink)
	route.Post("/clients/:id/portal/regenerate", middleware.RequireOrgRole("owner"), controllers.RegenerateClientPortalLink)
	route.Delete("/clients/:id/portal", middleware.RequireOrgRole("owner"), controllers.RevokeClientPortalLink)
}
