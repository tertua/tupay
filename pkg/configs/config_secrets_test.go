package configs

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEncryptionKey is a deterministic base64 32-byte key: only its shape
// matters for validation, never the bytes.
func testEncryptionKey() string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
}

// TestSecretsKeyValidation pins the at-rest encryption key contract: required in
// prod, malformed values rejected everywhere (a typo must not silently downgrade
// to plaintext), dev allowed with a boot warning.
func TestSecretsKeyValidation(t *testing.T) {
	t.Run("prod requires the key", func(t *testing.T) {
		setValidProdEnv(t)
		t.Setenv("SECRET_ENCRYPTION_KEY", "")
		_, err := Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SECRET_ENCRYPTION_KEY")
	})

	t.Run("malformed key is rejected", func(t *testing.T) {
		setValidProdEnv(t)
		t.Setenv("SECRET_ENCRYPTION_KEY", "not-base64")
		_, err := Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SECRET_ENCRYPTION_KEY")
	})

	t.Run("dev falls back to plaintext with a warning", func(t *testing.T) {
		t.Setenv("STAGE_STATUS", "dev")
		t.Setenv("SECRET_ENCRYPTION_KEY", "")
		cfg, err := Load()
		require.NoError(t, err)
		assert.Contains(t, strings.Join(cfg.Warnings(), "|"), "SECRET_ENCRYPTION_KEY")
	})
}
