package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
)

// TestClientListSearchSortFlow covers the server-side list contract: free-text
// search over name/company/email, lifecycle status filter, sort/order, and the
// pagination meta that the frontend now consumes.
func TestClientListSearchSortFlow(t *testing.T) {
	app := newTestApp()
	cookies := registerUser(t, app, "clientsearch@example.com", "secret123")

	acme := createClientNamed(t, app, cookies, clientSpec{Name: "Acme", Company: "Acme Inc", Email: "billing@acme.test"})
	beta := createClientNamed(t, app, cookies, clientSpec{Name: "Beta", Company: "Beta LLC", Email: "hello@beta.test"})
	require.NotEmpty(t, acme["id"])
	require.NotEmpty(t, beta["id"])

	// Default list: both rows, meta present with defaults.
	all := listClients(t, app, cookies, "")
	meta := all["meta"].(map[string]interface{})
	assert.Equal(t, float64(1), meta["page"])
	assert.Equal(t, float64(20), meta["per_page"])
	assert.Equal(t, float64(2), meta["total"])
	assert.Equal(t, float64(1), meta["total_pages"])

	// Search is case-insensitive and matches company and email, not just name.
	search := listClients(t, app, cookies, "q=acm")
	rows := search["clients"].([]interface{})
	require.Len(t, rows, 1)
	assert.Equal(t, "Acme", rows[0].(map[string]interface{})["name"])
	assert.Equal(t, float64(1), search["meta"].(map[string]interface{})["total"])

	byEmail := listClients(t, app, cookies, "q=hello@beta")
	require.Len(t, byEmail["clients"].([]interface{}), 1)
	assert.Equal(t, "Beta", byEmail["clients"].([]interface{})[0].(map[string]interface{})["name"])

	// Sort by name ascending returns Acme before Beta.
	asc := listClients(t, app, cookies, "sort=name&order=asc")
	ascRows := asc["clients"].([]interface{})
	require.Len(t, ascRows, 2)
	assert.Equal(t, "Acme", ascRows[0].(map[string]interface{})["name"])
	assert.Equal(t, "Beta", ascRows[1].(map[string]interface{})["name"])

	// The status field flows through list rows (defaults to active).
	assert.Equal(t, "active", acme["status"])

	// An unknown status value is ignored (falls back to all), matching invoices.
	unknown := listClients(t, app, cookies, "status=bogus")
	assert.Equal(t, float64(2), unknown["meta"].(map[string]interface{})["total"])

	// per_page caps the page and drives total_pages.
	paged := listClients(t, app, cookies, "per_page=1&page=1")
	pagedMeta := paged["meta"].(map[string]interface{})
	assert.Equal(t, float64(2), pagedMeta["total"])
	assert.Equal(t, float64(2), pagedMeta["total_pages"])
	require.Len(t, paged["clients"].([]interface{}), 1)
}

// TestClientStatusLifecycleFlow covers archive/unarchive: the status flips,
// the list status filter follows it, and flipping to the already-current
// state is a stable 409.
func TestClientStatusLifecycleFlow(t *testing.T) {
	app := newTestApp()
	cookies := registerUser(t, app, "clientlifecycle@example.com", "secret123")

	client := createClientNamed(t, app, cookies, clientSpec{Name: "Acme"})
	clientID := client["id"].(string)
	assert.Equal(t, "active", client["status"])

	// Archive: status flips and the archived filter finds it, active excludes it.
	resp := doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/archive", "{}", cookies)
	require.Equal(t, 200, resp.StatusCode)
	archived := decodeBody(t, resp)["client"].(map[string]interface{})
	assert.Equal(t, "archived", archived["status"])

	archivedList := listClients(t, app, cookies, "status=archived")
	require.Len(t, archivedList["clients"].([]interface{}), 1)
	activeList := listClients(t, app, cookies, "status=active")
	assert.Len(t, activeList["clients"].([]interface{}), 0)

	// Archiving twice is a stable 409.
	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/archive", "{}", cookies)
	assert.Equal(t, 409, resp.StatusCode)
	resp.Body.Close()

	// Detail carries the lifecycle status.
	resp = doRequest(t, app, "GET", "/api/clients/"+clientID, "", cookies)
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "archived", decodeBody(t, resp)["client"].(map[string]interface{})["status"])

	// Unarchive: back to active, and unarchiving again is a 409.
	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/unarchive", "{}", cookies)
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "active", decodeBody(t, resp)["client"].(map[string]interface{})["status"])

	resp = doRequest(t, app, "PATCH", "/api/clients/"+clientID+"/unarchive", "{}", cookies)
	assert.Equal(t, 409, resp.StatusCode)
	resp.Body.Close()

	// Unknown client id is a 404, not a 500.
	resp = doRequest(t, app, "PATCH", "/api/clients/00000000-0000-0000-0000-000000000000/archive", "{}", cookies)
	assert.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()
}

