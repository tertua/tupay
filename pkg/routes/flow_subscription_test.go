package routes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/outbox"
)

// subscriptionSpec is the subscription payload. Subscriptions are a new domain
// (not client-visible invoice JSON), so this builder lives with its flow test
// rather than in flow_fixtures_test.go; check:fixtures only polices
// POST /api/invoices bodies.
type subscriptionSpec struct {
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	Cadence       string        `json:"cadence"`
	StartDate     string        `json:"start_date"`
	LeadDays      int           `json:"lead_days,omitempty"`
	Currency      string        `json:"currency"`
	TaxRate       float64       `json:"tax_rate,omitempty"`
	Discount      string        `json:"discount,omitempty"`
	PaymentMethod string        `json:"payment_method,omitempty"`
	InvoiceStatus string        `json:"invoice_status"`
	DueDays       int           `json:"due_days,omitempty"`
	ClientID      string        `json:"client_id,omitempty"`
	Items         []invoiceLine `json:"items"`
}

// newSubscription returns an active monthly subscription that generates a sent
// IDR invoice starting today, with one line.
func newSubscription(clientID string) subscriptionSpec {
	return subscriptionSpec{
		Name:          "Retainer",
		Status:        models.SubscriptionStatusActive,
		Cadence:       models.SubscriptionCadenceMonthly,
		StartDate:     utils.FormatTime(utils.DateOnly(time.Now())),
		Currency:      "IDR",
		InvoiceStatus: models.InvoiceStatusSent,
		DueDays:       30,
		ClientID:      clientID,
		Items:         []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100000"}},
	}
}

// body renders the spec as a JSON request body.
func (s subscriptionSpec) body(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	return string(raw)
}

// TestSubscriptionFlow covers the end-to-end sweep: a monthly subscription due
// today generates exactly one invoice and one run row (with linkage) across two
// ProcessOnce ticks, and a paused subscription generates nothing.
func TestSubscriptionFlow(t *testing.T) {
	app := newTestApp()
	owner := registerUser(t, app, "subscription-owner@example.com", "secret123")
	orgID := myOrgID(t, app, owner)
	clientID := createClient(t, app, owner, "Retainer Co")

	resp := doRequest(t, app, "POST", "/api/subscriptions", newSubscription(clientID).body(t), owner)
	require.Equal(t, 201, resp.StatusCode, "create subscription")
	sub := decodeBody(t, resp)["subscription"].(map[string]interface{})
	subscriptionID := sub["id"].(string)

	// A second, paused subscription with the same start must never generate.
	paused := newSubscription(clientID)
	paused.Name = "Paused retainer"
	paused.Status = models.SubscriptionStatusPaused
	resp = doRequest(t, app, "POST", "/api/subscriptions", paused.body(t), owner)
	require.Equal(t, 201, resp.StatusCode, "create paused subscription")
	resp.Body.Close()

	db, err := database.OpenDBConnection()
	require.NoError(t, err)

	// Two sweeps: the claim makes the second a no-op.
	outbox.New().ProcessOnce(context.Background())
	outbox.New().ProcessOnce(context.Background())

	var invoices int64
	require.NoError(t, db.InvoiceQueries.Model(&models.Invoice{}).Where("org_id = ?", orgID).Count(&invoices).Error)
	assert.Equal(t, int64(1), invoices, "exactly one generated invoice across two sweeps")

	var runs int64
	require.NoError(t, db.InvoiceQueries.Model(&models.SubscriptionRun{}).Where("org_id = ?", orgID).Count(&runs).Error)
	assert.Equal(t, int64(1), runs, "exactly one claim row across two sweeps")

	// Linkage: the run row points at the generated invoice.
	var run models.SubscriptionRun
	require.NoError(t, db.InvoiceQueries.Where("org_id = ?", orgID).First(&run).Error)
	require.NotNil(t, run.InvoiceID, "run row links the generated invoice")

	var invoice models.Invoice
	require.NoError(t, db.InvoiceQueries.Where("id = ?", *run.InvoiceID).First(&invoice).Error)
	assert.Equal(t, orgID, invoice.OrgID.String())
	assert.Equal(t, models.InvoiceStatusSent, invoice.Status)

	// The cursor advanced past the occurrence.
	var after models.Subscription
	require.NoError(t, db.InvoiceQueries.Where("id = ?", subscriptionID).First(&after).Error)
	require.NotNil(t, after.NextRunDate)
	assert.True(t, after.NextRunDate.After(run.RunDate), "cursor advanced past the run date")

	// Tidy up so later global sweeps in the shared DB stay unaffected. Item
	// tables carry no org_id, so they are scoped through their parent rows.
	db.InvoiceQueries.Where("subscription_id IN (?)",
		db.InvoiceQueries.Model(&models.Subscription{}).Select("id").Where("org_id = ?", orgID)).
		Delete(&models.SubscriptionItem{})
	db.InvoiceQueries.Where("invoice_id IN (?)",
		db.InvoiceQueries.Model(&models.Invoice{}).Select("id").Where("org_id = ?", orgID)).
		Delete(&models.InvoiceItem{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.SubscriptionRun{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.Subscription{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.Invoice{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.Client{})
}
