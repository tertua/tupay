package relay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/tertua/tupay/pkg/configs"
)

// Stored project secrets carry a version prefix so the key can be rotated
// later: v1 rows are AES-256-GCM sealed, anything else is a legacy plaintext
// row written before encryption landed and is still read as-is. Writing a
// plaintext value into a sealed column is therefore indistinguishable from a
// legacy row — the guard is the admin API, not the format.
const (
	sealedPrefix  = "v1:"
	secretKeySize = 32
)

var (
	// ErrSecretKeyMissing means a sealed secret must be opened but the server
	// has no SECRET_ENCRYPTION_KEY.
	ErrSecretKeyMissing = errors.New("relay: secret encryption key is not configured")
	// ErrSecretKeyInvalid means SECRET_ENCRYPTION_KEY is not base64 of 32 bytes.
	ErrSecretKeyInvalid = errors.New("relay: secret encryption key must be base64 of 32 bytes")
	// ErrSecretUnreadable means the stored value is malformed or was sealed
	// with a different key.
	ErrSecretUnreadable = errors.New("relay: stored secret cannot be decrypted")
)

// gcm builds the AES-GCM cipher from the configured key.
func gcm() (cipher.AEAD, error) {
	raw := strings.TrimSpace(configs.Get().Secrets.EncryptionKey)
	if raw == "" {
		return nil, ErrSecretKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != secretKeySize {
		return nil, ErrSecretKeyInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrSecretKeyInvalid
	}
	return cipher.NewGCM(block)
}

// SealSecret encrypts a project webhook secret for storage. With no key set
// (dev only, warned at boot) the value is stored as plaintext so local setup
// needs no extra secret; prod refuses to start without a key.
func SealSecret(plain string) (string, error) {
	aead, err := gcm()
	if errors.Is(err, ErrSecretKeyMissing) {
		return plain, nil
	}
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("relay: seal secret: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	return sealedPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// SealStored upgrades a stored secret: legacy plaintext is sealed, while an
// empty or already-sealed value is returned unchanged (an empty secret has
// nothing to protect, and sealing it twice would make it unreadable).
func SealStored(stored string) (string, error) {
	if stored == "" || strings.HasPrefix(stored, sealedPrefix) {
		return stored, nil
	}
	return SealSecret(stored)
}

// ProjectSecret returns the usable plaintext for a stored project secret:
// sealed rows are opened, legacy plaintext rows pass through untouched.
func ProjectSecret(stored string) (string, error) {
	if !strings.HasPrefix(stored, sealedPrefix) {
		return stored, nil
	}
	aead, err := gcm()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, sealedPrefix))
	if err != nil || len(raw) < aead.NonceSize() {
		return "", ErrSecretUnreadable
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return "", ErrSecretUnreadable
	}
	return string(plain), nil
}
