package stats

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.headerLine() + "\n")
	if line := m.filterLine(); line != "" {
		b.WriteString(line + "\n")
	}

	switch {
	case m.fetchErr != nil:
		b.WriteString("\n" + ui.Err.Render("✗ "+m.fetchErr.Error()) + "\n")
	case m.res == nil:
		b.WriteString("\n" + ui.Dim.Render("loading…") + "\n")
	case m.focusMode:
		b.WriteString(m.focusView() + "\n")
	default:
		chartH := m.chartHeight()
		overviewW := m.overviewWidth()
		chartBoxW := m.width - overviewW - 1
		chartFocused := !m.focusMode && m.focus == 0
		tc := m.timeChartView()
		title, chartBody := tc.title(), ""
		if m.tableOn["time"] {
			title += " · table"
			chartBody = m.timeTableBody(chartBoxW-4, chartH+1)
		} else {
			chartBody = tc.legend() + "\n" + tc.render(chartBoxW-4, chartH)
		}
		overview := overviewCard{
			res: m.res, prev: m.prev, metric: m.metric,
			span: m.win.span, labelW: min(20, max(13, m.overviewWidth()-20)),
		}
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			m.boxed("overview", overview.render(), overviewW, chartH+4, false, ui.Accent),
			" ",
			m.boxed(title, chartBody, chartBoxW, chartH+4, chartFocused, m.metricHue()),
		)
		b.WriteString(top + "\n")
		b.WriteString(m.panelGrid() + "\n")
	}

	if m.status != "" {
		b.WriteString(m.status + "\n")
	}
	switch {
	case m.rangeMode:
		strip := ui.Title.Render("range ") + m.rangeBox.View()
		// a persistent cheat-sheet rides the strip — dimmer than regular
		// hints so the input stays the loudest thing on the line; errors
		// take its place
		faint := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
		tail, tailStyle := "e.g. 7d · 24h · 4h · 5m · now - 2w to now - 1w · 2026-01-01 to 2026-02-15 · enter apply · esc cancel", faint
		if m.rangeErr != "" {
			tail, tailStyle = "✗ "+m.rangeErr, ui.Err
		}
		if room := m.width - lipgloss.Width(strip) - 2; room > 4 {
			strip += "  " + tailStyle.Render(kit.TruncateToWidth(tail, room))
		}
		b.WriteString(strip)
	case m.focusMode:
		b.WriteString(m.helper.View(statsFocusKeys{}))
	default:
		b.WriteString(m.helper.View(statsDashKeys{}))
	}

	content := b.String()
	switch {
	case m.exportBox.open:
		content = kit.Center(content, m.exportBox.view(m.width), m.width, m.height)
	case m.switchMode:
		content = kit.Center(content, m.switcherView(), m.width, m.height)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion // click focuses, wheel scrolls
	return v
}

func (m Model) headerLine() string {
	target := "all links"
	if m.target != "" {
		target = m.target
	}
	h := ui.Title.Render("✦ spoo stats") + ui.Dim.Render("  ·  ") + target
	if m.res != nil && m.res.TimeRange.StartDate != "" {
		h += ui.Dim.Render("  ·  " + kit.ISODate(m.res.TimeRange.StartDate) + " → " + kit.ISODate(m.res.TimeRange.EndDate))
	} else {
		h += ui.Dim.Render("  ·  last " + m.win.label)
	}
	if m.offset > 0 {
		past := lipgloss.NewStyle().Foreground(ui.Yellow)
		h += ui.Dim.Render("  ·  ") + past.Render(fmt.Sprintf("≪ %d window%s back", m.offset, plural(m.offset)))
	}
	metricStyle := lipgloss.NewStyle().Bold(true).Foreground(m.metricHue())
	h += ui.Dim.Render("  ·  metric: ") + metricStyle.Render(strings.ReplaceAll(m.metric, "_", " "))
	if m.auto {
		h += ui.Dim.Render("  ·  ") + ui.OK.Render("auto 30s")
	}
	if m.loading {
		h += ui.Dim.Render("  ⟳")
	}
	return h
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) filterLine() string {
	if len(m.filters) == 0 {
		return ""
	}
	chips := make([]string, len(m.filters))
	for i, f := range m.filters {
		chips[i] = ui.Title.Render(f.dim) + ui.Dim.Render("=") + f.value
	}
	return ui.Dim.Render("  filtered: ") + strings.Join(chips, ui.Dim.Render(" · "))
}

