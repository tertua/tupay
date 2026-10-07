package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// PrivateRoutes func for describe group of private routes.
func PrivateRoutes(a *fiber.App) {
	PrivateRoutesAt(a, APILegacyPrefix)
}

// PrivateRoutesAt registers private routes under prefix (see versioning.go).
func PrivateRoutesAt(a *fiber.App, prefix string) {
	// Admin mounts first: group middleware becomes a prefix Use, so registering it after would run OrgContext on the global admin console (a schema rollback drops memberships and turned admin calls into 403 org.notMember).
	registerAdminRoutes(a, prefix)

	// Group matches the frontend apiClient baseURL; order is AuthRequired, OrgContext, RequireCSRF — only session-cookie mutations need the double-submit header (API-key relay has its own group).
	route := a.Group(prefix, middleware.GeneralLimiter(), middleware.AuthRequired(), middleware.OrgContext(), middleware.RequireCSRF())

	// Auth session routes:
	route.Get("/auth/me", controllers.Me)                     // get current session user
	route.Patch("/auth/profile", controllers.UpdateProfile)   // update display name
	route.Patch("/auth/password", controllers.ChangePassword) // change password
	route.Post("/auth/logout", controllers.Logout)            // end session

	// Client routes (CRUD + owner-only portal link lifecycle):
	registerClientRoutes(route)

	// Invoice routes:
	registerInvoiceRoutes(route)

	// Dashboard routes:
	route.Get("/dashboard", controllers.GetDashboard) // get dashboard aggregates

	// Live event stream (SSE, session cookie, no per-request timeout):
	route.Get("/events", controllers.StreamEvents) // subscribe to aggregate-change events

	// Catalog item routes:
	route.Get("/items", controllers.ListItems)
	route.Post("/items", controllers.CreateItem)
	route.Patch("/items/:id", controllers.UpdateItem)
	route.Delete("/items/:id", controllers.DeleteItem)

	// Expense routes:
	route.Get("/expenses", controllers.ListExpenses)
	route.Post("/expenses", controllers.CreateExpense)
	route.Patch("/expenses/:id", controllers.UpdateExpense)
	route.Delete("/expenses/:id", controllers.DeleteExpense)
	route.Post("/expenses/:id/receipt", controllers.UploadReceipt)
	route.Get("/expenses/:id/receipt", controllers.GetReceipt)
	route.Delete("/expenses/:id/receipt", controllers.DeleteReceipt)

	// Payment routes (mutating payment routes replay on Idempotency-Key).
	route.Get("/payments", controllers.ListPayments)
	route.Post("/payments", middleware.RequireOrgRole("owner"), middleware.Idempotency(middleware.SessionIdempotencyScope), controllers.CreatePayment)
	route.Delete("/payments/:id", middleware.RequireOrgRole("owner"), middleware.Idempotency(middleware.SessionIdempotencyScope), controllers.VoidPayment)
	route.Post("/payments/online", middleware.RequireOrgRole("owner"), middleware.Idempotency(middleware.SessionIdempotencyScope), controllers.CreateOnlineLink)
	route.Post("/payments/online/send", middleware.RequireOrgRole("owner"), controllers.SendOnlineLink)

	// Reports routes:
	route.Get("/reports", controllers.GetReports)

	// Settings (GET any member, PATCH owner-only), invoice approval (D11) and organization (D10) routes.
	registerSettingsRoutes(route)
	registerInvoiceApprovalRoutes(route)
	registerOrgRoutes(route)

	registerNotificationRoutes(route)

	// AI routes (slower upstream calls get a per-request timeout).
	route.Post("/ai/receipt-parse", middleware.WithAITimeout(controllers.ReceiptParse))
	route.Post("/ai/business-summary", middleware.WithAITimeout(controllers.BusinessSummary))
	route.Post("/ai/payment-reminder", middleware.WithAITimeout(controllers.PaymentReminder))
	route.Post("/ai/write-note", middleware.WithAITimeout(controllers.WriteNote))
}
