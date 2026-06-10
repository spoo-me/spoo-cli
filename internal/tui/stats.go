package tui

import (
	"context"
	"fmt"
	"math"
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
	twoColMin    = 96
	threeColMin  = 150
	defaultRange = 90
	overviewW    = 30 // fixed width of the overview panel
)

// rangeCycle is the windows the 't' key steps through (days).
var rangeCycle = []int{90, 30, 7}

// dashPanels are the breakdown panels in display and focus order.
var dashPanels = []struct{ key, title string }{
	{"browser", "browsers"},
	{"os", "operating systems"},
	{"country", "countries"},
	{"city", "cities"},
	{"referrer", "referrers"},
}

type statsLoadedMsg struct {
	res *api.StatsResponse
	err error
}

type filterEntry struct {
	dim   string
	value string
}

// StatsModel is the full-screen analytics dashboard: overview, a time
// chart, and focusable breakdown panels with server-side drill-down.
type StatsModel struct {
	client *api.Client
	target string // short code, or "" for account-wide
	scope  string // all | anon
	tz     string

	rangeDays int
	metric    string // clicks | unique_clicks
	filters   []filterEntry

	res      *api.StatsResponse
	fetchErr error
	loading  bool
	focus    int         // index into dashPanels
	sel      map[int]int // per-panel selection row

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
		height:    36,
	}
}

func (m StatsModel) Init() tea.Cmd { return m.fetch() }

func (m StatsModel) fetch() tea.Cmd {
	client := m.client
	q := api.StatsQuery{
		Scope:     m.scope,
		ShortCode: m.target,
		StartDate: time.Now().UTC().AddDate(0, 0, -m.rangeDays).Format(time.RFC3339),
		Timezone:  m.tz,
		GroupBy:   []string{"time", "browser", "os", "country", "city", "referrer"},
		Filters:   map[string][]string{},
	}
	for _, f := range m.filters {
		q.Filters[f.dim] = append(q.Filters[f.dim], f.value)
	}
	return func() tea.Msg {
		res, err := client.Stats(context.Background(), q)
		return statsLoadedMsg{res: res, err: err}
	}
}

// panelPoints returns a panel's sorted top rows for the active metric.
// Used by both rendering and drill-down so selection always matches.
func (m StatsModel) panelPoints(idx int) []api.MetricPoint {
	if m.res == nil {
		return nil
	}
	pts := m.res.Points(dashPanels[idx].key, m.metric)
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Value > pts[j].Value })
	if len(pts) > panelTopN {
		pts = pts[:panelTopN]
	}
	return pts
}

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case statsLoadedMsg:
		m.loading = false
		m.res, m.fetchErr = msg.res, msg.err
		return m, nil

	case tea.KeyPressMsg:
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
		case "tab", "right", "l":
			m.focus = (m.focus + 1) % len(dashPanels)
		case "shift+tab", "left", "h":
			m.focus = (m.focus + len(dashPanels) - 1) % len(dashPanels)
		case "down", "j":
			if n := len(m.panelPoints(m.focus)); n > 0 {
				m.sel[m.focus] = min(m.sel[m.focus]+1, n-1)
			}
		case "up", "k":
			m.sel[m.focus] = max(m.sel[m.focus]-1, 0)
		case "enter":
			pts := m.panelPoints(m.focus)
			i := min(m.sel[m.focus], len(pts)-1)
			if i < 0 {
				break
			}
			dim := dashPanels[m.focus].key
			f := filterEntry{dim: dim, value: pts[i].Label}
			if m.hasFilter(f) {
				break
			}
			m.filters = append(m.filters, f)
			m.sel = map[int]int{}
			m.loading = true
			return m, m.fetch()
		case "u":
			if m.metric == "clicks" {
				m.metric = "unique_clicks"
			} else {
				m.metric = "clicks"
			}
		case "t":
			m.rangeDays = nextRange(m.rangeDays)
			m.loading = true
			return m, m.fetch()
		case "r":
			m.loading = true
			return m, m.fetch()
		}
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

// metricTotal is the denominator for percentage columns.
func (m StatsModel) metricTotal() float64 {
	if m.metric == "unique_clicks" {
		return float64(m.res.Summary.UniqueClicks)
	}
	return float64(m.res.Summary.TotalClicks)
}

// ── layout ────────────────────────────────────────────────────────────

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

