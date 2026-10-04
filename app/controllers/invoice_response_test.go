package controllers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/database"
)

func marshalKeys(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// TestInvoiceDetailJSONShape pins the internal detail key set and the
// external_id omitempty contract the gateway route relies on.
func TestInvoiceDetailJSONShape(t *testing.T) {
	detail := invoiceDetailResponse{
		ID:              uuid.New(),
		InvoiceNumber:   "INV-1",
		Status:          "sent",
		EffectiveStatus: "sent",
		ClientID:        &uuid.Nil,
		Items:           []invoiceItemResponse{},
		Payments:        []invoicePaymentResponse{},
	}

	keys := marshalKeys(t, detail)
	for _, want := range []string{
		"id", "invoice_number", "status", "effective_status", "payment_link",
		"client_id", "client_name", "client_company", "client_email",
		"issue_date", "due_date", "currency", "subtotal", "discount",
		"tax_rate", "tax_amount", "total", "notes", "terms",
		"payment_method", "items", "payments", "paid_amount", "balance",
	} {
		if _, ok := keys[want]; !ok {
			t.Errorf("internal detail missing key %q", want)
		}
	}
	if _, ok := keys["external_id"]; ok {
		t.Error("external_id must be absent when unset (non-gateway callers)")
	}
	if string(keys["payment_link"]) != "null" {
		t.Errorf("payment_link without link = %s, want null", keys["payment_link"])
	}
	if len(keys) != 24 {
		t.Errorf("internal detail has %d keys, want 24", len(keys))
	}

	external := "order-1"
	detail.ExternalID = &external
	if got := marshalKeys(t, detail)["external_id"]; string(got) != `"order-1"` {
		t.Errorf("external_id = %s, want \"order-1\"", got)
	}
}

// TestPublicInvoiceDetailJSONShape pins the payer-facing payload: the
// embedded detail's private fields disappear and the payment rows carry only
// the four public keys.
func TestPublicInvoiceDetailJSONShape(t *testing.T) {
	detail := invoiceDetailResponse{
		ID:              uuid.New(),
		InvoiceNumber:   "INV-1",
		Status:          "sent",
		EffectiveStatus: "sent",
		ClientID:        &uuid.Nil,
		ClientEmail:     "billing@example.com",
		PaymentLink:     &paymentLinkResponse{Token: "tok", URL: "/pay/tok"},
		Items:           []invoiceItemResponse{},
		Payments: []invoicePaymentResponse{{
			ID: uuid.New(), TxnID: "txn-1", CanVoid: true,
		}},
	}
	pub := publicInvoiceDetailResponse{
		invoiceDetailResponse: detail,
		Payments: []publicPaymentResponse{{
			ID: uuid.New(), Method: "qris",
		}},
	}

	keys := marshalKeys(t, pub)
	for _, hidden := range []string{"client_id", "client_email", "payment_link"} {
		if _, ok := keys[hidden]; ok {
			t.Errorf("public detail must not expose %q", hidden)
		}
	}
	for _, want := range []string{"id", "invoice_number", "effective_status", "client_name", "payments", "balance"} {
		if _, ok := keys[want]; !ok {
			t.Errorf("public detail missing key %q", want)
		}
	}
	if len(keys) != 21 {
		t.Errorf("public detail has %d keys, want 21 (internal 24 minus 3 hidden)", len(keys))
	}

	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(keys["payments"], &rows); err != nil {
		t.Fatalf("payments: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("payments rows = %d, want 1", len(rows))
	}
	if len(rows[0]) != 4 {
		t.Errorf("public payment row has %d keys, want 4 (id, amount, paid_on, method)", len(rows[0]))
	}
	for _, public := range []string{"id", "amount", "paid_on", "method"} {
		if _, ok := rows[0][public]; !ok {
			t.Errorf("public payment row missing %q", public)
		}
	}
	if _, ok := rows[0]["txn_id"]; ok {
		t.Error("public payment row must not leak txn_id")
	}
	if _, ok := rows[0]["can_void"]; ok {
		t.Error("public payment row must not leak can_void")
	}
}

// TestInvoiceDetailCarriesItemsAndPayments pins the regression where
// invoiceDetail built out.Items/out.Payments but returned a struct literal
// that dropped both, so every read path answered items: null while the rows
// sat in invoice_items (the "save & send, result empty" report).
func TestInvoiceDetailCarriesItemsAndPayments(t *testing.T) {
	db, err := database.OpenDBConnection()
	require.NoError(t, err)

	userID := approvalSeedUser(t, "detail-items@example.com")
	orgID := approvalSeedOrg(t, userID, "Detail Items Org")

	now := time.Now()
	invoice := &models.Invoice{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: &now,
		UserID:    userID,
		OrgID:     orgID,
		Status:    models.InvoiceStatusSent,
		IssueDate: &now,
		DueDate:   &now,
		Currency:  "IDR",
		Subtotal:  decimal.RequireFromString("375000"),
		Total:     decimal.RequireFromString("375000"),
	}
	client := models.Client{ID: uuid.New(), UserID: userID, OrgID: orgID, Name: "Detail Client"}
	require.NoError(t, db.CreateClient(&client))
	invoice.ClientID = &client.ID

	items := []models.InvoiceItem{
		{ID: uuid.New(), InvoiceID: invoice.ID, Description: "Desain", Quantity: 1,
			Rate: decimal.RequireFromString("150000"), Amount: decimal.RequireFromString("150000"), Position: 0},
		{ID: uuid.New(), InvoiceID: invoice.ID, Description: "Development", Quantity: 2,
			Rate: decimal.RequireFromString("100000"), Amount: decimal.RequireFromString("200000"), Position: 1},
	}
	require.NoError(t, db.CreateInvoice(orgID, invoice, items))

	paidOn := now
	payment := &models.Payment{
		ID: uuid.New(), UserID: userID, OrgID: orgID, InvoiceID: invoice.ID,
		Amount: decimal.RequireFromString("375000"), Method: "cash", PaidOn: &paidOn, CreatedAt: now,
	}
	require.NoError(t, db.CreatePayment(payment))

	detail, err := invoiceDetail(*db, orgID, invoice.ID)
	require.NoError(t, err)

	require.Len(t, detail.Items, 2, "invoiceDetail must return the persisted line items")
	assert.Equal(t, "Desain", detail.Items[0].Description)
	assert.Equal(t, "Development", detail.Items[1].Description)
	require.Len(t, detail.Payments, 1, "invoiceDetail must return the persisted payments")
	assert.Equal(t, decimal.RequireFromString("375000").String(), detail.PaidAmount.String())

	// The wire payload is what the editor reloads, so assert it too — the struct
	// could carry the rows and still fail to serialize them.
	var rows []json.RawMessage
	blob, err := json.Marshal(detail)
	require.NoError(t, err)
	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(blob, &wire))
	require.NoError(t, json.Unmarshal(wire["items"], &rows), "items must serialize as an array, not null")
	assert.Len(t, rows, 2, "serialized items rows")
	require.NoError(t, json.Unmarshal(wire["payments"], &rows))
	assert.Len(t, rows, 1, "serialized payments rows")
}
