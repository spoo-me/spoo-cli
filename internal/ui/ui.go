// Package ui holds the lipgloss styles shared by every spoo command,
// so CLI output and future TUI views render with one visual language.
package ui

import (
	"image/color"
	"math"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

var (
	Accent  = lipgloss.Color("#A78BFA") // spoo violet
	Success = lipgloss.Color("#34D399")
	Danger  = lipgloss.Color("#F87171")
	Muted   = lipgloss.Color("#9CA3AF")

	// pastel chart palette (one hue per dashboard panel)
	Blue   = lipgloss.Color("#7DD3FC")
	Yellow = lipgloss.Color("#FDE68A")
	Pink   = lipgloss.Color("#F9A8D4")
	Teal   = lipgloss.Color("#5EEAD4")

	Title   = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	OK      = lipgloss.NewStyle().Bold(true).Foreground(Success)
	Err     = lipgloss.NewStyle().Bold(true).Foreground(Danger)
	Dim     = lipgloss.NewStyle().Foreground(Muted)
	Box     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Accent).Padding(0, 2)
	KeyHint = lipgloss.NewStyle().Foreground(Muted).Italic(true)
)

// SparkRunes are the eight block heights used for sparkline charts.
var SparkRunes = []rune("▁▂▃▄▅▆▇█")

// BarStyle selects how horizontal bars are drawn.
type BarStyle int

const (
	BarCapped     BarStyle = iota // ╺━━━╸ on ╌╌ track
	BarUpperHalf                  // ▀▀▀▀ on ·· track
	BarHalf                       // ▄▄▄▄ on ·· track
	BarSegmented                  // ▰▰▰▰ on ▱▱ track
	BarDoubleLine                 // ════ on ── track
	BarFade                       // ██▓▒░ on ·· track
)

// Bar renders a horizontal bar scaled to value/maxV over width columns.
func Bar(style BarStyle, value, maxV float64, width int, c color.Color) string {
	if width < 2 {
		width = 2
	}
	w := 0
	if maxV > 0 && value > 0 {
		w = max(1, int(math.Round(value/maxV*float64(width))))
	}
	fill := lipgloss.NewStyle().Foreground(c)

	switch style {
	case BarUpperHalf:
		return fill.Render(strings.Repeat("▀", w)) + Dim.Render(strings.Repeat("·", width-w))
	case BarHalf:
		return fill.Render(strings.Repeat("▄", w)) + Dim.Render(strings.Repeat("·", width-w))
	case BarSegmented:
		return fill.Render(strings.Repeat("▰", w)) + Dim.Render(strings.Repeat("▱", width-w))
	case BarDoubleLine:
		return fill.Render(strings.Repeat("═", w)) + Dim.Render(strings.Repeat("─", width-w))
	case BarFade:
		solid := max(0, w-3)
		tail := "▓▒░"[:min(3, w-solid)*3] // ▓▒░ are 3 bytes each
		return fill.Render(strings.Repeat("█", solid)+tail) + Dim.Render(strings.Repeat("·", width-w))
	default: // BarCapped
		track := Dim.Render(strings.Repeat("╌", width-w))
		switch w {
		case 0:
			return Dim.Render(strings.Repeat("╌", width))
		case 1:
			return fill.Render("╺") + track
		default:
			return fill.Render("╺"+strings.Repeat("━", w-2)+"╸") + track
		}
	}
}

// CountryLabel normalizes country codes for display. The backend
// reports unknown geo as "XX"; everything else passes through as the
// plain ISO alpha-2 code (no emoji — flag glyph support is too uneven
// across terminal fonts).
func CountryLabel(code string) string {
	if code == "XX" {
		return "Unknown" // match the backend's casing for unknown cities
	}
	return code
}
