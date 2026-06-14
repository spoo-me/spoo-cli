package stats

import (
	"fmt"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// overviewCard is the dashboard's summary panel: a stateless projection
// of a StatsResponse (and the previous window, for the delta badge). It
// declares exactly what it reads instead of reaching into the model.
type overviewCard struct {
	res, prev *api.StatsResponse
	metric    string
	span      time.Duration
	labelW    int
}

func (c overviewCard) render() string {
	s := c.res.Summary
	row := func(label, value string, style lipgloss.Style) string {
		return ui.Dim.Render(kit.PadToWidth(label, c.labelW)) + style.Render(value)
	}
	plain := lipgloss.NewStyle()

	clicksRow := row("clicks", fmt.Sprintf("%d", s.TotalClicks), ui.OK)
	if delta := c.deltaBadge(); delta != "" {
		clicksRow += "  " + delta
	}
	rows := []string{
		clicksRow,
		row("unique", fmt.Sprintf("%d", s.UniqueClicks), plain),
		row("avg redirect", fmt.Sprintf("%.0fms", s.AvgRedirectionTime), plain),
	}
	if rate, ok := c.res.ComputedMetrics["unique_click_rate"]; ok {
		rows = append(rows, row("unique rate", fmt.Sprintf("%.0f%%", rate), plain))
	}
	if rate, ok := c.res.ComputedMetrics["repeat_click_rate"]; ok {
		rows = append(rows, row("repeat rate", fmt.Sprintf("%.0f%%", rate), plain))
	}
	if cpv, ok := c.res.ComputedMetrics["average_clicks_per_visitor"]; ok {
		rows = append(rows, row("per visitor", fmt.Sprintf("%.1f", cpv), plain))
	}
	if best, ok := c.bestDay(); ok {
		rows = append(rows, row("best day", best, plain))
	}
	if active, ok := c.activeDays(); ok {
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
func (c overviewCard) deltaBadge() string {
	if c.prev == nil {
		return ""
	}
	cur := float64(c.res.Summary.TotalClicks)
	old := float64(c.prev.Summary.TotalClicks)
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

func (c overviewCard) bestDay() (string, bool) {
	var best api.MetricPoint
	for _, p := range c.res.Points("time", c.metric) {
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

func (c overviewCard) activeDays() (string, bool) {
	days := map[string]bool{}
	for _, p := range c.res.Points("time", c.metric) {
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
	spanDays := max(1, int(c.span/(24*time.Hour)))
	return fmt.Sprintf("%d of %d", len(days), spanDays), true
}
