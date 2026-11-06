package routes

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/database"
)

// totpCode mints a live authenticator code for a secret.
func totpCode(t *testing.T, secret string) string {
	t.Helper()
	return totpCodeIn(t, secret, 0)
}

// totpCodeIn mints a code for a step offset (in 30-second units) from now; the
// accepted skew is ±1 step, so +1 is still valid and lands on a fresh step.
func totpCodeIn(t *testing.T, secret string, steps int) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().Add(time.Duration(steps)*models.TOTPPeriod*time.Second))
	require.NoError(t, err)
	return code
}

// enrollTOTP runs the enroll + activate round trip for a session and returns
// the shared secret. Activation spends one step, so callers present a code from
// the next step (still inside the accepted skew).
func enrollTOTP(t *testing.T, app *fiber.App, cookies []*http.Cookie) string {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/auth/totp/enroll", "", cookies)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	secret := decodeBody(t, resp)["totp"].(map[string]interface{})["secret"].(string)
	require.NotEmpty(t, secret)

	resp = doRequest(t, app, "POST", "/api/auth/totp/activate", `{"code":"`+totpCode(t, secret)+`"}`, cookies)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
	return secret
}

// cleanupUser removes a test account so shared user counts stay exact.
func cleanupUser(t *testing.T, email string) {
	t.Helper()
	db, err := database.OpenDBConnection()
	require.NoError(t, err)
	require.NoError(t, db.InvoiceQueries.DB.Where("email = ?", email).Delete(&models.User{}).Error)
}

// TestTOTPEnrollAndLoginFlow covers the second factor end to end: enrollment
// stays inactive until confirmed, login then demands a code, and a replayed
// code is refused.
func TestTOTPEnrollAndLoginFlow(t *testing.T) {
	app := newTestApp()
	email := uniqueEmail("totp")
	cookies := adminSession(t, app, "Totp Admin", email)

	// A fresh account reports no second factor.
	resp := doRequest(t, app, "GET", "/api/auth/totp", "", cookies)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, false, decodeBody(t, resp)["totp"].(map[string]interface{})["enabled"])

	secret := enrollTOTP(t, app, cookies)

	// A second enrollment is refused once active.
	resp = doRequest(t, app, "POST", "/api/auth/totp/enroll", "", cookies)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	decodeBody(t, resp)

	// Login now needs the code; a password-only attempt is refused.
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"secret123"}`, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	details := decodeBody(t, resp)["error"].(map[string]interface{})["details"].(map[string]interface{})
	assert.Equal(t, "totp_required", details["code"])

	// The next step's code completes the login (activation spent the current one).
	code := totpCodeIn(t, secret, 1)
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"secret123","code":"`+code+`"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Replaying the same code is refused.
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"secret123","code":"`+code+`"}`, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	details = decodeBody(t, resp)["error"].(map[string]interface{})["details"].(map[string]interface{})
	assert.Equal(t, "totp_invalid", details["code"])

	cleanupUser(t, email)
}

// TestTOTPDisableRequiresBothFactors proves turning the second factor off needs
// the password and a live code.
func TestTOTPDisableRequiresBothFactors(t *testing.T) {
	app := newTestApp()
	email := uniqueEmail("totpoff")
	cookies := adminSession(t, app, "Totp Off", email)
	secret := enrollTOTP(t, app, cookies)

	// Wrong password is refused even with a valid code.
	resp := doRequest(t, app, "POST", "/api/auth/totp/disable",
		`{"password":"wrongpass","code":"`+totpCodeIn(t, secret, 1)+`"}`, cookies)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	decodeBody(t, resp)

	// Both factors together succeed.
	resp = doRequest(t, app, "POST", "/api/auth/totp/disable",
		`{"password":"secret123","code":"`+totpCodeIn(t, secret, 1)+`"}`, cookies)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp = doRequest(t, app, "GET", "/api/auth/totp", "", cookies)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, false, decodeBody(t, resp)["totp"].(map[string]interface{})["enabled"])

	cleanupUser(t, email)
}
