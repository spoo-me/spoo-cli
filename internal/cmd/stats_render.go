package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

const (
	barWidth   = 28
	topNPerDim = 5
)

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

type labelValue struct {
	label string
	value float64
}

// extractPoints pulls (label, value) pairs out of the loosely typed
// metrics payload: each point names its label under the dimension key
// ("browser": "Chrome") and its value under the metric key ("clicks": 70).
func extractPoints(points []map[string]any, dimension, metric string) []labelValue {
	out := make([]labelValue, 0, len(points))
	for _, p := range points {
		label, _ := p[dimension].(string)
		value, ok := p[metric].(float64)
		if label == "" || !ok {
			continue
		}
		out = append(out, labelValue{label: label, value: value})
	}
	return out
}

func renderBarChart(title string, points []labelValue, total float64) string {
	if len(points) == 0 {
		return ""
	}
	sort.Slice(points, func(i, j int) bool { return points[i].value > points[j].value })
	if len(points) > topNPerDim {
		points = points[:topNPerDim]
	}
	maxVal := points[0].value
	if maxVal == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(ui.Title.Render(title) + "\n")
	for _, p := range points {
		bar := strings.Repeat("█", max(1, int(p.value/maxVal*barWidth)))
		pct := ""
		if total > 0 {
			pct = fmt.Sprintf(" (%.0f%%)", p.value/total*100)
		}
		b.WriteString(fmt.Sprintf("  %-16s %s %.0f%s\n",
			truncate(p.label, 16), ui.OK.Render(bar), p.value, ui.Dim.Render(pct)))
	}
	return b.String()
}

func renderSparkline(points []labelValue) string {
	if len(points) == 0 {
		return ""
	}
	// time points arrive sorted by date label; keep the most recent window
	const window = 60
	if len(points) > window {
		points = points[len(points)-window:]
	}
	var maxVal float64
	for _, p := range points {
		maxVal = max(maxVal, p.value)
	}
	if maxVal == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(ui.Title.Render("Clicks over time") + "\n  ")
	for _, p := range points {
		idx := int(p.value / maxVal * float64(len(sparkRunes)-1))
		b.WriteRune(sparkRunes[idx])
	}
	b.WriteString("\n  " + ui.Dim.Render(fmt.Sprintf("%s … %s · peak %.0f",
		points[0].label, points[len(points)-1].label, maxVal)) + "\n")
	return b.String()
}

func renderStats(res *api.StatsResponse, target string) string {
	var sections []string

	header := "all links"
	if target != "" {
		header = target
	}
	summary := fmt.Sprintf("%s\n\n%s  %s\n%s  %s",
		ui.Title.Render("Stats · "+header),
		ui.OK.Render(fmt.Sprintf("%d clicks", res.Summary.TotalClicks)),
		ui.Dim.Render(fmt.Sprintf("%d unique", res.Summary.UniqueClicks)),
		firstLast(res.Summary),
		ui.Dim.Render(fmt.Sprintf("avg redirect %.0fms", res.Summary.AvgRedirectionTime*1000)),
	)
	sections = append(sections, ui.Box.Render(summary))

	if pts := extractPoints(res.Metrics["clicks_by_time"], "date", "clicks"); len(pts) > 0 {
		sections = append(sections, renderSparkline(pts))
	}
	total := float64(res.Summary.TotalClicks)
	for _, dim := range []struct{ key, title string }{
		{"browser", "Browsers"},
		{"os", "Operating systems"},
		{"country", "Countries"},
		{"referrer", "Referrers"},
	} {
		pts := extractPoints(res.Metrics["clicks_by_"+dim.key], dim.key, "clicks")
		if chart := renderBarChart(dim.title, pts, total); chart != "" {
			sections = append(sections, chart)
		}
	}
	return strings.Join(sections, "\n")
}

func firstLast(s api.StatsSummary) string {
	if s.FirstClick == "" {
		return ui.Dim.Render("no clicks yet")
	}
	first, last := s.FirstClick, s.LastClick
	if len(first) >= 10 {
		first = first[:10]
	}
	if len(last) >= 10 {
		last = last[:10]
	}
	return ui.Dim.Render("first " + first + " · last " + last)
}
