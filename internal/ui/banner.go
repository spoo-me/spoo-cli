package ui

import (
	_ "embed"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ghostArt is the spoo mascot — a density-ASCII render of the logo
// (ascii-image-converter, W=22). Regenerate with:
//
//	ascii-image-converter <white-ghost-on-black>.png -W 22
//
//go:embed ghost.txt
var ghostArt string

// ghostGradient runs vivid violet (top) → lavender (bottom), matching
// the logo's vertical fade.
var ghostGradient = []color.Color{
	lipgloss.Color("#7C3AED"),
	lipgloss.Color("#8B5CF6"),
	lipgloss.Color("#9B7CF6"),
	lipgloss.Color("#A78BFA"),
	lipgloss.Color("#C4B5FD"),
	lipgloss.Color("#DDD6FE"),
}

// Banner renders the gradient ghost beside the spoo wordmark.
func Banner() string {
	rows := strings.Split(strings.TrimRight(ghostArt, "\n"), "\n")
	n := len(rows)
	for i, ln := range rows {
		c := ghostGradient[min(i*len(ghostGradient)/max(1, n), len(ghostGradient)-1)]
		rows[i] = lipgloss.NewStyle().Foreground(c).Render(ln)
	}
	ghost := strings.Join(rows, "\n")

	wordmark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB")).Render("spoo") +
		"\n" + Dim.Render("the spoo.me CLI")
	// drop the wordmark to roughly the ghost's vertical center
	wordmark = strings.Repeat("\n", max(0, n/2-1)) + wordmark

	return lipgloss.JoinHorizontal(lipgloss.Top, ghost, "   ", wordmark)
}
