package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
)

// invoiceLine is one line item in an invoice spec.
type invoiceLine struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Rate        string  `json:"rate"`
}

// invoiceSpec describes the fields a flow test cares about; everything else
// falls back to newInvoice()'s defaults. Add fields here as tests need them
// rather than hand-writing JSON at each call site.
//
// ClientID is deliberately explicit: a test that bills someone calls
// createClient() first (see the helper below) so it also holds the id for
// later assertions. There is no implicit client creation — that ambiguity is
// how fixtures rot.
type invoiceSpec struct {
	Status        string        `json:"status"`
	Currency      string        `json:"currency"`
	ClientID      string        `json:"client_id,omitempty"`
	Issue         string        `json:"issue_date"`
	Due           string        `json:"due_date"`
	TaxRate       float64       `json:"tax_rate,omitempty"`
	Discount      string        `json:"discount,omitempty"`
	PaymentMethod string        `json:"payment_method,omitempty"`
	Items         []invoiceLine `json:"items"`
}

// newInvoice returns a sent IDR invoice with one line; Due is always ~30 days out so the default fixture never flips to overdue during a run.
// Tests override only the fields they exercise.
func newInvoice() invoiceSpec {
	return invoiceSpec{
		Status:   models.InvoiceStatusSent,
		Currency: "IDR",
		Issue:    "2026-09-01",
		Due:      time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02"),
		Items:    []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100000"}},
	}
}

// body renders the spec as the JSON request body.
func (s invoiceSpec) body(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	return string(raw)
}

// createInvoice posts the spec and returns the decoded "invoice" object.
// It fails the test on any non-201 response, so callers read as setup.
func createInvoice(t *testing.T, app *fiber.App, cookies []*http.Cookie, spec invoiceSpec) map[string]interface{} {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/invoices", spec.body(t), cookies)
	require.Equal(t, 201, resp.StatusCode, "createInvoice: %s", spec.body(t))
	return decodeBody(t, resp)["invoice"].(map[string]interface{})
}

// createInvoiceID is createInvoice for the common case where only the id matters.
func createInvoiceID(t *testing.T, app *fiber.App, cookies []*http.Cookie, spec invoiceSpec) string {
	t.Helper()
	return createInvoice(t, app, cookies, spec)["id"].(string)
}

// createClient makes a client and returns its id. Sent invoices must name a
// client, so flow tests that bill someone need this fixture.
func createClient(t *testing.T, app *fiber.App, cookies []*http.Cookie, name string) string {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/clients", `{"name":"`+name+`"}`, cookies)
	require.Equal(t, 201, resp.StatusCode)
	return decodeBody(t, resp)["client"].(map[string]interface{})["id"].(string)
}

// clientSpec describes the fields a client flow test cares about; empty
// fields are omitted from the payload so search/sort tests can set exactly
// the name/company/email they exercise.
type clientSpec struct {
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Company string `json:"company,omitempty"`
}

// clientBody renders the spec as the JSON request body.
func (s clientSpec) body(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	return string(raw)
}

// createClientNamed creates a client from a spec and returns the decoded
// "client" object so callers can assert on status and aggregates.
func createClientNamed(t *testing.T, app *fiber.App, cookies []*http.Cookie, spec clientSpec) map[string]interface{} {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/clients", spec.body(t), cookies)
	require.Equal(t, 201, resp.StatusCode, "createClientNamed: %s", spec.body(t))
	return decodeBody(t, resp)["client"].(map[string]interface{})
}

// listClients GETs /api/clients with a raw query string and returns the
// decoded data object ({clients, meta}); query is appended verbatim when set.
func listClients(t *testing.T, app *fiber.App, cookies []*http.Cookie, query string) map[string]interface{} {
	t.Helper()
	path := "/api/clients"
	if query != "" {
		path += "?" + query
	}
	resp := doRequest(t, app, "GET", path, "", cookies)
	require.Equal(t, 200, resp.StatusCode, "listClients: %s", query)
	return decodeBody(t, resp)
}

// registerUser registers an account (201, or 409 when the email is taken) and logs in, returning the session cookies; doRequest echoes the CSRF cookie as its header.
func registerUser(t *testing.T, app *fiber.App, email, password string) []*http.Cookie {
	t.Helper()
	name := strings.SplitN(email, "@", 2)[0]
	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"`+name+`","email":"`+email+`","password":"`+password+`"}`, nil)
	require.True(t, resp.StatusCode == 201 || resp.StatusCode == 409, "register %s: %d", email, resp.StatusCode)
	resp.Body.Close()
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`, nil)
	require.Equal(t, 200, resp.StatusCode, "login %s", email)
	cookies := resp.Cookies()
	resp.Body.Close()
	return cookies
}

// inviteAndAccept has the owner mint an invite link and the invitee redeem it, putting invitee in the owner's org (the routes carry CSRF via the cookies).
func inviteAndAccept(t *testing.T, app *fiber.App, owner, invitee []*http.Cookie) {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/orgs/invites", `{}`, owner)
	require.Equal(t, 201, resp.StatusCode, "create invite")
	token := decodeBody(t, resp)["invite"].(map[string]interface{})["token"].(string)
	resp = doRequest(t, app, "POST", "/api/orgs/invites/accept", `{"token":"`+token+`"}`, invitee)
	require.Equal(t, 200, resp.StatusCode, "accept invite")
	resp.Body.Close()
}

// inviteToken mints an invite as the owner and hands back the raw token without redeeming it, so expiry/revoke tests can drive the accept call themselves.
func inviteToken(t *testing.T, app *fiber.App, owner []*http.Cookie) string {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/orgs/invites", `{}`, owner)
	require.Equal(t, 201, resp.StatusCode, "create invite")
	return decodeBody(t, resp)["invite"].(map[string]interface{})["token"].(string)
}

// myOrgID reads GET /api/orgs/me and returns the session's active organization id.
func myOrgID(t *testing.T, app *fiber.App, cookies []*http.Cookie) string {
	t.Helper()
	resp := doRequest(t, app, "GET", "/api/orgs/me", "", cookies)
	require.Equal(t, 200, resp.StatusCode, "orgs/me")
	return decodeBody(t, resp)["org"].(map[string]interface{})["id"].(string)
}
