package stats

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// tableStyle selects how a table's header and rows are drawn. Both
// variants put the header in an inverted band (the winner of a live
// A/B); they differ only in whether rows get ├─ tree branches.
type tableStyle int

const (
	tsHeaderBand tableStyle = iota // header band, flat rows (the time table)
	tsTreeBand                     // header band over ├─ tree rows (panels)
)

var (
	tsBandBG = lipgloss.NewStyle().Background(lipgloss.Color("#2D2B3A")).Foreground(lipgloss.Color("#C4B5FD")).Bold(true)
	tsHeader = lipgloss.NewStyle().Foreground(ui.Muted).Bold(true)
)

// styledTable renders header+rows in the given style. labelIdx names
// the text column (left-aligned, absorbs the tree branch width — it is
// not always 0: a rank column may precede it). sel == -1 means no
// cursor row; rows beyond maxRows are dropped.
func styledTable(ts tableStyle, labelIdx int, widths []int, header []string, rows [][]string, sel, maxRows, width int) string {
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	tree := ts == tsTreeBand
	w := append([]int(nil), widths...)
	if tree {
		w[labelIdx] = max(6, w[labelIdx]-3) // branch glyphs take three columns
	}
	fmtRow := func(cells []string) string {
		parts := make([]string, len(cells))
		for i, c := range cells {
			if i == labelIdx {
				parts[i] = kit.PadToWidth(kit.TruncateToWidth(c, w[i]), w[i])
			} else {
				parts[i] = fmt.Sprintf("%*s", w[i], kit.TruncateToWidth(c, w[i]))
			}
		}
		return " " + strings.Join(parts, " ")
	}

	// the header band stretches across the full panel width
	out := []string{tsBandBG.Render(kit.PadToWidth(fmtRow(header), width))}

	for i, r := range rows {
		plain := fmtRow(r)
		branch := ""
		if tree {
			branch = "├─"
			if i == len(rows)-1 {
				branch = "╰─"
			}
		}
		content := plain
		if tree {
			content = " " + branch + plain
		}
		var line string
		switch {
		// a selected row renders as ONE styled run over the whole line —
		// a pre-styled branch would embed a reset that cuts it short
		case i == sel:
			line = ui.Title.Render(content)
		case tree:
			line = " " + ui.Dim.Render(branch) + plain
		default:
			line = plain
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
