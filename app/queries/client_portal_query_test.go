package queries

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// portalTestDB opens a private in-memory SQLite seeded with the tables the
// portal queries touch.
func portalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Client{}, &models.Invoice{}, &models.Payment{}, &models.PaymentLink{}, &models.ClientLink{}))
	return db
}

func seedPortalClient(t *testing.T, db *gorm.DB, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	clientID := uuid.New()
	require.NoError(t, db.Create(&models.Client{
		ID: clientID, OrgID: orgID, UserID: orgID, CreatedAt: time.Now(), Name: "Acme",
	}).Error)
	return clientID
}

func seedPortalInvoice(t *testing.T, db *gorm.DB, orgID, clientID uuid.UUID, status string, total string) uuid.UUID {
	t.Helper()
	invoiceID := uuid.New()
	require.NoError(t, db.Create(&models.Invoice{
		ID: invoiceID, OrgID: orgID, UserID: orgID, ClientID: &clientID,
		InvoiceNumber: "INV-" + invoiceID.String()[:8], Status: status,
		Currency: "IDR", Total: decimal.RequireFromString(total),
		CreatedAt: time.Now(),
	}).Error)
	return invoiceID
}

func TestClientLinkLifecycle(t *testing.T) {
	db := portalTestDB(t)
	q := &ClientPortalQueries{DB: db}
	orgID := uuid.New()
	clientID := seedPortalClient(t, db, orgID)

	link := &models.ClientLink{TokenHash: "hash-1", ClientID: clientID, OrgID: orgID, UserID: orgID, CreatedAt: time.Now()}
	require.NoError(t, q.CreateClientLink(link))

	got, err := q.GetClientLinkByHash("hash-1")
	require.NoError(t, err)
	assert.Equal(t, clientID, got.ClientID)

	// Rotate swaps the hash in place: old misses, new hits.
	require.NoError(t, q.RotateClientLink(orgID, clientID, "hash-2"))
	_, err = q.GetClientLinkByHash("hash-1")
	assert.Error(t, err)
	got, err = q.GetClientLinkByHash("hash-2")
	require.NoError(t, err)
	assert.Equal(t, clientID, got.ClientID)

	// Revoke tombstones the row: lookups miss and the owner lookup misses too.
	require.NoError(t, q.RevokeClientLink(orgID, clientID))
	_, err = q.GetClientLinkByHash("hash-2")
	assert.Error(t, err)
	_, err = q.GetClientLinkForClient(orgID, clientID)
	assert.Error(t, err)
}

func TestClientLinkOrgIsolation(t *testing.T) {
	db := portalTestDB(t)
	q := &ClientPortalQueries{DB: db}
	orgA, orgB := uuid.New(), uuid.New()
	clientA := seedPortalClient(t, db, orgA)

	require.NoError(t, q.CreateClientLink(&models.ClientLink{
		TokenHash: "hash-a", ClientID: clientA, OrgID: orgA, UserID: orgA, CreatedAt: time.Now(),
	}))

	// A foreign org cannot read the link by client id.
	_, err := q.GetClientLinkForClient(orgB, clientA)
	assert.Error(t, err)

	// Invoice and payment rollups for a foreign org return nothing.
	seedPortalInvoice(t, db, orgA, clientA, models.InvoiceStatusSent, "100")
	invoices, err := q.ClientPortalInvoices(orgB, clientA, 50)
	require.NoError(t, err)
	assert.Empty(t, invoices)

	payments, err := q.ClientPortalPayments(orgB, clientA, 50)
	require.NoError(t, err)
	assert.Empty(t, payments)
}

func TestClientPortalInvoiceRollupExcludesDraftsAndCaps(t *testing.T) {
	db := portalTestDB(t)
	q := &ClientPortalQueries{DB: db}
	orgID := uuid.New()
	clientID := seedPortalClient(t, db, orgID)

	seedPortalInvoice(t, db, orgID, clientID, models.InvoiceStatusDraft, "10")
	seedPortalInvoice(t, db, orgID, clientID, models.InvoiceStatusSent, "20")
	seedPortalInvoice(t, db, orgID, clientID, models.InvoiceStatusPaid, "30")

	rows, err := q.ClientPortalInvoices(orgID, clientID, 50)
	require.NoError(t, err)
	assert.Len(t, rows, 2, "drafts are excluded")

	// The cap bounds the list.
	capped, err := q.ClientPortalInvoices(orgID, clientID, 1)
	require.NoError(t, err)
	assert.Len(t, capped, 1)
}

func TestClientPortalPayTokensAndPayments(t *testing.T) {
	db := portalTestDB(t)
	q := &ClientPortalQueries{DB: db}
	orgID := uuid.New()
	clientID := seedPortalClient(t, db, orgID)
	invoiceID := seedPortalInvoice(t, db, orgID, clientID, models.InvoiceStatusSent, "100")
	require.NoError(t, db.Create(&models.PaymentLink{Token: "pay-token", InvoiceID: invoiceID, UserID: orgID, CreatedAt: time.Now()}).Error)

	tokens, err := q.ClientPortalPayTokens(orgID, clientID)
	require.NoError(t, err)
	assert.Equal(t, "pay-token", tokens[invoiceID])

	now := time.Now()
	require.NoError(t, db.Create(&models.Payment{
		ID: uuid.New(), OrgID: orgID, UserID: orgID, InvoiceID: invoiceID,
		Amount: decimal.RequireFromString("40"), Method: "Cash", PaidOn: &now, CreatedAt: now,
	}).Error)
	voided := now.Add(-time.Hour)
	require.NoError(t, db.Create(&models.Payment{
		ID: uuid.New(), OrgID: orgID, UserID: orgID, InvoiceID: invoiceID,
		Amount: decimal.RequireFromString("10"), Method: "Cash", PaidOn: &now,
		CreatedAt: now, VoidedAt: &voided,
	}).Error)

	payments, err := q.ClientPortalPayments(orgID, clientID, 50)
	require.NoError(t, err)
	require.Len(t, payments, 1, "voided payments are excluded")
	assert.Equal(t, "40", payments[0].Amount.String())
	assert.Equal(t, "Cash", payments[0].Method)
}
