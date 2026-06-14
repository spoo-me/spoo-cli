package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// newHelp builds a help bubble in the dashboard's palette: keys at
// hint gray, descriptions a step fainter.
func newHelp() help.Model {
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

func bind(keys, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(keys), key.WithHelp(keys, desc))
}

// statsDashKeys feeds the help bubble on the dashboard grid.
type statsDashKeys struct{}

func (statsDashKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		bind("↑↓←→", "navigate"),
		bind("enter", "drill down"),
		bind("f", "focus"),
		bind("g", "switch link"),
		bind("t", "table"),
		bind("T", "range"),
		bind("u", "metric"),
		bind("?", "more"),
		bind("q", "quit"),
	}
}

func (statsDashKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			bind("↑/↓", "select row"),
			bind("←/→ tab", "switch chart"),
			bind("enter", "drill down"),
			bind("x", "undo filter"),
		},
		{
			bind("f", "focus mode"),
			bind("g", "switch link"),
			bind("t", "table view"),
			bind("T", "time range"),
			bind("[/]", "older/newer"),
		},
		{
			bind("u", "clicks/unique"),
			bind("p", "vs previous"),
			bind("e", "export"),
			bind("a", "auto-refresh"),
		},
		{
			bind("r", "refresh"),
			bind("click", "focus/drill"),
			bind("wheel", "scroll rows"),
			bind("esc", "clear/quit"),
			bind("q", "quit"),
		},
	}
}

// statsFocusKeys feeds the help bubble in focus mode.
type statsFocusKeys struct{}

func (statsFocusKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		bind("←/→", "pane"),
		bind("↑/↓", "rows/charts"),
		bind("tab", "chart"),
		bind("enter", "drill"),
		bind("t", "table"),
		bind("x", "close"),
		bind("?", "more"),
	}
}

func (statsFocusKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			bind("←/→", "main/sidebar"),
			bind("↑/↓", "rows or charts"),
			bind("tab", "next chart"),
			bind("enter", "drill down"),
		},
		{
			bind("t", "table view"),
			bind("T", "time range"),
			bind("[/]", "older/newer"),
			bind("u", "clicks/unique"),
		},
		{
			bind("p", "vs previous"),
			bind("e", "export"),
			bind("r", "refresh"),
			bind("x", "exit focus"),
		},
	}
}

// linksKeys feeds the help bubble in the links browser.
type linksKeys struct{}

func (linksKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		bind("↑/↓", "move"),
		bind("enter", "details"),
		bind("/", "search"),
		bind("e", "edit"),
		bind("t", "archive"),
		bind("d", "delete"),
		bind("?", "more"),
		bind("q", "quit"),
	}
}

func (linksKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			bind("↑/↓", "move"),
			bind("←/→", "pages"),
			bind("enter", "details"),
			bind("/", "search"),
		},
		{
			bind("s", "sort"),
			bind("o", "open in browser"),
			bind("c", "copy short url"),
			bind("e", "edit link"),
		},
		{
			bind("t", "archive/activate"),
			bind("d", "delete"),
			bind("Q", "qr code"),
			bind("r", "refresh"),
		},
	}
}
