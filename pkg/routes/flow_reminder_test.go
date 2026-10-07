package routes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/outbox"
)

// reminderInvoice builds a sent invoice due in two days: strictly inside the
// default 7-day before-window with margin on both sides. The due date is
// derived from the local calendar date rather than "UTC now + 1d", which can
// collapse onto today near UTC midnight (server zones ahead of UTC) and fire
// neither leg — the test then fails only between 17:00 and 24:00 UTC.
func reminderInvoice(clientID string) invoiceSpec {
	today := utils.DateOnly(time.Now())
	spec := newInvoice()
	spec.ClientID = clientID
	spec.Due = today.AddDate(0, 0, 2).Format("2006-01-02")
	spec.Issue = today.Format("2006-01-02")
	return spec
}

// settingsWithReminder builds a full settings PATCH payload (partial payloads
// reset the reminder fields, so the test must send them explicitly).
func settingsWithReminder(before, after int, enabled bool) string {
	enabledJSON := "false"
	if enabled {
		enabledJSON = "true"
	}
	return `{"currency":"IDR","invoice_prefix":"INV-","reminder_enabled":` + enabledJSON +
		`,"reminder_before_days":` + itoa(before) + `,"reminder_after_days":` + itoa(after) + `}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// TestReminderFlow covers the end-to-end reminder path: owner configures the
// schedule, a sent invoice comes due, the worker sweep claims the leg once and
// queues exactly one email, and a second sweep never re-sends.
func TestReminderFlow(t *testing.T) {
	app := newTestApp()
	owner := registerUser(t, app, "reminder-owner@example.com", "secret123")
	orgID := myOrgID(t, app, owner)
	clientID := createClient(t, app, owner, "Reminder Co")
	createInvoice(t, app, owner, reminderInvoice(clientID))

	resp := doRequest(t, app, "PATCH", "/api/settings", settingsWithReminder(7, 3, true), owner)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	db, err := database.OpenDBConnection()
	require.NoError(t, err)

	// Billing email default is empty, so the org owner receives the reminder.
	outbox.New().ProcessOnce(context.Background())

	var logs int64
	require.NoError(t, db.InvoiceQueries.Model(&models.InvoiceReminderLog{}).Where("org_id = ?", orgID).Count(&logs).Error)
	assert.Equal(t, int64(1), logs, "exactly one before-leg reminder for the org")

	var mails int64
	require.NoError(t, db.InvoiceQueries.Model(&models.MailOutbox{}).Where("`to` = ?", "reminder-owner@example.com").Count(&mails).Error)
	assert.Equal(t, int64(1), mails, "one reminder email queued to the owner")

	// A second sweep must not re-remind the same leg.
	outbox.New().ProcessOnce(context.Background())
	require.NoError(t, db.InvoiceQueries.Model(&models.InvoiceReminderLog{}).Where("org_id = ?", orgID).Count(&logs).Error)
	assert.Equal(t, int64(1), logs, "leg stays claimed on the second sweep")

	// Tidy up so later global sweeps in this shared DB stay unaffected.
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.InvoiceReminderLog{})
}

// TestReminderFlowSkippedWhenDisabled verifies the org switch suppresses mail.
func TestReminderFlowSkippedWhenDisabled(t *testing.T) {
	app := newTestApp()
	owner := registerUser(t, app, "reminder-off@example.com", "secret123")
	orgID := myOrgID(t, app, owner)
	clientID := createClient(t, app, owner, "Quiet Co")
	createInvoice(t, app, owner, reminderInvoice(clientID))

	resp := doRequest(t, app, "PATCH", "/api/settings", settingsWithReminder(7, 3, false), owner)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	outbox.New().ProcessOnce(context.Background())

	var logs int64
	require.NoError(t, db.InvoiceQueries.Model(&models.InvoiceReminderLog{}).Where("org_id = ?", orgID).Count(&logs).Error)
	assert.Equal(t, int64(0), logs, "disabled org gets no reminder")
}

// TestReminderSettingsDefaults verifies a fresh org reads the documented defaults.
func TestReminderSettingsDefaults(t *testing.T) {
	app := newTestApp()
	owner := registerUser(t, app, "reminder-defaults@example.com", "secret123")

	resp := doRequest(t, app, "GET", "/api/settings", "", owner)
	require.Equal(t, 200, resp.StatusCode)
	settings := decodeBody(t, resp)["settings"].(map[string]interface{})
	assert.Equal(t, true, settings["reminder_enabled"])
	assert.Equal(t, float64(7), settings["reminder_before_days"])
	assert.Equal(t, float64(3), settings["reminder_after_days"])
}

// TestReminderSettingsOwnerOnly verifies staff cannot change the schedule.
func TestReminderSettingsOwnerOnly(t *testing.T) {
	app := newTestApp()
	owner := registerUser(t, app, "reminder-boss@example.com", "secret123")
	staff := registerUser(t, app, "reminder-staff@example.com", "secret123")
	inviteAndAccept(t, app, owner, staff)

	resp := doRequest(t, app, "PATCH", "/api/settings", settingsWithReminder(0, 0, false), staff)
	assert.Equal(t, 403, resp.StatusCode, "staff cannot patch settings")
	resp.Body.Close()
}
