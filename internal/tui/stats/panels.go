package stats

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// panelGrid lays the breakdown panels out in responsive columns. A
// one-column gutter visually matches the stacked borders between rows
// (terminal cells are ~2:1), and the division remainder widens the
// leading panels so each row spans the full terminal width.
func (m Model) panelGrid() string {
	lay := m.panelLayout()

	var rows []string
	for _, chunk := range m.panelChunks() {
		row := make([]string, 0, lay.cols*2)
		for n, i := range chunk {
			if len(row) > 0 {
				row = append(row, " ")
			}
			row = append(row, m.panelView(i, lay.panelWidth(n), lay.contentRows, panelTopN))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}

func (m Model) panelView(idx, width, contentRows, topN int) string {
	p := m.panels()[idx]
	focused := !m.focusMode && idx == m.focus-1
	innerW := width - 4 // border-box width minus borders and padding

	if m.tableOn[p.key] {
		return m.boxed(p.title, m.panelTableBody(idx, innerW, contentRows, topN, focused, false, false),
			width, contentRows+3, focused, panelColors[p.key])
	}

	pts := m.panelPoints(idx, topN)
	var maxV float64
	for _, pt := range pts {
		maxV = max(maxV, pt.Value)
	}
	total := m.metricTotal()
	if p.key == "weekday" {
		// weekday buckets span the whole series; their own sum is the
		// honest denominator (the summary total can differ on bucketing)
		total = 0
		for _, pt := range pts {
			total += pt.Value
		}
	}
	panelHue := panelColors[p.key]

	// tight columns: the label column hugs the longest visible label
	labelW := 8
	for _, pt := range pts {
		labelW = max(labelW, lipgloss.Width(m.rowLabel(p.key, pt.Label))+1)
	}
	labelW = min(labelW, max(10, innerW/3))
	barMax := max(8, innerW-labelW-2-5-5-1) // -1: gap between label and bar

	lines := make([]string, 0, contentRows)
	if len(pts) == 0 {
		lines = append(lines, ui.Dim.Render("no data"))
	}
	for i, pt := range pts {
		label := kit.PadToWidth(kit.TruncateToWidth(m.rowLabel(p.key, pt.Label), labelW), labelW)

		count := fmt.Sprintf("%5.0f", pt.Value)
		pct := "     "
		if total > 0 {
			pct = fmt.Sprintf("%4.0f%%", pt.Value/total*100)
		}

		marker, labelStyle := "  ", lipgloss.NewStyle()
		if focused && i == m.sel[idx] {
			marker, labelStyle = ui.Title.Render("▸ "), ui.Title
		}
		lines = append(lines, marker+labelStyle.Render(label)+" "+
			ui.Bar(dashBarStyle, pt.Value, maxV, barMax, entityColor(pt.Label, panelHue))+
			count+ui.Dim.Render(pct))
	}
	return m.boxed(p.title, strings.Join(lines, "\n"), width, contentRows+3, focused, panelHue)
}

// rowLabel normalizes a point label for display.
func (m Model) rowLabel(panelKey, label string) string {
	if panelKey == "country" {
		return ui.CountryLabel(label)
	}
	return label
}

// columnTitle is the singular header for a panel's table view.
var columnTitle = map[string]string{
	"short_code": "link",
	"browser":    "browser",
	"os":         "os",
	"country":    "country",
	"city":       "city",
	"referrer":   "referrer",
	"weekday":    "weekday",
}

// dashTableStyle is the table style used across the dashboard panels
// (winner of the live A/B): tree rows under a header band. The time
// table stays band-only — dates aren't a hierarchy.
const dashTableStyle = tsTreeBand

// panelTableBody renders a panel's data as a styled table. withRank
// adds a leaderboard # column and withTotals a Σ footer (focus mode).
func (m Model) panelTableBody(idx, innerW, height, topN int, focused, withRank, withTotals bool) string {
	p := m.panels()[idx]
	pts := m.panelPoints(idx, topN)
	if len(pts) == 0 {
		return ui.Dim.Render("no data")
	}
	total := m.metricTotal()
	if p.key == "weekday" {
		total = 0
		for _, pt := range pts {
			total += pt.Value
		}
	}
	metricCol := "clicks"
	if m.metric == "unique_clicks" {
		metricCol = "unique"
	}

	header := []string{columnTitle[p.key], metricCol, "share"}
	widths := []int{max(10, innerW-22), 8, 8}
	labelIdx := 0
	if withRank {
		header = append([]string{"#"}, header...)
		labelIdx = 1
		widths = append([]int{3}, widths...)
		widths[1] = max(10, widths[1]-4)
	}
	var sum float64
	rows := make([][]string, 0, len(pts))
	for i, pt := range pts {
		sum += pt.Value
		share := ""
		if total > 0 {
			share = fmt.Sprintf("%.1f%%", pt.Value/total*100)
		}
		row := []string{m.rowLabel(p.key, pt.Label), fmt.Sprintf("%.0f", pt.Value), share}
		if withRank {
			row = append([]string{fmt.Sprintf("%d", i+1)}, row...)
		}
		rows = append(rows, row)
	}

	sel := -1
	if focused {
		sel = min(m.sel[idx], len(rows)-1)
	}
	out := styledTable(dashTableStyle, labelIdx, widths, header, rows, sel, height-2, innerW)

	if withTotals && total > 0 {
		totalsRow := []string{"Σ", fmt.Sprintf("%.0f", sum), fmt.Sprintf("%.1f%%", sum/total*100)}
		if withRank {
			totalsRow = append([]string{""}, totalsRow...)
		}
		out += "\n" + ui.Dim.Render(strings.Repeat("─", min(innerW, 40))) +
			"\n" + tsHeader.Render(styledTotals(widths, totalsRow))
	}
	return out
}

// styledTotals formats a totals row with the same column widths.
func styledTotals(widths []int, cells []string) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		if i == 0 {
			parts[i] = kit.PadToWidth(kit.TruncateToWidth(c, widths[i]), widths[i])
		} else {
			parts[i] = fmt.Sprintf("%*s", widths[i], c)
		}
	}
	return " " + strings.Join(parts, " ")
}

