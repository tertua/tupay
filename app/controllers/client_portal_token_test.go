package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClientPortalTokenShape proves the raw/hash separation: the raw token
// is 48 hex chars (24 bytes) and the stored hash is its sha256 hex, never the
// raw value itself.
func TestNewClientPortalTokenShape(t *testing.T) {
	raw, hash, err := newClientPortalToken()
	require.NoError(t, err)
	assert.Len(t, raw, 48, "raw token is 24 random bytes hex-encoded")
	assert.Len(t, hash, 64, "sha256 hex is 64 chars")
	assert.NotEqual(t, raw, hash)
	assert.Equal(t, hashClientPortalToken(raw), hash, "hash must be sha256 of the raw token")
}

// TestClientPortalTokenHashStable proves the lookup path hashes the same way as
// the mint path, so a freshly minted token resolves by its stored hash.
func TestClientPortalTokenHashStable(t *testing.T) {
	raw, hash, err := newClientPortalToken()
	require.NoError(t, err)
	assert.Equal(t, hash, hashClientPortalToken(raw))
	// Two mints never collide.
	raw2, hash2, err := newClientPortalToken()
	require.NoError(t, err)
	assert.NotEqual(t, raw, raw2)
	assert.NotEqual(t, hash, hash2)
}
