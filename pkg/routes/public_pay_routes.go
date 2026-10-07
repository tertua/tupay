package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerPublicPayRoutes wires the hosted public pay surface (one invoice per
// shareable token). Split out of public_routes.go so the router index stays
// within its size ratchet.
func registerPublicPayRoutes(route fiber.Router) {
	publicPay := middleware.PublicPayLimiter()
	route.Get("/public/pay/:token", publicPay, controllers.GetPublicPayment)
	route.Post("/public/pay/:token/transaction", publicPay, middleware.Idempotency(middleware.PublicPayIdempotencyScope), controllers.CreatePublicTransaction)
	route.Get("/public/pay/:token/status", publicPay, controllers.GetPublicPaymentStatus)
	route.Get("/public/pay/:token/qr", publicPay, controllers.GetPublicQrImage)
	route.Get("/public/gateway/config", publicPay, controllers.GatewayConfig)
	route.Get("/public/gateway/status", publicPay, controllers.GatewayStatus)
}
