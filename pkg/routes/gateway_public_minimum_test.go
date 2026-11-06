package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The same invoice can clear the LTC minimum while sitting under USDT (BSC),
// so the list marks the row below_minimum with the provider's own number
// instead of dropping it.
func TestPublicPayCryptoMinimumPerAsset(t *testing.T) {
	npServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/min-amount" {
			to := r.URL.Query().Get("currency_to")
			min := map[string]string{"usdtbsc": "18.81", "ltc": "1.00"}[to]
			if min == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"currency_from":"usd","currency_to":"` + to + `","min_amount":` + min + `}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"inv_minasset","invoice_url":"https://nowpayments.io/payment/?iid=minasset"}`))
	}))
	defer npServer.Close()

	t.Setenv("MIDTRANS_SERVER_KEY", "min-asset-server-key")
	t.Setenv("NOWPAYMENTS_API_KEY", "min-asset-np-key")
	t.Setenv("NOWPAYMENTS_BASE_URL", npServer.URL+"/v1")
	t.Setenv("NOWPAYMENTS_SANDBOX", "true")

	app := newTestApp()

	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Min Asset","email":"min-asset@example.com","password":"secret123"}`, nil)
	require.Equal(t, 201, resp.StatusCode)
	cookies := resp.Cookies()
	resp.Body.Close()

	resp = doRequest(t, app, "PATCH", "/api/settings",
		`{"company_name":"Min Asset","currency":"USD","tax_rate":0,"invoice_prefix":"INV-","usd_to_idr":"18000"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	spec := newInvoice()
	spec.ClientID, spec.Currency = createClient(t, app, cookies, "Min Asset Payer"), "USD"
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "10"}}
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp = doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token := decodeBody(t, resp)["token"].(string)
	resp.Body.Close()

	cryptoMethod := func(query string) map[string]interface{} {
		r := doRequest(t, app, "GET", "/api/public/pay/"+token+query, "", nil)
		require.Equal(t, 200, r.StatusCode)
		defer r.Body.Close()
		for _, m := range decodeBody(t, r)["methods"].([]interface{}) {
			if m.(map[string]interface{})["id"].(string) == "crypto" {
				return m.(map[string]interface{})
			}
		}
		return nil
	}

	assert.NotNil(t, cryptoMethod(""), "no asset picked yet: crypto stays offered and unmarked")
	assert.Nil(t, cryptoMethod("")["below_minimum"])
	blocked := cryptoMethod("?pay_currency=usdtbsc")
	require.NotNil(t, blocked, "$10 under the USDT (BSC) minimum stays listed so the payer learns why")
	assert.Equal(t, true, blocked["below_minimum"])
	assert.Equal(t, "18.81", blocked["min_amount"])
	assert.Nil(t, cryptoMethod("?pay_currency=ltc")["below_minimum"], "the same invoice clears the LTC minimum")
}
