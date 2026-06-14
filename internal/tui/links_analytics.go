package tui

import (
	"fmt"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// analyticsLines renders the per-link stats section of the detail pane
// from the debounced cache; before the fetch lands it shows a loader.
func (m LinksModel) analyticsLines(alias string, label func(string) string, width int) []string {
	e, ok := m.stats[alias]
	if !ok {
		return []string{ui.Dim.Render("loading…")}
	}
	if e.err != nil || e.res == nil {
		return []string{ui.Dim.Render("unavailable")}
	}
	res := e.res
	if res.Summary.TotalClicks == 0 {
		return []string{ui.Dim.Render(fmt.Sprintf("no clicks in the last %d days", api.MaxRangeDays))}
	}
	total := float64(res.Summary.TotalClicks)
	unique := fmt.Sprintf("%d of %d clicks", res.Summary.UniqueClicks, res.Summary.TotalClicks)
	if rate, ok := res.ComputedMetrics["unique_click_rate"]; ok {
		unique += ui.Dim.Render(fmt.Sprintf(" (%.0f%%)", rate))
	}
	return []string{
		label("trend (90d)") + miniSpark(res.Points("time", "clicks"), max(20, width-24)),
		label("unique") + unique,
		label("avg redirect") + fmt.Sprintf("%.0fms", res.Summary.AvgRedirectionTime),
		label("top browser") + topOf(res, "browser", total, nil),
		label("top os") + topOf(res, "os", total, nil),
		label("top country") + topOf(res, "country", total, ui.CountryLabel),
		label("top referrer") + topOf(res, "referrer", total, nil),
	}
}

// topOf names the dominant label of a dimension with its share; format
// optionally decorates the label (e.g. country flag emoji).
func topOf(res *api.StatsResponse, dimension string, total float64, format func(string) string) string {
	pts := res.Points(dimension, "clicks")
	if len(pts) == 0 {
		return "—"
	}
	best := pts[0]
	for _, p := range pts[1:] {
		if p.Value > best.Value {
			best = p
		}
	}
	name := best.Label
	if format != nil {
		name = format(name)
	}
	if total > 0 {
		return fmt.Sprintf("%s %s", name, ui.Dim.Render(fmt.Sprintf("(%.0f%%)", best.Value/total*100)))
	}
	return name
}
