package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientPortalOwnerGuard proves the portal mutations are owner-only: a
// non-owner member gets 403 org.ownerRequired on every route.
func TestClientPortalOwnerGuard(t *testing.T) {
	t.Setenv("RATE_LIMIT_AUTH", "1000")
	app := newTestApp()

	owner := registerUser(t, app, "portal-guard-owner@example.com", "secret123")
	staff := registerUser(t, app, "portal-guard-staff@example.com", "secret123")
	inviteAndAccept(t, app, owner, staff)
	clientID := createClient(t, app, owner, "Guarded Client")

	for _, tc := range []struct{ method, path string }{
		{"PATCH", "/api/clients/" + clientID + "/portal"},
		{"POST", "/api/clients/" + clientID + "/portal/regenerate"},
		{"DELETE", "/api/clients/" + clientID + "/portal"},
	} {
		resp := doRequest(t, app, tc.method, tc.path, "", staff)
		require.Equal(t, 403, resp.StatusCode, tc.path)
		assert.Equal(t, "org.ownerRequired", envelopeMessage(t, resp), tc.path)
	}
}

// TestClientPortalRegenerateAndRevoke proves regenerate swaps the token (old
// dies, new lives) and revoke is idempotent.
func TestClientPortalRegenerateAndRevoke(t *testing.T) {
	t.Setenv("RATE_LIMIT_PUBLIC", "1000")
	app := newTestApp()

	owner := registerUser(t, app, "portal-regen@example.com", "secret123")
	clientID := createClient(t, app, owner, "Regen Client")

	// Mint.
	resp := doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 200, resp.StatusCode)
	first := decodeBody(t, resp)["client_portal"].(map[string]interface{})["token"].(string)
	require.NotEmpty(t, first)

	// A second ensure is idempotent: same link, token not re-exposed.
	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 200, resp.StatusCode)
	again := decodeBody(t, resp)["client_portal"].(map[string]interface{})
	assert.Empty(t, again["token"], "an existing link never re-exposes its token")
	assert.Equal(t, true, again["active"])

	// Regenerate returns a new token and kills the old one.
	resp = doRequest(t, app, "POST", "/api/clients/"+clientID+"/portal/regenerate", "", owner)
	require.Equal(t, 200, resp.StatusCode)
	second := decodeBody(t, resp)["client_portal"].(map[string]interface{})["token"].(string)
	require.NotEmpty(t, second)
	assert.NotEqual(t, first, second)

	resp = doRequest(t, app, "GET", "/api/public/client/"+first, "", nil)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()
	resp = doRequest(t, app, "GET", "/api/public/client/"+second, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	// Revoke twice is fine (idempotent).
	resp = doRequest(t, app, "DELETE", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()
	resp = doRequest(t, app, "DELETE", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	resp = doRequest(t, app, "GET", "/api/public/client/"+second, "", nil)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()
}

// TestClientPortalOrgIsolation proves a portal token from one org never
// returns another org's rows: the link is scoped by its own OrgID.
func TestClientPortalOrgIsolation(t *testing.T) {
	t.Setenv("RATE_LIMIT_PUBLIC", "1000")
	app := newTestApp()

	ownerA := registerUser(t, app, "portal-orga@example.com", "secret123")
	ownerB := registerUser(t, app, "portal-orgb@example.com", "secret123")

	clientA := createClient(t, app, ownerA, "Org A Client")
	spec := newInvoice()
	spec.ClientID = clientA
	spec.Items = []invoiceLine{{Description: "A only", Quantity: 1, Rate: "100"}}
	createInvoiceID(t, app, ownerA, spec)

	// Org B has its own client + invoice.
	clientB := createClient(t, app, ownerB, "Org B Client")
	specB := newInvoice()
	specB.ClientID = clientB
	specB.Items = []invoiceLine{{Description: "B only", Quantity: 1, Rate: "200"}}
	createInvoiceID(t, app, ownerB, specB)

	// Org B cannot ensure a portal for org A's client (404 client, not owner).
	resp := doRequest(t, app, "PATCH", "/api/clients/"+clientA+"/portal", "", ownerB)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()

	// Org A's portal returns only org A's invoice.
	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientA+"/portal", "", ownerA)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["client_portal"].(map[string]interface{})["token"].(string)

	resp = doRequest(t, app, "GET", "/api/public/client/"+token, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	invoices := decodeBody(t, resp)["invoices"].([]interface{})
	require.Len(t, invoices, 1, "only org A's single invoice is visible")
}
