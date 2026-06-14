package tui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// padToWidth right-pads by display width (emoji-safe, unlike %-*s).
func padToWidth(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// truncateToWidth trims a string to at most w display columns.
func truncateToWidth(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// isoDate keeps the YYYY-MM-DD prefix of an ISO 8601 timestamp.
func isoDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// orNever renders an empty value as "never".
func orNever(s string) string {
	if s == "" {
		return "never"
	}
	return s
}
