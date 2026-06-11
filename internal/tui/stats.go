package tui

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	tslc "github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

const (
	panelTopN    = 6
	focusTopN    = 24 // rows shown for a panel promoted to focus mode
	twoColMin    = 96
	threeColMin  = 140
	defaultRange = 90
	sidebarW     = 30 // focus-mode sidebar width
	autoEvery    = 30 * time.Second
)

// rangeCycle is the windows the 't' key steps through (days).
var rangeCycle = []int{90, 30, 7, 1}

type panelDef struct{ key, title string }

// dashBarStyle is the bar style used across the dashboard (picked from
// a live A/B of six candidates).
const dashBarStyle = ui.BarUpperHalf

// panelColors gives every panel its own pastel hue (entity brand
// colors in colors.go override per row where known).
var panelColors = map[string]color.Color{
	"short_code": ui.Accent,
	"browser":    ui.Success,
	"os":         ui.Blue,
	"country":    ui.Yellow,
	"city":       ui.Pink,
	"referrer":   ui.Teal,
	"weekday":    ui.Accent,
}

type statsLoadedMsg struct {
	res  *api.StatsResponse
	prev *api.StatsResponse // previous window, for period-over-period deltas
	err  error
}

type exportDoneMsg struct {
	name string
	err  error
}

type autoTickMsg struct{}

type filterEntry struct {
	dim   string
	value string
}

// StatsModel is the full-screen analytics dashboard: overview with
// period deltas, a dual-series time chart, focusable breakdown panels
// with server-side drill-down, window paging, and a focus mode.
type StatsModel struct {
	client *api.Client
	target string // short code, or "" for account-wide
	scope  string // all | anon
	tz     string

	rangeDays int
	offset    int // how many windows back in time ('[' / ']')
	metric    string
	filters   []filterEntry
	auto      bool

	res      *api.StatsResponse
	prev     *api.StatsResponse
	fetchErr error
	loading  bool
	status   string
	focus    int         // index into panels()
	sel      map[int]int // per-panel selection row

	focusMode bool
	focusItem int // 0 = time chart, 1.. = panels()[focusItem-1]

	width  int
	height int
}

func NewStats(client *api.Client, target, scope, tz string) StatsModel {
	return StatsModel{
		client:    client,
		target:    target,
		scope:     scope,
		tz:        tz,
		rangeDays: defaultRange,
		metric:    "clicks",
		sel:       map[int]int{},
		loading:   true,
		width:     100,
		height:    40,
	}
}

// panels returns the breakdown panels for the current view. Account-
// wide gets the drillable top-links leaderboard first; a single link
// gets the weekday distribution instead.
func (m StatsModel) panels() []panelDef {
	if m.target == "" {
		return []panelDef{
			{"short_code", "top links"},
			{"browser", "browsers"},
			{"os", "operating systems"},
			{"country", "countries"},
			{"city", "cities"},
			{"referrer", "referrers"},
		}
	}
	return []panelDef{
		{"browser", "browsers"},
		{"os", "operating systems"},
		{"country", "countries"},
		{"city", "cities"},
		{"referrer", "referrers"},
		{"weekday", "weekdays"},
	}
}

func (m StatsModel) Init() tea.Cmd { return m.fetch() }

// query builds the stats request for the current dashboard state.
func (m StatsModel) query() api.StatsQuery {
	now := time.Now().UTC()
	end := now.AddDate(0, 0, -m.offset*m.rangeDays)
	start := end.AddDate(0, 0, -m.rangeDays)
	groupBy := []string{"time", "browser", "os", "country", "city", "referrer"}
	if m.target == "" {
		groupBy = append(groupBy, "short_code")
	}
	q := api.StatsQuery{
		Scope:     m.scope,
		ShortCode: m.target,
		StartDate: start.Format(time.RFC3339),
		Timezone:  m.tz,
		GroupBy:   groupBy,
		Filters:   map[string][]string{},
	}
	if m.offset > 0 {
		q.EndDate = end.Format(time.RFC3339)
	}
	for _, f := range m.filters {
		q.Filters[f.dim] = append(q.Filters[f.dim], f.value)
	}
	return q
}

