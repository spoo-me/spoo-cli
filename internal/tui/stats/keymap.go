package stats

import (
	"charm.land/bubbles/v2/key"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
)

// statsDashKeys feeds the help bubble on the dashboard grid.
type statsDashKeys struct{}

func (statsDashKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		kit.Bind("↑↓←→", "navigate"),
		kit.Bind("enter", "drill down"),
		kit.Bind("f", "focus"),
		kit.Bind("g", "switch link"),
		kit.Bind("t", "table"),
		kit.Bind("T", "range"),
		kit.Bind("u", "metric"),
		kit.Bind("?", "more"),
		kit.Bind("q", "quit"),
	}
}

func (statsDashKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			kit.Bind("↑/↓", "select row"),
			kit.Bind("←/→ tab", "switch chart"),
			kit.Bind("enter", "drill down"),
			kit.Bind("x", "undo filter"),
		},
		{
			kit.Bind("f", "focus mode"),
			kit.Bind("g", "switch link"),
			kit.Bind("t", "table view"),
			kit.Bind("T", "time range"),
			kit.Bind("[/]", "older/newer"),
		},
		{
			kit.Bind("u", "clicks/unique"),
			kit.Bind("p", "vs previous"),
			kit.Bind("e", "export"),
			kit.Bind("a", "auto-refresh"),
		},
		{
			kit.Bind("r", "refresh"),
			kit.Bind("click", "focus/drill"),
			kit.Bind("wheel", "scroll rows"),
			kit.Bind("esc", "clear/quit"),
			kit.Bind("q", "quit"),
		},
	}
}

// statsFocusKeys feeds the help bubble in focus mode.
type statsFocusKeys struct{}

func (statsFocusKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		kit.Bind("←/→", "pane"),
		kit.Bind("↑/↓", "rows/charts"),
		kit.Bind("tab", "chart"),
		kit.Bind("enter", "drill"),
		kit.Bind("t", "table"),
		kit.Bind("x", "close"),
		kit.Bind("?", "more"),
	}
}

func (statsFocusKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			kit.Bind("←/→", "main/sidebar"),
			kit.Bind("↑/↓", "rows or charts"),
			kit.Bind("tab", "next chart"),
			kit.Bind("enter", "drill down"),
		},
		{
			kit.Bind("t", "table view"),
			kit.Bind("T", "time range"),
			kit.Bind("[/]", "older/newer"),
			kit.Bind("u", "clicks/unique"),
		},
		{
			kit.Bind("p", "vs previous"),
			kit.Bind("e", "export"),
			kit.Bind("r", "refresh"),
			kit.Bind("x", "exit focus"),
		},
	}
}
