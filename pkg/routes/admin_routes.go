package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerAdminRoutes mounts the session-cookie admin group (called per prefix, see versioning.go).
func registerAdminRoutes(a *fiber.App, prefix string) {
	admin := a.Group(prefix+"/admin", middleware.GeneralLimiter(), middleware.AuthRequired(), middleware.RequireCSRF(), middleware.RequireRoles("admin"))
	admin.Get("/users", controllers.ListUsers)
	admin.Patch("/users/:id/role", controllers.UpdateUserRole)
	admin.Patch("/users/:id/status", controllers.UpdateUserStatus)
	admin.Delete("/users/:id", controllers.DeleteUser)
	admin.Get("/audit-logs", controllers.ListAuditLogs)
	admin.Post("/orgs", controllers.CreateOrg)
	admin.Post("/migrate/down", controllers.MigrateDown)
	admin.Get("/outbox/status", controllers.OutboxStatus)
}
