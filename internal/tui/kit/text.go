// Package kit holds the shared Bubble Tea widgets and rendering
// primitives used by the stats and links screens.
package kit

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// PadToWidth right-pads by display width (emoji-safe, unlike %-*s).
func PadToWidth(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// TruncateToWidth trims a string to at most w display columns.
func TruncateToWidth(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// ISODate keeps the YYYY-MM-DD prefix of an ISO 8601 timestamp.
func ISODate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// OrNever renders an empty value as "never".
func OrNever(s string) string {
	if s == "" {
		return "never"
	}
	return s
}
