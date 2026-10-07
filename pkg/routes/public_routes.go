package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// PublicRoutes func for describe group of public routes.
//
// NOTE: limiters are attached per-route (not via Group("", ...)): in Fiber
// a subgroup with an empty prefix mounts its middleware on the parent
// prefix, which would throttle every /api route including private ones.
func PublicRoutes(a *fiber.App) {
	PublicRoutesAt(a, APILegacyPrefix)
}

// PublicRoutesAt registers public routes under prefix (see versioning.go).
func PublicRoutesAt(a *fiber.App, prefix string) {
	// Create routes group matching the frontend apiClient baseURL.
	route := a.Group(prefix)

	// Brute-forceable auth endpoints get the strict limiter.
	registerPublicAuthRoutes(route)

	// Public payment pages (shared links, higher abuse potential).
	registerPublicPayRoutes(route)

	// Public client portal (one client's invoices + payment history via a
	// shareable token); its own limiter namespace so a busy pay page cannot
	// starve it.
	registerPublicClientRoutes(route, middleware.PublicClientLimiter())

	route.Get("/config", controllers.AppConfig) // public branding for the MPA

	registerWebhooks(route)
}
