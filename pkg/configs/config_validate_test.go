package configs

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatePostgresDSN passes with a production-like DSN and secrets.
// Moved here from config_test.go (which is ratcheted at 121 lines) so the
// prod URL requirements below can share the prod fixture.
func TestValidatePostgresDSN(t *testing.T) {
	setValidProdEnv(t)
	t.Setenv("SQL_DSN", "postgres://postgres:password@localhost/postgres?sslmode=disable")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://postgres:password@localhost/postgres?sslmode=disable", cfg.DSN())
}

// TestLoadRejectsBadIntEnv covers D2: an explicitly-set but invalid integer is
// a startup error naming the key and the raw value, never a silent fallback.
func TestLoadRejectsBadIntEnv(t *testing.T) {
	for _, bad := range []string{"0", "-5", "abc"} {
		t.Run("GATEWAY_API_TIMEOUT_SECONDS="+bad, func(t *testing.T) {
			t.Setenv("GATEWAY_API_TIMEOUT_SECONDS", bad)
			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "GATEWAY_API_TIMEOUT_SECONDS")
			assert.Contains(t, err.Error(), bad)
		})
	}

	t.Setenv("RATE_LIMIT_GENERAL", "0")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_GENERAL")
	assert.Contains(t, err.Error(), "0")
}

// TestLoadAcceptsGoodIntEnv is the D2 negative: valid values and unset keys
// never error.
func TestLoadAcceptsGoodIntEnv(t *testing.T) {
	t.Setenv("GATEWAY_API_TIMEOUT_SECONDS", "20")
	t.Setenv("RATE_LIMIT_GENERAL", "50")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 20, cfg.Gateway.APITimeoutSec)
	assert.Equal(t, 50, cfg.RateLimit.General)
}

// TestValidateProdRequiresPublicURLs covers D1: prod must fail fast when the
// public URLs are empty or point at localhost.
func TestValidateProdRequiresPublicURLs(t *testing.T) {
	for _, tc := range []struct{ name, relay, app string }{
		{"empty relay", "", "https://app.example.com"},
		{"localhost relay", "http://localhost:5000", "https://app.example.com"},
		{"empty app", "https://api.example.com", ""},
		{"localhost app", "https://api.example.com", "http://127.0.0.1:5173"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setValidProdEnv(t)
			t.Setenv("TUPAY_PUBLIC_URL", tc.relay)
			t.Setenv("APP_PUBLIC_URL", tc.app)
			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must be a public URL in prod")
		})
	}

	t.Run("full prod", func(t *testing.T) {
		setValidProdEnv(t)
		_, err := Load()
		require.NoError(t, err)
	})
}

// TestWarningsCaptchaProd covers D1: captcha with no secret warns in prod
// (non-fatal) and stays silent in dev.
func TestWarningsCaptchaProd(t *testing.T) {
	setValidProdEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.NotEmpty(t, cfg.Warnings())
	assert.Contains(t, strings.Join(cfg.Warnings(), " "), "captcha")

	t.Setenv("STAGE_STATUS", "dev")
	cfg, err = Load() // dev
	require.NoError(t, err)
	assert.NotContains(t, strings.Join(cfg.Warnings(), " "), "captcha")
}

// TestNewEnvKeysDefaults verifies D3: unset keys default to the old constants.
func TestNewEnvKeysDefaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 15, cfg.Gateway.ReconcileMinutes)
	assert.Equal(t, 15, cfg.Gateway.APITimeoutSec)
	assert.Equal(t, 25, cfg.Gateway.PaymentTimeoutSec)
	assert.Equal(t, 10, cfg.Gateway.QRFetchTimeoutSec)
	assert.Equal(t, 10, cfg.Gateway.WebhookForwardTimeoutSec)
	assert.Equal(t, 3, cfg.Gateway.MaxAttempts)
	assert.Equal(t, 1, cfg.Gateway.RetryBaseSec)
	assert.Equal(t, 30, cfg.Gateway.RetryMaxSec)
	assert.Equal(t, 5, cfg.Gateway.WebhookRetryMinutes)
	assert.Equal(t, 10, cfg.Outbox.MaxAttempts)
	assert.Equal(t, 120, cfg.Outbox.MaxBackoffMinutes)
	assert.Equal(t, 3, cfg.Captcha.TimeoutSec)
}

