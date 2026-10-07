package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// clientPortalPaymentCap bounds the recent lists a public portal returns: the
// payload is read by an unauthenticated holder, so it stays small and cheap.
const clientPortalPaymentCap = 50

// ClientPortalQueries provides the public client-portal reads and the
// owner-facing link lifecycle. Every child query is scoped by the link's
// OrgID so a token from one org can never read another org's rows.
type ClientPortalQueries struct {
	*gorm.DB
}

// CreateClientLink persists a fresh public link (token already hashed).
func (q *ClientPortalQueries) CreateClientLink(link *models.ClientLink) error {
	return DoRetry(func() error {
		return q.Create(link).Error
	})
}

// GetClientLinkByHash resolves a live link by its token hash. A revoked row
// is an audit tombstone and must never resolve, so revocation is immediate.
func (q *ClientPortalQueries) GetClientLinkByHash(hash string) (models.ClientLink, error) {
	link := models.ClientLink{}
	if err := q.Where("token_hash = ? AND revoked_at IS NULL", hash).First(&link).Error; err != nil {
		return link, notFound(err)
	}
	return link, nil
}

// GetClientLinkForClient returns the live link of a client within an org (the
// owner UI uses it to show/regenerate/revoke without exposing the hash).
func (q *ClientPortalQueries) GetClientLinkForClient(orgID, clientID uuid.UUID) (models.ClientLink, error) {
	link := models.ClientLink{}
	if err := q.Where("org_id = ? AND client_id = ? AND revoked_at IS NULL", orgID, clientID).First(&link).Error; err != nil {
		return link, notFound(err)
	}
	return link, nil
}

// RevokeClientLink tombstones the client's live link so every lookup misses
// immediately. A second revoke is a no-op (RowsAffected 0 is not an error).
func (q *ClientPortalQueries) RevokeClientLink(orgID, clientID uuid.UUID) error {
	return DoRetry(func() error {
		return q.Model(&models.ClientLink{}).
			Where("org_id = ? AND client_id = ? AND revoked_at IS NULL", orgID, clientID).
			Update("revoked_at", time.Now()).Error
	})
}

// RotateClientLink replaces a live link's token hash in place and clears any
// tombstone, so the old token stops working and the new one is the only live
// row. The unique client_id index guarantees at most one row per client.
func (q *ClientPortalQueries) RotateClientLink(orgID, clientID uuid.UUID, newHash string) error {
	return DoRetry(func() error {
		return q.Model(&models.ClientLink{}).
			Where("org_id = ? AND client_id = ?", orgID, clientID).
			Updates(map[string]any{
				"token_hash": newHash,
				"revoked_at": nil,
				"created_at": time.Now(),
			}).Error
	})
}

// ClientPortalInvoices returns the client's non-draft invoices with their paid
// aggregate, newest due date first, so the portal never surfaces an unsent
// draft. Mirrors ClientInvoices' shape.
func (q *ClientPortalQueries) ClientPortalInvoices(orgID, clientID uuid.UUID, limit int) ([]models.InvoiceListRow, error) {
	rows := []models.InvoiceListRow{}

	paidSubquery := q.Model(&models.Payment{}).
		Select("invoice_id, SUM(amount) AS paid").
		Where("voided_at IS NULL").
		Group("invoice_id")

	if err := q.Table("invoices").
		Select(`invoices.id, invoices.invoice_number, invoices.issue_date, invoices.due_date,
			invoices.total, invoices.currency, invoices.status,
			COALESCE(pay.paid, 0) AS paid_amount`).
		Joins("LEFT JOIN (?) AS pay ON pay.invoice_id = invoices.id", paidSubquery).
		Where("invoices.org_id = ? AND invoices.client_id = ? AND invoices.status <> ?", orgID, clientID, models.InvoiceStatusDraft).
		Order("invoices.due_date DESC").Order("invoices.created_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return rows, err
	}

	return rows, nil
}

// ClientPortalPayTokens maps each of the client's invoice IDs to its existing
// public pay-link token. It never mints a link (a public read must not run a
// money path); invoices without a link are simply absent from the map.
func (q *ClientPortalQueries) ClientPortalPayTokens(orgID, clientID uuid.UUID) (map[uuid.UUID]string, error) {
	type row struct {
		InvoiceID uuid.UUID `db:"invoice_id"`
		Token     string    `db:"token"`
	}
	rows := []row{}
	if err := q.Table("payment_links").
		Select("payment_links.invoice_id, payment_links.token").
		Joins("JOIN invoices ON invoices.id = payment_links.invoice_id").
		Where("invoices.org_id = ? AND invoices.client_id = ?", orgID, clientID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	tokens := make(map[uuid.UUID]string, len(rows))
	for _, r := range rows {
		tokens[r.InvoiceID] = r.Token
	}
	return tokens, nil
}

// ClientPortalPayments returns the non-voided payments over the client's
// invoices, newest first (paid_on DESC, created_at DESC), capped.
func (q *ClientPortalQueries) ClientPortalPayments(orgID, clientID uuid.UUID, limit int) ([]models.PaymentListRow, error) {
	payments := []models.PaymentListRow{}
	err := q.Table("payments").
		Select(`payments.id AS payment_id, payments.invoice_id, invoices.invoice_number,
			COALESCE(clients.name, '') AS client_name, invoices.currency AS invoice_currency,
			payments.amount, payments.method, payments.paid_on, payments.txn_id, payments.notes,
			payments.gateway_order_id`).
		Joins("JOIN invoices ON invoices.id = payments.invoice_id").
		Joins("LEFT JOIN clients ON clients.id = invoices.client_id").
		Where("payments.org_id = ? AND invoices.org_id = ? AND invoices.client_id = ? AND payments.voided_at IS NULL",
			orgID, orgID, clientID).
		Order("payments.paid_on DESC").Order("payments.created_at DESC").
		Limit(limit).
		Scan(&payments).Error
	return payments, err
}

// ClientPortalPaymentCap exposes the shared cap so controllers and tests agree.
func ClientPortalPaymentCap() int { return clientPortalPaymentCap }
