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

	w := append([]int(nil), widths...)
	if ts == tsTree {
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
	case tsHeaderBand:
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
		line := fmtRow(r)
		if ts == tsTree {
			branch := "├─"
			if i == len(rows)-1 {
				branch = "╰─"
			}
			line = " " + ui.Dim.Render(branch) + line
		}
		switch {
		case i == sel && ts == tsSelectedBand:
			line = tsSelBand.Render(line)
		case i == sel:
			line = ui.Title.Render(line)
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
