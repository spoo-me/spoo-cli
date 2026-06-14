package cmd

import (
	"time"

	"github.com/spoo-me/spoo-cli/internal/api"
)

// parseExpiry normalizes --expires input to RFC 3339. Thin delegate to
// api.ParseExpiry so the shared form code (TUI) and the commands agree
// on the format.
func parseExpiry(raw string, now time.Time) (string, error) {
	return api.ParseExpiry(raw, now)
}