// ── focus mode ────────────────────────────────────────────────────────

// focusView fills the screen with one chart and lists the rest in a
// sidebar; j/k walks the sidebar and the main area follows.
func (m Model) focusView() string {
	mainW := m.width - sidebarW - 1
	mainH := m.height - 4
	if m.status != "" {
		mainH--
	}

	mainFocused := m.focusPane == 0
	var main string
	if m.focusItem == 0 {
		tc := m.timeChartView()
		if m.tableOn["time"] {
			main = m.boxed(tc.title()+" · table", m.timeTableBody(mainW-4, mainH-3), mainW, mainH, mainFocused, m.metricHue())
		} else {
			main = m.boxed(tc.title(), tc.legend()+"\n"+tc.render(mainW-4, mainH-4), mainW, mainH, mainFocused, m.metricHue())
		}
	} else {
		idx := m.focusItem - 1
		p := m.panels()[idx]
		body := m.focusPanelBody(idx, mainW)
		if m.tableOn[p.key] {
			body = m.panelTableBody(idx, mainW-4, mainH-3, focusTopN, mainFocused, true, true)
		}
		main = m.boxed(p.title, body, mainW, mainH, mainFocused, panelColors[p.key])
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, main, " ", m.sidebar(mainH))
}

// sidebar lists all focusable charts, each with a mini preview: a
// sparkline for the time chart, the top rows as tiny bars for panels.
// Previews shrink away on short terminals.
func (m Model) sidebar(height int) string {
	panels := m.panels()
	nItems := len(panels) + 1
	innerW := sidebarW - 4
	avail := height - 3 // borders + "✦ charts" heading

	// how many preview lines fit per item (plus a spacer line when roomy)
	previewLines, spacer := 0, 0
	switch {
	case avail >= nItems*4-1:
		previewLines, spacer = 2, 1
	case avail >= nItems*3:
		previewLines = 2
	case avail >= nItems*2:
		previewLines = 1
	}

	titleFor := func(i int) string {
		if i == 0 {
			return "traffic over time"
		}
		return panels[i-1].title
	}

	var lines []string
	for i := 0; i < nItems; i++ {
		if i == m.focusItem {
			lines = append(lines, ui.Title.Render("▸ "+titleFor(i)))
		} else {
			lines = append(lines, ui.Dim.Render("  "+titleFor(i)))
		}
		if previewLines > 0 {
			lines = append(lines, m.sidebarPreview(i, innerW-2, previewLines)...)
		}
		if spacer > 0 && i < nItems-1 {
			lines = append(lines, "")
		}
	}
	return m.boxed("charts", strings.Join(lines, "\n"), sidebarW, height, m.focusPane == 1, ui.Accent)
}

// sidebarPreview renders up to n compact lines for a sidebar item.
func (m Model) sidebarPreview(item, width, n int) []string {
	if item == 0 { // time chart → one-line sparkline
		spark := kit.MiniSpark(m.res.Points("time", m.metric), width)
		return []string{"  " + ui.OK.Render(spark)}[:min(1, n)]
	}
	p := m.panels()[item-1]
	pts := m.panelPoints(item-1, n)
	if len(pts) == 0 {
		return []string{"  " + ui.Dim.Render("no data")}
	}
	maxV := pts[0].Value
	for _, pt := range pts {
		maxV = max(maxV, pt.Value)
	}
	labelW := 10
	barW := max(4, width-labelW-1)
	out := make([]string, 0, len(pts))
	for _, pt := range pts {
		label := kit.PadToWidth(kit.TruncateToWidth(m.rowLabel(p.key, pt.Label), labelW), labelW)
		out = append(out, "  "+ui.Dim.Render(label)+
			ui.Bar(dashBarStyle, pt.Value, maxV, barW, entityColor(pt.Label, panelColors[p.key])))
	}
	return out
}
