package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postJSONOnce posts without the require-based helper, because a failed
// assertion inside a goroutine is not allowed to abort the test.
func postJSONOnce(app *fiber.App, path, body string, cookies []*http.Cookie) (int, error) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if cookie.Name == "csrf_token" && cookie.Value != "" {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// TestConcurrentPaymentCannotOverpay proves the balance guard runs inside the
// inserting transaction: two payments of 60 racing on a 100 invoice leave
// exactly one row behind, instead of both reading a zero balance and both
// landing (which would overpay the invoice).
func TestConcurrentPaymentCannotOverpay(t *testing.T) {
	app := newTestApp()

	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Race Payer","email":"`+uniqueEmail("payrace")+`","password":"secret123"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	cookies := resp.Cookies()
	decodeBody(t, resp)

	spec := newInvoice()
	spec.ClientID, spec.Currency = createClient(t, app, cookies, "Payer"), "USD"
	spec.Items = []invoiceLine{{Description: "Service", Quantity: 1, Rate: "100"}}
	invoiceID := createInvoiceID(t, app, cookies, spec)

	body := `{"invoiceId":"` + invoiceID + `","amount":60,"method":"Cash","paid_on":"2026-09-21"}`

	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, err := postJSONOnce(app, "/api/payments", body, cookies)
			if err != nil {
				errs <- err
				return
			}
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent payment failed: %v", err)
	}
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	assert.Equal(t, 1, counts[http.StatusCreated], "exactly one payment may land")
	assert.Equal(t, 1, counts[http.StatusBadRequest], "the other must be refused")

	resp = doRequest(t, app, "GET", "/api/invoices/"+invoiceID, "", cookies)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	detail := decodeBody(t, resp)["invoice"].(map[string]interface{})
	assert.Equal(t, "60", detail["paid_amount"], "the invoice must stay inside its total")
	assert.Equal(t, "sent", detail["effective_status"])
}
