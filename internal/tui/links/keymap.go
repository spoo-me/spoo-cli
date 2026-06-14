package links

import (
	"charm.land/bubbles/v2/key"

	"github.com/spoo-me/spoo-cli/internal/tui/kit"
)

// linksKeys feeds the help bubble in the links browser.
type linksKeys struct{}

func (linksKeys) ShortHelp() []key.Binding {
	return []key.Binding{
		kit.Bind("↑/↓", "move"),
		kit.Bind("enter", "details"),
		kit.Bind("/", "search"),
		kit.Bind("e", "edit"),
		kit.Bind("t", "archive"),
		kit.Bind("d", "delete"),
		kit.Bind("?", "more"),
		kit.Bind("q", "quit"),
	}
}

func (linksKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			kit.Bind("↑/↓", "move"),
			kit.Bind("←/→", "pages"),
			kit.Bind("enter", "details"),
			kit.Bind("/", "search"),
		},
		{
			kit.Bind("s", "sort"),
			kit.Bind("o", "open in browser"),
			kit.Bind("c", "copy short url"),
			kit.Bind("e", "edit link"),
		},
		{
			kit.Bind("t", "archive/activate"),
			kit.Bind("d", "delete"),
			kit.Bind("Q", "qr code"),
			kit.Bind("r", "refresh"),
		},
	}
}
