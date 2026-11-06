package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerGatewayAdminRoutes mounts the session-cookie admin group for the
// central payment relay (called per prefix, see versioning.go).
func registerGatewayAdminRoutes(a *fiber.App, prefix string) {
	admin := a.Group(prefix+"/admin/gateway", middleware.AuthRequired(), middleware.OrgContext(), middleware.RequireCSRF(), middleware.RequireRoles("admin"))
	admin.Post("/projects", controllers.CreateProject)
	admin.Get("/projects", controllers.ListProjects)
	admin.Patch("/projects/:slug", controllers.UpdateProject)
	admin.Delete("/projects/:slug", controllers.DeleteProject)
	admin.Post("/projects/:slug/rotate-key", middleware.RequireStepUp(), controllers.RotateProjectKey)
	admin.Post("/projects/:slug/rotate-secret", middleware.RequireStepUp(), controllers.RotateProjectSecret)
	admin.Get("/transactions", controllers.ListAllTransactions)
	admin.Get("/settlement", controllers.GetSettlement)
	admin.Get("/deliveries", controllers.ListDeliveries)
	admin.Post("/deliveries/:id/retry", controllers.RetryDelivery)
}
