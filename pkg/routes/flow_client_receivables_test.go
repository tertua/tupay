package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
