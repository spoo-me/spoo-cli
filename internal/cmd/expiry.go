package cmd

import (
	"fmt"
	"strconv"
	"time"
)

// parseExpiry normalizes --expires input. Durations ("30m", "72h")
// become epoch seconds relative to now; anything else passes through
// for the backend to validate (ISO 8601 or epoch).
func parseExpiry(raw string, now time.Time) (string, error) {
	if raw == "" {
		return "", nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if d <= 0 {
			return "", fmt.Errorf("--expires duration must be positive, got %q", raw)
		}
		return strconv.FormatInt(now.Add(d).Unix(), 10), nil
	}
	return raw, nil
}
