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

// registerClientRoutes wires the tenant's client CRUD plus the owner-only
// client-portal link lifecycle onto the private session group. Split out of
// private_routes.go so the router index stays within its size ratchet.
func registerClientRoutes(route fiber.Router) {
	route.Get("/clients", controllers.ListClients)         // get all clients
	route.Post("/clients", controllers.CreateClient)       // create a new client
	route.Get("/clients/:id", controllers.GetClient)       // get client with invoices and stats
	route.Patch("/clients/:id", controllers.UpdateClient)  // update a client
	route.Delete("/clients/:id", controllers.DeleteClient) // delete a client

	// Owner-only portal link lifecycle: PATCH ensures/mints, POST regenerates,
	// DELETE revokes. RequireOrgRole("owner") gates every mutation.
	route.Patch("/clients/:id/portal", middleware.RequireOrgRole("owner"), controllers.EnsureClientPortalLink)
	route.Post("/clients/:id/portal/regenerate", middleware.RequireOrgRole("owner"), controllers.RegenerateClientPortalLink)
	route.Delete("/clients/:id/portal", middleware.RequireOrgRole("owner"), controllers.RevokeClientPortalLink)
}
