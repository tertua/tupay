package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public pay page lists QRIS as the only Midtrans method (Midtrans is
// locked to QRIS) plus crypto from the other provider, and only opens the
// gateway intent after a method is chosen. A USD invoice is shown converted
// (IDR via the manual rate) for the IDR-only provider, and unconverted for
// the crypto provider.
func TestPublicPayMethodSelection(t *testing.T) {
	npServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inv_pub","invoice_url":"https://nowpayments.io/payment/?iid=pub"}`))
	}))
	defer npServer.Close()

	t.Setenv("MIDTRANS_SERVER_KEY", "pub-server-key")
	t.Setenv("NOWPAYMENTS_API_KEY", "pub-np-key")
	t.Setenv("NOWPAYMENTS_BASE_URL", npServer.URL+"/v1")
	t.Setenv("NOWPAYMENTS_SANDBOX", "true")

	app := newTestApp()

	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Public MP","email":"public-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 201, resp.StatusCode)
	decodeBody(t, resp)
	cookies := resp.Cookies()

	// Manual rate: $1 = Rp18000.
	resp = doRequest(t, app, "PATCH", "/api/settings",
		`{"company_name":"Public MP","currency":"USD","tax_rate":0,"invoice_prefix":"INV-","usd_to_idr":"18000"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	spec := newInvoice()
	spec.ClientID, spec.Currency = createClient(t, app, cookies, "Public Payer USD"), "USD"
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100"}}
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp = doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["token"].(string)

	resp = doRequest(t, app, "GET", "/api/public/pay/"+token, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	publicData := decodeBody(t, resp)
	methods := publicData["methods"].([]interface{})
	require.NotEmpty(t, methods)
	byID := map[string]map[string]interface{}{}
	for _, m := range methods {
		entry := m.(map[string]interface{})
		byID[entry["id"].(string)] = entry
	}
	require.Contains(t, byID, "qris")
	assert.Equal(t, "IDR", byID["qris"]["currency"])
	assert.Equal(t, "1800000", byID["qris"]["amount"]) // $100 × 18000
	require.Contains(t, byID, "crypto")
	assert.Equal(t, "USD", byID["crypto"]["currency"])
	assert.Equal(t, "100", byID["crypto"]["amount"])
	resp.Body.Close()

	// Choosing gopay is rejected: QRIS is the only Midtrans method.
	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"gopay"}`, nil)
	assert.Equal(t, 400, resp.StatusCode)
	resp.Body.Close()

	// Choosing crypto routes to NOWPayments and returns a hosted payment URL.
	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"crypto"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	cryptoIntent := decodeBody(t, resp)
	assert.Equal(t, "https://nowpayments.io/payment/?iid=pub", cryptoIntent["payment_url"])
	// A hosted payment-page id is never exposed as a widget token: the
	// snap_token key only appears for BrowserSDKProvider tokens.
	assert.Empty(t, cryptoIntent["snap_token"])
	assert.Empty(t, cryptoIntent["provider_token"], "provider-neutral alias is exposed only with snap_token")
	resp.Body.Close()
}

