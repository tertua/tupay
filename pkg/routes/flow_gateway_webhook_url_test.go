package routes

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drainBody reads and closes a response body so it can be asserted as text.
func drainBody(t *testing.T, resp *http.Response) string {
	return string(mustRead(t, resp))
}

// TestGatewayProjectWebhookURLGuard pins the SSRF guard on project registration:
// the outbox worker POSTs to this URL, so a scheme-only, local-file or otherwise
// unusable target must be refused on both create and update.
func TestGatewayProjectWebhookURLGuard(t *testing.T) {
	app := newTestApp()
	admin := adminSession(t, app, "Webhook Guard", "webhook-guard@example.com")
	slug := "guard-" + uuid.NewString()

	for _, bad := range []string{
		"ftp://example.com/hook",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"not-a-url",
	} {
		resp := doRequest(t, app, "POST", "/api/admin/gateway/projects",
			`{"slug":"`+slug+`","name":"Guard","webhook_url":"`+bad+`"}`, admin)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "webhook_url %q must be rejected", bad)
		resp.Body.Close()
	}

	// The rejected attempts must not have registered the project.
	resp := doRequest(t, app, "GET", "/api/admin/gateway/projects", "", admin)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, drainBody(t, resp), slug)

	resp = doRequest(t, app, "POST", "/api/admin/gateway/projects",
		`{"slug":"`+slug+`","name":"Guard","webhook_url":"https://downstream.example/hook"}`, admin)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// An update cannot smuggle a bad target past the create-time check.
	resp = doRequest(t, app, "PATCH", "/api/admin/gateway/projects/"+slug,
		`{"webhook_url":"gopher://internal.example/"}`, admin)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	resp = doRequest(t, app, "GET", "/api/admin/gateway/projects", "", admin)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, drainBody(t, resp), "https://downstream.example/hook", "the stored target must survive a rejected update")
}