// fetch loads the current window and, for the overview deltas, the
// window before it (summary only) — one command, one message.
func (m StatsModel) fetch() tea.Cmd {
	client := m.client
	q := m.query()
	prevQ := q
	prevQ.GroupBy = []string{"time"}
	now := time.Now().UTC()
	prevQ.StartDate = now.AddDate(0, 0, -(m.offset+2)*m.rangeDays).Format(time.RFC3339)
	prevQ.EndDate = now.AddDate(0, 0, -(m.offset+1)*m.rangeDays).Format(time.RFC3339)

	return func() tea.Msg {
		res, err := client.Stats(context.Background(), q)
		var prev *api.StatsResponse
		if err == nil {
			prev, _ = client.Stats(context.Background(), prevQ) // best-effort
		}
		return statsLoadedMsg{res: res, prev: prev, err: err}
	}
}

func (m StatsModel) export() tea.Cmd {
	client := m.client
	q := m.query()
	return func() tea.Msg {
		name, data, err := client.Export(context.Background(), q, "xlsx")
		if err == nil {
			err = os.WriteFile(name, data, 0o644)
		}
		return exportDoneMsg{name: name, err: err}
	}
}

func autoTick() tea.Cmd {
	return tea.Tick(autoEvery, func(time.Time) tea.Msg { return autoTickMsg{} })
}

// panelPoints returns a panel's rows for the active metric, capped to
// n. Used by both rendering and drill-down so selection always matches.
func (m StatsModel) panelPoints(idx, n int) []api.MetricPoint {
	if m.res == nil {
		return nil
	}
	key := m.panels()[idx].key
	if key == "weekday" {
		return m.weekdayPoints()
	}
	pts := m.res.Points(key, m.metric)
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Value > pts[j].Value })
	if len(pts) > n {
		pts = pts[:n]
	}
	return pts
}

// weekdayPoints folds the time series into a Mon→Sun distribution.
func (m StatsModel) weekdayPoints() []api.MetricPoint {
	var totals [7]float64
	for _, p := range m.res.Points("time", m.metric) {
		if ts, ok := parseBucketTime(p.Label); ok {
			totals[int(ts.Weekday())] += p.Value
		}
	}
	names := [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	out := make([]api.MetricPoint, 0, 7)
	for i := 1; i <= 7; i++ { // Monday first
		idx := i % 7
		out = append(out, api.MetricPoint{Label: names[idx], Value: totals[idx]})
	}
	return out
}

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
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

	case tea.KeyPressMsg:
		m.status = ""
		if m.focusMode {
			return m.updateFocusMode(msg)
		}
		return m.updateDashboard(msg)
	}
	return m, nil
}

// updateFocusMode handles keys while a single chart fills the screen.
func (m StatsModel) updateFocusMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	items := len(m.panels()) + 1 // + the time chart
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "x", "esc", "f":
		m.focusMode = false
		return m, nil
	case "j", "down", "tab":
		m.focusItem = (m.focusItem + 1) % items
	case "k", "up", "shift+tab":
		m.focusItem = (m.focusItem + items - 1) % items
	case "u":
		m.metric = otherMetricKey(m.metric)
	case "t":
		m.rangeDays = nextRange(m.rangeDays)
		m.offset = 0
		m.loading = true
		return m, m.fetch()
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
		m.status = ui.Dim.Render("exporting…")
		return m, m.export()
	case "r":
		m.loading = true
		return m, m.fetch()
	}
	return m, nil
}

