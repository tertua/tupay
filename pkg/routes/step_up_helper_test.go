package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// StepUpHeaderName mirrors the middleware header the MPA echoes back.
const StepUpHeaderName = "X-Step-Up-Token"

// doStepUpRequest posts to /api/auth/step-up and returns the minted token.
func mintStepUp(t *testing.T, app *fiber.App, body string, cookies []*http.Cookie) string {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/auth/step-up", body, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["step_up"].(map[string]interface{})["token"].(string)
	require.NotEmpty(t, token)
	return token
}

// doStepUpRequest performs a request carrying a step-up token (empty token
// sends none, which is how the refusal path is exercised).
func doStepUpRequest(t *testing.T, app *fiber.App, method, route, body string, cookies []*http.Cookie, token string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, route, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(StepUpHeaderName, token)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if cookie.Name == "csrf_token" && cookie.Value != "" {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	require.NoError(t, err)
	return resp
}

// stepUpBody builds the confirmation payload for a password-only account.
func stepUpBody(password string) string {
	return `{"password":"` + password + `"}`
}

// plainUserSession registers (or reuses) a non-admin account and returns its
// session, so a role-guard test does not need to inline the register+login pair.
func plainUserSession(t *testing.T, app *fiber.App, name, email string) []*http.Cookie {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"`+name+`","email":"`+email+`","password":"secret123"}`, nil)
	require.True(t, resp.StatusCode == 201 || resp.StatusCode == 409)
	resp.Body.Close()
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"secret123"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	cookies := resp.Cookies()
	resp.Body.Close()
	return cookies
}
