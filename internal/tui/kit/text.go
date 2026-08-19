// Package kit holds the shared Bubble Tea widgets and rendering
// primitives used by the stats and links screens.
package kit

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	spoo "github.com/spoo-me/spoo-go"
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

// Day renders a timestamp as YYYY-MM-DD, empty when unset.
func Day(t spoo.Timestamp) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// OrNever renders an empty value as "never".
func OrNever(s string) string {
	if s == "" {
		return "never"
	}
	return s
}
