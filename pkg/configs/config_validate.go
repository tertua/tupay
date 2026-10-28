package configs

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// Validate fail-fasts on configuration that would break or insecurely run
// the server. Dev stays permissive; prod requires real secrets and public
// URLs so provider webhooks and email links never point at localhost.
func (c Config) Validate() error {
	if c.Stage != "dev" && c.Stage != "prod" {
		return fmt.Errorf("invalid STAGE_STATUS %q: must be \"dev\" or \"prod\"", c.Stage)
	}
	if c.Stage == "prod" {
		if c.JWT.Secret == "" || c.JWT.Secret == "secret" {
			return fmt.Errorf("JWT_SECRET_KEY must be set to a non-default value in prod")
		}
		if c.JWT.RefreshKey == "" || c.JWT.RefreshKey == "refresh" {
			return fmt.Errorf("JWT_REFRESH_KEY must be set to a non-default value in prod")
		}
		if !isPublicURL(c.Relay.PublicURL) {
			return fmt.Errorf("TUPAY_PUBLIC_URL must be a public URL in prod: payment webhooks and email links need it")
		}
		if !isPublicURL(c.Mail.AppPublicURL) {
			return fmt.Errorf("APP_PUBLIC_URL must be a public URL in prod: email links and password resets need it")
		}
	}
	if c.DSN() != "" && !strings.HasPrefix(c.DSN(), "postgres://") && !strings.HasPrefix(c.DSN(), "postgresql://") {
		return fmt.Errorf("unsupported SQL_DSN: leave it empty for SQLite or use a postgres://... DSN")
	}
	if port, err := strconv.Atoi(c.Server.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid SERVER_PORT %q: must be 1-65535", c.Server.Port)
	}
	if c.Server.ReadTimeoutSec <= 0 {
		return fmt.Errorf("invalid SERVER_READ_TIMEOUT %q: must be > 0", os.Getenv("SERVER_READ_TIMEOUT"))
	}
	for _, p := range c.Server.TrustedProxies {
		if _, err := netip.ParsePrefix(p); err != nil {
			if _, err := netip.ParseAddr(p); err != nil {
				return fmt.Errorf("invalid TRUSTED_PROXIES entry %q: must be an IP or CIDR", p)
			}
		}
	}
	// Note: ProxyHeader needs no validation — envOr substitutes the
	// X-Forwarded-For default for blank values, so it is never empty.
	if c.JWT.AccessMinutes <= 0 || c.JWT.RefreshHours <= 0 {
		return fmt.Errorf("invalid JWT lifetimes: access minutes and refresh hours must be > 0")
	}
	if err := c.validateLimits(); err != nil {
		return err
	}
	if c.Redis.Enabled() {
		if port, err := strconv.Atoi(c.Redis.Port); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid REDIS_PORT %q: must be 1-65535", c.Redis.Port)
		}
		if c.Redis.DBNumber < 0 {
			return fmt.Errorf("invalid REDIS_DB_NUMBER: must be >= 0")
		}
	}
	if c.Debug.Port != "" {
		if port, err := strconv.Atoi(c.Debug.Port); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid DEBUG_PORT %q: must be 1-65535", c.Debug.Port)
		}
	}
	if c.Storage.Backend != "local" && c.Storage.Backend != "s3" {
		return fmt.Errorf("invalid STORAGE_BACKEND %q: must be \"local\" or \"s3\"", c.Storage.Backend)
	}
	if c.Storage.Backend == "s3" {
		if c.Storage.S3Endpoint == "" || c.Storage.S3Bucket == "" {
			return fmt.Errorf("S3_ENDPOINT and S3_BUCKET are required when STORAGE_BACKEND=s3")
		}
		if c.Storage.S3AccessKey == "" || c.Storage.S3SecretKey == "" {
			return fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required when STORAGE_BACKEND=s3")
		}
	}
	if c.Mail.SMTPHost != "" {
		if port, err := strconv.Atoi(c.Mail.SMTPPort); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid SMTP_PORT %q: must be 1-65535", c.Mail.SMTPPort)
		}
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid LOG_LEVEL %q: must be debug|info|warn|error", c.Log.Level)
	}
	return nil
}

// Warnings returns non-fatal configuration smells worth logging at startup
// (main.go calls logger.L().Warn for each). Unlike Validate, these never
// block boot: the server runs, but an operator should know.
func (c Config) Warnings() []string {
	var out []string
	if c.Stage == "prod" && strings.TrimSpace(c.Captcha.TurnstileSecret) == "" {
		out = append(out, "captcha disabled in prod: bots can register")
	}
	return out
}

// isPublicURL reports whether raw is a non-localhost http(s) URL. Prod uses
// it to reject the localhost fallbacks that would leak into provider webhook
// callbacks and email links.
func isPublicURL(raw string) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		return false
	}
	if strings.Contains(v, "localhost") || strings.Contains(v, "127.0.0.1") || strings.Contains(v, "[::1]") {
		return false
	}
	return true
}
