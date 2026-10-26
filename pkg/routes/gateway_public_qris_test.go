package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/platform/database"
)

// newQRISCoreStub serves fake Midtrans Core API charge responses: every call
// bumps the counter and returns its own QR image URL, so a test can tell a
// reused intent (one charge) from a regenerated one (a fresh charge). expiry
// is Midtrans' WIB shape ("2006-01-02 15:04:05").
func newQRISCoreStub(t *testing.T, expiry string) (srv *httptest.Server, calls *atomic.Int32) {
	t.Helper()
	calls = &atomic.Int32{}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"transaction_id":"txn-qris-%d","order_id":"x","payment_type":"qris","transaction_status":"pending","expiry_time":%q,"qr_string":"000201-payload-%d","actions":[{"name":"generate-qr-code","url":"https://api.test/qris/%d"}]}`,
			n, expiry, n, n)
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

// qrisFixture registers an owner restricted to QRIS, opens an unpaid IDR
// invoice and returns its payment-link token.
func qrisFixture(t *testing.T, app *fiber.App, email string) (cookies []*http.Cookie, token string) {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"QRIS Lab","email":"`+email+`","password":"secret123"}`, nil)
	require.Equal(t, 201, resp.StatusCode)
	cookies = resp.Cookies()
	resp.Body.Close()

	resp = doRequest(t, app, "PATCH", "/api/settings",
		`{"company_name":"QRIS Lab","currency":"IDR","tax_rate":0,"invoice_prefix":"INV-","provider_methods":"qris"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	spec := newInvoice()
	spec.ClientID = createClient(t, app, cookies, "QRIS Payer")
	invoiceID := createInvoiceID(t, app, cookies, spec)

	resp = doRequest(t, app, "POST", "/api/payments/online", `{"invoiceId":"`+invoiceID+`"}`, cookies)
	require.Equal(t, 200, resp.StatusCode)
	token = decodeBody(t, resp)["token"].(string)
	return cookies, token
}

// A QRIS intent whose provider expiry is still in the future is reused for
// every click: one Midtrans charge, one order id, one QR image. Regenerating
// on every click would burn gateway quota and invalidate the QR on screen.
func TestPublicPayQrisReusedWhileLive(t *testing.T) {
	core, calls := newQRISCoreStub(t, "2099-01-01 00:00:00")
	t.Setenv("MIDTRANS_SERVER_KEY", "qris-live-key")
	t.Setenv("MIDTRANS_CORE_BASE_URL", core.URL)

	app := newTestApp()
	_, token := qrisFixture(t, app, "qris-live@example.com")

	resp := doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	first := decodeBody(t, resp)
	require.Equal(t, "https://api.test/qris/1", first["payment_url"])
	require.Equal(t, "000201-payload-1", first["qr_string"])
	require.Equal(t, "2099-01-01T00:00:00+07:00", first["expires_at"])

	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	second := decodeBody(t, resp)
	assert.Equal(t, first["order_id"], second["order_id"], "live intent must be reused, not recharged")
	assert.Equal(t, first["payment_url"], second["payment_url"])
	assert.Equal(t, first["qr_string"], second["qr_string"], "the stored payload must be served on reuse, not just on charge")
	assert.EqualValues(t, 1, calls.Load(), "second click must not hit the Core API again")
}

// Once the stored intent is past the provider's expiry the next click opens a
// fresh charge under a retry suffix (-r1, -r2, ...): Midtrans rejects a
// duplicate order id, and the old QR can no longer be scanned.
func TestPublicPayQrisRegeneratesAfterExpiry(t *testing.T) {
	core, calls := newQRISCoreStub(t, "2000-01-01 00:00:00")
	t.Setenv("MIDTRANS_SERVER_KEY", "qris-expired-key")
	t.Setenv("MIDTRANS_CORE_BASE_URL", core.URL)

	app := newTestApp()
	_, token := qrisFixture(t, app, "qris-expired@example.com")

	resp := doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	first := decodeBody(t, resp)
	require.Equal(t, "https://api.test/qris/1", first["payment_url"])

	// The stored intent is already expired (expiry_time was in the past), so
	// the retry must charge again instead of handing back the dead QR.
	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	second := decodeBody(t, resp)
	assert.Equal(t, first["order_id"].(string)+"-r1", second["order_id"])
	assert.Equal(t, "https://api.test/qris/2", second["payment_url"])
	assert.EqualValues(t, 2, calls.Load())

	// The retry is expired too (same stubbed expiry): one more charge, -r2.
	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	third := decodeBody(t, resp)
	assert.Equal(t, first["order_id"].(string)+"-r2", third["order_id"])
	assert.Equal(t, "https://api.test/qris/3", third["payment_url"])
	assert.EqualValues(t, 3, calls.Load())
}

// A failed row (e.g. provider denied) is never reused either: the payer has
// to be able to start over on the same public page.
func TestPublicPayRetryAfterFailedIntent(t *testing.T) {
	core, calls := newQRISCoreStub(t, "2099-01-01 00:00:00")
	t.Setenv("MIDTRANS_SERVER_KEY", "qris-failed-key")
	t.Setenv("MIDTRANS_CORE_BASE_URL", core.URL)

	app := newTestApp()
	_, token := qrisFixture(t, app, "qris-failed@example.com")

	resp := doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	first := decodeBody(t, resp)
	resp.Body.Close()

	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	stored, err := db.GetTransaction(first["order_id"].(string))
	require.NoError(t, err)
	stored.Status = "failed"
	require.NoError(t, db.SaveTransaction(&stored))

	resp = doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
	require.Equal(t, 200, resp.StatusCode)
	second := decodeBody(t, resp)
	assert.Equal(t, first["order_id"].(string)+"-r1", second["order_id"])
	assert.EqualValues(t, 2, calls.Load(), "a failed intent must be replaced by a new charge")
}

// Two concurrent clicks (StrictMode double-effect, double-tap) must collapse
// onto one intent: no 500, same order id and QR on both responses, exactly
// one Core API charge. The stub sleeps inside the charge so both requests
// claim the base order id before either charge resolves; the loser waits for
// the winner instead of opening its own charge (zero orphan at the gateway).
func TestPublicPayQrisConcurrentClicksCollapse(t *testing.T) {
	calls := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"transaction_id":"txn-qris-race-%d","order_id":"x","payment_type":"qris","transaction_status":"pending","expiry_time":"2099-01-01 00:00:00","actions":[{"name":"generate-qr-code","url":"https://api.test/qris/race/%d"}]}`,
			n, n)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MIDTRANS_SERVER_KEY", "qris-race-key")
	t.Setenv("MIDTRANS_CORE_BASE_URL", srv.URL)

	app := newTestApp()
	_, token := qrisFixture(t, app, "qris-race@example.com")

	type outcome struct {
		status int
		body   map[string]interface{}
	}
	results := make([]outcome, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := doRequest(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil)
			results[i] = outcome{status: resp.StatusCode, body: decodeBody(t, resp)}
		}(i)
	}
	wg.Wait()

	require.Equal(t, 200, results[0].status, "concurrent click must not 500")
	require.Equal(t, 200, results[1].status, "concurrent click must not 500")
	assert.Equal(t, results[0].body["order_id"], results[1].body["order_id"], "both clicks collapse onto one intent")
	assert.Equal(t, results[0].body["payment_url"], results[1].body["payment_url"])
	assert.NotEmpty(t, results[0].body["payment_url"])
	assert.EqualValues(t, 1, calls.Load(), "the loser must wait and reuse, never charge the gateway twice")
}

