package routes

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/platform/database"
)

// TestAdminUserDeleteFlow walks DELETE /admin/users/:id: an admin removes a
// plain account without invoice records (204, row gone, memberships gone,
// session revoked, audit entry), while self-delete, unknown targets, malformed
// ids and non-admin callers are refused with the stable codes. A user with any
// invoice record — even one — answers 409 and stays on the books.
func TestAdminUserDeleteFlow(t *testing.T) {
	t.Setenv("RATE_LIMIT_AUTH", "100")
	app := newTestApp()

	admin := adminSession(t, app, "Delete Admin", "delete-admin@example.com")
	adminID := userID(t, app, admin)
	target := registerUser(t, app, "delete-target@example.com", "secret123")
	targetID := userID(t, app, target)
	db, err := database.OpenDBConnection()
	require.NoError(t, err)

	// Delete the clean target: 204 with an empty body.
	resp := doRequest(t, app, "DELETE", "/api/admin/users/"+targetID, "", admin)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	// The row is gone and its memberships went with it.
	_, err = db.GetUserByID(uuid.MustParse(targetID))
	require.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrNoRows), "expected sql.ErrNoRows, got %v", err)
	memberships, err := db.CountByUser(uuid.MustParse(targetID))
	require.NoError(t, err)
	assert.Equal(t, int64(0), memberships)

	// The delete revoked the target's live session.
	resp = doRequest(t, app, "GET", "/api/auth/me", "", target)
	require.Equal(t, 401, resp.StatusCode)
	resp.Body.Close()

	// The delete is on the audit trail, attributed to the admin.
	logs, err := db.ListAuditLogs(50, 0)
	require.NoError(t, err)
	found := false
	for _, entry := range logs {
		if entry.Action == "user.delete" && entry.Entity == "user" && entry.EntityID == targetID {
			assert.Equal(t, uuid.MustParse(adminID), entry.UserID)
			found = true
			break
		}
	}
	assert.True(t, found, "user.delete audit entry not found")

	// An admin may not delete their own account.
	resp = doRequest(t, app, "DELETE", "/api/admin/users/"+adminID, "", admin)
	require.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, "you cannot delete your own account", envelopeMessage(t, resp))

	// A user with an invoice record is refused with 409 and stays.
	billed := registerUser(t, app, "delete-billed@example.com", "secret123")
	billedID := userID(t, app, billed)
	clientID := createClient(t, app, billed, "Billed Co")
	spec := newInvoice()
	spec.ClientID = clientID
	createInvoice(t, app, billed, spec)
	resp = doRequest(t, app, "DELETE", "/api/admin/users/"+billedID, "", admin)
	require.Equal(t, 409, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, "user has invoice records", body["error"].(map[string]any)["message"])
	details := body["error"].(map[string]any)["details"].(map[string]any)
	assert.Equal(t, float64(1), details["invoices"])
	_, err = db.GetUserByID(uuid.MustParse(billedID))
	require.NoError(t, err)

	// A malformed id is refused before any lookup.
	resp = doRequest(t, app, "DELETE", "/api/admin/users/not-a-uuid", "", admin)
	require.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, "invalid user id", envelopeMessage(t, resp))

	// A non-admin caller is refused by RequireRoles.
	resp = doRequest(t, app, "DELETE", "/api/admin/users/"+billedID, "", billed)
	require.Equal(t, 403, resp.StatusCode)
	resp.Body.Close()

	// An unknown target is a 404.
	resp = doRequest(t, app, "DELETE", "/api/admin/users/"+uuid.NewString(), "", admin)
	require.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "user not found", envelopeMessage(t, resp))
}
