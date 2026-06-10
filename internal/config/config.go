// Package config resolves spoo CLI configuration and filesystem paths.
package config

import (
	"os"
	"path/filepath"
)

// DefaultAPIBase is the production spoo.me endpoint.
const DefaultAPIBase = "https://spoo.me"

type Config struct {
	APIBase string
}

// Load resolves configuration. SPOO_API_URL overrides the default,
// which keeps the CLI pointable at a local dev server.
func Load() Config {
	cfg := Config{APIBase: DefaultAPIBase}
	if v := os.Getenv("SPOO_API_URL"); v != "" {
		cfg.APIBase = v
	}
	return cfg
}

// Dir returns the spoo config directory, creating it if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "spoo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
