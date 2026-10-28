package routes

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginLockoutFlow pins the per-account sign-in lockout: after the threshold
// of bad credentials the account is refused with 429 even when the password is
// correct (a distributed spray cannot keep guessing), the refusal carries
// Retry-After, no session is minted, and the lock lifts on its own.
func TestLoginLockoutFlow(t *testing.T) {
	t.Setenv("RATE_LIMIT_AUTH", "100")
	t.Setenv("AUTH_LOCKOUT_THRESHOLD", "3")
	t.Setenv("AUTH_LOCKOUT_BASE_SECONDS", "2")
	t.Setenv("AUTH_LOCKOUT_MAX_SECONDS", "5")
	app := newTestApp()

	const email = "lockout-flow@example.com"
	const password = "secret123"
	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Lockout","email":"`+email+`","password":"`+password+`"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	for i := 0; i < 3; i++ {
		resp = doRequest(t, app, "POST", "/api/auth/login",
			`{"email":"`+email+`","password":"wrong-pass-`+strconv.Itoa(i)+`"}`, nil)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	}

	// Locked: the real password is refused and the client is told when to retry.
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`, nil)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"), "a locked sign-in must tell the client when to retry")
	assert.Empty(t, resp.Cookies(), "a locked sign-in must not mint a session")
	resp.Body.Close()

	// The lock is time-boxed: the owner gets back in once it lifts.
	time.Sleep(2200 * time.Millisecond)
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, resp.Cookies())
	resp.Body.Close()
}

// TestLoginLockoutUnknownEmail proves throttling keys on the submitted address
// alone, so a probe cannot tell a registered account from an unregistered one by
// hammering it (same 429, no hint that the account exists).
func TestLoginLockoutUnknownEmail(t *testing.T) {
	t.Setenv("RATE_LIMIT_AUTH", "100")
	t.Setenv("AUTH_LOCKOUT_THRESHOLD", "2")
	t.Setenv("AUTH_LOCKOUT_BASE_SECONDS", "2")
	t.Setenv("AUTH_LOCKOUT_MAX_SECONDS", "5")
	app := newTestApp()

	const email = "no-such-user@example.com"
	for i := 0; i < 2; i++ {
		resp := doRequest(t, app, "POST", "/api/auth/login",
			`{"email":"`+email+`","password":"guess-`+strconv.Itoa(i)+`"}`, nil)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	}

	resp := doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"guess-again"}`, nil)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	resp.Body.Close()
}

// TestLoginLockoutIgnoresStatusGate pins that only bad credentials count: an
// unverified account presenting the RIGHT password is refused by the status
// gate, and repeating that must never arm a lockout — the password was not wrong,
// so counting it would trap every not-yet-verified user.
func TestLoginLockoutIgnoresStatusGate(t *testing.T) {
	app := newTestApp()
	verifyFlagOn(t, app)
	t.Setenv("AUTH_LOCKOUT_THRESHOLD", "2")
	t.Setenv("AUTH_LOCKOUT_BASE_SECONDS", "5")
	t.Setenv("AUTH_LOCKOUT_MAX_SECONDS", "5")

	email := uniqueEmail("pending-lockout")
	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"Pending","email":"`+email+`","password":"secret123"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	for i := 0; i < 3; i++ {
		resp = doRequest(t, app, "POST", "/api/auth/login",
			`{"email":"`+email+`","password":"secret123"}`, nil)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "attempt %d must stay a status gate, not a lockout", i+1)
		resp.Body.Close()
	}
}
