package stats

import (
	"fmt"
	"time"

	tslc "github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// timeChartView is the dual-series traffic chart: a stateless render of
// the time buckets (clicks + unique, plus an optional previous-period
// ghost). It reads only the series it is handed, not the model.
type timeChartView struct {
	clicks, uniques, prev []api.MetricPoint
	span                  time.Duration
	label                 string
	showPrev              bool
}

// timeChartView builds the chart component from the current model state.
func (m Model) timeChartView() timeChartView {
	var prev []api.MetricPoint
	if m.prev != nil {
		prev = m.prev.Points("time", m.metric)
	}
	return timeChartView{
		clicks:   m.res.Points("time", "clicks"),
		uniques:  m.res.Points("time", "unique_clicks"),
		prev:     prev,
		span:     m.win.span,
		label:    m.win.label,
		showPrev: m.showPrev,
	}
}

func (c timeChartView) title() string {
	return "traffic over time · " + c.label
}

// legend names the chart's series, including the previous-period ghost
// while it's shown.
func (c timeChartView) legend() string {
	legend := chartClicks.Render("─── clicks") + "  " + chartUnique.Render("─── unique")
	if c.showPrev {
		legend += "  " + ui.Dim.Render("─── previous "+c.label)
	}
	return legend
}

// render draws clicks and unique clicks as braille lines.
func (c timeChartView) render(width, height int) string {
	if len(c.clicks) == 0 {
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
	clickSeries, maxV := toSeries(c.clicks)
	uniqueSeries, _ := toSeries(c.uniques)
	if len(clickSeries) == 0 {
		return ui.Dim.Render("no time series data")
	}
	if maxV == 0 {
		return ui.Dim.Render("no activity in this window")
	}

	// the previous window's series, shifted forward one span so both
	// periods share the x-axis — the ghost behind the current line
	var prevSeries []tslc.TimePoint
	if c.showPrev && len(c.prev) > 0 {
		var prevMax float64
		prevSeries, prevMax = toSeries(c.prev)
		for i := range prevSeries {
			prevSeries[i].Time = prevSeries[i].Time.Add(c.span)
		}
		maxV = max(maxV, prevMax) // a taller last period must not clip
	}

	// pad Y labels to the top value's width: ntcharts sizes the label
	// gutter by sampling step labels and would clip a wider top label
	yMax := kit.NiceCeil(maxV)
	yWidth := len(kit.CompactNum(yMax))
	chart := tslc.New(max(40, width-2), max(6, height),
		tslc.WithTimeSeries(clickSeries),
		tslc.WithYRange(0, yMax),
		tslc.WithXYSteps(10, 4),
		tslc.WithYLabelFormatter(func(_ int, v float64) string {
			return fmt.Sprintf("%*s", yWidth, kit.CompactNum(v))
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