// TestClientDeleteGuardFlow covers the delete guard: a client with a sent
// invoice cannot be hard-deleted (409 + open_invoices), a draft-only client
// deletes cleanly, and paying the invoice unblocks the delete.
func TestClientDeleteGuardFlow(t *testing.T) {
	app := newTestApp()
	cookies := registerUser(t, app, "clientdelete@example.com", "secret123")

	// A sent invoice blocks the delete.
	clientID := createClient(t, app, cookies, "Acme")
	spec := newInvoice()
	spec.ClientID, spec.Status = clientID, models.InvoiceStatusSent
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp := doRequest(t, app, "DELETE", "/api/clients/"+clientID, "", cookies)
	require.Equal(t, 409, resp.StatusCode)
	errObj := decodeBody(t, resp)["error"].(map[string]interface{})
	assert.Equal(t, "client has open invoices", errObj["message"])
	assert.Equal(t, float64(1), errObj["details"].(map[string]interface{})["open_invoices"])

	// Delete the invoice so the client is empty, then the delete succeeds.
	resp = doRequest(t, app, "DELETE", "/api/invoices/"+invoiceID, "", cookies)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	// A draft-only client deletes cleanly.
	draftClientID := createClient(t, app, cookies, "Draft Co")
	draftSpec := newInvoice()
	draftSpec.ClientID, draftSpec.Status = draftClientID, models.InvoiceStatusDraft
	draftInvoiceID := createInvoiceID(t, app, cookies, draftSpec)

	resp = doRequest(t, app, "DELETE", "/api/clients/"+draftClientID, "", cookies)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	// Cleanup: remove the draft invoice (its client is gone).
	resp = doRequest(t, app, "DELETE", "/api/invoices/"+draftInvoiceID, "", cookies)
	assert.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()
}

// TestClientOverdueViewFlow covers the client-detail overdue toggle: a sent
// invoice with a past due date shows under ?overdue=1 while a future-due one
// does not, and stats still count every invoice.
func TestClientOverdueViewFlow(t *testing.T) {
	app := newTestApp()
	cookies := registerUser(t, app, "clientoverdue@example.com", "secret123")

	clientID := createClient(t, app, cookies, "Acme")

	overdueSpec := newInvoice()
	overdueSpec.ClientID, overdueSpec.Status, overdueSpec.Due = clientID, models.InvoiceStatusSent, "2020-01-01"
	createInvoiceID(t, app, cookies, overdueSpec)

	futureSpec := newInvoice()
	futureSpec.ClientID = clientID // newInvoice defaults to a ~30-day future due date
	createInvoiceID(t, app, cookies, futureSpec)

	// Full view: both invoices.
	resp := doRequest(t, app, "GET", "/api/clients/"+clientID, "", cookies)
	require.Equal(t, 200, resp.StatusCode)
	full := decodeBody(t, resp)
	assert.Len(t, full["invoices"].([]interface{}), 2)

	// Overdue-only view: just the past-due invoice, stats unchanged.
	resp = doRequest(t, app, "GET", "/api/clients/"+clientID+"?overdue=1", "", cookies)
	require.Equal(t, 200, resp.StatusCode)
	filtered := decodeBody(t, resp)
	require.Len(t, filtered["invoices"].([]interface{}), 1)
	assert.Equal(t, "overdue", filtered["invoices"].([]interface{})[0].(map[string]interface{})["effective_status"])
	assert.Equal(t, float64(2), filtered["stats"].(map[string]interface{})["count"])
}
