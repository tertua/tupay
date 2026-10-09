package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
)

// sendEmailTestApp wires SendInvoiceEmail behind a stub that mimics the
// OrgContext middleware (the route's owner gate lives in pkg/routes).
func sendEmailTestApp(userID, orgID uuid.UUID, role string) *fiber.App {
	withOrg := func(c fiber.Ctx) error {
		c.Locals(utils.SessionUserIDKey, userID)
		c.Locals(utils.SessionOrgIDKey, orgID)
		c.Locals(utils.SessionOrgRoleKey, role)
		return c.Next()
	}
	app := fiber.New()
	app.Post("/invoices/:id/send-email", withOrg, SendInvoiceEmail)
	return app
}

// sendEmail performs a POST and returns status + decoded JSON body.
func sendEmail(t *testing.T, app *fiber.App, route string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(""))
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	require.NoError(t, err)
	defer resp.Body.Close()
	body := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// seedClientWithEmail inserts a client with an explicit email and returns its id.
func seedClientWithEmail(t *testing.T, orgID, userID uuid.UUID, name, email string) uuid.UUID {
	t.Helper()
	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	client := models.Client{ID: uuid.New(), UserID: userID, OrgID: orgID, Name: name, Email: email}
	require.NoError(t, db.CreateClient(&client))
	return client.ID
}

// TestInvoiceRecipientResolvesClientEmail proves the happy path returns the client address.
func TestInvoiceRecipientResolvesClientEmail(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-recipient@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email Recipient Org")
	clientID := seedClientWithEmail(t, orgID, userID, "Billing Co", "billing@acme.test")

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	inv := models.Invoice{ID: uuid.New(), OrgID: orgID, ClientID: &clientID}

	to, status, msg, details := invoiceRecipient(*db, orgID, inv)
	assert.Equal(t, "billing@acme.test", to)
	assert.Equal(t, 0, status, msg)
	assert.Nil(t, details)
}

// TestInvoiceRecipientRejectsMissingEmail proves a client without an address is a stable 409.
func TestInvoiceRecipientRejectsMissingEmail(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-nomail@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email NoMail Org")
	clientID := seedClientWithEmail(t, orgID, userID, "No Mail Co", "")

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	inv := models.Invoice{ID: uuid.New(), OrgID: orgID, ClientID: &clientID}

	to, status, msg, details := invoiceRecipient(*db, orgID, inv)
	assert.Equal(t, "", to)
	assert.Equal(t, fiber.StatusConflict, status)
	assert.Equal(t, "client has no email", msg)
	gotID, ok := details["client_id"].(*uuid.UUID)
	require.True(t, ok, "client_id should carry the offending client id")
	assert.Equal(t, clientID, *gotID)
}

// TestInvoiceRecipientRejectsNilClient proves an invoice with no client is a 409 with a null client id.
func TestInvoiceRecipientRejectsNilClient(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-noclient@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email NoClient Org")

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	inv := models.Invoice{ID: uuid.New(), OrgID: orgID}

	to, status, msg, details := invoiceRecipient(*db, orgID, inv)
	assert.Equal(t, "", to)
	assert.Equal(t, fiber.StatusConflict, status)
	assert.Equal(t, "client has no email", msg)
	assert.Nil(t, details["client_id"])
}

// TestInvoiceRecipientUnknownClient proves a vanished client maps to 404.
func TestInvoiceRecipientUnknownClient(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-unknown@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email Unknown Org")

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	missing := uuid.New()
	inv := models.Invoice{ID: uuid.New(), OrgID: orgID, ClientID: &missing}

	_, status, msg, _ := invoiceRecipient(*db, orgID, inv)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, "client not found", msg)
}

// TestInvoiceEmailURLFallsBackToRoot proves the CTA uses the app root when the invoice has no pay link.
func TestInvoiceEmailURLFallsBackToRoot(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-url@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email URL Org")

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	inv := models.Invoice{ID: uuid.New(), OrgID: orgID}

	assert.Equal(t, publicURL("/"), invoiceEmailURL(*db, orgID, inv))
}

// TestSendInvoiceEmailDraftConflict proves a draft invoice is a 409 before any mail work.
func TestSendInvoiceEmailDraftConflict(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-draft@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email Draft Org")
	invoiceID := approvalSeedInvoice(t, orgID, userID, models.InvoiceStatusDraft, true)

	app := sendEmailTestApp(userID, orgID, models.RoleOwner)
	status, body := sendEmail(t, app, "/invoices/"+invoiceID.String()+"/send-email")
	assert.Equal(t, fiber.StatusConflict, status, body)
	assert.Equal(t, "invoice is still a draft", approvalErrMessage(body))
}

// TestSendInvoiceEmailNoMailConflict proves a sent invoice for a client without an email is a 409.
func TestSendInvoiceEmailNoMailConflict(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-client-nomail@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email Client NoMail Org")
	invoiceID := approvalSeedInvoice(t, orgID, userID, models.InvoiceStatusSent, true)

	app := sendEmailTestApp(userID, orgID, models.RoleOwner)
	status, body := sendEmail(t, app, "/invoices/"+invoiceID.String()+"/send-email")
	assert.Equal(t, fiber.StatusConflict, status, body)
	assert.Equal(t, "client has no email", approvalErrMessage(body))
}

// TestSendInvoiceEmailGuards proves 401 without org, 400 on a bad uuid, 404 on an unknown invoice.
func TestSendInvoiceEmailGuards(t *testing.T) {
	userID := approvalSeedUser(t, "send-email-guards@example.com")
	orgID := approvalSeedOrg(t, userID, "Send Email Guards Org")

	noOrg := sendEmailTestApp(userID, uuid.Nil, models.RoleOwner)
	status, body := sendEmail(t, noOrg, "/invoices/"+uuid.NewString()+"/send-email")
	assert.Equal(t, fiber.StatusUnauthorized, status, body)

	app := sendEmailTestApp(userID, orgID, models.RoleOwner)
	status, body = sendEmail(t, app, "/invoices/not-a-uuid/send-email")
	assert.Equal(t, fiber.StatusBadRequest, status, body)

	status, body = sendEmail(t, app, "/invoices/"+uuid.NewString()+"/send-email")
	assert.Equal(t, fiber.StatusNotFound, status, body)
}
