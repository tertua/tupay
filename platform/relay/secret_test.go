package relay

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestKey returns a fresh base64 32-byte key for SECRET_ENCRYPTION_KEY.
func newTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(key)
}

// TestSealSecretRoundTrip pins the at-rest contract: a sealed value carries the
// version prefix, never contains the plaintext, and opens back to it.
func TestSealSecretRoundTrip(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", newTestKey(t))

	const plain = "whsec_0123456789abcdef"
	sealed, err := SealSecret(plain)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(sealed, sealedPrefix))
	assert.NotContains(t, sealed, plain)

	opened, err := OpenSecret(sealed)
	require.NoError(t, err)
	assert.Equal(t, plain, opened)
}

// TestSealSecretWithoutKeyStoresPlaintext pins the dev fallback: no key means no
// encryption (prod refuses to boot without one), and reads still work.
func TestSealSecretWithoutKeyStoresPlaintext(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", "")

	const plain = "whsec_plain_dev"
	sealed, err := SealSecret(plain)
	require.NoError(t, err)
	assert.Equal(t, plain, sealed)

	opened, err := OpenSecret(sealed)
	require.NoError(t, err)
	assert.Equal(t, plain, opened)
}

// TestOpenSecretPassesLegacyPlaintext keeps rows written before encryption
// readable, so enabling the key does not break existing projects.
func TestOpenSecretPassesLegacyPlaintext(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", newTestKey(t))

	opened, err := OpenSecret("whsec_legacy")
	require.NoError(t, err)
	assert.Equal(t, "whsec_legacy", opened)
}

// TestOpenSecretRejectsTamperedCiphertext proves GCM authentication: a
// modified stored value is an error, never garbage plaintext signed downstream.
func TestOpenSecretRejectsTamperedCiphertext(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", newTestKey(t))

	sealed, err := SealSecret("whsec_tamper")
	require.NoError(t, err)
	tampered := sealed[:len(sealed)-2] + "AA"

	_, err = OpenSecret(tampered)
	assert.ErrorIs(t, err, ErrSecretUnreadable)
}

// TestOpenSecretRejectsWrongKey covers a key swap: old rows must fail loudly
// instead of producing a signature the customer cannot verify.
func TestOpenSecretRejectsWrongKey(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", newTestKey(t))
	sealed, err := SealSecret("whsec_rotate")
	require.NoError(t, err)

	t.Setenv("SECRET_ENCRYPTION_KEY", newTestKey(t))
	_, err = OpenSecret(sealed)
	assert.ErrorIs(t, err, ErrSecretUnreadable)

	t.Setenv("SECRET_ENCRYPTION_KEY", "")
	_, err = OpenSecret(sealed)
	assert.ErrorIs(t, err, ErrSecretKeyMissing)
}

// TestSealStoredIsIdempotent guards the update path: re-sealing a sealed row
// would wrap the ciphertext and make the secret unreadable.
func TestSealStoredIsIdempotent(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", newTestKey(t))

	sealed, err := SealSecret("whsec_once")
	require.NoError(t, err)

	again, err := SealStored(sealed)
	require.NoError(t, err)
	assert.Equal(t, sealed, again)

	opened, err := OpenSecret(again)
	require.NoError(t, err)
	assert.Equal(t, "whsec_once", opened)

	// An empty secret has nothing to seal and must stay empty (no v1: wrapper).
	empty, err := SealStored("")
	require.NoError(t, err)
	assert.Equal(t, "", empty)
}

// TestSealSecretRejectsInvalidKey proves a typo never silently downgrades to
// plaintext: the error surfaces at the call site (and at startup in prod).
func TestSealSecretRejectsInvalidKey(t *testing.T) {
	t.Setenv("SECRET_ENCRYPTION_KEY", "not-base64")

	_, err := SealSecret("whsec_x")
	assert.ErrorIs(t, err, ErrSecretKeyInvalid)
}
