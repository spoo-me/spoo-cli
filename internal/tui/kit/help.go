package kit

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// NewHelp builds a help bubble in the dashboard's palette: keys at
// hint gray, descriptions a step fainter.
func NewHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(ui.Muted)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("#3C4048"))
	h.Styles.Ellipsis = h.Styles.ShortSeparator
	h.Styles.FullKey = h.Styles.ShortKey
	h.Styles.FullDesc = h.Styles.ShortDesc
	h.Styles.FullSeparator = h.Styles.ShortSeparator
	return h
}

// Bind is shorthand for a key.Binding whose help key and trigger match.
func Bind(keys, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(keys), key.WithHelp(keys, desc))
}
