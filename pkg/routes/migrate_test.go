package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/database"
)

// TestMigrateDownFlow covers the admin schema-rollback endpoint: step-up,
// validation, refusal, roles, and the round trip. The shared test database is
// restored to the current version at the end so later flow tests keep their
// columns.
func TestMigrateDownFlow(t *testing.T) {
	defer func() {
		require.NoError(t, database.Migrate())
	}()

	app := newTestApp()
	adminCookies := adminSession(t, app, "Mig Admin", "migadmin@example.com")

	// The route is step-up guarded: no token is refused before any parsing.
	resp := doStepUpRequest(t, app, "POST", "/api/admin/migrate/down",
		`{"target_version":1,"confirm":false}`, adminCookies, "")
	assert.Equal(t, 401, resp.StatusCode)
	decodeBody(t, resp)

	// Confirmation is mandatory.
	resp = doStepUpRequest(t, app, "POST", "/api/admin/migrate/down",
		`{"target_version":1,"confirm":false}`, adminCookies, mintStepUp(t, app, stepUpBody("secret123"), adminCookies))
	assert.Equal(t, 400, resp.StatusCode)
	decodeBody(t, resp)

	// Non-admins are rejected by roles, not version logic — before rollback, since v1 lacks new columns.
	userCookies := plainUserSession(t, app, "Mig User", "miguser@example.com")
	resp = doRequest(t, app, "POST", "/api/admin/migrate/down",
		`{"target_version":1,"confirm":true}`, userCookies)
	assert.Equal(t, 403, resp.StatusCode)
	decodeBody(t, resp)

	// Roll back to v1. Both tokens are minted first: a rollback to v1 drops the
	// columns the confirmation reads, so no token can be minted afterwards.
	rollbackToken := mintStepUp(t, app, stepUpBody("secret123"), adminCookies)
	retryToken := mintStepUp(t, app, stepUpBody("secret123"), adminCookies)
	resp = doStepUpRequest(t, app, "POST", "/api/admin/migrate/down",
		`{"target_version":1,"confirm":true}`, adminCookies, rollbackToken)
	assert.Equal(t, 200, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, float64(1), body["version"])
	stamp, err := database.CurrentSchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, 1, stamp)

	// Same target again is refused.
	resp = doStepUpRequest(t, app, "POST", "/api/admin/migrate/down",
		`{"target_version":1,"confirm":true}`, adminCookies, retryToken)
	assert.Equal(t, 400, resp.StatusCode)
	decodeBody(t, resp)

	// The suite shares one database: remove our accounts so user counts
	// in other flow tests stay exact.
	q, err := database.OpenDBConnection()
	require.NoError(t, err)
	h := q.InvoiceQueries.DB
	require.NoError(t, h.Where("email IN ?", []string{"migadmin@example.com", "miguser@example.com"}).Delete(&models.User{}).Error)
}
