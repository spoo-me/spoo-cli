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

func renderBarChart(title string, points []api.MetricPoint, total float64) string {
	if len(points) == 0 {
		return ""
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Value > points[j].Value })
	if len(points) > topNPerDim {
		points = points[:topNPerDim]
	}
	maxVal := points[0].Value
	if maxVal == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(ui.Title.Render(title) + "\n")
	for _, p := range points {
		bar := strings.Repeat("█", max(1, int(p.Value/maxVal*barWidth)))
		pct := ""
		if total > 0 {
			pct = fmt.Sprintf(" (%.0f%%)", p.Value/total*100)
		}
		fmt.Fprintf(&b, "  %-16s %s %.0f%s\n",
			truncate(p.Label, 16), ui.OK.Render(bar), p.Value, ui.Dim.Render(pct))
	}
	return b.String()
}

func renderSparkline(points []api.MetricPoint) string {
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
		maxVal = max(maxVal, p.Value)
	}
	if maxVal == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(ui.Title.Render("Clicks over time") + "\n  ")
	for _, p := range points {
		idx := int(p.Value / maxVal * float64(len(ui.SparkRunes)-1))
		b.WriteRune(ui.SparkRunes[idx])
	}
	b.WriteString("\n  " + ui.Dim.Render(fmt.Sprintf("%s … %s · peak %.0f",
		points[0].Label, points[len(points)-1].Label, maxVal)) + "\n")
	return b.String()
}

func renderStats(res *api.StatsResponse, target string) string {
	var sections []string

	header := "all links"
	if target != "" {
		header = target
	}
	title := ui.Title.Render("Stats · " + header)
	if res.TimeRange.StartDate != "" {
		title += ui.Dim.Render("  " + isoDay(res.TimeRange.StartDate) + " → " + isoDay(res.TimeRange.EndDate))
	}
	summary := fmt.Sprintf("%s\n\n%s  %s\n%s  %s",
		title,
		ui.OK.Render(fmt.Sprintf("%d clicks", res.Summary.TotalClicks)),
		ui.Dim.Render(fmt.Sprintf("%d unique", res.Summary.UniqueClicks)),
		firstLast(res.Summary),
		ui.Dim.Render(fmt.Sprintf("avg redirect %.0fms", res.Summary.AvgRedirectionTime)),
	)
	sections = append(sections, ui.Box.Render(summary))

	if pts := res.Points("time", "clicks"); len(pts) > 0 {
		if chart := renderSparkline(pts); chart != "" {
			sections = append(sections, chart)
		}
	}
	total := float64(res.Summary.TotalClicks)
	for _, dim := range []struct{ key, title string }{
		{"browser", "Browsers"},
		{"os", "Operating systems"},
		{"country", "Countries"},
		{"referrer", "Referrers"},
	} {
		pts := res.Points(dim.key, "clicks")
		if dim.key == "country" {
			for i := range pts {
				pts[i].Label = ui.CountryLabel(pts[i].Label)
			}
		}
		if chart := renderBarChart(dim.title, pts, total); chart != "" {
			sections = append(sections, chart)
		}
	}
	return strings.Join(sections, "\n")
}

func isoDay(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
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