// chartHeight sizes the time chart to absorb the height the panel grid
// and chrome don't need, so the dashboard fills the terminal.
func (m StatsModel) chartHeight() int {
	cols := m.gridCols()
	gridRows := (len(dashPanels) + cols - 1) / cols
	overhead := 2 /*header+filters*/ + 2 /*top boxes borders*/ +
		gridRows*(panelTopN+3) + 2 /*footer*/
	return min(18, max(8, m.height-overhead))
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
	default:
		chartH := m.chartHeight()
		chartBoxW := m.width - overviewW - 1
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			m.boxed("overview", m.overviewBody(), overviewW, chartH+3, false),
			" ",
			m.boxed(m.chartTitle(), m.timeChart(chartBoxW-4, chartH), chartBoxW, chartH+3, false),
		)
		b.WriteString(top + "\n")
		b.WriteString(m.panelGrid() + "\n")
	}

	hint := "↑/↓ select · ←/→ panel · enter drill down · x undo filter · u " + otherMetric(m.metric) +
		" · t range · r refresh · q quit"
	b.WriteString(ui.KeyHint.Render(hint))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func otherMetric(current string) string {
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
		h += ui.Dim.Render(fmt.Sprintf("  ·  last %dd", m.rangeDays))
	}
	h += ui.Dim.Render("  ·  metric: ") + ui.OK.Render(strings.ReplaceAll(m.metric, "_", " "))
	if m.loading {
		h += ui.Dim.Render("  ⟳")
	}
	return h
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
// width/height are border-box totals (lipgloss v2 semantics), so the
// content area is width-4 × height-3 (borders + padding + title row).
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
	row := func(label, value string, style lipgloss.Style) string {
		return ui.Dim.Render(padToWidth(label, 15)) + style.Render(value)
	}
	rows := []string{
		row("clicks", fmt.Sprintf("%d", s.TotalClicks), ui.OK),
		row("unique", fmt.Sprintf("%d", s.UniqueClicks), lipgloss.NewStyle()),
		row("avg redirect", fmt.Sprintf("%.0fms", s.AvgRedirectionTime), lipgloss.NewStyle()),
	}
	if rate, ok := m.res.ComputedMetrics["unique_click_rate"]; ok {
		rows = append(rows, row("unique rate", fmt.Sprintf("%.0f%%", rate), lipgloss.NewStyle()))
	}
	if rate, ok := m.res.ComputedMetrics["repeat_click_rate"]; ok {
		rows = append(rows, row("repeat rate", fmt.Sprintf("%.0f%%", rate), lipgloss.NewStyle()))
	}
	if cpv, ok := m.res.ComputedMetrics["average_clicks_per_visitor"]; ok {
		rows = append(rows, row("per visitor", fmt.Sprintf("%.1f", cpv), lipgloss.NewStyle()))
	}
	if s.FirstClick != "" {
		rows = append(rows,
			row("first click", isoDate(s.FirstClick), lipgloss.NewStyle()),
			row("last click", isoDate(s.LastClick), lipgloss.NewStyle()))
	}
	return strings.Join(rows, "\n")
}

func (m StatsModel) chartTitle() string {
	return strings.ReplaceAll(m.metric, "_", " ") + fmt.Sprintf(" over time · %dd", m.rangeDays)
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

// timeChart renders the time series as a braille line chart with axes.
func (m StatsModel) timeChart(width, height int) string {
	pts := m.res.Points("time", m.metric)
	if len(pts) == 0 {
		return ui.Dim.Render("no time series data")
	}
	tps := make([]tslc.TimePoint, 0, len(pts))
	var maxV float64
	for _, p := range pts {
		ts, ok := parseBucketTime(p.Label)
		if !ok {
			continue
		}
		tps = append(tps, tslc.TimePoint{Time: ts, Value: p.Value})
		maxV = max(maxV, p.Value)
	}
	if len(tps) == 0 {
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
		tslc.WithTimeSeries(tps),
		tslc.WithYRange(0, yMax),
		tslc.WithXYSteps(10, 4),
		tslc.WithYLabelFormatter(func(_ int, v float64) string {
			return fmt.Sprintf("%*.0f", yWidth, v)
		}),
		tslc.WithAxesStyles(ui.Dim, ui.Dim),
		tslc.WithStyle(ui.OK),
	)
	chart.DrawBraille()
	return chart.View()
}

// panelGrid lays the breakdown panels out in responsive columns with a
// one-column gutter between boxes.
func (m StatsModel) panelGrid() string {
	cols := m.gridCols()
	panelW := (m.width - (cols - 1)) / cols

	boxes := make([]string, len(dashPanels))
	for i := range dashPanels {
		boxes[i] = m.panelView(i, panelW)
	}

	var rows []string
	for start := 0; start < len(boxes); start += cols {
		end := min(start+cols, len(boxes))
		row := make([]string, 0, cols*2)
		for _, box := range boxes[start:end] {
			if len(row) > 0 {
				row = append(row, " ")
			}
			row = append(row, box)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}

func (m StatsModel) panelView(idx, width int) string {
	p := dashPanels[idx]
	focused := idx == m.focus
	innerW := width - 4 // border-box width minus borders and padding

	pts := m.panelPoints(idx)
	var maxV float64
	for _, pt := range pts {
		maxV = max(maxV, pt.Value)
	}
	total := m.metricTotal()

	// columns: marker(2) label(adaptive) bar(rest) count(5) pct(5)
	labelW := min(24, max(12, innerW*4/10))
	barMax := max(8, innerW-labelW-2-5-5)

	lines := make([]string, 0, panelTopN)
	if len(pts) == 0 {
		lines = append(lines, ui.Dim.Render("no data"))
	}
	for i, pt := range pts {
		label := pt.Label
		if p.key == "country" {
			label = ui.CountryLabel(label)
		}
		label = padToWidth(truncateToWidth(label, labelW), labelW)

		barW := max(1, int(math.Round(pt.Value/maxV*float64(barMax))))
		bar := strings.Repeat("█", barW) + strings.Repeat(" ", barMax-barW)

		count := fmt.Sprintf("%5.0f", pt.Value)
		pct := "     "
		if total > 0 {
			pct = fmt.Sprintf("%4.0f%%", pt.Value/total*100)
		}

		marker, labelStyle := "  ", lipgloss.NewStyle()
		if focused && i == m.sel[m.focus] {
			marker, labelStyle = ui.Title.Render("▸ "), ui.Title
		}
		lines = append(lines, marker+labelStyle.Render(label)+ui.OK.Render(bar)+
			count+ui.Dim.Render(pct))
	}
	return m.boxed(p.title, strings.Join(lines, "\n"), width, panelTopN+3, focused)
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
