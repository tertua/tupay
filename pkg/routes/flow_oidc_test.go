package routes

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIdP is an in-process OIDC provider (discovery + JWKS + authorize + token)
// so the SSO flow test needs no network or external service.
type fakeIdP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	signer jose.Signer

	// nextClaims is what the token endpoint signs for the next exchange.
	nextClaims map[string]any
	// discoveryHits counts /.well-known/openid-configuration fetches (cache test).
	discoveryHits atomic.Int64
	// omitIDToken makes /token succeed without an id_token (claim-gap test).
	omitIDToken bool
	// tokenOverride rewrites the claims the next exchange signs.
	tokenOverride func(map[string]any) map[string]any
	// badSigner, when set, signs the id_token (wrong key -> bad signature).
	badSigner jose.Signer
	// claimsByState lets concurrent flows pin claims per state (race test).
	claimsByState sync.Map
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	idp := &fakeIdP{key: key, signer: signer}
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		idp.discoveryHits.Add(1)
		writeJSON(w, map[string]any{
			"issuer":                                idp.server.URL,
			"authorization_endpoint":                idp.server.URL + "/authorize",
			"token_endpoint":                        idp.server.URL + "/token",
			"jwks_uri":                              idp.server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key: &key.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig",
		}}})
	})

	// Authorize is a stub: it bounces straight back with a code (the browser
	// would otherwise have to render a consent screen). The code carries the
	// state so a concurrent token exchange can pick the matching claims.
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		redirect := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		http.Redirect(w, r, redirect+"?code="+url.QueryEscape(state)+"&state="+url.QueryEscape(state), http.StatusFound)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		claims := idp.nextClaims
		if state := r.FormValue("code"); state != "" {
			if pinned, ok := idp.claimsByState.Load(state); ok {
				claims = pinned.(map[string]any)
			}
		}
		if idp.tokenOverride != nil {
			claims = idp.tokenOverride(claims)
		}
		out := map[string]any{
			"access_token": "access-token", "token_type": "Bearer", "expires_in": 3600,
		}
		if !idp.omitIDToken {
			signer := idp.signer
			if idp.badSigner != nil {
				signer = idp.badSigner
			}
			token, err := jwt.Signed(signer).Claims(claims).Serialize()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			out["id_token"] = token
		}
		writeJSON(w, out)
	})

	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

// mutatingTokenOverride returns a hook that copies and mutates the token claims.
func mutatingTokenOverride(mutate func(map[string]any)) func(map[string]any) map[string]any {
	return func(claims map[string]any) map[string]any {
		copied := map[string]any{}
		for k, v := range claims {
			copied[k] = v
		}
		mutate(copied)
		return copied
	}
}

// newBadSigner builds a signer with an unrelated key (valid JWT shape, bad sig).
func newBadSigner(t *testing.T) jose.Signer {
	t.Helper()
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: other}, (&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)
	return s
}

