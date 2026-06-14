package stats

import (
	"context"
	"image/color"
	"os"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// window resolves the current view to concrete bounds: the configured
// window, paged back offset times by its own span.
func (m Model) window() (start, end time.Time) {
	end = time.Now().UTC()
	if !m.win.anchored() {
		end = m.win.end
	}
	end = end.Add(-time.Duration(m.offset) * m.win.span)
	return end.Add(-m.win.span), end
}

// query builds the stats request for the current dashboard state.
func (m Model) query() api.StatsQuery {
	start, end := m.window()
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
	if m.offset > 0 || !m.win.anchored() {
		q.EndDate = end.Format(time.RFC3339)
	}
	for _, f := range m.filters {
		q.Filters[f.dim] = append(q.Filters[f.dim], f.value)
	}
	return q
}

// fetch loads the current window and, for the overview deltas, the
// window before it (summary only) — one command, one message.
func (m Model) fetch() tea.Cmd {
	client := m.client
	q := m.query()
	prevQ := q
	prevQ.GroupBy = []string{"time"}
	start, _ := m.window()
	prevQ.StartDate = start.Add(-m.win.span).Format(time.RFC3339)
	prevQ.EndDate = start.Format(time.RFC3339)

	return func() tea.Msg {
		res, err := client.Stats(context.Background(), q)
		var prev *api.StatsResponse
		if err == nil {
			prev, _ = client.Stats(context.Background(), prevQ) // best-effort
		}
		return statsLoadedMsg{res: res, prev: prev, err: err}
	}
}

// openExport pops the export dialog with a dated default filename.
func (m Model) openExport() (tea.Model, tea.Cmd) {
	subject := "stats-all"
	if m.target != "" {
		subject = "stats-" + m.target
	}
	var cmd tea.Cmd
	m.exportBox, cmd = m.exportBox.show(defaultExportName(subject, time.Now().Format("2006-01-02")))
	return m, cmd
}

// export downloads the current view in the requested format and
// writes it where the dialog pointed.
func (m Model) export(req exportRequest) tea.Cmd {
	client := m.client
	q := m.query()
	return func() tea.Msg {
		_, data, err := client.Export(context.Background(), q, req.format)
		if err == nil {
			err = os.WriteFile(req.path, data, 0o644)
		}
		return exportDoneMsg{name: collapseHome(req.path), err: err}
	}
}

func autoTick() tea.Cmd {
	return tea.Tick(autoEvery, func(time.Time) tea.Msg { return autoTickMsg{} })
}

// panelPoints returns a panel's rows for the active metric, capped to
// n. Used by both rendering and drill-down so selection always matches.
func (m Model) panelPoints(idx, n int) []api.MetricPoint {
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
func (m Model) weekdayPoints() []api.MetricPoint {
	var totals [7]float64
	for _, p := range m.res.Points("time", m.metric) {
		if ts, ok := kit.ParseBucketTime(p.Label); ok {
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

func (m Model) hasFilter(f filterEntry) bool {
	for _, e := range m.filters {
		if e == f {
			return true
		}
	}
	return false
}

func otherMetricKey(current string) string {
	if current == "clicks" {
		return "unique_clicks"
	}
	return "clicks"
}

// metricTotal is the denominator for percentage columns.
func (m Model) metricTotal() float64 {
	if m.metric == "unique_clicks" {
		return float64(m.res.Summary.UniqueClicks)
	}
	return float64(m.res.Summary.TotalClicks)
}

// metricHue is the pastel identity of the active metric; the time
// panel's focus shades follow it (clicks sky, unique pink).
func (m Model) metricHue() color.Color {
	if m.metric == "unique_clicks" {
		return ui.Pink
	}
	return ui.Blue
}
