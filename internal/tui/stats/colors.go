package stats

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// hueSaturated maps each panel pastel to its saturated cut — focused
// panels wear it on both border and title. Picked in the focus-style
// A/B over solid violet, thick/double borders, edge accents, and dim
// frames.
var hueSaturated = map[color.Color]color.Color{
	ui.Accent:  lipgloss.Color("#8B5CF6"),
	ui.Success: lipgloss.Color("#10B981"),
	ui.Blue:    lipgloss.Color("#0EA5E9"),
	ui.Yellow:  lipgloss.Color("#F59E0B"),
	ui.Pink:    lipgloss.Color("#EC4899"),
	ui.Teal:    lipgloss.Color("#14B8A6"),
}

// hueFor returns the focused-panel cut of a pastel, falling back to
// the violet for hues outside the dashboard palette.
func hueFor(pastel color.Color) color.Color {
	if c, ok := hueSaturated[pastel]; ok {
		return c
	}
	return hueSaturated[ui.Accent]
}

// entityColors maps well-known browsers, platforms, and referrers to
// the colors people associate with them (pastel-shifted for dark
// terminals). Matching is case-insensitive substring, first hit wins.
var entityColors = []struct {
	match string
	color color.Color
}{
	// browsers
	{"firefox", lipgloss.Color("#FB923C")},
	{"safari", lipgloss.Color("#38BDF8")},
	{"edge", lipgloss.Color("#2DD4BF")},
	{"opera", lipgloss.Color("#F87171")},
	{"brave", lipgloss.Color("#F97316")},
	{"chrome", lipgloss.Color("#FCD34D")},
	// operating systems
	{"windows", lipgloss.Color("#60A5FA")},
	{"android", lipgloss.Color("#4ADE80")},
	{"ubuntu", lipgloss.Color("#FB923C")},
	{"linux", lipgloss.Color("#FDE68A")},
	{"mac", lipgloss.Color("#E5E7EB")},
	{"ios", lipgloss.Color("#E5E7EB")},
	// referrers
	{"youtube", lipgloss.Color("#F87171")},
	{"twitter", lipgloss.Color("#7DD3FC")},
	{"x.com", lipgloss.Color("#7DD3FC")},
	{"t.co", lipgloss.Color("#7DD3FC")},
	{"facebook", lipgloss.Color("#818CF8")},
	{"instagram", lipgloss.Color("#F9A8D4")},
	{"reddit", lipgloss.Color("#FB923C")},
	{"github", lipgloss.Color("#D1D5DB")},
	{"linkedin", lipgloss.Color("#38BDF8")},
	{"discord", lipgloss.Color("#818CF8")},
	{"telegram", lipgloss.Color("#7DD3FC")},
	{"whatsapp", lipgloss.Color("#4ADE80")},
	{"google", lipgloss.Color("#93C5FD")},
	{"direct", lipgloss.Color("#9CA3AF")},
}

// entityColor picks a brand color for a label, falling back to the
// panel's own hue when the entity isn't a household name.
func entityColor(label string, fallback color.Color) color.Color {
	l := strings.ToLower(label)
	for _, e := range entityColors {
		if strings.Contains(l, e.match) {
			return e.color
		}
	}
	return fallback
}
