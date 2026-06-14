package kit

import (
	"math"
	"strings"
	"time"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// NiceCeil rounds up to a 1/2/2.5/5×10ⁿ boundary so axis steps are even.
func NiceCeil(v float64) float64 {
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

// ParseBucketTime parses a backend time-bucket label.
func ParseBucketTime(label string) (time.Time, bool) {
	for _, layout := range bucketTimeLayouts {
		if ts, err := time.Parse(layout, label); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

// MiniSpark draws a compact sparkline covering the WHOLE series: when
// there are more points than columns they are summed into buckets, so
// old activity is never silently cut off the left edge.
func MiniSpark(pts []api.MetricPoint, width int) string {
	if len(pts) == 0 || width < 1 {
		return ui.Dim.Render("no data")
	}
	buckets := make([]float64, min(width, len(pts)))
	for i, p := range pts {
		buckets[i*len(buckets)/len(pts)] += p.Value
	}
	var maxV float64
	for _, v := range buckets {
		maxV = max(maxV, v)
	}
	if maxV == 0 {
		return ui.Dim.Render("flat")
	}
	var b strings.Builder
	for _, v := range buckets {
		b.WriteRune(ui.SparkRunes[int(v/maxV*float64(len(ui.SparkRunes)-1))])
	}
	return b.String()
}
