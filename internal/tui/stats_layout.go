package tui

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// ── layout ────────────────────────────────────────────────────────────

// overviewWidth scales the overview panel with the terminal (~30% of
// the width) instead of pinching it on wide screens.
func (m StatsModel) overviewWidth() int {
	return min(64, max(36, m.width*3/10))
}

func (m StatsModel) gridCols() int {
	switch {
	case m.width >= threeColMin:
		return 3
	case m.width >= twoColMin:
		return 2
	default:
		return 1
	}
}

// uniformRows is the shared content height of every breakdown panel:
// sized by the fullest panel so the grid reads as one deliberate unit.
func (m StatsModel) uniformRows() int {
	rows := 3
	for i := range m.panels() {
		rows = max(rows, len(m.panelPoints(i, panelTopN)))
	}
	return min(rows, panelTopN)
}

func (m StatsModel) panelChunks() [][]int {
	cols := m.gridCols()
	n := len(m.panels())
	var chunks [][]int
	for start := 0; start < n; start += cols {
		end := min(start+cols, n)
		row := make([]int, 0, cols)
		for i := start; i < end; i++ {
			row = append(row, i)
		}
		chunks = append(chunks, row)
	}
	return chunks
}

// chartHeight gives the time chart the height the grid doesn't need.
func (m StatsModel) chartHeight() int {
	used := 2 /*header+footer*/ + 2 /*chart box borders*/ + 2 /*title+legend*/
	if m.helper.ShowAll {
		used += 3 // the full help table runs four lines, not one
	}
	if len(m.filters) > 0 {
		used++
	}
	rows := m.uniformRows()
	used += len(m.panelChunks()) * (rows + 3)
	return min(20, max(7, m.height-used-1))
}

// boxed wraps content in the dashboard's standard bordered panel.
// width/height are border-box totals (lipgloss v2 semantics). hue is
// the panel's pastel; when focused, border and title both take its
// saturated cut.
func (m StatsModel) boxed(title, body string, width, height int, focused bool, hue color.Color) string {
	borderColor, titleStyle := color.Color(ui.Muted), ui.Dim.Bold(true)
	if focused {
		sat := hueFor(hue)
		borderColor = sat
		titleStyle = lipgloss.NewStyle().Bold(true).Foreground(sat)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(height).
		Render(titleStyle.Render("✦ "+title) + "\n" + body)
}
