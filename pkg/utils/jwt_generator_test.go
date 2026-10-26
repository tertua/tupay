package utils

import (
	"regexp"
	"testing"
	"time"

	"github.com/tertua/tupay/pkg/configs"
)

// refreshTokenPattern pins the shape consumers depend on: 64 hex chars, a dot,
// then the unix expiry.
var refreshTokenPattern = regexp.MustCompile(`^[0-9a-f]{64}\.\d+$`)

// TestGenerateNewRefreshTokenUnique proves two mints never collide (the token is
// drawn from the CSPRNG, not derived from the clock).
func TestGenerateNewRefreshTokenUnique(t *testing.T) {
	first, err := generateNewRefreshToken()
	if err != nil {
		t.Fatalf("expected a refresh token, got: %v", err)
	}
	second, err := generateNewRefreshToken()
	if err != nil {
		t.Fatalf("expected a refresh token, got: %v", err)
	}
	if first == second {
		t.Fatal("expected two independently generated refresh tokens to differ")
	}
}

// TestRefreshTokenFormat pins the `<hex64>.<unix>` wire format.
func TestRefreshTokenFormat(t *testing.T) {
	token, err := generateNewRefreshToken()
	if err != nil {
		t.Fatalf("expected a refresh token, got: %v", err)
	}
	if !refreshTokenPattern.MatchString(token) {
		t.Fatalf("expected format hex64.unix, got %q", token)
	}
}

// TestParseRefreshTokenRoundTrip proves the minted expiry is readable back and
// lands near now + RefreshHours.
func TestParseRefreshTokenRoundTrip(t *testing.T) {
	token, err := generateNewRefreshToken()
	if err != nil {
		t.Fatalf("expected a refresh token, got: %v", err)
	}
	expiry, err := ParseRefreshToken(token)
	if err != nil {
		t.Fatalf("expected to parse the token, got: %v", err)
	}
	want := time.Now().Add(time.Hour * time.Duration(configs.Get().JWT.RefreshHours)).Unix()
	if diff := expiry - want; diff < -5 || diff > 5 {
		t.Fatalf("expected expiry near %d, got %d", want, expiry)
	}
}

// TestParseRefreshTokenInvalid pins the rejection of malformed tokens: the
// parser requires exactly one dot and a non-empty integer expiry after it.
func TestParseRefreshTokenInvalid(t *testing.T) {
	for _, token := range []string{"", "abc", "a.", "5.", "a.b", "a.1.2"} {
		if _, err := ParseRefreshToken(token); err == nil {
			t.Fatalf("expected %q to be rejected", token)
		}
	}
}