// TestNewEnvKeysOverride verifies D3: every new key actually fills its field.
func TestNewEnvKeysOverride(t *testing.T) {
	t.Setenv("GATEWAY_RECONCILE_MINUTES", "20")
	t.Setenv("GATEWAY_API_TIMEOUT_SECONDS", "11")
	t.Setenv("GATEWAY_PAYMENT_TIMEOUT_SECONDS", "26")
	t.Setenv("QR_FETCH_TIMEOUT_SECONDS", "12")
	t.Setenv("WEBHOOK_FORWARD_TIMEOUT_SECONDS", "13")
	t.Setenv("GATEWAY_MAX_ATTEMPTS", "4")
	t.Setenv("GATEWAY_RETRY_BASE_SECONDS", "2")
	t.Setenv("GATEWAY_RETRY_MAX_SECONDS", "40")
	t.Setenv("WEBHOOK_RETRY_MINUTES", "6")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "11")
	t.Setenv("OUTBOX_MAX_BACKOFF_MINUTES", "130")
	t.Setenv("CAPTCHA_TIMEOUT_SECONDS", "9")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 20, cfg.Gateway.ReconcileMinutes)
	assert.Equal(t, 11, cfg.Gateway.APITimeoutSec)
	assert.Equal(t, 26, cfg.Gateway.PaymentTimeoutSec)
	assert.Equal(t, 12, cfg.Gateway.QRFetchTimeoutSec)
	assert.Equal(t, 13, cfg.Gateway.WebhookForwardTimeoutSec)
	assert.Equal(t, 4, cfg.Gateway.MaxAttempts)
	assert.Equal(t, 2, cfg.Gateway.RetryBaseSec)
	assert.Equal(t, 40, cfg.Gateway.RetryMaxSec)
	assert.Equal(t, 6, cfg.Gateway.WebhookRetryMinutes)
	assert.Equal(t, 11, cfg.Outbox.MaxAttempts)
	assert.Equal(t, 130, cfg.Outbox.MaxBackoffMinutes)
	assert.Equal(t, 9, cfg.Captcha.TimeoutSec)
}

// TestAIUnification guards D3's single knob: the Gemini HTTP client derives its
// timeout from AI_TIMEOUT_SECONDS (plus buffer) instead of the removed
// GeminiAPITimeout constant.
func TestAIUnification(t *testing.T) {
	// The constant must be gone: this file referencing it would not compile.
	for _, name := range []string{"GATEWAY_API_TIMEOUT_SECONDS", "AI_TIMEOUT_SECONDS"} {
		t.Setenv(name, "12")
	}
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 12, cfg.AI.TimeoutSec)

	src, err := os.ReadFile("../../platform/ai/gemini.go")
	require.NoError(t, err)
	assert.NotContains(t, string(src), "GeminiAPITimeout")
	assert.Contains(t, string(src), "TimeoutSec")
}

// setValidProdEnv sets the minimum valid prod configuration: real secrets and
// public URLs, matching Validate's requirements.
func setValidProdEnv(t *testing.T) {
	t.Setenv("STAGE_STATUS", "prod")
	t.Setenv("JWT_SECRET_KEY", "test-secret-value")
	t.Setenv("JWT_REFRESH_KEY", "test-refresh-value")
	t.Setenv("TUPAY_PUBLIC_URL", "https://api.example.com")
	t.Setenv("APP_PUBLIC_URL", "https://app.example.com")
	t.Setenv("SECRET_ENCRYPTION_KEY", testEncryptionKey())
}