// updateDashboard handles keys in the regular grid view.
func (m StatsModel) updateDashboard(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	panels := m.panels()
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
		m.focusItem = m.focus + 1 // promote the focused panel
		return m, nil
	case "tab", "right", "l":
		m.focus = (m.focus + 1) % len(panels)
	case "shift+tab", "left", "h":
		m.focus = (m.focus + len(panels) - 1) % len(panels)
	case "down", "j":
		if n := len(m.panelPoints(m.focus, panelTopN)); n > 0 {
			m.sel[m.focus] = min(m.sel[m.focus]+1, n-1)
		}
	case "up", "k":
		m.sel[m.focus] = max(m.sel[m.focus]-1, 0)
	case "enter":
		dim := panels[m.focus].key
		if dim == "weekday" {
			m.status = ui.Dim.Render("weekdays are computed locally — nothing to drill into")
			break
		}
		pts := m.panelPoints(m.focus, panelTopN)
		i := min(m.sel[m.focus], len(pts)-1)
		if i < 0 {
			break
		}
		f := filterEntry{dim: dim, value: pts[i].Label}
		if m.hasFilter(f) {
			break
		}
		m.filters = append(m.filters, f)
		m.sel = map[int]int{}
		m.loading = true
		return m, m.fetch()
	case "u":
		m.metric = otherMetricKey(m.metric)
	case "t":
		m.rangeDays = nextRange(m.rangeDays)
		m.offset = 0
		m.loading = true
		return m, m.fetch()
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
		m.status = ui.Dim.Render("exporting…")
		return m, m.export()
	case "a":
		m.auto = !m.auto
		if m.auto {
			return m, autoTick()
		}
	case "r":
		m.loading = true
		return m, m.fetch()
	}
	return m, nil
}

func (m StatsModel) hasFilter(f filterEntry) bool {
	for _, e := range m.filters {
		if e == f {
			return true
		}
	}
	return false
}

func nextRange(current int) int {
	for i, d := range rangeCycle {
		if d == current {
			return rangeCycle[(i+1)%len(rangeCycle)]
		}
	}
	return rangeCycle[0]
}

func rangeLabel(days int) string {
	if days == 1 {
		return "24h"
	}
	return fmt.Sprintf("%dd", days)
}

func otherMetricKey(current string) string {
	if current == "clicks" {
		return "unique_clicks"
	}
	return "clicks"
}

// metricTotal is the denominator for percentage columns.
func (m StatsModel) metricTotal() float64 {
	if m.metric == "unique_clicks" {
		return float64(m.res.Summary.UniqueClicks)
	}
	return float64(m.res.Summary.TotalClicks)
}

// ── layout ────────────────────────────────────────────────────────────

// overviewWidth scales the overview panel with the terminal (~25% of
// the width) instead of pinching it on wide screens.
func (m StatsModel) overviewWidth() int {
	return min(56, max(32, m.width/4))
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
	if len(m.filters) > 0 {
		used++
	}
	rows := m.uniformRows()
	used += len(m.panelChunks()) * (rows + 3)
	return min(20, max(7, m.height-used-1))
}

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
		legend := ui.OK.Render("─── clicks") + "  " + ui.Title.Render("─── unique")
		chartBody := legend + "\n" + m.timeChart(chartBoxW-4, chartH)
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			m.boxed("overview", m.overviewBody(), overviewW, chartH+4, false),
			" ",
			m.boxed(m.chartTitle(), chartBody, chartBoxW, chartH+4, false),
		)
		b.WriteString(top + "\n")
		b.WriteString(m.panelGrid() + "\n")
	}

	if m.status != "" {
		b.WriteString(m.status + "\n")
	}
	hint := "↑/↓ ←/→ navigate · enter drill down · f focus · x undo · u " + otherMetricLabel(m.metric) +
		" · t range · [/] older/newer · e export · a auto · r refresh · q quit"
	if m.focusMode {
		hint = "j/k switch chart · x close · u " + otherMetricLabel(m.metric) +
			" · t range · [/] older/newer · e export · r refresh · q quit"
	}
	b.WriteString(ui.KeyHint.Render(hint))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func otherMetricLabel(current string) string {
	if current == "clicks" {
		return "unique"
	}
	return "clicks"
}