// claimSet builds standard ID token claims for a subject.
func (idp *fakeIdP) claimSet(sub, email string, emailVerified bool, nonce string) map[string]any {
	return map[string]any{
		"iss":            idp.server.URL,
		"aud":            "test-client",
		"sub":            sub,
		"email":          email,
		"email_verified": emailVerified,
		"name":           "SSO Tester",
		"nonce":          nonce,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// seedOIDCEnv flips the OIDC config on for the current test and points the
// issuer at the fake IdP.
func seedOIDCEnv(t *testing.T, idp *fakeIdP) {
	t.Helper()
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER", idp.server.URL)
	t.Setenv("OIDC_CLIENT_ID", "test-client")
	t.Setenv("OIDC_CLIENT_SECRET", "test-secret")
	t.Setenv("OIDC_SCOPES", "openid email profile")
	t.Setenv("RATE_LIMIT_AUTH", "1000")
	t.Setenv("RATE_LIMIT_PUBLIC", "1000")
}

// fetchLocation returns the Location header without following the redirect.
func fetchLocation(resp *http.Response) string {
	return resp.Header.Get("Location")
}

// oidcStart performs GET /api/auth/oidc/login and returns the provider
// redirect's parsed query so callers can read the state and nonce the server
// stored for exactly that attempt.
func oidcStart(t *testing.T, app *fiber.App) (resp *http.Response, query url.Values) {
	t.Helper()
	resp = doRequest(t, app, "GET", "/api/auth/oidc/login", "", nil)
	loc := fetchLocation(resp)
	u, err := url.Parse(loc)
	require.NoError(t, err)
	return resp, u.Query()
}

// oidcCallback builds the callback URL the provider would send the browser to.
func oidcCallback(state, code string) string {
	return "/api/auth/oidc/callback?code=" + code + "&state=" + url.QueryEscape(state)
}

func TestOIDCFlow(t *testing.T) {
	idp := newFakeIdP(t)
	seedOIDCEnv(t, idp)
	app := newTestApp()

	t.Run("login redirects to the provider with state and PKCE", func(t *testing.T) {
		resp, q := oidcStart(t, app)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.NotEmpty(t, q.Get("state"))
		assert.NotEmpty(t, q.Get("nonce"))
		loc := fetchLocation(resp)
		assert.True(t, strings.HasPrefix(loc, idp.server.URL+"/authorize"), loc)
		assert.Contains(t, loc, "code_challenge_method=S256")
		assert.Contains(t, loc, "code_challenge=")
	})

	t.Run("successful callback starts a valid session", func(t *testing.T) {
		_, q := oidcStart(t, app)
		idp.nextClaims = idp.claimSet("sub-ok", "ok@example.com", true, q.Get("nonce"))

		resp := doRequest(t, app, "GET", oidcCallback(q.Get("state"), "test-code"), "", nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/dashboard", fetchLocation(resp))

		cookies := resp.Cookies()
		me := doRequest(t, app, "GET", "/api/auth/me", "", cookies)
		defer me.Body.Close()
		assert.Equal(t, http.StatusOK, me.StatusCode)
		body := decodeBody(t, me)
		user := body["user"].(map[string]interface{})
		assert.Equal(t, "ok@example.com", user["email"])
	})

	t.Run("state is single-use", func(t *testing.T) {
		_, q := oidcStart(t, app)
		idp.nextClaims = idp.claimSet("sub-reuse", "reuse@example.com", true, q.Get("nonce"))

		first := doRequest(t, app, "GET", oidcCallback(q.Get("state"), "test-code"), "", nil)
		first.Body.Close()
		require.Equal(t, http.StatusFound, first.StatusCode)
		require.Equal(t, "/dashboard", fetchLocation(first))

		second := doRequest(t, app, "GET", oidcCallback(q.Get("state"), "test-code"), "", nil)
		defer second.Body.Close()
		assert.Equal(t, http.StatusFound, second.StatusCode)
		assert.Equal(t, "/login?oidc_error=state", fetchLocation(second))
	})

	t.Run("nonce mismatch is rejected", func(t *testing.T) {
		_, q := oidcStart(t, app)
		idp.nextClaims = idp.claimSet("sub-nonce", "nonce@example.com", true, "wrong-nonce")

		resp := doRequest(t, app, "GET", oidcCallback(q.Get("state"), "test-code"), "", nil)
		defer resp.Body.Close()
		assert.Equal(t, "/login?oidc_error=state", fetchLocation(resp))
	})

	t.Run("unverified email on existing account is rejected", func(t *testing.T) {
		// Pre-create a local account with this email.
		registerAndLogin(t, app, "taken@example.com", "secret123")

		_, q := oidcStart(t, app)
		idp.nextClaims = idp.claimSet("sub-taken", "taken@example.com", false, q.Get("nonce"))

		resp := doRequest(t, app, "GET", oidcCallback(q.Get("state"), "test-code"), "", nil)
		defer resp.Body.Close()
		assert.Equal(t, "/login?oidc_error=email", fetchLocation(resp))
	})

	t.Run("provision denied when registration is closed", func(t *testing.T) {
		t.Setenv("ALLOW_REGISTRATION", "false")
		t.Cleanup(func() { t.Setenv("ALLOW_REGISTRATION", "true") })

		_, q := oidcStart(t, app)
		idp.nextClaims = idp.claimSet("sub-new-closed", "brandnew@example.com", true, q.Get("nonce"))

		resp := doRequest(t, app, "GET", oidcCallback(q.Get("state"), "test-code"), "", nil)
		defer resp.Body.Close()
		assert.Equal(t, "/login?oidc_error=denied", fetchLocation(resp))
	})

	t.Run("provider error is mapped to a stable code", func(t *testing.T) {
		resp := doRequest(t, app, "GET", "/api/auth/oidc/callback?error=access_denied", "", nil)
		defer resp.Body.Close()
		assert.Equal(t, "/login?oidc_error=provider", fetchLocation(resp))
	})

	t.Run("disabled provider redirects to the disabled code", func(t *testing.T) {
		t.Setenv("OIDC_ENABLED", "false")
		t.Cleanup(func() { t.Setenv("OIDC_ENABLED", "true") })
		resp := doRequest(t, app, "GET", "/api/auth/oidc/login", "", nil)
		defer resp.Body.Close()
		assert.Equal(t, "/login?oidc_error=disabled", fetchLocation(resp))
	})
}

// registerAndLogin registers a user (or logs in if the email already exists)
// and returns the session cookies.
func registerAndLogin(t *testing.T, app *fiber.App, email, password string) []*http.Cookie {
	t.Helper()
	resp := doRequest(t, app, "POST", "/api/auth/register",
		`{"name":"OIDC Local","email":"`+email+`","password":"`+password+`"}`, nil)
	resp.Body.Close()
	resp = doRequest(t, app, "POST", "/api/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "login should succeed for "+email)
	cookies := resp.Cookies()
	resp.Body.Close()
	require.NotEmpty(t, cookies)
	return cookies
}
