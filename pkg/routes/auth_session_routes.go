package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/app/controllers"
	"github.com/tertua/tupay/pkg/middleware"
)

// registerAuthSessionRoutes mounts the session-cookie auth endpoints: profile,
// password, logout, plus the second factor and the step-up mint. Split out of
// private_routes.go to keep that file inside its size baseline.
func registerAuthSessionRoutes(route fiber.Router) {
	route.Get("/auth/me", controllers.Me)                     // get current session user
	route.Patch("/auth/profile", controllers.UpdateProfile)   // update display name
	route.Patch("/auth/password", controllers.ChangePassword) // change password
	route.Post("/auth/logout", controllers.Logout)            // end session

	route.Get("/auth/totp", controllers.TOTPStatus)                           // read second-factor status
	route.Post("/auth/totp/enroll", controllers.EnrollTOTP)                   // mint a pending secret
	route.Post("/auth/totp/activate", controllers.ActivateTOTP)               // confirm with a first code
	route.Post("/auth/totp/disable", controllers.DisableTOTP)                 // turn the second factor off
	route.Post("/auth/step-up", middleware.AuthLimiter(), controllers.StepUp) // mint a single-use step-up token
}
