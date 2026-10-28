package routes

import "os"

// seedRouteTestEnv sets the env every route test needs (in-memory SQLite, no
// Redis, deterministic JWT secrets). Verification defaults OFF here so the
// legacy suite keeps its register-auto-login path; dedicated tests flip it on
// per test (configs.Get re-reads env on every call).
func seedRouteTestEnv() {
	os.Setenv("STAGE_STATUS", "dev")
	os.Setenv("JWT_SECRET_KEY", "test-secret")
	os.Setenv("JWT_SECRET_KEY_EXPIRE_MINUTES_COUNT", "15")
	os.Setenv("JWT_REFRESH_KEY", "test-refresh")
	os.Setenv("JWT_REFRESH_KEY_EXPIRE_HOURS_COUNT", "720")
	os.Setenv("SQL_DSN", "")
	os.Setenv("SQLITE_PATH", "file::memory:?cache=shared")
	os.Setenv("REDIS_HOST", "")
	os.Setenv("REQUIRE_EMAIL_VERIFICATION", "false")
	// Seal project webhook secrets here too, so the relay flow also covers the
	// sealed-at-rest -> open-before-sign path (32 bytes of "k", base64).
	os.Setenv("SECRET_ENCRYPTION_KEY", "a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=")
}
