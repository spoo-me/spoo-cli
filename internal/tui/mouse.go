package tui

import (
	tea "charm.land/bubbletea/v2"
)

// Mouse support for the dashboard grid: click a panel to focus it,
// click a row to select it (again to drill), wheel to move the row
// selection — or page the window when wheeling over the time chart.
// Regions are derived from the same math the renderer uses, so they
// can't drift from what's on screen.

// hitRegion is one clickable rectangle, named by its focus-space item
// (0 = time chart, 1.. = panels()).
type hitRegion struct {
	x, y, w, h int
	item       int
}

func (r hitRegion) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// hitRegions maps the dashboard layout. Modals and focus mode have
// their own (keyboard) interaction models, so they get no regions.
func (m StatsModel) hitRegions() []hitRegion {
	if m.res == nil || m.focusMode || m.exportBox.open || m.switchMode || m.rangeMode {
		return nil
	}
	y0 := 1 // header line
	if len(m.filters) > 0 {
		y0++
	}
	chartH := m.chartHeight()
	overviewW := m.overviewWidth()
	topH := chartH + 4
	regions := []hitRegion{
		{x: overviewW + 1, y: y0, w: m.width - overviewW - 1, h: topH, item: 0},
	}

	lay := m.panelLayout()
	panelH := lay.contentRows + 3
	gridY := y0 + topH
	for r, chunk := range m.panelChunks() {
		x := 0
		for n, i := range chunk {
			w := lay.panelWidth(n)
			regions = append(regions, hitRegion{x: x, y: gridY + r*panelH, w: w, h: panelH, item: i + 1})
			x += w + 1
		}
	}
	return regions
}

// rowAt translates a click's y inside a panel region to a row index,
// honoring the extra header line of table mode.
func (m StatsModel) rowAt(reg hitRegion, y int) int {
	offset := 2 // border + title
	if m.tableOn[m.panels()[reg.item-1].key] {
		offset = 3 // + the table's header band
	}
	return y - reg.y - offset
}

// handleClick focuses what was clicked; clicking the already-selected
// row drills into it.
func (m StatsModel) handleClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	for _, reg := range m.hitRegions() {
		if !reg.contains(msg.X, msg.Y) {
			continue
		}
		m.status = ""
		prevFocus := m.focus
		m.focus = reg.item
		if reg.item == 0 {
			return m, nil
		}
		idx := reg.item - 1
		row := m.rowAt(reg, msg.Y)
		if pts := m.panelPoints(idx, panelTopN); row >= 0 && row < len(pts) {
			if prevFocus == reg.item && m.sel[idx] == row {
				return m.drill(idx, panelTopN)
			}
			m.sel[idx] = row
		}
		return m, nil
	}
	return m, nil
}

// handleWheel scrolls the row selection of the panel under the
// pointer; over the time chart it pages the window.
func (m StatsModel) handleWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	delta := 0
	switch msg.Button {
	case tea.MouseWheelDown:
		delta = 1
	case tea.MouseWheelUp:
		delta = -1
	default:
		return m, nil
	}
	for _, reg := range m.hitRegions() {
		if !reg.contains(msg.X, msg.Y) {
			continue
		}
		if reg.item == 0 { // time chart: wheel pages through history
			if delta > 0 {
				m.offset++
			} else if m.offset > 0 {
				m.offset--
			} else {
				return m, nil
			}
			m.loading = true
			return m, m.fetch()
		}
		idx := reg.item - 1
		m.focus = reg.item
		if n := len(m.panelPoints(idx, panelTopN)); n > 0 {
			m.sel[idx] = min(max(m.sel[idx]+delta, 0), n-1)
		}
		return m, nil
	}
	return m, nil
}
