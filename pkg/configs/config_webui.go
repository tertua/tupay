package configs

import (
	"os"
	"strings"
)

// WebUIConfig holds the optional embedded-frontend settings.
type WebUIConfig struct {
	Dir string // SERVE_WEBUI (legacy SERVE_SPA_DIR): empty = API-only (default)
}

// loadWebUI parses the embedded-frontend env.
func loadWebUI() WebUIConfig {
	return WebUIConfig{Dir: strings.TrimSpace(envOr("SERVE_WEBUI", os.Getenv("SERVE_SPA_DIR")))}
}
