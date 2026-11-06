package routes

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStepUpGuardsDestructiveRoutes pins the contract of the three guarded
// admin routes: refused without a token, refused with a spent token, and
// allowed with a fresh one.
func TestStepUpGuardsDestructiveRoutes(t *testing.T) {
	app := newTestApp()
	admin := adminSession(t, app, "Step Admin", uniqueEmail("stepup"))
	slug := "step-" + uuid.NewString()

	resp := doRequest(t, app, "POST", "/api/admin/gateway/projects",
		`{"slug":"`+slug+`","name":"Step","webhook_url":"https://step.example/hook"}`, admin)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	for _, route := range []string{"/rotate-key", "/rotate-secret"} {
		// No token: refused with the machine-readable code the MPA reacts to.
		resp = doStepUpRequest(t, app, "POST", "/api/admin/gateway/projects/"+slug+route, "", admin, "")
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		details := decodeBody(t, resp)["error"].(map[string]interface{})["details"].(map[string]interface{})
		assert.Equal(t, "step_up_required", details["code"])

		// A fresh token allows exactly one call.
		token := mintStepUp(t, app, stepUpBody("secret123"), admin)
		resp = doStepUpRequest(t, app, "POST", "/api/admin/gateway/projects/"+slug+route, "", admin, token)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// The same token is spent: a replay is refused.
		resp = doStepUpRequest(t, app, "POST", "/api/admin/gateway/projects/"+slug+route, "", admin, token)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		decodeBody(t, resp)
	}
}

// TestStepUpRejectsWrongPassword keeps a wrong confirmation from minting a token.
func TestStepUpRejectsWrongPassword(t *testing.T) {
	app := newTestApp()
	admin := adminSession(t, app, "Step Bad", uniqueEmail("stepbad"))

	resp := doRequest(t, app, "POST", "/api/auth/step-up", stepUpBody("wrongpass"), admin)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	details := decodeBody(t, resp)["error"].(map[string]interface{})["details"].(map[string]interface{})
	assert.Equal(t, "step_up_invalid", details["code"])
}

// TestStepUpRequiresTOTPWhenEnrolled proves an enrolled account cannot step up
// with the password alone.
func TestStepUpRequiresTOTPWhenEnrolled(t *testing.T) {
	app := newTestApp()
	email := uniqueEmail("steptotp")
	admin := adminSession(t, app, "Step Totp", email)
	secret := enrollTOTP(t, app, admin)

	// Password alone is not enough once a second factor exists.
	resp := doRequest(t, app, "POST", "/api/auth/step-up", stepUpBody("secret123"), admin)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	details := decodeBody(t, resp)["error"].(map[string]interface{})["details"].(map[string]interface{})
	assert.Equal(t, "totp_required", details["code"])

	// Password plus a live code mints the token.
	resp = doRequest(t, app, "POST", "/api/auth/step-up",
		`{"password":"secret123","code":"`+totpCodeIn(t, secret, 1)+`"}`, admin)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	token := decodeBody(t, resp)["step_up"].(map[string]interface{})["token"].(string)
	assert.NotEmpty(t, token)

	cleanupUser(t, email)
}
