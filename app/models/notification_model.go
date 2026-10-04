package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Outbound notification event types (v1). Relay-internal gateway events
// stay in webhook_deliveries; these are user billing events for n8n/GOWA.
const (
	NotifEventInvoiceCreated       = "invoice.created"
	NotifEventInvoiceStatusUpdated = "invoice.status_updated"
	NotifEventPaymentCreated       = "payment.created"
	NotifEventPaymentVoided        = "payment.voided"
	NotifEventTest                 = "notification.test"
)

// NotifAllowedEvents are subscribable via endpoint Events CSV. Empty Events means all (notification.test bypasses the filter).
var NotifAllowedEvents = []string{
	NotifEventInvoiceCreated,
	NotifEventInvoiceStatusUpdated,
	NotifEventInvoiceReminder,
	NotifEventPaymentCreated,
	NotifEventPaymentVoided,
}

// Notification delivery statuses. Rows are written by controllers and
// consumed by the background worker; "processing" is a short-lived claim.
const (
	NotifStatusPending    = "pending"
	NotifStatusProcessing = "processing"
	NotifStatusDelivered  = "delivered"
	NotifStatusFailed     = "failed"
	NotifStatusDead       = "dead"
)

// NotificationEndpoint is one user-owned webhook target (e.g. n8n).
// Identity comes from the session user, never from client-supplied fields.
type NotificationEndpoint struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" db:"id" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" db:"user_id" json:"-"`
	TargetURL string    `gorm:"size:1024" db:"target_url" json:"target_url" validate:"required,lte=1024"`
	Secret    string    `gorm:"size:128" db:"secret" json:"-"`
	Events    string    `gorm:"size:255" db:"events" json:"events" validate:"omitempty,lte=255"`
	IsActive  bool      `gorm:"default:true" db:"is_active" json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CreateEndpointInput is the session-user payload for registering a target.
type CreateEndpointInput struct {
	TargetURL string `json:"target_url" validate:"required,lte=1024"`
	Events    string `json:"events" validate:"omitempty,lte=255"`
}

// UpdateEndpointInput edits endpoint metadata (not the secret).
// Events is a pointer: nil leaves the filter untouched, empty string
// clears it (subscribe to all), otherwise the CSV is sanitized.
type UpdateEndpointInput struct {
	TargetURL string  `json:"target_url" validate:"omitempty,lte=1024"`
	Events    *string `json:"events" validate:"omitempty,lte=255"`
	IsActive  *bool   `json:"is_active"`
}

// NotificationDelivery tracks one event forward to one endpoint.
type NotificationDelivery struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" db:"id" json:"id"`
	UserID      uuid.UUID  `gorm:"type:uuid;index" db:"user_id" json:"-"`
	EndpointID  uuid.UUID  `gorm:"type:uuid;index" db:"endpoint_id" json:"endpoint_id"`
	EventID     string     `gorm:"size:64;uniqueIndex" db:"event_id" json:"event_id"`
	EventType   string     `gorm:"size:64;index" db:"event_type" json:"event_type"`
	TargetURL   string     `gorm:"size:1024" db:"target_url" json:"target_url"`
	Payload     string     `gorm:"type:text" db:"payload" json:"-"`
	Status      string     `gorm:"size:16;index;default:pending" db:"status" json:"status"`
	Attempt     int        `db:"attempt" json:"attempt"`
	RespCode    int        `db:"resp_code" json:"resp_code"`
	RespBody    string     `gorm:"type:text" db:"resp_body" json:"-"`
	NextRetryAt *time.Time `gorm:"index" db:"next_retry_at" json:"-"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// ParseNotifEvents splits an Events CSV into trimmed tokens.
func ParseNotifEvents(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// NotifEventAllowed reports whether eventType is in the allowed set.
func NotifEventAllowed(eventType string) bool {
	for _, e := range NotifAllowedEvents {
		if e == eventType {
			return true
		}
	}
	return false
}

// EndpointSubscribed reports whether an endpoint wants eventType.
// Empty Events means all subscribable events.
func EndpointSubscribed(eventsCSV, eventType string) bool {
	if eventType == NotifEventTest {
		return true
	}
	events := ParseNotifEvents(eventsCSV)
	if len(events) == 0 {
		return NotifEventAllowed(eventType)
	}
	for _, e := range events {
		if e == eventType {
			return true
		}
	}
	return false
}
