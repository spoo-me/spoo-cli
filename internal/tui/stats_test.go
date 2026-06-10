package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/zalando/go-keyring"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
)

func testStatsResponse() *api.StatsResponse {
	return &api.StatsResponse{
		Scope:   "all",
		Summary: api.StatsSummary{TotalClicks: 100, UniqueClicks: 40, AvgRedirectionTime: 88},
		TimeRange: api.StatsTimeRange{
			StartDate: "2026-03-12T00:00:00Z", EndDate: "2026-06-10T00:00:00Z",
		},
		ComputedMetrics: map[string]float64{"unique_click_rate": 40, "average_clicks_per_visitor": 2.5},
		Metrics: map[string][]map[string]any{
			"clicks_by_time": {
				{"time": "2026-06-01", "clicks": 60.0},
				{"time": "2026-06-02", "clicks": 40.0},
			},
			"clicks_by_browser":        {{"browser": "Chrome", "clicks": 70.0}, {"browser": "Safari", "clicks": 30.0}},
			"unique_clicks_by_browser": {{"browser": "Safari", "unique_clicks": 25.0}, {"browser": "Chrome", "unique_clicks": 15.0}},
			"clicks_by_os":             {{"os": "Windows", "clicks": 100.0}},
			"clicks_by_country":        {{"country": "IN", "clicks": 100.0}},
			"clicks_by_city":           {{"city": "Pune", "clicks": 100.0}},
			"clicks_by_referrer":       {{"referrer": "twitter.com", "clicks": 100.0}},
		},
	}
}

func newStatsModel(t *testing.T, srvURL string) StatsModel {
	t.Helper()
	keyring.MockInit()
	client := api.New(srvURL, auth.NewStore(t.TempDir()))
	m := NewStats(client, "", "all", "")
	next, _ := m.Update(statsLoadedMsg{res: testStatsResponse()})
	return next.(StatsModel)
}

func statsKey(t *testing.T, m StatsModel, key string) (StatsModel, tea.Cmd) {
	t.Helper()
	var msg tea.KeyPressMsg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		msg = tea.KeyPressMsg{Code: tea.KeyTab}
	default:
		msg = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	next, cmd := m.Update(msg)
	return next.(StatsModel), cmd
}

func TestDashboardRendersAllSections(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	view := m.View().Content
	for _, want := range []string{
		"spoo stats", "✦ overview", "clicks", "unique", "avg redirect", "88ms",
		"✦ browsers", "✦ operating systems", "✦ countries", "✦ cities", "✦ referrers",
		"Chrome", "🇮🇳 IN", "Pune", "twitter.com", "2026-03-12",
		"70%", // percentage column: Chrome 70 of 100 clicks
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestDrillDownAddsServerSideFilter(t *testing.T) {
	var gotBrowser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBrowser = r.URL.Query().Get("browser")
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":70},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL) // focus 0 = Browsers, selection 0 = Chrome
	m, cmd := statsKey(t, m, "enter")
	if len(m.filters) != 1 || m.filters[0] != (filterEntry{dim: "browser", value: "Chrome"}) {
		t.Fatalf("filters = %+v", m.filters)
	}
	if cmd == nil {
		t.Fatal("drill-down did not refetch")
	}
	cmd()
	if gotBrowser != "Chrome" {
		t.Fatalf("browser param = %q, want Chrome", gotBrowser)
	}

	// 'x' removes the filter and refetches unfiltered
	m, cmd = statsKey(t, m, "x")
	if len(m.filters) != 0 || cmd == nil {
		t.Fatalf("x did not undo the filter (filters=%v cmd=%v)", m.filters, cmd)
	}
	cmd()
	if gotBrowser != "" {
		t.Fatalf("browser param after undo = %q, want empty", gotBrowser)
	}
}

func TestMetricToggleDoesNotRefetch(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m, cmd := statsKey(t, m, "u")
	if cmd != nil {
		t.Fatal("metric toggle must not refetch — both metrics are already in the payload")
	}
	if m.metric != "unique_clicks" {
		t.Fatalf("metric = %q", m.metric)
	}
	// panels re-rank by the new metric: Safari leads unique_clicks
	pts := m.panelPoints(0)
	if pts[0].Label != "Safari" {
		t.Fatalf("top browser by unique = %q, want Safari", pts[0].Label)
	}
}

func TestRangeCycleRefetches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, cmd := statsKey(t, m, "t")
	if m.rangeDays != 30 || cmd == nil {
		t.Fatalf("rangeDays = %d, cmd = %v; want 30 with refetch", m.rangeDays, cmd)
	}
	cmd()
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	m, _ = statsKey(t, m, "t")
	if m.rangeDays != 7 {
		t.Fatalf("rangeDays = %d, want 7", m.rangeDays)
	}
}

func TestPanelFocusAndSelection(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m, _ = statsKey(t, m, "tab")
	if m.focus != 1 {
		t.Fatalf("focus = %d, want 1 (OS)", m.focus)
	}
	m, _ = statsKey(t, m, "j")
	if m.sel[1] != 0 { // OS panel has one row; selection clamps
		t.Fatalf("sel = %d, want clamped 0", m.sel[1])
	}
}

func TestEscClearsFiltersThenQuits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, _ = statsKey(t, m, "enter") // add a filter
	m, cmd := statsKey(t, m, "esc")
	if len(m.filters) != 0 || cmd == nil {
		t.Fatal("first esc should clear filters and refetch")
	}
	_, cmd = statsKey(t, m, "esc")
	if cmd == nil {
		t.Fatal("second esc should quit")
	}
}