// timeTableBody renders the time series as a table (focus mode),
// most recent bucket first, with a Σ footer.
func (m Model) timeTableBody(innerW, height int) string {
	clicks := m.res.Points("time", "clicks")
	if len(clicks) == 0 {
		return ui.Dim.Render("no time series data")
	}
	uniq := map[string]float64{}
	for _, p := range m.res.Points("time", "unique_clicks") {
		uniq[p.Label] = p.Value
	}
	header := []string{"time", "clicks", "unique"}
	widths := []int{max(12, innerW-24), 10, 10}
	var sumC, sumU float64
	rows := make([][]string, 0, len(clicks))
	for i := len(clicks) - 1; i >= 0; i-- {
		p := clicks[i]
		sumC += p.Value
		sumU += uniq[p.Label]
		rows = append(rows, []string{
			p.Label, fmt.Sprintf("%.0f", p.Value), fmt.Sprintf("%.0f", uniq[p.Label]),
		})
	}
	out := styledTable(tsHeaderBand, 0, widths, header, rows, -1, height-4, innerW)
	out += "\n" + ui.Dim.Render(strings.Repeat("─", min(innerW, 40))) +
		"\n" + tsHeader.Render(styledTotals(widths, []string{"Σ", fmt.Sprintf("%.0f", sumC), fmt.Sprintf("%.0f", sumU)}))
	return out
}

// focusPanelBody renders a panel's rows at full size for focus mode.
func (m Model) focusPanelBody(idx, width int) string {
	p := m.panels()[idx]
	innerW := width - 4
	pts := m.panelPoints(idx, focusTopN)
	if len(pts) == 0 {
		return ui.Dim.Render("no data")
	}
	var maxV float64
	for _, pt := range pts {
		maxV = max(maxV, pt.Value)
	}
	total := m.metricTotal()
	if p.key == "weekday" {
		total = 0
		for _, pt := range pts {
			total += pt.Value
		}
	}
	panelHue := panelColors[p.key]

	labelW := 10
	for _, pt := range pts {
		labelW = max(labelW, lipgloss.Width(m.rowLabel(p.key, pt.Label))+1)
	}
	labelW = min(labelW, max(12, innerW*2/5))
	barMax := max(10, innerW-labelW-5-5-1-2) // -2: selection marker column

	lines := make([]string, 0, len(pts))
	for i, pt := range pts {
		label := kit.PadToWidth(kit.TruncateToWidth(m.rowLabel(p.key, pt.Label), labelW), labelW)
		count := fmt.Sprintf("%5.0f", pt.Value)
		pct := "     "
		if total > 0 {
			pct = fmt.Sprintf("%4.0f%%", pt.Value/total*100)
		}
		marker, labelStyle := "  ", lipgloss.NewStyle()
		if m.focusPane == 0 && i == m.sel[idx] {
			marker, labelStyle = ui.Title.Render("▸ "), ui.Title
		}
		lines = append(lines, marker+labelStyle.Render(label)+" "+
			ui.Bar(dashBarStyle, pt.Value, maxV, barMax, entityColor(pt.Label, panelHue))+
			count+ui.Dim.Render(pct))
	}
	return strings.Join(lines, "\n")
}
