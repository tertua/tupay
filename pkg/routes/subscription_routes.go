package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerSubscriptionRoutes wires the subscription CRUD endpoints. It is
// called from registerInvoiceRoutes (and thus indirectly from
// private_routes.go) so the literal /subscriptions paths never collide with the
// /invoices/:id param route, and private_routes.go stays within its ratchet.
// Reads are open to any member; every mutation is owner-only.
func registerSubscriptionRoutes(route fiber.Router) {
	route.Get("/subscriptions", controllers.ListSubscriptions)   // list subscriptions
	route.Get("/subscriptions/:id", controllers.GetSubscription) // get one subscription + items
	route.Post("/subscriptions",
		middleware.RequireOrgRole("owner"), controllers.CreateSubscription) // create a subscription
	route.Patch("/subscriptions/:id",
		middleware.RequireOrgRole("owner"), controllers.UpdateSubscription) // replace a subscription
	route.Patch("/subscriptions/:id/status",
		middleware.RequireOrgRole("owner"), controllers.UpdateSubscriptionStatus) // active<->paused
	route.Delete("/subscriptions/:id",
		middleware.RequireOrgRole("owner"), controllers.DeleteSubscription) // delete an unused subscription
}
