package outbox

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/database"
)

// EnqueueInvoiceReminder claims one manual reminder leg for a client invoice,
// then fires the shared email + SSE/webhook side effects (see sendReminderMail
// and publishReminderEvent in reminder.go). It returns false without side
// effects when the (invoice, kind) leg was already claimed, so a double-click
// stays idempotent while leaving the scheduled before/after legs untouched.
//
// The caller is a controller (client_lifecycle_controller.go), which cannot
// reach the package-private sweep helpers; this exported path keeps the copy
// and event delivery single-sourced with the scheduled sweep.
func EnqueueInvoiceReminder(orgID uuid.UUID, row models.ClientReminderRow, kind string) (bool, error) {
	db, err := database.OpenDBConnection()
	if err != nil {
		return false, err
	}
	claimed, err := db.ClaimInvoiceReminder(orgID, row.InvoiceID, kind, time.Now())
	if err != nil || !claimed {
		return false, err
	}
	sendReminderMail(db, row, kind)
	publishReminderEvent(db, row, kind)
	return true, nil
}
