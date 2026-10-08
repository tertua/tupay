package outbox

// Package outbox runs the in-process background worker (stdlib only).
//
// Controllers enqueue durable jobs and return fast; the worker delivers
// them with retries:
//   - mail_outbox rows -> SMTP via platform/mail (SendMail hook, testable)
//   - webhook_deliveries rows -> downstream forward via platform/relay
//   - expired idempotency_keys rows -> purged
//
// Claiming is atomic per row (single UPDATE guarded by status), so the
// worker is safe with several replicas polling the same database. Jobs are
// idempotent by design (resend = same content), making at-least-once
// delivery harmless.

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/constants"
	"github.com/tertua/tupay/pkg/logger"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/mail"
	"github.com/tertua/tupay/platform/relay"
)

// Retry policy lives in policy.go, driven by OUTBOX_MAX_ATTEMPTS and
// OUTBOX_MAX_BACKOFF_MINUTES (defaults: dead after 10, capped at 2h).
const drainTimeout = 5 * time.Second

// SendMail delivers one email. It is a variable (not a direct mail call)
// so tests can capture sends without an SMTP server.
var SendMail = func(to, subject, textBody, htmlBody string) error {
	mailer, err := mail.NewFromEnv()
	if err != nil {
		return err
	}
	return mailer.SendHTML(to, subject, textBody, htmlBody)
}

// ForwardNotification POSTs one notification payload. It is a variable so
// tests can capture forwards without an HTTP server.
var ForwardNotification = func(ctx context.Context, targetURL, eventID string, payload []byte, secret string) (*relay.ForwardResult, error) {
	return relay.Forward(ctx, targetURL, "local", eventID, payload, secret)
}

// Worker polls due jobs until its context is cancelled.
type Worker struct {
	poll  time.Duration
	batch int

	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
	done   chan struct{}
}

// New builds a worker from the central config.
func New() *Worker {
	cfg := configs.Get().Outbox
	return &Worker{poll: time.Duration(cfg.PollSeconds) * time.Second, batch: cfg.Batch}
}

// Start begins polling in the background. Stop cancels and drains.
func (w *Worker) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	w.mu.Lock()
	w.cancel = cancel
	w.done = make(chan struct{})
	w.mu.Unlock()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer close(w.done)
		ticker := time.NewTicker(w.poll)
		defer ticker.Stop()
		w.ProcessOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.ProcessOnce(ctx)
			}
		}
	}()
}

// Stop cancels polling and waits for in-flight jobs up to drainTimeout.
func (w *Worker) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(drainTimeout):
	}
}

// ProcessOnce runs one tick (mail, deliveries, notifications, gateway reconcile, purge); exported for tests and drains.
func (w *Worker) ProcessOnce(ctx context.Context) {
	w.processMail(ctx)
	w.processDeliveries(ctx)
	w.processNotifications(ctx)
	w.reconcileGateway(ctx)
	w.remindInvoices(ctx)
	w.generateRecurringInvoices(ctx)
	w.purgeIdempotency()
}

func (w *Worker) processMail(ctx context.Context) {
	db, err := database.OpenDBConnection()
	if err != nil {
		logger.L().Warn("outbox mail tick skipped, database unavailable", "err", err)
		return
	}
	now := time.Now()
	rows, err := db.DueMail(now, w.batch)
	if err != nil {
		logger.L().Warn("outbox mail tick failed", "err", err)
		return
	}
	for _, m := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.sendOneMail(db, m, now)
	}
}

func (w *Worker) sendOneMail(db *database.Queries, m models.MailOutbox, now time.Time) {
	claimed, err := db.ClaimMail(m.ID, now)
	if err != nil || !claimed {
		return
	}
	if err := SendMail(m.To, m.Subject, m.Body, m.HtmlBody); err != nil {
		if errors.Is(err, mail.ErrNotConfigured) {
			// No SMTP configured (dev): leave pending without burning
			// attempts; reset the claim so the next tick retries.
			recordErr("mark mail failed", db.MarkMailFailed(m.ID, m.Attempt, &[]time.Time{now.Add(w.poll)}[0], now), "mail_id", m.ID.String())
			logger.L().Debug("outbox mail skipped, provider not configured", "mail_id", m.ID.String(), "to", m.To)
			return
		}
		retry := nextRetryAt(m.Attempt+1, now)
		recordErr("mark mail failed", db.MarkMailFailed(m.ID, m.Attempt+1, retry, now), "mail_id", m.ID.String())
		logger.L().Warn("outbox mail failed", "mail_id", m.ID.String(), "to", m.To, "attempt", m.Attempt+1, "err", err)
		return
	}
	recordErr("mark mail sent", db.MarkMailSent(m.ID, now), "mail_id", m.ID.String())
	logger.L().Info("outbox mail sent", "mail_id", m.ID.String(), "to", m.To)
}

func (w *Worker) processDeliveries(ctx context.Context) {
	db, err := database.OpenDBConnection()
	if err != nil {
		logger.L().Warn("outbox delivery tick skipped, database unavailable", "err", err)
		return
	}
	now := time.Now()
	rows, err := db.PendingDeliveries(now, w.batch)
	if err != nil {
		logger.L().Warn("outbox delivery tick failed", "err", err)
		return
	}
	for _, d := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.forwardOne(ctx, db, d, now)
	}
}

