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

// templateSpec is the recurring-invoice-template payload. Templates are a new
// domain (not client-visible invoice JSON), so this builder lives with its
// flow test rather than in flow_fixtures_test.go; check:fixtures only polices
// POST /api/invoices bodies.
type templateSpec struct {
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

// newTemplate returns an active monthly template that generates a sent IDR
// invoice starting today, with one line.
func newTemplate(clientID string) templateSpec {
	return templateSpec{
		Name:          "Retainer",
		Status:        models.InvoiceTemplateStatusActive,
		Cadence:       models.InvoiceTemplateCadenceMonthly,
		StartDate:     utils.FormatTime(utils.DateOnly(time.Now())),
		Currency:      "IDR",
		InvoiceStatus: models.InvoiceStatusSent,
		DueDays:       30,
		ClientID:      clientID,
		Items:         []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100000"}},
	}
}

// body renders the spec as a JSON request body.
func (s templateSpec) body(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	return string(raw)
}

// TestRecurringFlow covers the end-to-end sweep: a monthly template due today
// generates exactly one invoice and one run row (with linkage) across two
// ProcessOnce ticks, and a paused template generates nothing.
func TestRecurringFlow(t *testing.T) {
	app := newTestApp()
	owner := registerUser(t, app, "recurring-owner@example.com", "secret123")
	orgID := myOrgID(t, app, owner)
	clientID := createClient(t, app, owner, "Retainer Co")

	resp := doRequest(t, app, "POST", "/api/invoice-templates", newTemplate(clientID).body(t), owner)
	require.Equal(t, 201, resp.StatusCode, "create recurring template")
	tpl := decodeBody(t, resp)["invoice_template"].(map[string]interface{})
	templateID := tpl["id"].(string)

	// A second, paused template with the same start must never generate.
	paused := newTemplate(clientID)
	paused.Name = "Paused retainer"
	paused.Status = models.InvoiceTemplateStatusPaused
	resp = doRequest(t, app, "POST", "/api/invoice-templates", paused.body(t), owner)
	require.Equal(t, 201, resp.StatusCode, "create paused template")
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
	require.NoError(t, db.InvoiceQueries.Model(&models.InvoiceTemplateRun{}).Where("org_id = ?", orgID).Count(&runs).Error)
	assert.Equal(t, int64(1), runs, "exactly one claim row across two sweeps")

	// Linkage: the run row points at the generated invoice.
	var run models.InvoiceTemplateRun
	require.NoError(t, db.InvoiceQueries.Where("org_id = ?", orgID).First(&run).Error)
	require.NotNil(t, run.InvoiceID, "run row links the generated invoice")

	var invoice models.Invoice
	require.NoError(t, db.InvoiceQueries.Where("id = ?", *run.InvoiceID).First(&invoice).Error)
	assert.Equal(t, orgID, invoice.OrgID.String())
	assert.Equal(t, models.InvoiceStatusSent, invoice.Status)

	// The cursor advanced past the occurrence.
	var after models.InvoiceTemplate
	require.NoError(t, db.InvoiceQueries.Where("id = ?", templateID).First(&after).Error)
	require.NotNil(t, after.NextRunDate)
	assert.True(t, after.NextRunDate.After(run.RunDate), "cursor advanced past the run date")

	// Tidy up so later global sweeps in the shared DB stay unaffected. Item
	// tables carry no org_id, so they are scoped through their parent rows.
	db.InvoiceQueries.Where("template_id IN (?)",
		db.InvoiceQueries.Model(&models.InvoiceTemplate{}).Select("id").Where("org_id = ?", orgID)).
		Delete(&models.InvoiceTemplateItem{})
	db.InvoiceQueries.Where("invoice_id IN (?)",
		db.InvoiceQueries.Model(&models.Invoice{}).Select("id").Where("org_id = ?", orgID)).
		Delete(&models.InvoiceItem{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.InvoiceTemplateRun{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.InvoiceTemplate{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.Invoice{})
	db.InvoiceQueries.Where("org_id = ?", orgID).Delete(&models.Client{})
}
