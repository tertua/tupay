package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/database"
)

// TestInvoiceSendEmailFlow covers POST /invoices/:id/send-email end to end:
// success enqueues a mail_outbox row to the client address, a resend enqueues a
// second row (idempotency is intentionally off), a draft is a 409, a client
// without an email is a 409, an unknown/cross-org invoice is a 404, and an
// unconfigured mailer is a 501.
func TestInvoiceSendEmailFlow(t *testing.T) {
	app := newTestApp()

	// Success: SMTP configured so the endpoint enqueues instead of 501ing.
	t.Setenv("SMTP_HOST", "smtp.test")
	t.Setenv("SMTP_FROM", "no-reply@test")

	owner := registerUser(t, app, "send-email-owner@example.com", "secret123")
	client := createClientNamed(t, app, owner, clientSpec{Name: "Acme Billing", Email: "billing@acme.test"})
	clientID := client["id"].(string)

	spec := newInvoice()
	spec.ClientID = clientID
	invoice := createInvoice(t, app, owner, spec)
	invoiceID := invoice["id"].(string)

	resp := doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", owner)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, true, body["queued"])

	db, err := database.OpenDBConnection()
	require.NoError(t, err)

	countMail := func() int64 {
		var n int64
		require.NoError(t, db.InvoiceQueries.Model(&models.MailOutbox{}).Where("`to` = ?", "billing@acme.test").Count(&n).Error)
		return n
	}
	assert.Equal(t, int64(1), countMail(), "one invoice email queued to the client address")

	// Resend enqueues a second mail: manual resend is legitimate.
	resp = doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", owner)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()
	assert.Equal(t, int64(2), countMail(), "a resend enqueues a second mail")

	// The queued HTML carries the invoice number.
	var last models.MailOutbox
	require.NoError(t, db.InvoiceQueries.Model(&models.MailOutbox{}).Where("`to` = ?", "billing@acme.test").
		Order("created_at DESC").First(&last).Error)
	assert.Contains(t, last.HtmlBody, invoice["invoice_number"].(string))

	// Cross-org: another owner cannot send someone else's invoice.
	stranger := registerUser(t, app, "send-email-stranger@example.com", "secret123")
	resp = doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", stranger)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// TestInvoiceSendEmailDraftConflict proves a draft invoice is a 409 before any mail work.
func TestInvoiceSendEmailDraftConflict(t *testing.T) {
	app := newTestApp()
	t.Setenv("SMTP_HOST", "smtp.test")
	t.Setenv("SMTP_FROM", "no-reply@test")

	owner := registerUser(t, app, "send-email-draft@example.com", "secret123")
	clientID := createClientNamed(t, app, owner, clientSpec{Name: "Draft Co", Email: "draft@acme.test"})["id"].(string)

	spec := newInvoice()
	spec.Status = models.InvoiceStatusDraft
	spec.ClientID = clientID
	invoiceID := createInvoice(t, app, owner, spec)["id"].(string)

	resp := doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", owner)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, "invoice is still a draft", body["error"].(map[string]interface{})["message"])
}

// TestInvoiceSendEmailNoClientEmail proves a client without an address is a 409.
func TestInvoiceSendEmailNoClientEmail(t *testing.T) {
	app := newTestApp()
	t.Setenv("SMTP_HOST", "smtp.test")
	t.Setenv("SMTP_FROM", "no-reply@test")

	owner := registerUser(t, app, "send-email-nomail@example.com", "secret123")
	clientID := createClient(t, app, owner, "No Mail Co")

	spec := newInvoice()
	spec.ClientID = clientID
	invoiceID := createInvoice(t, app, owner, spec)["id"].(string)

	resp := doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", owner)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	errObj := decodeBody(t, resp)["error"].(map[string]interface{})
	assert.Equal(t, "client has no email", errObj["message"])
}

// TestInvoiceSendEmailUnconfigured proves an unconfigured mailer is a 501 and
// no mail is enqueued.
func TestInvoiceSendEmailUnconfigured(t *testing.T) {
	app := newTestApp()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM", "")

	owner := registerUser(t, app, "send-email-unconfigured@example.com", "secret123")
	clientID := createClientNamed(t, app, owner, clientSpec{Name: "Config Co", Email: "config@acme.test"})["id"].(string)

	spec := newInvoice()
	spec.ClientID = clientID
	invoiceID := createInvoice(t, app, owner, spec)["id"].(string)

	resp := doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", owner)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	errObj := decodeBody(t, resp)["error"].(map[string]interface{})
	assert.Equal(t, "email provider is not configured", errObj["message"])

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	var n int64
	require.NoError(t, db.InvoiceQueries.Model(&models.MailOutbox{}).Where("`to` = ?", "config@acme.test").Count(&n).Error)
	assert.Equal(t, int64(0), n, "no mail is queued when the provider is unconfigured")
}

// TestInvoiceSendEmailStaffForbidden proves the owner-only route gate rejects staff.
func TestInvoiceSendEmailStaffForbidden(t *testing.T) {
	app := newTestApp()
	t.Setenv("SMTP_HOST", "smtp.test")
	t.Setenv("SMTP_FROM", "no-reply@test")

	owner := registerUser(t, app, "send-email-boss@example.com", "secret123")
	staff := registerUser(t, app, "send-email-staff@example.com", "secret123")
	inviteAndAccept(t, app, owner, staff)

	clientID := createClientNamed(t, app, owner, clientSpec{Name: "Staff Co", Email: "staff@acme.test"})["id"].(string)
	spec := newInvoice()
	spec.ClientID = clientID
	spec.Status = models.InvoiceStatusDraft
	// Staff cannot create a sent invoice (owner-only create rule); a draft is
	// enough to prove the route gate fires before the controller.
	invoiceID := createInvoice(t, app, owner, spec)["id"].(string)

	resp := doRequest(t, app, "POST", "/api/invoices/"+invoiceID+"/send-email", "{}", staff)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
}
