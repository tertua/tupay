package routes

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientPortalRoutesRegistered asserts the new public and owner routes
// exist on the wired app (public portal GETs + owner-only lifecycle).
func TestClientPortalRoutesRegistered(t *testing.T) {
	app := newTestApp()

	found := map[string]bool{}
	for _, r := range app.GetRoutes() {
		found[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{
		"GET /api/public/client/:token",
		"GET /api/public/client/:token/invoice/:id",
		"PATCH /api/clients/:id/portal",
		"POST /api/clients/:id/portal/regenerate",
		"DELETE /api/clients/:id/portal",
	} {
		assert.True(t, found[want], "expected route %s", want)
	}
}

// TestClientPortalRateLimited proves the public portal shares the abuse
// limiter: with a tiny budget the third anonymous hit gets 429 with the shared
// envelope.
func TestClientPortalRateLimited(t *testing.T) {
	t.Setenv("RATE_LIMIT_PUBLIC", "2")

	app := fiber.New()
	PublicRoutes(app)

	for i := 0; i < 2; i++ {
		resp := doRequest(t, app, "GET", "/api/public/client/any-token", "", nil)
		resp.Body.Close()
		require.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode)
	}
	resp := doRequest(t, app, "GET", "/api/public/client/any-token", "", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "rate limit exceeded, try again later", envelopeMessage(t, resp))
}
