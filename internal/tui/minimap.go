package tui

import (
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// The timeline minimap is the two-row strip under the traffic chart:
// a long-lookback sparkline of every window fetched so far with the
// current window bracketed. It is passive — paging with [/] fills it
// in — and it inherits the active drill-down filters (the cache is
// cleared whenever they change).

const (
	histDay      = 24 * time.Hour
	histDayKey   = "2006-01-02"
	minHistRange = 7  // hourly windows would shadow daily totals
	minLookback  = 90 // days the strip spans even before any paging
	minimapRows  = 2  // spark row + bracket row
)

type histPoint struct{ clicks, unique float64 }

// mergeHist folds one fetched window into the lookback cache. start
// and end bound the window so zero days register as known rather than
// unfetched. Sub-daily windows are skipped: their partial-day sums
// would overwrite honest daily totals.
func mergeHist(hist map[string]histPoint, res *api.StatsResponse, start, end time.Time, rangeDays int) {
	if res == nil || rangeDays < minHistRange {
		return
	}
	for day := start.Truncate(histDay); !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format(histDayKey)
		if _, ok := hist[key]; !ok {
			hist[key] = histPoint{}
		}
	}
	merge := func(metric string, set func(*histPoint, float64)) {
		days := map[string]float64{}
		for _, p := range res.Points("time", metric) {
			if ts, ok := parseBucketTime(p.Label); ok {
				days[ts.Format(histDayKey)] += p.Value
			}
		}
		for k, v := range days {
			hp := hist[k]
			set(&hp, v)
			hist[k] = hp
		}
	}
	merge("clicks", func(h *histPoint, v float64) { h.clicks = v })
	merge("unique_clicks", func(h *histPoint, v float64) { h.unique = v })
}

// minimap renders the strip at the given inner width.
func (m StatsModel) minimap(width int) string {
	if width < 20 {
		return ""
	}
	winStart, winEnd := m.window()
	winStartDay, winEndDay := winStart.Truncate(histDay), winEnd.Truncate(histDay)
	today := time.Now().UTC().Truncate(histDay)

	oldest := today.AddDate(0, 0, -minLookback)
	if winStartDay.Before(oldest) {
		oldest = winStartDay
	}
	for k := range m.hist {
		if t, err := time.Parse(histDayKey, k); err == nil && t.Before(oldest) {
			oldest = t
		}
	}
	span := int(today.Sub(oldest)/histDay) + 1

	// every column maps to a slice of the lookback; the right edge is
	// always today, aligned with the chart's right edge
	type col struct {
		v            float64
		known, inWin bool
	}
	cols := make([]col, width)
	var maxV float64
	for i := range cols {
		from, to := i*span/width, max(i*span/width+1, (i+1)*span/width)
		for d := from; d < min(to, span); d++ {
			day := oldest.AddDate(0, 0, d)
			if hp, ok := m.hist[day.Format(histDayKey)]; ok {
				cols[i].known = true
				if m.metric == "unique_clicks" {
					cols[i].v += hp.unique
				} else {
					cols[i].v += hp.clicks
				}
			}
			if !day.Before(winStartDay) && !day.After(winEndDay) {
				cols[i].inWin = true
			}
		}
		maxV = max(maxV, cols[i].v)
	}

	fill := lipgloss.NewStyle().Foreground(m.metricHue())
	var spark strings.Builder
	for _, c := range cols {
		switch {
		case !c.known:
			spark.WriteString(ui.Dim.Render("·"))
		default:
			lvl := 0
			if maxV > 0 && c.v > 0 {
				lvl = min(len(ui.SparkRunes)-1, 1+int(c.v/maxV*float64(len(ui.SparkRunes)-2)))
			}
			r := string(ui.SparkRunes[lvl])
			if c.inWin {
				spark.WriteString(fill.Render(r))
			} else {
				spark.WriteString(ui.Dim.Render(r))
			}
		}
	}

	// bracket row: the window bracket, with edge date labels rendered
	// only into the gaps it leaves (at offset 0 the bracket itself
	// touching the right edge is the "today" marker)
	row := []rune(strings.Repeat(" ", width))
	first, last := -1, -1
	for i, c := range cols {
		if c.inWin {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	if first != -1 {
		if first == last {
			row[first] = '▴'
		} else {
			row[first], row[last] = '╰', '╯'
			for i := first + 1; i < last; i++ {
				row[i] = '─'
			}
		}
	}
	left, right := oldest.Format(histDayKey), "today"
	if first > len(left)+1 || first == -1 {
		copy(row, []rune(left))
	}
	if last < width-len(right)-1 {
		copy(row[width-len(right):], []rune(right))
	}
	bracket := ui.Dim.Render(string(row))
	if first != -1 {
		sat := lipgloss.NewStyle().Foreground(hueFor(m.metricHue()))
		bracket = ui.Dim.Render(string(row[:first])) +
			sat.Render(string(row[first:last+1])) +
			ui.Dim.Render(string(row[last+1:]))
	}
	return spark.String() + "\n" + bracket
}
