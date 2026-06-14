package ui

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ghostArt is the spoo mascot in blocks: a capped dome, gradient body,
// and notched feet — faceless, like the logo.
var ghostArt = []string{
	" ▄███▄ ",
	"███████",
	"███████",
	"███████",
	"███████",
	"█ █ █ █",
}

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

// Banner renders the ghost beside the spoo wordmark and tagline.
func Banner() string {
	lines := make([]string, len(ghostArt))
	for i, ln := range ghostArt {
		c := ghostGradient[min(i, len(ghostGradient)-1)]
		lines[i] = lipgloss.NewStyle().Foreground(c).Render(ln)
	}
	ghost := strings.Join(lines, "\n")

	wordmark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB")).Render("spoo") +
		"\n" + Dim.Render("the spoo.me CLI")
	// vertically center the wordmark against the ghost
	wordmark = "\n\n" + wordmark

	return lipgloss.JoinHorizontal(lipgloss.Top, ghost, "   ", wordmark)
}
