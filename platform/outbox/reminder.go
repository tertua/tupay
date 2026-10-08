package outbox

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/logger"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/events"
	"github.com/tertua/tupay/platform/mail"
)

// remindInvoices sweeps invoices due for a payment reminder once per tick.
// It runs between the gateway reconcile and the idempotency purge in
// ProcessOnce, and is safe on several replicas: each (invoice, kind) leg is
// claimed atomically through the unique index on invoice_reminder_log before
// any email/SSE side effect fires.
func (w *Worker) remindInvoices(ctx context.Context) {
	db, err := database.OpenDBConnection()
	if err != nil {
		logger.L().Warn("outbox reminder tick skipped, database unavailable", "err", err)
		return
	}
	now := time.Now()
	rows, err := db.DueInvoiceReminders(now, w.batch)
	if err != nil {
		logger.L().Warn("outbox reminder tick failed", "err", err)
		return
	}
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.remindOne(ctx, db, row, now)
	}
}

// reminderLeg returns which leg (if any) is due for the row at today, honoring
// the per-org before/after day windows. 0 days disables the leg.
func reminderLeg(row models.ReminderScanRow, today time.Time) string {
	if row.DueDate == nil || !row.Enabled {
		return ""
	}
	due := utils.DateOnly(*row.DueDate)
	if row.BeforeDays > 0 {
		start := due.AddDate(0, 0, -row.BeforeDays)
		if !today.Before(start) && today.Before(due) {
			return models.InvoiceReminderKindBefore
		}
	}
	if row.AfterDays > 0 {
		start := due.AddDate(0, 0, row.AfterDays)
		if !today.Before(start) {
			return models.InvoiceReminderKindAfter
		}
	}
	return ""
}

// remindOne claims one leg then fires the email + SSE side effects. The claim
// (a unique insert) happens first so a leg is at-most-once even across
// replicas; a mail enqueue failure is logged best-effort because mail_outbox
// has its own retry and SSE cannot be replayed.
func (w *Worker) remindOne(ctx context.Context, db *database.Queries, row models.ReminderScanRow, now time.Time) {
	kind := reminderLeg(row, utils.DateOnly(now))
	if kind == "" {
		return
	}
	claimed, err := db.ClaimInvoiceReminder(row.OrgID, row.InvoiceID, kind, now)
	if err != nil {
		logger.L().Warn("outbox reminder claim failed", "invoice_id", row.InvoiceID.String(), "kind", kind, "err", err)
		return
	}
	if !claimed {
		return
	}
	manual := manualRow(row)
	sendReminderMail(db, manual, kind)
	publishReminderEvent(db, manual, kind)
	logger.L().Info("outbox reminder sent", "invoice_id", row.InvoiceID.String(), "kind", kind)
}

// manualRow narrows a scheduled scan row to the fields the shared reminder
// copy/event path needs, so the sweep and the manual endpoint share one shape.
func manualRow(row models.ReminderScanRow) models.ClientReminderRow {
	return models.ClientReminderRow{
		InvoiceID:     row.InvoiceID,
		OrgID:         row.OrgID,
		InvoiceNumber: row.InvoiceNumber,
		Currency:      row.Currency,
		Total:         row.Total,
		DueDate:       row.DueDate,
		BillingEmail:  row.BillingEmail,
		Language:      row.Language,
	}
}

// sendReminderMail renders and queues the reminder email for one claimed leg.
// The recipient is the org billing email, falling back to the owner's email
// when the org has none configured. It is shared by the scheduled sweep and
// the manual endpoint (see reminder_manual.go).
func sendReminderMail(db *database.Queries, row models.ClientReminderRow, kind string) {
	to := row.BillingEmail
	if to == "" {
		to = ownerEmail(db, row.OrgID)
	}
	if to == "" {
		logger.L().Warn("outbox reminder has no recipient", "org_id", row.OrgID.String(), "invoice_id", row.InvoiceID.String())
		return
	}
	subject, htmlBody := reminderCopy(row, kind)
	if err := db.EnqueueMail(&models.MailOutbox{To: to, Subject: subject, Body: subject, HtmlBody: htmlBody}); err != nil {
		logger.L().Warn("outbox reminder mail enqueue failed", "invoice_id", row.InvoiceID.String(), "kind", kind, "err", err)
	}
}