// The same Idempotency-Key replays the first charge: one Core API call, the
// second response carries the replay marker. This is the enterprise path the
// MPA uses (one UUID per QR generation); the race test above is the safety
// net for clients that send none.
func TestPublicPayQrisIdempotencyKeyReplay(t *testing.T) {
	core, calls := newQRISCoreStub(t, "2099-01-01 00:00:00")
	t.Setenv("MIDTRANS_SERVER_KEY", "qris-idem-key")
	t.Setenv("MIDTRANS_CORE_BASE_URL", core.URL)

	app := newTestApp()
	_, token := qrisFixture(t, app, "qris-idem@example.com")

	headers := map[string]string{"Idempotency-Key": "qris-replay-1"}
	resp := doRequestWithHeaders(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil, headers)
	status, first, respHeaders := readBody(t, resp)
	require.Equal(t, 200, status)
	assert.Empty(t, respHeaders.Get("Idempotent-Replayed"))

	resp = doRequestWithHeaders(t, app, "POST", "/api/public/pay/"+token+"/transaction", `{"payment_method":"qris"}`, nil, headers)
	status, second, respHeaders := readBody(t, resp)
	require.Equal(t, 200, status)
	assert.Equal(t, "true", respHeaders.Get("Idempotent-Replayed"))
	assert.Equal(t, first["order_id"], second["order_id"])
	assert.EqualValues(t, 1, calls.Load(), "replay must not recharge the gateway")
}
