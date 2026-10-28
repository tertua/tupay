package routes

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/relay"
)

// testSealKey is a deterministic base64 32-byte key for SECRET_ENCRYPTION_KEY.
func testSealKey(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
}

// TestGatewaySecretSealedAtRest pins the storage contract: the raw webhook
// secret is revealed once (create / rotate) and only the AES-GCM sealed form
// reaches the row, while list and update never echo a secret at all.
func TestGatewaySecretSealedAtRest(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", testSealKey(t))
	app := newTestApp()
	admin := adminSession(t, app, "Seal Admin", "seal-admin@example.com")
	slug := "seal-" + uuid.NewString()

	resp := doRequest(t, app, "POST", "/api/admin/gateway/projects",
		`{"slug":"`+slug+`","name":"Seal","webhook_url":"https://seal.example/hook"}`, admin)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	raw, _ := decodeBody(t, resp)["project"].(map[string]interface{})["webhook_secret"].(string)
	require.True(t, strings.HasPrefix(raw, "whsec_"), "create must reveal the secret once")

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	project, err := db.GetProjectBySlug(slug)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(project.WebhookSecret, "v1:"), "stored secret must be sealed")
	assert.NotContains(t, project.WebhookSecret, raw, "the raw secret must not survive in the row")

	opened, err := relay.ProjectSecret(project.WebhookSecret)
	require.NoError(t, err)
	assert.Equal(t, raw, opened, "the sealed value must open back to the revealed secret")

	// Rotation mints a new secret and re-seals it.
	resp = doRequest(t, app, "POST", "/api/admin/gateway/projects/"+slug+"/rotate-secret", "", admin)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	rotated, _ := decodeBody(t, resp)["project"].(map[string]interface{})["webhook_secret"].(string)
	require.NotEmpty(t, rotated)
	assert.NotEqual(t, raw, rotated)

	project, err = db.GetProjectBySlug(slug)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(project.WebhookSecret, "v1:"))
	opened, err = relay.ProjectSecret(project.WebhookSecret)
	require.NoError(t, err)
	assert.Equal(t, rotated, opened)

	// Reading the row back (list/update) must never reveal anything usable.
	resp = doRequest(t, app, "PATCH", "/api/admin/gateway/projects/"+slug, `{"name":"Seal Renamed"}`, admin)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, drainBody(t, resp), "webhook_secret")

	resp = doRequest(t, app, "GET", "/api/admin/gateway/projects", "", admin)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, drainBody(t, resp), "webhook_secret")
}

// TestGatewaySecretUpgradesLegacyRow proves the migration path: a project
// created before the key existed keeps working, and the next admin update
// seals it without changing the secret the customer already holds.
func TestGatewaySecretUpgradesLegacyRow(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", "")
	app := newTestApp()
	admin := adminSession(t, app, "Legacy Admin", "legacy-seal-admin@example.com")
	slug := "legacy-seal-" + uuid.NewString()

	resp := doRequest(t, app, "POST", "/api/admin/gateway/projects",
		`{"slug":"`+slug+`","name":"Legacy","webhook_url":"https://legacy.example/hook"}`, admin)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	raw, _ := decodeBody(t, resp)["project"].(map[string]interface{})["webhook_secret"].(string)
	require.NotEmpty(t, raw)

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	project, err := db.GetProjectBySlug(slug)
	require.NoError(t, err)
	assert.Equal(t, raw, project.WebhookSecret, "without a key the row stays plaintext (dev only)")

	// Enabling the key and saving the row seals the legacy value in place.
	t.Setenv("SECRET_ENCRYPTION_KEY", testSealKey(t))
	resp = doRequest(t, app, "PATCH", "/api/admin/gateway/projects/"+slug, `{"name":"Legacy Sealed"}`, admin)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	project, err = db.GetProjectBySlug(slug)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(project.WebhookSecret, "v1:"), "update must seal a legacy row")
	opened, err := relay.ProjectSecret(project.WebhookSecret)
	require.NoError(t, err)
	assert.Equal(t, raw, opened, "sealing on update must not rotate the secret")
}
