package stats

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spoo-me/spoo-cli/internal/api"
)

// timeWindow is the dashboard's stats window. Windows anchored to
// "now" (zero end) re-resolve at every fetch so auto-refresh tracks
// the clock; absolute windows stay put.
type timeWindow struct {
	span  time.Duration
	end   time.Time // zero = anchored to now
	label string    // the expression that produced it, for the header
}

func (w timeWindow) anchored() bool { return w.end.IsZero() }

var durTermRe = regexp.MustCompile(`^(\d+)\s*(mo|min|m|h|d|w)`)

var durUnits = map[string]time.Duration{
	"min": time.Minute,
	"m":   time.Minute,
	"h":   time.Hour,
	"d":   24 * time.Hour,
	"w":   7 * 24 * time.Hour,
	"mo":  30 * 24 * time.Hour,
}

// parseDur reads compound durations like "7d", "4h", "90min", "1w2d".
// m (and min) means minutes; mo means months of 30 days.
func parseDur(s string) (time.Duration, bool) {
	var total time.Duration
	rest := s
	for rest != "" {
		loc := durTermRe.FindStringSubmatchIndex(rest)
		if loc == nil {
			return 0, false
		}
		n, err := strconv.Atoi(rest[loc[2]:loc[3]])
		if err != nil {
			return 0, false
		}
		total += time.Duration(n) * durUnits[rest[loc[4]:loc[5]]]
		rest = strings.TrimSpace(rest[loc[1]:])
	}
	return total, total > 0
}

// absoluteLayouts accept lowercased input ('t' matches the lowered
// ISO separator); dates are read in the machine's local zone.
var absoluteLayouts = []string{
	"2006-01-02t15:04:05",
	"2006-01-02t15:04",
	"2006-01-02 15:04",
	"2006-01-02",
}

// parseEndpoint resolves one side of a range expression: "now",
// "now - 4h", a bare duration ("2w", read as that long ago), or an
// absolute date/datetime. dateOnly reports a bare date so callers can
// extend a "to" endpoint to the end of that day.
func parseEndpoint(s string, now time.Time) (t time.Time, dateOnly bool, err error) {
	if s == "now" {
		return now, false, nil
	}
	if rest, ok := strings.CutPrefix(s, "now"); ok {
		rest = strings.TrimSpace(rest)
		if back, ok := strings.CutPrefix(rest, "-"); ok {
			if d, ok := parseDur(strings.TrimSpace(back)); ok {
				return now.Add(-d), false, nil
			}
		}
		return time.Time{}, false, fmt.Errorf("can't read %q (try now - 4h)", s)
	}
	if d, ok := parseDur(strings.TrimSuffix(s, " ago")); ok {
		return now.Add(-d), false, nil
	}
	for _, layout := range absoluteLayouts {
		if abs, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return abs, layout == "2006-01-02", nil
		}
	}
	return time.Time{}, false, fmt.Errorf("can't read %q (try 7d, now - 4h, or 2026-01-15)", s)
}

// parseRangeExpr turns a range expression into a window. Accepted
// shapes: a trailing window ("7d", "4h", "5m", "now - 2w"), an
// explicit range ("now - 2w to now - 1w", "2026-01-01 to 2026-02-15"),
// or a single day ("2026-01-15"). Spans are capped at the server's
// 90-day limit.
func parseRangeExpr(input string, now time.Time) (timeWindow, error) {
	s := strings.Join(strings.Fields(strings.ToLower(input)), " ")
	if s == "" {
		return timeWindow{}, fmt.Errorf("empty range")
	}

	var from, to time.Time
	if before, after, found := strings.Cut(s, " to "); found {
		var err error
		if from, _, err = parseEndpoint(before, now); err != nil {
			return timeWindow{}, err
		}
		var dateOnly bool
		if to, dateOnly, err = parseEndpoint(after, now); err != nil {
			return timeWindow{}, err
		}
		if dateOnly {
			to = to.Add(24 * time.Hour) // a bare end date means through that day
		}
	} else if d, ok := parseDur(s); ok {
		from, to = now.Add(-d), now
	} else {
		t, dateOnly, err := parseEndpoint(s, now)
		if err != nil {
			return timeWindow{}, err
		}
		if dateOnly {
			from, to = t, t.Add(24*time.Hour)
		} else {
			from, to = t, now
		}
	}

	span := to.Sub(from)
	switch {
	case span <= 0:
		return timeWindow{}, fmt.Errorf("start must precede end")
	case span < time.Minute:
		return timeWindow{}, fmt.Errorf("range must cover at least a minute")
	case span > api.MaxRangeDays*24*time.Hour:
		return timeWindow{}, fmt.Errorf("range exceeds the server's %dd cap", api.MaxRangeDays)
	}

	w := timeWindow{span: span, label: s}
	if !to.Equal(now) {
		w.end = to
	}
	return w, nil
}
