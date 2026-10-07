package routes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustMarshal renders a decoded JSON body back to a string so a test can
// assert absence of a substring across the whole payload.
func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// TestClientPortalFlow walks the whole public portal surface: an owner mints a
// link, the public read returns only that client's non-draft invoices with
// balance and payment history, no provider name and no client email, an
// unknown token 404s, and revoke kills the link immediately.
func TestClientPortalFlow(t *testing.T) {
	t.Setenv("RATE_LIMIT_PUBLIC", "1000")
	app := newTestApp()

	owner := registerUser(t, app, "portal-owner@example.com", "secret123")
	clientID := createClient(t, app, owner, "Portal Client")

	// One sent invoice and one draft: only the sent one may surface.
	sent := newInvoice()
	sent.ClientID, sent.Currency = clientID, "USD"
	sent.Items = []invoiceLine{{Description: "Design", Quantity: 1, Rate: "100"}}
	sentID := createInvoiceID(t, app, owner, sent)

	draft := newInvoice()
	draft.ClientID, draft.Status = clientID, "draft"
	draft.Items = []invoiceLine{{Description: "Hidden", Quantity: 1, Rate: "999"}}
	createInvoiceID(t, app, owner, draft)

	// Record a payment so the portal has history.
	resp := doRequest(t, app, "POST", "/api/payments", `{"invoiceId":"`+sentID+`","amount":40,"method":"Cash","paid_on":"2026-09-21"}`, owner)
	require.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()

	// Mint the portal link (owner-only).
	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 200, resp.StatusCode)
	portal := decodeBody(t, resp)["client_portal"].(map[string]interface{})
	token := portal["token"].(string)
	require.NotEmpty(t, token)
	assert.Equal(t, "/client/"+token, portal["url_path"])

	// Public read: only the non-draft invoice, with balance and history.
	resp = doRequest(t, app, "GET", "/api/public/client/"+token, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	body := decodeBody(t, resp)
	client := body["client"].(map[string]interface{})
	assert.Equal(t, "Portal Client", client["name"])
	assert.NotContains(t, client, "email")
	assert.NotContains(t, client, "notes")
	assert.NotContains(t, client, "org_id")

	invoices := body["invoices"].([]interface{})
	require.Len(t, invoices, 1, "draft must not be exposed")
	row := invoices[0].(map[string]interface{})
	assert.Equal(t, sentID, row["id"])
	assert.Equal(t, "sent", row["effective_status"])
	assert.Equal(t, "60", row["balance"])
	assert.NotEmpty(t, row["due_date"])

	payments := body["payments"].([]interface{})
	require.Len(t, payments, 1)
	pay := payments[0].(map[string]interface{})
	assert.Equal(t, "40", pay["amount"])
	assert.Equal(t, "Cash", pay["method"])
	assert.Equal(t, "2026-09-21", pay["paid_on"])

	// No provider name anywhere in the payload.
	raw := mustMarshal(t, body)
	for _, needle := range []string{"midtrans", "nowpayments", "snap_token", "provider_token"} {
		assert.NotContains(t, raw, needle)
	}

	// An unknown token 404s with the stable message.
	resp = doRequest(t, app, "GET", "/api/public/client/unknown-token", "", nil)
	require.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "client link not found", envelopeMessage(t, resp))

	// Revoke kills the same token immediately.
	resp = doRequest(t, app, "DELETE", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	resp = doRequest(t, app, "GET", "/api/public/client/"+token, "", nil)
	require.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "client link not found", envelopeMessage(t, resp))
}

// TestClientPortalPayReuse proves the portal's public_pay_token drives the
// untouched /pay flow: mint an online link, read it back through the portal,
// then hit /public/pay with the same token.
func TestClientPortalPayReuse(t *testing.T) {
	t.Setenv("RATE_LIMIT_PUBLIC", "1000")
	app := newTestApp()

	owner := registerUser(t, app, "portal-pay@example.com", "secret123")
	clientID := createClient(t, app, owner, "Pay Client")
	spec := newInvoice()
	spec.ClientID = clientID
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100"}}
	invoiceID := createInvoiceID(t, app, owner, spec)

	// Create the invoice's own public pay link.
	resp := doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, owner)
	require.Equal(t, 200, resp.StatusCode)
	payToken := decodeBody(t, resp)["token"].(string)

	// The portal surfaces that same token on the invoice row.
	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/portal", "", owner)
	require.Equal(t, 200, resp.StatusCode)
	portalToken := decodeBody(t, resp)["client_portal"].(map[string]interface{})["token"].(string)

	resp = doRequest(t, app, "GET", "/api/public/client/"+portalToken, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	row := decodeBody(t, resp)["invoices"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, payToken, row["public_pay_token"])

	// That token drives the existing public pay page.
	resp = doRequest(t, app, "GET", "/api/public/pay/"+payToken, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	assert.True(t, decodeBody(t, resp)["can_pay"].(bool))
}