// Midtrans is locked to QRIS: the public page shows only qris (plus crypto
// from the other provider), a non-QRIS Midtrans method is hidden and
// rejected, and a non-Midtrans method (crypto) is unaffected.
func TestPublicPayMidtransMethodAllowlist(t *testing.T) {
	snapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"snap-allow","redirect_url":"https://snap.test/allow"}`))
	}))
	defer snapServer.Close()
	npServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inv_allow","invoice_url":"https://nowpayments.io/payment/?iid=allow"}`))
	}))
	defer npServer.Close()
	coreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_id":"txn-allow","order_id":"x","payment_type":"qris","transaction_status":"pending","expiry_time":"2026-09-25 10:30:00","actions":[{"name":"generate-qr-code","url":"https://api.test/qris/allow/qr-code"}]}`))
	}))
	defer coreServer.Close()

	t.Setenv("MIDTRANS_SERVER_KEY", "allow-server-key")
	t.Setenv("MIDTRANS_SNAP_BASE_URL", snapServer.URL)
	t.Setenv("MIDTRANS_CORE_BASE_URL", coreServer.URL)
	t.Setenv("NOWPAYMENTS_API_KEY", "allow-np-key")
	t.Setenv("NOWPAYMENTS_BASE_URL", npServer.URL+"/v1")
	t.Setenv("NOWPAYMENTS_SANDBOX", "true")

	app := newTestApp()

	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Allow MP","email":"allow-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"allow-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	cookies := resp.Cookies()
	resp.Body.Close()

	// Allow only QRIS on Midtrans (plus the manual rate for the USD invoice).
	resp = doRequest(t, app, "PATCH", "/api/settings",
		`{"company_name":"Allow MP","currency":"USD","tax_rate":0,"invoice_prefix":"INV-","usd_to_idr":"18000","midtrans_methods":"qris,nonsense"}`,
		cookies)
	require.Equal(t, 200, resp.StatusCode)
	settings := decodeBody(t, resp)["settings"].(map[string]interface{})
	assert.Equal(t, "qris", settings["provider_methods"]) // unknown id dropped; input alias accepted
	resp.Body.Close()

	spec := newInvoice()
	spec.ClientID, spec.Currency = createClient(t, app, cookies, "Allow Payer"), "USD"
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100"}}
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp = doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["token"].(string)

	resp = doRequest(t, app, "GET", "/api/public/pay/"+token, "", nil)
	require.Equal(t, 200, resp.StatusCode)
	methods := decodeBody(t, resp)["methods"].([]interface{})
	ids := map[string]bool{}
	for _, m := range methods {
		ids[m.(map[string]interface{})["id"].(string)] = true
	}
	assert.True(t, ids["qris"])
	assert.False(t, ids["gopay"], "gopay should be hidden by the allowlist")
	assert.True(t, ids["crypto"], "the Midtrans allowlist must not hide crypto")
	resp.Body.Close()

	// A method hidden by the allowlist is rejected, not silently rerouted.
	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"gopay"}`, nil)
	assert.Equal(t, 400, resp.StatusCode)
	resp.Body.Close()

	// The allowed method still works: QRIS now returns an on-page QR image
	// (Core API, acquirer=gopay) instead of a Snap token.
	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	qrisIntent := decodeBody(t, resp)
	assert.Equal(t, "https://api.test/qris/allow/qr-code", qrisIntent["payment_url"])
	assert.Empty(t, qrisIntent["snap_token"])
	assert.Empty(t, qrisIntent["provider_token"], "provider_token aliases snap_token")
	assert.Equal(t, "2026-09-25T10:30:00+07:00", qrisIntent["expires_at"])
	resp.Body.Close()
}

// A USD invoice paid via crypto with a pay_currency returns an on-page USDT
// deposit address (no hosted redirect URL) so the payer stays on the page.
func TestPublicPayCryptoWidgetNoRedirect(t *testing.T) {
	snapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"snap-widget","redirect_url":"https://snap.test/widget"}`))
	}))
	defer snapServer.Close()
	sawPayment := false
	npServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/payment" {
			sawPayment = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"payment_id":"pay_widget","pay_address":"TXUSDTTRC20","pay_amount":"100","pay_currency":"usdttrc20","price_amount":"100","price_currency":"USD","order_id":"INV-1"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inv_widget","invoice_url":"https://nowpayments.io/payment/?iid=widget"}`))
	}))
	defer npServer.Close()

	t.Setenv("MIDTRANS_SERVER_KEY", "widget-server-key")
	t.Setenv("MIDTRANS_SNAP_BASE_URL", snapServer.URL)
	t.Setenv("NOWPAYMENTS_API_KEY", "widget-np-key")
	t.Setenv("NOWPAYMENTS_BASE_URL", npServer.URL+"/v1")
	t.Setenv("NOWPAYMENTS_SANDBOX", "true")

	app := newTestApp()

	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Widget MP","email":"widget-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"widget-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	cookies := resp.Cookies()
	resp.Body.Close()

	resp = doRequest(t, app, "PATCH", "/api/settings",
		`{"company_name":"Widget MP","currency":"USD","tax_rate":0,"invoice_prefix":"INV-","usd_to_idr":"18000"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	spec := newInvoice()
	spec.ClientID, spec.Currency = createClient(t, app, cookies, "Widget Payer"), "USD"
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100"}}
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp = doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["token"].(string)

	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"crypto","pay_currency":"usdttrc20"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	intent := decodeBody(t, resp)
	assert.True(t, sawPayment, "expected the direct /payment endpoint to be hit")
	assert.Equal(t, "TXUSDTTRC20", intent["address"])
	assert.Equal(t, "100", intent["pay_amount"])
	assert.Equal(t, "usdttrc20", intent["pay_currency"])
	assert.Empty(t, intent["payment_url"], "no redirect; the widget stays on page")
	// NOWPayments has no browser SDK widget, so no snap_token key is exposed.
	assert.Empty(t, intent["snap_token"])
	assert.Empty(t, intent["provider_token"], "provider-neutral alias is exposed only with snap_token")
	resp.Body.Close()
}

func TestPublicPayCryptoAmountBelowMinimum(t *testing.T) {
	npServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":false,"statusCode":400,"code":"AMOUNT_MINIMAL_ERROR","message":"Crypto amount 10.485047 is less than minimal"}`))
	}))
	defer npServer.Close()

	t.Setenv("MIDTRANS_SERVER_KEY", "min-server-key")
	t.Setenv("NOWPAYMENTS_API_KEY", "min-np-key")
	t.Setenv("NOWPAYMENTS_BASE_URL", npServer.URL+"/v1")
	t.Setenv("NOWPAYMENTS_SANDBOX", "true")

	app := newTestApp()

	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Min MP","email":"min-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"min-mp@example.com","password":"secret123"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	cookies := resp.Cookies()
	resp.Body.Close()

	resp = doRequest(t, app, "PATCH", "/api/settings",
		`{"company_name":"Min MP","currency":"USD","tax_rate":0,"invoice_prefix":"INV-","usd_to_idr":"18000"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	spec := newInvoice()
	spec.ClientID, spec.Currency = createClient(t, app, cookies, "Min Payer"), "USD"
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "10"}}
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp = doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["token"].(string)

	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"crypto","pay_currency":"usdttrc20"}`, nil)
	require.Equal(t, 400, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, "payment amount is below the gateway minimum", body["error"].(map[string]any)["message"])
	details := body["error"].(map[string]any)["details"].(map[string]any)
	assert.Equal(t, "amount_below_minimum", details["code"])
	resp.Body.Close()
}
