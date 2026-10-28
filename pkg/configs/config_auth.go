package configs

import "time"

// AuthConfig holds public auth toggles and the per-account sign-in lockout.
type AuthConfig struct {
	AllowRegistration        bool // ALLOW_REGISTRATION
	RequireEmailVerification bool // REQUIRE_EMAIL_VERIFICATION
	LockoutThreshold         int  // AUTH_LOCKOUT_THRESHOLD
	LockoutBaseSec           int  // AUTH_LOCKOUT_BASE_SECONDS
	LockoutMaxSec            int  // AUTH_LOCKOUT_MAX_SECONDS
}

// LockoutBase is the lock armed by the first failure past the threshold.
func (a AuthConfig) LockoutBase() time.Duration {
	return time.Duration(a.LockoutBaseSec) * time.Second
}

// LockoutMax caps the exponential escalation and doubles as the counter
// window: a quiet window the length of the cap forgets the ladder entirely.
func (a AuthConfig) LockoutMax() time.Duration {
	return time.Duration(a.LockoutMaxSec) * time.Second
}

// loadAuth parses the auth env block (intEnv is the Load collector so a bad
// integer still fails startup with the key name).
func loadAuth(intEnv func(string, int) int) AuthConfig {
	return AuthConfig{
		AllowRegistration:        envBool("ALLOW_REGISTRATION", true),
		RequireEmailVerification: envBool("REQUIRE_EMAIL_VERIFICATION", true),
		LockoutThreshold:         intEnv("AUTH_LOCKOUT_THRESHOLD", 5),
		LockoutBaseSec:           intEnv("AUTH_LOCKOUT_BASE_SECONDS", 60),
		LockoutMaxSec:            intEnv("AUTH_LOCKOUT_MAX_SECONDS", 900),
	}
}
