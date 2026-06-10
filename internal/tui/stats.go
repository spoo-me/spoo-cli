package tui

import (
	"context"
	"fmt"
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
	chartHeight   = 6
	panelTopN     = 6
	twoColMin     = 96
	threeColMin   = 150
	defaultRange  = 90
	panelChromeW  = 6 // border + padding per panel box
	dashFooterTop = 3 // header + filter + summary lines
)

// rangeCycle is the windows the 't' key steps through (days).
var rangeCycle = []int{90, 30, 7}

// dashPanels are the breakdown panels in display and focus order.
var dashPanels = []struct{ key, title string }{
	{"browser", "Browsers"},
	{"os", "Operating systems"},
	{"country", "Countries"},
	{"city", "Cities"},
	{"referrer", "Referrers"},
}

type statsLoadedMsg struct {
	res *api.StatsResponse
	err error
}

type filterEntry struct {
	dim   string
	value string
}

// StatsModel is the full-screen analytics dashboard: summary, a time
// chart, and focusable breakdown panels with server-side drill-down.
type StatsModel struct {
	client *api.Client
	target string // short code, or "" for account-wide
	scope  string // all | anon
	tz     string

	rangeDays int
	metric    string // clicks | unique_clicks
	filters   []filterEntry

	res     *api.StatsResponse
	fetchErr error
	loading bool
	focus   int         // index into dashPanels
	sel     map[int]int // per-panel selection row

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
		b.WriteString(m.summaryLine() + "\n\n")
		b.WriteString(m.timeChart() + "\n")
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
	h := ui.Title.Render("spoo stats") + ui.Dim.Render("  "+target)
	if m.res != nil && m.res.TimeRange.StartDate != "" {
		h += ui.Dim.Render("  " + isoDate(m.res.TimeRange.StartDate) + " → " + isoDate(m.res.TimeRange.EndDate))
	} else {
		h += ui.Dim.Render(fmt.Sprintf("  last %dd", m.rangeDays))
	}
	h += ui.Dim.Render("  metric: " + strings.ReplaceAll(m.metric, "_", " "))
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
	return ui.Dim.Render("filtered: ") + strings.Join(chips, ui.Dim.Render(" · "))
}

func (m StatsModel) summaryLine() string {
	s := m.res.Summary
	parts := []string{
		ui.OK.Render(fmt.Sprintf("%d clicks", s.TotalClicks)),
		fmt.Sprintf("%d unique", s.UniqueClicks),
		fmt.Sprintf("avg redirect %.0fms", s.AvgRedirectionTime),
	}
	if rate, ok := m.res.ComputedMetrics["unique_click_rate"]; ok {
		parts = append(parts, fmt.Sprintf("unique rate %.0f%%", rate))
	}
	if cpv, ok := m.res.ComputedMetrics["average_clicks_per_visitor"]; ok {
		parts = append(parts, fmt.Sprintf("%.1f clicks/visitor", cpv))
	}
	return strings.Join(parts, ui.Dim.Render("  ·  "))
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

// timeChart renders the time series as a braille line chart with real
// axes (ntcharts), sized to the full dashboard width.
func (m StatsModel) timeChart() string {
	pts := m.res.Points("time", m.metric)
	if len(pts) == 0 {
		return ui.Dim.Render("no time series data") + "\n"
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
		return ui.Dim.Render("no time series data") + "\n"
	}
	if maxV == 0 {
		return ui.Dim.Render("no activity in this window") + "\n"
	}

	chart := tslc.New(max(40, m.width-2), chartHeight+3,
		tslc.WithTimeSeries(tps),
		tslc.WithYRange(0, maxV),
		tslc.WithAxesStyles(ui.Dim, ui.Dim),
		tslc.WithStyle(ui.OK),
	)
	chart.DrawBraille()
	return chart.View() + "\n"
}

// panelGrid lays the breakdown panels out in responsive columns.
func (m StatsModel) panelGrid() string {
	cols := 1
	switch {
	case m.width >= threeColMin:
		cols = 3
	case m.width >= twoColMin:
		cols = 2
	}
	panelW := m.width/cols - 2

	boxes := make([]string, len(dashPanels))
	for i := range dashPanels {
		boxes[i] = m.panelView(i, panelW)
	}

	var rows []string
	for start := 0; start < len(boxes); start += cols {
		end := min(start+cols, len(boxes))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, boxes[start:end]...))
	}
	return strings.Join(rows, "\n")
}

func (m StatsModel) panelView(idx, width int) string {
	p := dashPanels[idx]
	focused := idx == m.focus

	border := ui.Dim.GetForeground()
	titleStyle := ui.Dim
	if focused {
		border = ui.Accent
		titleStyle = ui.Title
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(max(28, width) - panelChromeW + 4)

	pts := m.panelPoints(idx)
	innerW := max(24, width-panelChromeW)
	lines := []string{titleStyle.Render(p.title)}
	if len(pts) == 0 {
		lines = append(lines, ui.Dim.Render("no data"))
	}
	var total float64
	for _, pt := range pts {
		total = max(total, pt.Value)
	}
	for i, pt := range pts {
		label := pt.Label
		if p.key == "country" {
			label = ui.CountryLabel(label)
		}
		barW := max(1, int(pt.Value/total*float64(innerW-22)))
		line := fmt.Sprintf("%-14s %s %.0f",
			truncateTo(label, 14), strings.Repeat("█", barW), pt.Value)
		switch {
		case focused && i == m.sel[m.focus]:
			line = ui.Title.Render("▸ ") + ui.OK.Render(line)
		default:
			line = "  " + line
		}
		lines = append(lines, line)
	}
	return box.Render(strings.Join(lines, "\n"))
}

func truncateTo(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// FetchErr reports a fetch error so the command can surface it on exit.
func (m StatsModel) FetchErr() error { return m.fetchErr }
