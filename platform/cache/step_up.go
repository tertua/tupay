package cache

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/pkg/configs"
)

// StepUpKeyPrefix namespaces the single-use step-up tokens in the session
// store; the token itself is the key, so a leaked value is the only thing that
// grants the elevation and it dies on first use.
const StepUpKeyPrefix = "auth:stepup:"

// StepUpKey builds the store key for a minted token.
func StepUpKey(token string) string { return StepUpKeyPrefix + token }

// EncodeStepUpValue packs the token payload: the account it was minted for, the
// session it is bound to, and its absolute expiry (the store TTL is the same
// window, the explicit stamp guards a store that forgets to expire).
func EncodeStepUpValue(userID uuid.UUID, sid string, expiresAt time.Time) string {
	return userID.String() + "|" + sid + "|" + strconv.FormatInt(expiresAt.Unix(), 10)
}

// DecodeStepUpValue splits a stored payload and reports whether it is still
// valid at now. A malformed or expired value is treated as absent.
func DecodeStepUpValue(value string, now time.Time) (userID uuid.UUID, sid string, ok bool) {
	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return uuid.Nil, "", false
	}
	parsed, err := uuid.Parse(parts[0])
	if err != nil || parsed == uuid.Nil || parts[1] == "" {
		return uuid.Nil, "", false
	}
	expiry, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || !now.Before(time.Unix(expiry, 0)) {
		return uuid.Nil, "", false
	}
	return parsed, parts[1], true
}

// StoreStepUp saves one token for its whole lifetime.
func StoreStepUp(ctx context.Context, token, value string, ttl time.Duration) error {
	store, err := Sessions()
	if err != nil {
		return err
	}
	return store.Set(ctx, StepUpKey(token), value, ttl)
}

// ConsumeStepUp takes a token out of the store in one atomic step: the
// elevation is spent whether or not the caller ends up using it, so a captured
// token cannot be replayed — not even by a concurrent pair of requests.
func ConsumeStepUp(ctx context.Context, token string) (string, bool) {
	store, err := Sessions()
	if err != nil {
		return "", false
	}
	taker, ok := store.(takeStore)
	if !ok {
		return "", false
	}
	value, err := taker.Take(ctx, StepUpKey(token))
	if err != nil {
		return "", false
	}
	return value, true
}

// StepUpTTL returns the configured step-up window.
func StepUpTTL() time.Duration { return configs.Get().Auth.StepUp() }
