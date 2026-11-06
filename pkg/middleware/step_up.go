package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/cache"
)

// StepUpHeader carries the single-use token minted by POST /auth/step-up. It
// is a header (not a cookie) so a cross-site form post cannot supply it.
const StepUpHeader = "X-Step-Up-Token"

// RequireStepUp rejects a destructive request unless it presents a live step-up
// token minted for the current account and session. The token is consumed
// before the handler runs, so one elevation covers exactly one request.
//
// Must be mounted after AuthRequired (it reads the authenticated user).
func RequireStepUp() fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, err := utils.CurrentUserID(c)
		if err != nil {
			return utils.Fail(c, fiber.StatusUnauthorized, "unauthorized, please sign in again", nil)
		}
		token := c.Get(StepUpHeader)
		if token == "" {
			return stepUpRequired(c)
		}
		raw, ok := cache.ConsumeStepUp(c.Context(), token)
		if !ok {
			return stepUpRequired(c)
		}
		tokenUser, tokenSID, ok := cache.DecodeStepUpValue(raw, time.Now())
		if !ok || tokenUser != userID || !sessionMatches(c, userID, tokenSID) {
			return stepUpRequired(c)
		}
		return c.Next()
	}
}

// stepUpRequired answers 401 with a machine-readable code so the frontend can
// open the confirmation prompt instead of logging the user out.
func stepUpRequired(c fiber.Ctx) error {
	return utils.Fail(c, fiber.StatusUnauthorized, "step-up confirmation required", fiber.Map{"code": "step_up_required"})
}
