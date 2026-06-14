package stats

import (
	"strings"
	"testing"
	"time"
)

func TestParseRangeExpr(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	cases := []struct {
		in       string
		span     time.Duration
		anchored bool
		end      time.Time // checked when not anchored
	}{
		{"7d", 7 * day, true, time.Time{}},
		{"24h", 24 * time.Hour, true, time.Time{}},
		{"4h", 4 * time.Hour, true, time.Time{}},
		{"5m", 5 * time.Minute, true, time.Time{}},
		{"90min", 90 * time.Minute, true, time.Time{}},
		{"2w", 14 * day, true, time.Time{}},
		{"1mo", 30 * day, true, time.Time{}},
		{"1w2d", 9 * day, true, time.Time{}},
		{"now - 7d", 7 * day, true, time.Time{}},
		{"now-7d", 7 * day, true, time.Time{}},
		{"NOW - 7D", 7 * day, true, time.Time{}},
		{"3d ago", 3 * day, true, time.Time{}},
		{"now - 2w to now - 1w", 7 * day, false, now.Add(-7 * day)},
		{"now - 2w to now", 14 * day, true, time.Time{}},
		{"2w to 1w", 7 * day, false, now.Add(-7 * day)},
	}
	for _, c := range cases {
		w, err := parseRangeExpr(c.in, now)
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if w.span != c.span {
			t.Errorf("%q: span = %v, want %v", c.in, w.span, c.span)
		}
		if w.anchored() != c.anchored {
			t.Errorf("%q: anchored = %v, want %v", c.in, w.anchored(), c.anchored)
		}
		if !c.anchored && !w.end.Equal(c.end) {
			t.Errorf("%q: end = %v, want %v", c.in, w.end, c.end)
		}
	}
}

func TestParseRangeExprAbsoluteDates(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	w, err := parseRangeExpr("2026-01-01 to 2026-01-31", now)
	if err != nil {
		t.Fatal(err)
	}
	// the end date is inclusive: through the end of Jan 31
	if want := 31 * 24 * time.Hour; w.span != want {
		t.Fatalf("span = %v, want %v", w.span, want)
	}
	if w.anchored() {
		t.Fatal("absolute range should not be anchored to now")
	}

	// a single day covers that whole day
	w, err = parseRangeExpr("2026-01-15", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := 24 * time.Hour; w.span != want {
		t.Fatalf("span = %v, want %v", w.span, want)
	}
}

func TestParseRangeExprErrors(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		in      string
		errPart string
	}{
		{"", "empty"},
		{"banana", "can't read"},
		{"now - 1w to now - 2w", "start must precede end"},
		{"30s", "can't read"},
		{"120d", "90d cap"},
		{"2026-01-01 to 2026-06-01", "90d cap"},
	}
	for _, c := range cases {
		_, err := parseRangeExpr(c.in, now)
		if err == nil {
			t.Errorf("%q: expected error, got none", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.errPart) {
			t.Errorf("%q: error %q, want it to mention %q", c.in, err, c.errPart)
		}
	}
}
