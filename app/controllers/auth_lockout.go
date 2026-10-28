package controllers

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/tertua/tupay/pkg/configs"
	"github.com/tertua/tupay/pkg/utils"
	"github.com/tertua/tupay/platform/cache"
	"github.com/tertua/tupay/platform/database"
	"github.com/tertua/tupay/platform/relay"
)

// Sign-in throttling is keyed by a one-way hash of the email: the key never
// stores the address, and an unknown address throttles exactly like a real one
// so the response cannot be used to enumerate accounts.
const (
	loginFailKeyPrefix = "auth:fail:"
	loginLockKeyPrefix = "auth:lock:"
)

// loginThrottleKey builds the counter/lock key for one email.
func loginThrottleKey(prefix, email string) string {
	return prefix + relay.HashKey(normalizeEmail(email))
}

// loginLockRemaining reports how long the account lock still applies. A store
// outage fails open: the IP limiter is still in force, and locking every user
// out on a cache blip is the worse failure.
func loginLockRemaining(ctx context.Context, email string) time.Duration {
	store, err := cache.Sessions()
	if err != nil {
		return 0
	}
	raw, err := store.Get(ctx, loginThrottleKey(loginLockKeyPrefix, email))
	if err != nil {
		return 0
	}
	deadline, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	remaining := time.Until(time.Unix(deadline, 0))
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// noteLoginFailure counts one rejected credential and arms the lock once the
// threshold is crossed. Each further failure in the same window doubles the
// lock, capped by AUTH_LOCKOUT_MAX_SECONDS (which is also the window, so a
// quiet spell forgets the ladder).
func noteLoginFailure(ctx context.Context, email string) time.Duration {
	cfg := configs.Get().Auth
	store, err := cache.Sessions()
	if err != nil {
		return 0
	}
	maxLock := cfg.LockoutMax()
	count, err := store.Incr(ctx, loginThrottleKey(loginFailKeyPrefix, email), maxLock)
	if err != nil {
		return 0
	}
	steps := count - int64(cfg.LockoutThreshold)
	if steps < 0 {
		return 0
	}
	lock := maxLock
	if steps < 16 {
		if d := cfg.LockoutBase() << uint(steps); d > 0 {
			lock = d
		}
	}
	if lock > maxLock {
		lock = maxLock
	}
	deadline := strconv.FormatInt(time.Now().Add(lock).Unix(), 10)
	if err := store.Set(ctx, loginThrottleKey(loginLockKeyPrefix, email), deadline, lock); err != nil {
		return 0
	}
	return lock
}

// clearLoginFailures forgets the counter and any armed lock, so a legitimate
// sign-in (or a password reset that proves inbox control) starts clean.
func clearLoginFailures(ctx context.Context, email string) {
	store, err := cache.Sessions()
	if err != nil {
		return
	}
	_ = store.Delete(ctx, loginThrottleKey(loginFailKeyPrefix, email))
	_ = store.Delete(ctx, loginThrottleKey(loginLockKeyPrefix, email))
}

// loginLocked answers 429 with the remaining lock in Retry-After so a client
// backs off instead of retrying into the same wall.
func loginLocked(c fiber.Ctx, remaining time.Duration) error {
	seconds := int(math.Ceil(remaining.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))
	return utils.Fail(c, fiber.StatusTooManyRequests, "too many failed sign-in attempts, try again later",
		fiber.Map{"code": "auth_locked", "retry_after": seconds})
}

// recordBadCredential audits one rejected password and advances the lockout.
// Only credentials land here: status gates (pending verification, disabled) and
// provider errors stay on recordLoginFailure alone, so a correct password that
// hits a gate is never counted as a guess.
func recordBadCredential(c fiber.Ctx, db *database.Queries, email, reason string, userID uuid.UUID) {
	recordLoginFailure(c, db, email, reason, userID)
	lock := noteLoginFailure(c.Context(), email)
	if lock <= 0 {
		return
	}
	meta := `{"lock_seconds":` + strconv.Itoa(int(lock.Seconds())) + `}`
	entity := relay.HashKey(normalizeEmail(email))
	recordAudit(c, db, uuid.Nil, "auth.login.locked", "auth", entity, meta)
}
