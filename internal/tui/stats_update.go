package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.helper.SetWidth(msg.Width)
		return m, nil

	case statsLoadedMsg:
		m.loading = false
		m.res, m.prev, m.fetchErr = msg.res, msg.prev, msg.err
		return m, nil

	case exportDoneMsg:
		if msg.err != nil {
			m.status = ui.Err.Render("✗ export failed: " + msg.err.Error())
		} else {
			m.status = ui.OK.Render("✓ exported " + msg.name)
		}
		return m, nil

	case autoTickMsg:
		if !m.auto {
			return m, nil
		}
		m.loading = true
		return m, tea.Batch(m.fetch(), autoTick())

	case linksListMsg:
		if msg.err != nil {
			m.switchMode = false
			m.status = ui.Err.Render("✗ couldn't list links: " + msg.err.Error())
			return m, nil
		}
		m.switchAll = msg.items
		return m, nil

	case tea.MouseClickMsg:
		return m.handleClick(msg)

	case tea.MouseWheelMsg:
		return m.handleWheel(msg)

	case tea.KeyPressMsg:
		m.status = ""
		if m.exportBox.open {
			return m.updateExport(msg)
		}
		if m.switchMode {
			return m.updateSwitcher(msg)
		}
		if m.rangeMode {
			return m.updateRange(msg)
		}
		if m.focusMode {
			return m.updateFocusMode(msg)
		}
		return m.updateDashboard(msg)
	}
	if m.exportBox.open { // directory reads, blink ticks
		return m.updateExport(msg)
	}
	return m, nil
}

// updateExport routes traffic to the export dialog and fires the
// download once it confirms.
func (m StatsModel) updateExport(msg tea.Msg) (tea.Model, tea.Cmd) {
	var req *exportRequest
	var cmd tea.Cmd
	m.exportBox, req, cmd = m.exportBox.handle(msg)
	if req != nil {
		m.status = ui.Dim.Render("exporting…")
		return m, m.export(*req)
	}
	return m, cmd
}

// updateRange handles keys while the range-expression strip is open.
func (m StatsModel) updateRange(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.rangeMode = false
		m.rangeErr = ""
		m.rangeBox.Blur()
		return m, nil
	case "enter":
		win, err := parseRangeExpr(m.rangeBox.Value(), time.Now().UTC())
		if err != nil {
			m.rangeErr = err.Error()
			return m, nil
		}
		m.rangeMode = false
		m.rangeErr = ""
		m.rangeBox.Blur()
		m.win = win
		m.offset = 0
		m.loading = true
		return m, m.fetch()
	}
	var cmd tea.Cmd
	m.rangeBox, cmd = m.rangeBox.Update(msg)
	m.rangeErr = ""
	return m, cmd
}

// updateFocusMode handles keys while a single chart fills the screen.
// ←/→ moves focus between the main view and the sidebar; ↑/↓ moves
// rows in the main view or switches charts in the sidebar.
func (m StatsModel) updateFocusMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	items := len(m.panels()) + 1 // + the time chart
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "x", "esc", "f":
		m.focusMode = false
		m.focus = m.focusItem // keep the dashboard cursor where you left off
		return m, nil
	case "left", "h":
		m.focusPane = 0
	case "right", "l":
		m.focusPane = 1
	case "tab":
		m.focusItem = (m.focusItem + 1) % items
	case "shift+tab":
		m.focusItem = (m.focusItem + items - 1) % items
	case "down", "j":
		if m.focusPane == 1 {
			m.focusItem = (m.focusItem + 1) % items
			break
		}
		if m.focusItem > 0 {
			idx := m.focusItem - 1
			if n := len(m.panelPoints(idx, focusTopN)); n > 0 {
				m.sel[idx] = min(m.sel[idx]+1, n-1)
			}
		}
	case "up", "k":
		if m.focusPane == 1 {
			m.focusItem = (m.focusItem + items - 1) % items
			break
		}
		if m.focusItem > 0 {
			idx := m.focusItem - 1
			m.sel[idx] = max(m.sel[idx]-1, 0)
		}
	case "enter":
		if m.focusPane == 0 && m.focusItem > 0 {
			return m.drill(m.focusItem-1, focusTopN)
		}
	case "u":
		m.metric = otherMetricKey(m.metric)
	case "t":
		key := "time"
		if m.focusItem > 0 {
			key = m.panels()[m.focusItem-1].key
		}
		m.tableOn[key] = !m.tableOn[key]
	case "T", "shift+t":
		return m.openRange()
	case "[":
		m.offset++
		m.loading = true
		return m, m.fetch()
	case "]":
		if m.offset > 0 {
			m.offset--
			m.loading = true
			return m, m.fetch()
		}
	case "e":
		return m.openExport()
	case "g":
		return m.openSwitcher()
	case "p":
		m.showPrev = !m.showPrev
		if m.showPrev && m.prev == nil {
			m.status = ui.Dim.Render("no previous-window data yet")
		}
	case "?":
		m.helper.ShowAll = !m.helper.ShowAll
	case "r":
		m.loading = true
		return m, m.fetch()
	}
	return m, nil
}

