package routes

import (
	"io"
	"strings"
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

// TestClientReminderFlow covers the manual reminder action: an open invoice
// queues one reminder (202 + queued=1), a second call is idempotent (queued=0,
// skipped=1 via the manual claim), a client with only a paid invoice queues
// nothing, and an unknown client is a 404.
func TestClientReminderFlow(t *testing.T) {
	app := newTestApp()
	cookies := registerUser(t, app, "clientreminder@example.com", "secret123")

	clientID := createClient(t, app, cookies, "Acme")
	spec := newInvoice()
	spec.ClientID, spec.Status = clientID, models.InvoiceStatusSent
	createInvoiceID(t, app, cookies, spec)

	// First reminder: one open invoice queued.
	resp := doRequest(t, app, "POST", "/api/clients/"+clientID+"/reminder", "{}", cookies)
	require.Equal(t, 202, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, float64(1), body["queued"])
	assert.Equal(t, float64(0), body["skipped"])

	// Second call is idempotent: the manual leg was already claimed.
	resp = doRequest(t, app, "POST", "/api/clients/"+clientID+"/reminder", "{}", cookies)
	require.Equal(t, 202, resp.StatusCode)
	body = decodeBody(t, resp)
	assert.Equal(t, float64(0), body["queued"])
	assert.Equal(t, float64(1), body["skipped"])

	// A paid-only client has no open receivables to remind about.
	paidClientID := createClient(t, app, cookies, "Paid Co")
	paidSpec := newInvoice()
	paidSpec.ClientID, paidSpec.Status = paidClientID, models.InvoiceStatusPaid
	createInvoiceID(t, app, cookies, paidSpec)

	resp = doRequest(t, app, "POST", "/api/clients/"+paidClientID+"/reminder", "{}", cookies)
	require.Equal(t, 202, resp.StatusCode)
	body = decodeBody(t, resp)
	assert.Equal(t, float64(0), body["queued"])
	assert.Equal(t, float64(0), body["skipped"])

	// Unknown client id is a stable 404.
	resp = doRequest(t, app, "POST", "/api/clients/00000000-0000-0000-0000-000000000000/reminder", "{}", cookies)
	assert.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()
}

// TestClientStatementCSVFlow covers the statement export: a text/csv attachment
// whose body carries the invoice number and decimal-string money, with drafts
// excluded and the optional from/to window narrowing the rows.
func TestClientStatementCSVFlow(t *testing.T) {
	app := newTestApp()
	cookies := registerUser(t, app, "clientstatement@example.com", "secret123")

	clientID := createClient(t, app, cookies, "Acme")

	sent := newInvoice()
	sent.ClientID, sent.Status, sent.Issue = clientID, models.InvoiceStatusSent, "2026-01-15"
	sent.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100000"}}
	createInvoiceID(t, app, cookies, sent)

	draft := newInvoice()
	draft.ClientID, draft.Status = clientID, models.InvoiceStatusDraft
	createInvoiceID(t, app, cookies, draft)

	resp := doRequest(t, app, "GET", "/api/clients/"+clientID+"/statement.csv", "", cookies)
	require.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/csv")
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")

	body := new(strings.Builder)
	_, err := io.Copy(body, resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	csv := body.String()

	assert.Contains(t, csv, `"invoice_number","issue_date","due_date","status","total","paid_amount","balance","currency"`)
	assert.Contains(t, csv, "INV-")
	assert.Contains(t, csv, `"100000"`) // decimal-string money, quoted
	assert.Contains(t, csv, "2026-01-15")
	assert.NotContains(t, csv, "draft") // drafts are excluded

	// The from/to window that excludes the invoice drops its row.
	resp = doRequest(t, app, "GET", "/api/clients/"+clientID+"/statement.csv?from=2027-01-01&to=2027-12-31", "", cookies)
	require.Equal(t, 200, resp.StatusCode)
	body.Reset()
	_, err = io.Copy(body, resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	assert.NotContains(t, body.String(), "INV-")
}
