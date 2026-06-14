package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	tslc "github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func (m StatsModel) View() tea.View {
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
		title, chartBody := m.chartTitle(), ""
		if m.tableOn["time"] {
			title += " · table"
			chartBody = m.timeTableBody(chartBoxW-4, chartH+1)
		} else {
			chartBody = m.chartLegend() + "\n" + m.timeChart(chartBoxW-4, chartH)
		}
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			m.boxed("overview", m.overviewBody(), overviewW, chartH+4, false, ui.Accent),
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

func (m StatsModel) headerLine() string {
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

func (m StatsModel) filterLine() string {
	if len(m.filters) == 0 {
		return ""
	}
	chips := make([]string, len(m.filters))
	for i, f := range m.filters {
		chips[i] = ui.Title.Render(f.dim) + ui.Dim.Render("=") + f.value
	}
	return ui.Dim.Render("  filtered: ") + strings.Join(chips, ui.Dim.Render(" · "))
}

func (m StatsModel) overviewBody() string {
	s := m.res.Summary
	labelW := min(20, max(13, m.overviewWidth()-20))
	row := func(label, value string, style lipgloss.Style) string {
		return ui.Dim.Render(kit.PadToWidth(label, labelW)) + style.Render(value)
	}
	plain := lipgloss.NewStyle()

	clicksRow := row("clicks", fmt.Sprintf("%d", s.TotalClicks), ui.OK)
	if delta := m.deltaBadge(); delta != "" {
		clicksRow += "  " + delta
	}
	rows := []string{
		clicksRow,
		row("unique", fmt.Sprintf("%d", s.UniqueClicks), plain),
		row("avg redirect", fmt.Sprintf("%.0fms", s.AvgRedirectionTime), plain),
	}
	if rate, ok := m.res.ComputedMetrics["unique_click_rate"]; ok {
		rows = append(rows, row("unique rate", fmt.Sprintf("%.0f%%", rate), plain))
	}
	if rate, ok := m.res.ComputedMetrics["repeat_click_rate"]; ok {
		rows = append(rows, row("repeat rate", fmt.Sprintf("%.0f%%", rate), plain))
	}
	if cpv, ok := m.res.ComputedMetrics["average_clicks_per_visitor"]; ok {
		rows = append(rows, row("per visitor", fmt.Sprintf("%.1f", cpv), plain))
	}
	if best, ok := m.bestDay(); ok {
		rows = append(rows, row("best day", best, plain))
	}
	if active, ok := m.activeDays(); ok {
		rows = append(rows, row("active days", active, plain))
	}
	if s.FirstClick != "" {
		rows = append(rows,
			row("first click", kit.ISODate(s.FirstClick), plain),
			row("last click", kit.ISODate(s.LastClick), plain))
	}
	return strings.Join(rows, "\n")
}

// deltaBadge compares this window's clicks to the previous window.
func (m StatsModel) deltaBadge() string {
	if m.prev == nil {
		return ""
	}
	cur := float64(m.res.Summary.TotalClicks)
	old := float64(m.prev.Summary.TotalClicks)
	switch {
	case old == 0 && cur == 0:
		return ""
	case old == 0:
		return ui.OK.Render("new")
	}
	pct := (cur - old) / old * 100
	badge := fmt.Sprintf("▲ %+.0f%%", pct)
	style := ui.OK
	if pct < 0 {
		badge = fmt.Sprintf("▼ %.0f%%", pct)
		style = ui.Err
	}
	return style.Render(badge)
}

func (m StatsModel) bestDay() (string, bool) {
	var best api.MetricPoint
	for _, p := range m.res.Points("time", m.metric) {
		if p.Value > best.Value {
			best = p
		}
	}
	if best.Value == 0 {
		return "", false
	}
	day := best.Label
	if ts, ok := kit.ParseBucketTime(best.Label); ok {
		day = ts.Format("Jan 02")
	}
	return fmt.Sprintf("%s · %.0f", day, best.Value), true
}

func (m StatsModel) activeDays() (string, bool) {
	days := map[string]bool{}
	for _, p := range m.res.Points("time", m.metric) {
		if p.Value <= 0 {
			continue
		}
		if ts, ok := kit.ParseBucketTime(p.Label); ok {
			days[ts.Format("2006-01-02")] = true
		}
	}
	if len(days) == 0 {
		return "", false
	}
	spanDays := max(1, int(m.win.span/(24*time.Hour)))
	return fmt.Sprintf("%d of %d", len(days), spanDays), true
}

func (m StatsModel) chartTitle() string {
	return "traffic over time · " + m.win.label
}

// chartLegend names the time chart's series, including the previous-
// period ghost while it's shown.
func (m StatsModel) chartLegend() string {
	legend := chartClicks.Render("─── clicks") + "  " + chartUnique.Render("─── unique")
	if m.showPrev {
		legend += "  " + ui.Dim.Render("─── previous "+m.win.label)
	}
	return legend
}

// timeChart renders clicks and unique clicks as braille lines.
func (m StatsModel) timeChart(width, height int) string {
	clicks := m.res.Points("time", "clicks")
	uniques := m.res.Points("time", "unique_clicks")
	if len(clicks) == 0 {
		return ui.Dim.Render("no time series data")
	}

	toSeries := func(pts []api.MetricPoint) ([]tslc.TimePoint, float64) {
		out := make([]tslc.TimePoint, 0, len(pts))
		var maxV float64
		for _, p := range pts {
			if ts, ok := kit.ParseBucketTime(p.Label); ok {
				out = append(out, tslc.TimePoint{Time: ts, Value: p.Value})
				maxV = max(maxV, p.Value)
			}
		}
		return out, maxV
	}
	clickSeries, maxV := toSeries(clicks)
	uniqueSeries, _ := toSeries(uniques)
	if len(clickSeries) == 0 {
		return ui.Dim.Render("no time series data")
	}
	if maxV == 0 {
		return ui.Dim.Render("no activity in this window")
	}

	// the previous window's series, shifted forward one span so both
	// periods share the x-axis — the ghost behind the current line
	var prevSeries []tslc.TimePoint
	if m.showPrev && m.prev != nil {
		var prevMax float64
		prevSeries, prevMax = toSeries(m.prev.Points("time", m.metric))
		for i := range prevSeries {
			prevSeries[i].Time = prevSeries[i].Time.Add(m.win.span)
		}
		maxV = max(maxV, prevMax) // a taller last period must not clip
	}

	// pad Y labels to the top value's width: ntcharts sizes the label
	// gutter by sampling step labels and would clip a wider top label
	yMax := kit.NiceCeil(maxV)
	yWidth := len(fmt.Sprintf("%.0f", yMax))
	chart := tslc.New(max(40, width-2), max(6, height),
		tslc.WithTimeSeries(clickSeries),
		tslc.WithYRange(0, yMax),
		tslc.WithXYSteps(10, 4),
		tslc.WithYLabelFormatter(func(_ int, v float64) string {
			return fmt.Sprintf("%*.0f", yWidth, v)
		}),
		tslc.WithAxesStyles(ui.Dim, ui.Dim),
		tslc.WithStyle(chartClicks),
	)
	if len(prevSeries) > 0 {
		for _, tp := range prevSeries {
			chart.PushDataSet("previous", tp)
		}
		chart.SetDataSetStyle("previous", ui.Dim)
	}
	if len(uniqueSeries) > 0 {
		for _, tp := range uniqueSeries {
			chart.PushDataSet("unique", tp)
		}
		chart.SetDataSetStyle("unique", chartUnique)
	}
	chart.DrawBrailleAll()
	return chart.View()
}

// ── focus mode ────────────────────────────────────────────────────────

// focusView fills the screen with one chart and lists the rest in a
// sidebar; j/k walks the sidebar and the main area follows.
func (m StatsModel) focusView() string {
	mainW := m.width - sidebarW - 1
	mainH := m.height - 4
	if m.status != "" {
		mainH--
	}

	mainFocused := m.focusPane == 0
	var main string
	if m.focusItem == 0 {
		if m.tableOn["time"] {
			main = m.boxed(m.chartTitle()+" · table", m.timeTableBody(mainW-4, mainH-3), mainW, mainH, mainFocused, m.metricHue())
		} else {
			main = m.boxed(m.chartTitle(), m.chartLegend()+"\n"+m.timeChart(mainW-4, mainH-4), mainW, mainH, mainFocused, m.metricHue())
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
func (m StatsModel) sidebar(height int) string {
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
func (m StatsModel) sidebarPreview(item, width, n int) []string {
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