func (m StatsModel) headerLine() string {
	target := "all links"
	if m.target != "" {
		target = m.target
	}
	h := ui.Title.Render("✦ spoo stats") + ui.Dim.Render("  ·  ") + target
	if m.res != nil && m.res.TimeRange.StartDate != "" {
		h += ui.Dim.Render("  ·  " + isoDate(m.res.TimeRange.StartDate) + " → " + isoDate(m.res.TimeRange.EndDate))
	} else {
		h += ui.Dim.Render("  ·  last " + rangeLabel(m.rangeDays))
	}
	if m.offset > 0 {
		past := lipgloss.NewStyle().Foreground(ui.Yellow)
		h += ui.Dim.Render("  ·  ") + past.Render(fmt.Sprintf("≪ %d window%s back", m.offset, plural(m.offset)))
	}
	h += ui.Dim.Render("  ·  metric: ") + ui.OK.Render(strings.ReplaceAll(m.metric, "_", " "))
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

// boxed wraps content in the dashboard's standard bordered panel.
// width/height are border-box totals (lipgloss v2 semantics).
func (m StatsModel) boxed(title, body string, width, height int, focused bool) string {
	borderColor, titleStyle := ui.Muted, ui.Dim.Bold(true)
	if focused {
		borderColor, titleStyle = ui.Accent, ui.Title
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(height).
		Render(titleStyle.Render("✦ "+title) + "\n" + body)
}

func (m StatsModel) overviewBody() string {
	s := m.res.Summary
	labelW := min(20, max(13, m.overviewWidth()-20))
	row := func(label, value string, style lipgloss.Style) string {
		return ui.Dim.Render(padToWidth(label, labelW)) + style.Render(value)
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
			row("first click", isoDate(s.FirstClick), plain),
			row("last click", isoDate(s.LastClick), plain))
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
	if ts, ok := parseBucketTime(best.Label); ok {
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
		if ts, ok := parseBucketTime(p.Label); ok {
			days[ts.Format("2006-01-02")] = true
		}
	}
	if len(days) == 0 {
		return "", false
	}
	return fmt.Sprintf("%d of %d", len(days), max(1, m.rangeDays)), true
}

func (m StatsModel) chartTitle() string {
	return "traffic over time · " + rangeLabel(m.rangeDays)
}

// niceCeil rounds up to a 1/2/2.5/5×10ⁿ boundary so axis steps are even.
func niceCeil(v float64) float64 {
	if v <= 5 {
		return 5
	}
	mag := math.Pow(10, math.Floor(math.Log10(v)))
	for _, mult := range []float64{1, 2, 2.5, 5, 10} {
		if v <= mult*mag {
			return mult * mag
		}
	}
	return 10 * mag
}

// bucketTimeLayouts are the formats the backend uses for time-bucket
// labels across its hourly/daily/weekly/monthly strategies.
var bucketTimeLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02 15",
	"2006-01-02",
	"2006-01",
}

func parseBucketTime(label string) (time.Time, bool) {
	for _, layout := range bucketTimeLayouts {
		if ts, err := time.Parse(layout, label); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
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
			if ts, ok := parseBucketTime(p.Label); ok {
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

	// pad Y labels to the top value's width: ntcharts sizes the label
	// gutter by sampling step labels and would clip a wider top label
	yMax := niceCeil(maxV)
	yWidth := len(fmt.Sprintf("%.0f", yMax))
	chart := tslc.New(max(40, width-2), max(6, height),
		tslc.WithTimeSeries(clickSeries),
		tslc.WithYRange(0, yMax),
		tslc.WithXYSteps(10, 4),
		tslc.WithYLabelFormatter(func(_ int, v float64) string {
			return fmt.Sprintf("%*.0f", yWidth, v)
		}),
		tslc.WithAxesStyles(ui.Dim, ui.Dim),
		tslc.WithStyle(ui.OK),
	)
	if len(uniqueSeries) > 0 {
		for _, tp := range uniqueSeries {
			chart.PushDataSet("unique", tp)
		}
		chart.SetDataSetStyle("unique", ui.Title)
	}
	chart.DrawBrailleAll()
	return chart.View()
}

// panelGrid lays the breakdown panels out in responsive columns with a
// one-column gutter; every panel shares the same height.
func (m StatsModel) panelGrid() string {
	cols := m.gridCols()
	panelW := (m.width - (cols - 1)) / cols
	contentRows := m.uniformRows()

	var rows []string
	for _, chunk := range m.panelChunks() {
		row := make([]string, 0, cols*2)
		for _, i := range chunk {
			if len(row) > 0 {
				row = append(row, " ")
			}
			row = append(row, m.panelView(i, panelW, contentRows, panelTopN))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}

func (m StatsModel) panelView(idx, width, contentRows, topN int) string {
	p := m.panels()[idx]
	focused := !m.focusMode && idx == m.focus
	innerW := width - 4 // border-box width minus borders and padding

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
		label := padToWidth(truncateToWidth(m.rowLabel(p.key, pt.Label), labelW), labelW)

		count := fmt.Sprintf("%5.0f", pt.Value)
		pct := "     "
		if total > 0 {
			pct = fmt.Sprintf("%4.0f%%", pt.Value/total*100)
		}

		marker, labelStyle := "  ", lipgloss.NewStyle()
		if focused && i == m.sel[m.focus] {
			marker, labelStyle = ui.Title.Render("▸ "), ui.Title
		}
		lines = append(lines, marker+labelStyle.Render(label)+" "+
			ui.Bar(dashBarStyle, pt.Value, maxV, barMax, entityColor(pt.Label, panelHue))+
			count+ui.Dim.Render(pct))
	}
	return m.boxed(p.title, strings.Join(lines, "\n"), width, contentRows+3, focused)
}

// rowLabel normalizes a point label for display.
func (m StatsModel) rowLabel(panelKey, label string) string {
	if panelKey == "country" {
		return ui.CountryLabel(label)
	}
	return label
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

	var main string
	if m.focusItem == 0 {
		legend := ui.OK.Render("─── clicks") + "  " + ui.Title.Render("─── unique")
		main = m.boxed(m.chartTitle(), legend+"\n"+m.timeChart(mainW-4, mainH-4), mainW, mainH, true)
	} else {
		idx := m.focusItem - 1
		main = m.boxed(m.panels()[idx].title, m.focusPanelBody(idx, mainW), mainW, mainH, true)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, main, " ", m.sidebar(mainH))
}

// focusPanelBody renders a panel's rows at full size for focus mode.
func (m StatsModel) focusPanelBody(idx, width int) string {
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
	barMax := max(10, innerW-labelW-5-5-1)

	lines := make([]string, 0, len(pts))
	for _, pt := range pts {
		label := padToWidth(truncateToWidth(m.rowLabel(p.key, pt.Label), labelW), labelW)
		count := fmt.Sprintf("%5.0f", pt.Value)
		pct := "     "
		if total > 0 {
			pct = fmt.Sprintf("%4.0f%%", pt.Value/total*100)
		}
		lines = append(lines, label+" "+
			ui.Bar(dashBarStyle, pt.Value, maxV, barMax, entityColor(pt.Label, panelHue))+
			count+ui.Dim.Render(pct))
	}
	return strings.Join(lines, "\n")
}

// sidebar lists all focusable charts with the active one highlighted.
func (m StatsModel) sidebar(height int) string {
	items := []string{"traffic over time"}
	for _, p := range m.panels() {
		items = append(items, p.title)
	}
	lines := make([]string, 0, len(items))
	for i, item := range items {
		if i == m.focusItem {
			lines = append(lines, ui.Title.Render("▸ "+item))
		} else {
			lines = append(lines, ui.Dim.Render("  "+item))
		}
	}
	return m.boxed("charts", strings.Join(lines, "\n"), sidebarW, height, false)
}

// padToWidth right-pads by display width (emoji-safe, unlike %-*s).
func padToWidth(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// truncateToWidth trims a string to at most w display columns.
func truncateToWidth(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// FetchErr reports a fetch error so the command can surface it on exit.
func (m StatsModel) FetchErr() error { return m.fetchErr }
