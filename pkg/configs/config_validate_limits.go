package configs

import (
	"fmt"
	"strconv"
)

// validateLimits rejects every non-positive numeric knob plus the one
// relational rule the env-int map cannot express: a lockout base above its cap
// would be silently clamped at runtime, so it is a startup error instead.
func (c Config) validateLimits() error {
	for name, v := range map[string]int{
		"DB_MAX_CONNECTIONS": c.DB.MaxConn, "DB_MAX_IDLE_CONNECTIONS": c.DB.MaxIdle,
		"DB_MAX_LIFETIME_CONNECTIONS": c.DB.MaxLifetimeSec,
		"RATE_LIMIT_GENERAL":          c.RateLimit.General, "RATE_LIMIT_AUTH": c.RateLimit.Auth,
		"RATE_LIMIT_PUBLIC": c.RateLimit.Public, "RATE_LIMIT_WEBHOOK": c.RateLimit.Webhook,
		"RATE_LIMIT_GATEWAY": c.RateLimit.Gateway, "AI_TIMEOUT_SECONDS": c.AI.TimeoutSec,
		"IDEMPOTENCY_TTL_HOURS": c.Idempotency.TTLHours,
		"OUTBOX_POLL_SECONDS":   c.Outbox.PollSeconds, "OUTBOX_BATCH": c.Outbox.Batch,
		"CACHE_AGG_TTL_SECONDS": c.Cache.AggTTLSeconds,
		// D3 knobs: every one must be > 0 (disabling via 0 is not supported).
		"GATEWAY_RECONCILE_MINUTES":       c.Gateway.ReconcileMinutes,
		"GATEWAY_API_TIMEOUT_SECONDS":     c.Gateway.APITimeoutSec,
		"GATEWAY_PAYMENT_TIMEOUT_SECONDS": c.Gateway.PaymentTimeoutSec,
		"QR_FETCH_TIMEOUT_SECONDS":        c.Gateway.QRFetchTimeoutSec,
		"WEBHOOK_FORWARD_TIMEOUT_SECONDS": c.Gateway.WebhookForwardTimeoutSec,
		"CAPTCHA_TIMEOUT_SECONDS":         c.Captcha.TimeoutSec,
		"GATEWAY_MAX_ATTEMPTS":            c.Gateway.MaxAttempts,
		"GATEWAY_RETRY_BASE_SECONDS":      c.Gateway.RetryBaseSec,
		"GATEWAY_RETRY_MAX_SECONDS":       c.Gateway.RetryMaxSec,
		"OUTBOX_MAX_ATTEMPTS":             c.Outbox.MaxAttempts,
		"OUTBOX_MAX_BACKOFF_MINUTES":      c.Outbox.MaxBackoffMinutes,
		"WEBHOOK_RETRY_MINUTES":           c.Gateway.WebhookRetryMinutes,
		"AUTH_LOCKOUT_THRESHOLD":          c.Auth.LockoutThreshold,
		"AUTH_LOCKOUT_BASE_SECONDS":       c.Auth.LockoutBaseSec,
		"AUTH_LOCKOUT_MAX_SECONDS":        c.Auth.LockoutMaxSec,
	} {
		if v <= 0 {
			return fmt.Errorf("invalid %s: must be > 0", name)
		}
	}
	if c.Auth.LockoutBaseSec > c.Auth.LockoutMaxSec {
		return fmt.Errorf("invalid AUTH_LOCKOUT_BASE_SECONDS %d: must be <= AUTH_LOCKOUT_MAX_SECONDS %d",
			c.Auth.LockoutBaseSec, c.Auth.LockoutMaxSec)
	}
	if c.Redis.Enabled() {
		if port, err := strconv.Atoi(c.Redis.Port); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid REDIS_PORT %q: must be 1-65535", c.Redis.Port)
		}
		if c.Redis.DBNumber < 0 {
			return fmt.Errorf("invalid REDIS_DB_NUMBER: must be >= 0")
		}
	}
	return nil
}