// openRange opens the range-expression strip in place of the hints.
func (m StatsModel) openRange() (tea.Model, tea.Cmd) {
	m.rangeMode = true
	m.rangeErr = ""
	m.rangeBox.SetValue("")
	return m, m.rangeBox.Focus()
}

// updateDashboard handles keys in the regular grid view. The focus
// space matches focus mode: 0 is the time chart, 1.. the grid panels.
func (m StatsModel) updateDashboard(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	items := len(m.panels()) + 1
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if len(m.filters) > 0 {
			m.filters = nil
			m.loading = true
			return m, m.fetch()
		}
		return m, tea.Quit
	case "x", "backspace":
		if len(m.filters) > 0 {
			m.filters = m.filters[:len(m.filters)-1]
			m.loading = true
			return m, m.fetch()
		}
	case "f":
		m.focusMode = true
		m.focusItem = m.focus // same index space
		m.focusPane = 0
		return m, nil
	case "tab", "right", "l":
		m.focus = (m.focus + 1) % items
	case "shift+tab", "left", "h":
		m.focus = (m.focus + items - 1) % items
	case "down", "j":
		if m.focus > 0 {
			idx := m.focus - 1
			if n := len(m.panelPoints(idx, panelTopN)); n > 0 {
				m.sel[idx] = min(m.sel[idx]+1, n-1)
			}
		}
	case "up", "k":
		if m.focus > 0 {
			idx := m.focus - 1
			m.sel[idx] = max(m.sel[idx]-1, 0)
		}
	case "enter":
		if m.focus > 0 {
			return m.drill(m.focus-1, panelTopN)
		}
	case "u":
		m.metric = otherMetricKey(m.metric)
	case "t":
		key := "time"
		if m.focus > 0 {
			key = m.panels()[m.focus-1].key
		}
		m.tableOn[key] = !m.tableOn[key]
	case "T", "shift+t":
		return m.openRange()
	case "[":
		m.offset++
		m.loading = true
		return m, m.fetch()
	case "]":
		if m.offset > 0 {
			m.offset--
			m.loading = true
			return m, m.fetch()
		}
	case "e":
		return m.openExport()
	case "a":
		m.auto = !m.auto
		if m.auto {
			return m, autoTick()
		}
	case "g":
		return m.openSwitcher()
	case "p":
		m.showPrev = !m.showPrev
		if m.showPrev && m.prev == nil {
			m.status = ui.Dim.Render("no previous-window data yet")
		}
	case "?":
		m.helper.ShowAll = !m.helper.ShowAll
	case "r":
		m.loading = true
		return m, m.fetch()
	}
	return m, nil
}

// drill adds a server-side filter for the selected row of panel idx.
func (m StatsModel) drill(idx, topN int) (tea.Model, tea.Cmd) {
	dim := m.panels()[idx].key
	if dim == "weekday" {
		m.status = ui.Dim.Render("weekdays are computed locally — nothing to drill into")
		return m, nil
	}
	pts := m.panelPoints(idx, topN)
	i := min(m.sel[idx], len(pts)-1)
	if i < 0 {
		return m, nil
	}
	f := filterEntry{dim: dim, value: pts[i].Label}
	if m.hasFilter(f) {
		return m, nil
	}
	m.filters = append(m.filters, f)
	m.sel = map[int]int{}
	m.loading = true
	return m, m.fetch()
}