// reminderCopy builds the reminder subject and rendered HTML body for a leg,
// using the invoice's public pay link when one exists.
func reminderCopy(row models.ClientReminderRow, kind string) (string, string) {
	subject := "Payment reminder for invoice " + row.InvoiceNumber
	if kind == models.InvoiceReminderKindAfter {
		subject = "Overdue: invoice " + row.InvoiceNumber
	}
	body, err := mail.Render("reminder", mail.TemplateData{
		AppName:       configs.Get().AppName,
		InvoiceNumber: row.InvoiceNumber,
		URL:           invoicePayURL(row),
		Total:         row.Total.String(),
		Currency:      row.Currency,
		DueDate:       utils.FormatDate(row.DueDate),
		Kind:          kind,
	})
	if err != nil {
		logger.L().Warn("outbox reminder render failed", "invoice_id", row.InvoiceID.String(), "err", err)
		return subject, subject
	}
	return subject, body
}

// invoicePayURL returns the invoice's public pay link (minted earlier by the
// controller path) or the app root when the invoice has no link yet.
func invoicePayURL(row models.ClientReminderRow) string {
	base := configs.Get().Mail.AppPublicURL
	link, err := payLinkToken(row)
	if err != nil || link == "" {
		return base
	}
	return trimSlash(base) + "/pay/" + link
}

// payLinkToken loads the invoice's public payment link token, if any.
func payLinkToken(row models.ClientReminderRow) (string, error) {
	db, err := database.OpenDBConnection()
	if err != nil {
		return "", err
	}
	link, err := db.GetPaymentLinkForInvoice(row.InvoiceID, row.OrgID)
	if err != nil {
		return "", err
	}
	return link.Token, nil
}

// ownerEmail returns the email of the org's owner membership, or "" when the
// org has no owner row (best-effort: the reminder is skipped, never fatal).
func ownerEmail(db *database.Queries, orgID uuid.UUID) string {
	members, err := db.ListByOrg(orgID)
	if err != nil {
		return ""
	}
	for _, m := range members {
		if m.Role != models.RoleOwner {
			continue
		}
		user, err := db.GetUserByID(m.UserID)
		if err == nil && user.Email != "" {
			return user.Email
		}
	}
	return ""
}

// publishReminderEvent nudges the live SSE stream and the org's webhook
// endpoints for one claimed leg; it reuses the same per-user path as
// enqueueNotification so the bell and n8n consumers stay in sync.
func publishReminderEvent(db *database.Queries, row models.ClientReminderRow, kind string) {
	members, err := db.ListByOrg(row.OrgID)
	if err != nil || len(members) == 0 {
		return
	}
	eventID := "evt_reminder_" + row.InvoiceID.String()[:8] + "_" + kind
	for _, m := range members {
		events.Default.Publish(m.UserID.String(), events.Event{Type: models.NotifEventInvoiceReminder, Data: "{}"})
		endpoints, err := db.ListActiveEndpoints(m.UserID, models.NotifEventInvoiceReminder)
		if err != nil || len(endpoints) == 0 {
			continue
		}
		enqueueReminderDelivery(db, m.UserID, endpoints, row, kind, eventID)
	}
}

// enqueueReminderDelivery queues one reminder event for every subscribed
// endpoint of a member, sharing the payload event_id per member.
func enqueueReminderDelivery(db *database.Queries, userID uuid.UUID, endpoints []models.NotificationEndpoint, row models.ClientReminderRow, kind, eventID string) {
	payload := reminderPayload(row, kind)
	for _, e := range endpoints {
		if err := db.EnqueueDelivery(&models.NotificationDelivery{
			UserID:     userID,
			EndpointID: e.ID,
			EventID:    eventID + ":" + e.ID.String(),
			EventType:  models.NotifEventInvoiceReminder,
			TargetURL:  e.TargetURL,
			Payload:    payload,
		}); err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.L().Debug("outbox reminder delivery enqueue failed", "endpoint_id", e.ID.String(), "err", err)
		}
	}
}

// reminderPayload is the stable v1 webhook envelope for a reminder event.
func reminderPayload(row models.ClientReminderRow, kind string) string {
	amount := row.Total.String()
	isAfter := kind == models.InvoiceReminderKindAfter
	overdue := "false"
	if isAfter {
		overdue = "true"
	}
	return `{"type":"` + models.NotifEventInvoiceReminder + `","version":"v1","data":{"invoice_id":"` +
		row.InvoiceID.String() + `","invoice_number":"` + row.InvoiceNumber + `","kind":"` + kind +
		`","amount":"` + amount + `","currency":"` + row.Currency + `","due_date":"` +
		utils.FormatDate(row.DueDate) + `","overdue":` + overdue + `}}`
}

// trimSlash removes one trailing slash from a URL base.
func trimSlash(s string) string {
	if len(s) > 0 && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	return s
}