func (w *Worker) forwardOne(ctx context.Context, db *database.Queries, d models.WebhookDelivery, now time.Time) {
	claimed, err := db.ClaimDelivery(d.ID, now)
	if err != nil || !claimed {
		return
	}
	project, err := db.GetProjectBySlug(d.ProjectSlug)
	if err != nil {
		retry := nextRetryAt(d.Attempt+1, now)
		failDelivery(db, d.ID, d.Attempt+1, retry, "project not found", now)
		return
	}
	payload := []byte(d.Payload)
	d.Signature = relay.SignPayload(payload, project.WebhookSecret)
	d.Attempt++
	d.UpdatedAt = now
	result, ferr := relay.Forward(ctx, d.TargetURL, project.Slug, "evt_retry_"+d.ID.String(), payload, project.WebhookSecret)
	if ferr != nil {
		retry := nextRetryAt(d.Attempt, now)
		failDelivery(db, d.ID, d.Attempt, retry, constants.TruncateLog(ferr.Error()), now)
		logger.L().Warn("outbox delivery failed", "delivery_id", d.ID.String(), "order_id", d.OrderID, "attempt", d.Attempt, "err", ferr)
		return
	}
	d.RespCode = result.StatusCode
	d.RespBody = constants.TruncateLog(result.Body)
	if result.StatusCode >= 200 && result.StatusCode < 300 {
		d.Status = "delivered"
		d.NextRetryAt = nil
	} else {
		d.Status = "failed"
		d.NextRetryAt = nextRetryAt(d.Attempt, now)
		if d.NextRetryAt == nil {
			d.Status = "dead"
		}
	}
	recordErr("save delivery", db.SaveDelivery(&d), "delivery_id", d.ID.String(), "order_id", d.OrderID)
	if d.Status == "delivered" {
		logger.L().Info("outbox delivery sent", "delivery_id", d.ID.String(), "order_id", d.OrderID, "attempt", d.Attempt)
	}
}

func failDelivery(db *database.Queries, id uuid.UUID, attempt int, retry *time.Time, msg string, now time.Time) {
	status := "failed"
	if retry == nil {
		status = "dead"
	}
	if err := db.FailDelivery(id, status, attempt, retry, msg, now); err != nil {
		logger.L().Warn("outbox failed to record delivery failure", "err", err)
	}
}

// processNotifications forwards due user webhook events (n8n, GOWA
// bridges). Secrets are resolved per endpoint at send time so rotated
// secrets apply to already-queued rows.
func (w *Worker) processNotifications(ctx context.Context) {
	db, err := database.OpenDBConnection()
	if err != nil {
		logger.L().Warn("outbox notification tick skipped, database unavailable", "err", err)
		return
	}
	now := time.Now()
	rows, err := db.DueNotifications(now, w.batch)
	if err != nil {
		logger.L().Warn("outbox notification tick failed", "err", err)
		return
	}
	for _, d := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.forwardOneNotification(ctx, db, d, now)
	}
}

func (w *Worker) forwardOneNotification(ctx context.Context, db *database.Queries, d models.NotificationDelivery, now time.Time) {
	claimed, err := db.ClaimNotification(d.ID, now)
	if err != nil || !claimed {
		return
	}
	endpoint, err := db.GetEndpoint(d.UserID, d.EndpointID)
	if err != nil {
		retry := nextRetryAt(d.Attempt+1, now)
		recordErr("mark notification failed", db.MarkNotificationFailed(d.ID, d.Attempt+1, retry, 0, "endpoint not found", now), "delivery_id", d.ID.String())
		return
	}
	payload := []byte(d.Payload)
	d.Attempt++
	result, ferr := ForwardNotification(ctx, d.TargetURL, "evt_retry_"+d.ID.String(), payload, endpoint.Secret)
	if ferr != nil {
		retry := nextRetryAt(d.Attempt, now)
		recordErr("mark notification failed", db.MarkNotificationFailed(d.ID, d.Attempt, retry, 0, constants.TruncateLog(ferr.Error()), now), "delivery_id", d.ID.String())
		logger.L().Warn("outbox notification failed", "delivery_id", d.ID.String(), "event_type", d.EventType, "attempt", d.Attempt, "err", ferr)
		return
	}
	if result.StatusCode >= 200 && result.StatusCode < 300 {
		recordErr("mark notification sent", db.MarkNotificationSent(d.ID, result.StatusCode, constants.TruncateLog(result.Body), now), "delivery_id", d.ID.String())
		logger.L().Info("outbox notification sent", "delivery_id", d.ID.String(), "event_type", d.EventType, "attempt", d.Attempt)
		return
	}
	retry := nextRetryAt(d.Attempt, now)
	recordErr("mark notification failed", db.MarkNotificationFailed(d.ID, d.Attempt, retry, result.StatusCode, constants.TruncateLog(result.Body), now), "delivery_id", d.ID.String())
}

func (w *Worker) purgeIdempotency() {
	db, err := database.OpenDBConnection()
	if err != nil {
		return
	}
	if n, err := db.DeleteExpiredIdempotencyKeys(time.Now()); err == nil && n > 0 {
		logger.L().Info("outbox purged expired idempotency keys", "count", n)
	}
}
