package api

import (
	"fmt"
	"strconv"
	"time"
)

// ParseExpiry normalizes user expiry input to RFC 3339, which the
// backend always accepts. Durations ("30m", "72h") are relative to
// now; bare epoch seconds are converted; anything else passes through
// as ISO 8601. Empty input yields an empty string (no expiry change).
func ParseExpiry(raw string, now time.Time) (string, error) {
	if raw == "" {
		return "", nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if d <= 0 {
			return "", fmt.Errorf("expiry duration must be positive, got %q", raw)
		}
		return now.Add(d).UTC().Format(time.RFC3339), nil
	}
	if epoch, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(epoch, 0).UTC().Format(time.RFC3339), nil
	}
	return raw, nil
}
