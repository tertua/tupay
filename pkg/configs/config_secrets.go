package configs

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// SecretsConfig holds the key that seals stored project secrets at rest (the
// gateway webhook signing secret, see platform/relay/secret.go).
type SecretsConfig struct {
	EncryptionKey string // SECRET_ENCRYPTION_KEY: base64 of 32 random bytes
}

// loadSecrets parses the at-rest encryption env block. Unset is allowed in dev
// (secrets are then stored in plaintext) and rejected in prod by validate.
func loadSecrets() SecretsConfig {
	return SecretsConfig{EncryptionKey: strings.TrimSpace(os.Getenv("SECRET_ENCRYPTION_KEY"))}
}

// validate rejects a key that is set but unusable (wrong base64 or length) so a
// typo never silently falls back to plaintext, and requires one in prod.
func (s SecretsConfig) validate(stage string) error {
	if s.EncryptionKey == "" {
		if stage == "prod" {
			return fmt.Errorf("SECRET_ENCRYPTION_KEY must be set in prod: stored project secrets would be plaintext")
		}
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(s.EncryptionKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("invalid SECRET_ENCRYPTION_KEY: must be base64 of 32 bytes (see: openssl rand -base64 32)")
	}
	return nil
}

// warnings flags the dev-only plaintext fallback, so an operator running a
// staging box as dev still sees it at boot.
func (s SecretsConfig) warnings(stage string) []string {
	if stage != "prod" && s.EncryptionKey == "" {
		return []string{"SECRET_ENCRYPTION_KEY unset: gateway webhook secrets are stored in plaintext"}
	}
	return nil
}
