package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// tableStyle selects how a panel's table view is drawn.
type tableStyle int

const (
	tsUnderline    tableStyle = iota // UPPERCASE header over a ─ rule
	tsHeaderBand                     // inverted header strip
	tsSelectedBand                   // cursor row gets a background band
	tsTree                           // ├─ rows
	tsTreeBand                       // tree rows under a header band
)

var (
	tsBandBG  = lipgloss.NewStyle().Background(lipgloss.Color("#2D2B3A")).Foreground(lipgloss.Color("#C4B5FD")).Bold(true)
	tsSelBand = lipgloss.NewStyle().Background(lipgloss.Color("#312E45")).Foreground(lipgloss.Color("#C4B5FD")).Bold(true)
	tsHeader  = lipgloss.NewStyle().Foreground(ui.Muted).Bold(true)
)

// styledTable renders header+rows in the given style. sel == -1 means
// no cursor row; rows beyond maxRows are dropped.
func styledTable(ts tableStyle, widths []int, header []string, rows [][]string, sel, maxRows, width int) string {
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	tree := ts == tsTree || ts == tsTreeBand
	w := append([]int(nil), widths...)
	if tree {
		w[0] = max(6, w[0]-3) // branch glyphs take three columns
	}
	fmtRow := func(cells []string) string {
		parts := make([]string, len(cells))
		for i, c := range cells {
			if i == 0 {
				parts[i] = padToWidth(truncateToWidth(c, w[i]), w[i])
			} else {
				parts[i] = fmt.Sprintf("%*s", w[i], truncateToWidth(c, w[i]))
			}
		}
		return " " + strings.Join(parts, " ")
	}

	var out []string
	switch ts {
	case tsHeaderBand, tsTreeBand:
		out = append(out, tsBandBG.Render(fmtRow(header)))
	case tsUnderline:
		up := make([]string, len(header))
		for i, h := range header {
			up[i] = strings.ToUpper(h)
		}
		out = append(out, tsHeader.Render(fmtRow(up)))
		rule := 1
		for _, x := range w {
			rule += x + 1
		}
		out = append(out, ui.Dim.Render(strings.Repeat("─", min(rule, width))))
	default:
		out = append(out, tsHeader.Render(fmtRow(header)))
	}

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
		// selected rows get ONE style over the whole plain line — a
		// pre-styled branch would embed a reset that cuts it short
		case i == sel && ts == tsSelectedBand:
			line = tsSelBand.Render(content)
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
