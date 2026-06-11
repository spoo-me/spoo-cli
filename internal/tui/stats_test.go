package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
			"clicks_by_short_code":     {{"short_code": "launch", "clicks": 60.0}, {"short_code": "promo", "clicks": 40.0}},
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
		"✦ top links", "✦ browsers", "✦ operating systems", "✦ countries", "✦ cities", "✦ referrers",
		"launch", "Chrome", "Pune", "twitter.com", "2026-03-12",
		"70%", // percentage column: Chrome 70 of 100 clicks
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard missing %q", want)
		}
	}
}

func TestDrillDownAddsServerSideFilter(t *testing.T) {
	var gotCode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCode = r.URL.Query().Get("short_code")
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":70},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, _ = statsKey(t, m, "tab") // focus 1 = top links, selection 0 = launch
	m, cmd := statsKey(t, m, "enter")
	if len(m.filters) != 1 || m.filters[0] != (filterEntry{dim: "short_code", value: "launch"}) {
		t.Fatalf("filters = %+v", m.filters)
	}
	if cmd == nil {
		t.Fatal("drill-down did not refetch")
	}
	cmd()
	if gotCode != "launch" {
		t.Fatalf("short_code param = %q, want launch", gotCode)
	}

	// 'x' removes the filter and refetches unfiltered
	m, cmd = statsKey(t, m, "x")
	if len(m.filters) != 0 || cmd == nil {
		t.Fatalf("x did not undo the filter (filters=%v cmd=%v)", m.filters, cmd)
	}
	cmd()
	if gotCode != "" {
		t.Fatalf("short_code param after undo = %q, want empty", gotCode)
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
	pts := m.panelPoints(1, panelTopN) // panel 1 = browsers
	if pts[0].Label != "Safari" {
		t.Fatalf("top browser by unique = %q, want Safari", pts[0].Label)
	}
}

func TestRangeExpressionApplies(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, _ = statsKey(t, m, "T")
	if !m.rangeMode {
		t.Fatal("T should open the range strip")
	}
	for _, ch := range []string{"3", "0", "d"} {
		m, _ = statsKey(t, m, ch)
	}
	m, cmd := statsKey(t, m, "enter")
	if m.rangeMode || cmd == nil {
		t.Fatalf("rangeMode = %v, cmd = %v; want closed strip with refetch", m.rangeMode, cmd)
	}
	if want := 30 * 24 * time.Hour; m.win.span != want {
		t.Fatalf("span = %v, want %v", m.win.span, want)
	}
	cmd()
	if calls != 2 { // current window + previous window (for deltas)
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestRangeExpressionRejectsGarbage(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m, _ = statsKey(t, m, "T")
	for _, ch := range []string{"n", "o", "p", "e"} {
		m, _ = statsKey(t, m, ch)
	}
	m, cmd := statsKey(t, m, "enter")
	if !m.rangeMode || m.rangeErr == "" || cmd != nil {
		t.Fatalf("rangeMode=%v err=%q cmd=%v; want open strip with error and no fetch", m.rangeMode, m.rangeErr, cmd)
	}
	m, _ = statsKey(t, m, "esc")
	if m.rangeMode || m.win != defaultWindow {
		t.Fatalf("esc should close the strip and keep the window (mode=%v win=%+v)", m.rangeMode, m.win)
	}
}

func TestPanelFocusAndSelection(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	if m.focus != 0 {
		t.Fatalf("focus = %d, want the time chart focused first", m.focus)
	}
	for range 3 {
		m, _ = statsKey(t, m, "tab")
	}
	if m.focus != 3 {
		t.Fatalf("focus = %d, want 3 (OS)", m.focus)
	}
	m, _ = statsKey(t, m, "j")
	if m.sel[2] != 0 { // OS panel has one row; selection clamps
		t.Fatalf("sel = %d, want clamped 0", m.sel[2])
	}
}

// f promotes the focused panel. ↑/↓ moves rows in the main pane;
// ←/→ switches panes, and in the sidebar ↑/↓ walks the charts.
func TestFocusModePromotesAndCycles(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m, _ = statsKey(t, m, "tab") // top links
	m, _ = statsKey(t, m, "tab") // browsers
	m, _ = statsKey(t, m, "f")
	if !m.focusMode || m.focusItem != 2 { // item 0 = time chart, 2 = browsers
		t.Fatalf("focusMode=%v item=%d, want focused browsers (2)", m.focusMode, m.focusItem)
	}
	if m.focusPane != 0 {
		t.Fatalf("focusPane = %d, want main on entry", m.focusPane)
	}
	view := m.View().Content
	for _, want := range []string{"✦ charts", "▸ browsers", "traffic over time", "Chrome"} {
		if !strings.Contains(view, want) {
			t.Fatalf("focus view missing %q", want)
		}
	}

	// main pane: j moves the row selection, not the chart
	m, _ = statsKey(t, m, "j")
	if m.focusItem != 2 || m.sel[1] != 1 {
		t.Fatalf("main-pane j: item=%d sel=%d, want item 2 / row 1", m.focusItem, m.sel[1])
	}

	// sidebar pane: j switches charts
	m, _ = statsKey(t, m, "l")
	if m.focusPane != 1 {
		t.Fatalf("focusPane = %d, want sidebar after l", m.focusPane)
	}
	m, _ = statsKey(t, m, "j")
	if m.focusItem != 3 {
		t.Fatalf("sidebar j: focusItem = %d, want 3", m.focusItem)
	}

	m, _ = statsKey(t, m, "x")
	if m.focusMode {
		t.Fatal("x did not exit focus mode")
	}
	if m.focus != 3 { // exit hands the dashboard cursor the last focused chart
		t.Fatalf("focus after exit = %d, want 3", m.focus)
	}
}

// enter in the main pane drills down on the selected row.
func TestFocusModeEnterDrills(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, _ = statsKey(t, m, "tab") // top links
	m, _ = statsKey(t, m, "tab") // browsers
	m, _ = statsKey(t, m, "f")
	m, _ = statsKey(t, m, "j") // select Safari
	m, cmd := statsKey(t, m, "enter")
	if cmd == nil || len(m.filters) != 1 || m.filters[0] != (filterEntry{dim: "browser", value: "Safari"}) {
		t.Fatalf("focus-mode drill failed: filters=%v", m.filters)
	}
}

// [ pages the window back in time and refetches; ] returns forward.
func TestWindowPaging(t *testing.T) {
	var gotStart, gotEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("group_by") != "time" { // ignore the prev-window probe
			gotStart = r.URL.Query().Get("start_date")
			gotEnd = r.URL.Query().Get("end_date")
		}
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, cmd := statsKey(t, m, "[")
	if m.offset != 1 || cmd == nil {
		t.Fatalf("offset = %d, want 1 with refetch", m.offset)
	}
	cmd()
	if gotStart == "" || gotEnd == "" {
		t.Fatal("paged-back window must send both start_date and end_date")
	}
	m, _ = statsKey(t, m, "]")
	if m.offset != 0 {
		t.Fatalf("offset = %d, want 0 after ]", m.offset)
	}
}

func TestEscClearsFiltersThenQuits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, _ = statsKey(t, m, "tab")   // top links
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

// t toggles the focused panel between bars and a bubbles table; each
// panel keeps its own mode, so several tables can be open at once.
func TestTableToggleIsPerPanel(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m, _ = statsKey(t, m, "tab")  // focus 1 = top links
	m, cmd := statsKey(t, m, "t") // toggle its table
	if cmd != nil {
		t.Fatal("table toggle must not refetch")
	}
	if !m.tableOn["short_code"] {
		t.Fatal("t did not enable table mode for top links")
	}
	view := m.View().Content
	if !strings.Contains(view, "link") || !strings.Contains(view, "share") {
		t.Fatalf("table headers missing from view")
	}

	m, _ = statsKey(t, m, "tab") // browsers
	m, _ = statsKey(t, m, "t")
	if !m.tableOn["short_code"] || !m.tableOn["browser"] {
		t.Fatalf("expected both panels in table mode: %v", m.tableOn)
	}

	m, _ = statsKey(t, m, "t") // toggle browsers back off
	if m.tableOn["browser"] {
		t.Fatal("second t did not toggle browsers back to bars")
	}
}

// g opens the link picker; choosing a link re-targets the dashboard.
func TestLinkSwitcher(t *testing.T) {
	var gotCode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/urls") {
			w.Write([]byte(`{"items":[{"alias":"launch","long_url":"https://a.com","total_clicks":60},{"alias":"promo","long_url":"https://b.com","total_clicks":40}],"page":1,"pageSize":100,"total":2}`))
			return
		}
		gotCode = r.URL.Query().Get("short_code")
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, cmd := statsKey(t, m, "g")
	if !m.switchMode || cmd == nil {
		t.Fatal("g should open the picker and fetch the link list")
	}
	// deliver the list (the Focus and list cmds are batched; run the batch)
	for _, msg := range drainCmd(cmd) {
		next, _ := m.Update(msg)
		m = next.(StatsModel)
	}
	if len(m.switchAll) != 2 {
		t.Fatalf("switchAll = %d items, want 2", len(m.switchAll))
	}

	// type "pro" to filter, ↓ to the only match, enter to switch
	for _, ch := range []string{"p", "r", "o"} {
		m, _ = statsKey(t, m, ch)
	}
	m, _ = statsKey(t, m, "down")
	m, cmd = statsKey(t, m, "enter")
	if m.switchMode || m.target != "promo" || cmd == nil {
		t.Fatalf("switch failed: mode=%v target=%q", m.switchMode, m.target)
	}
	cmd()
	if gotCode != "promo" {
		t.Fatalf("short_code param = %q, want promo", gotCode)
	}

	// esc just closes without switching
	m, _ = statsKey(t, m, "g")
	m, _ = statsKey(t, m, "esc")
	if m.switchMode || m.target != "promo" {
		t.Fatal("esc should close the picker and keep the target")
	}
}

// drainCmd runs a command (flattening batches) and returns its messages.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainCmd(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// p ghosts the previous window's series on the time chart.
func TestPrevPeriodGhost(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m.prev = &api.StatsResponse{
		Summary: api.StatsSummary{TotalClicks: 30},
		Metrics: map[string][]map[string]any{
			"clicks_by_time": {{"time": "2026-03-05", "clicks": 30.0}},
		},
	}
	if strings.Contains(m.View().Content, "previous") {
		t.Fatal("ghost legend should be off by default")
	}
	m, cmd := statsKey(t, m, "p")
	if cmd != nil {
		t.Fatal("toggling the ghost must not refetch — the data is already loaded")
	}
	if !m.showPrev || !strings.Contains(m.View().Content, "─── previous 90d") {
		t.Fatalf("ghost legend missing (showPrev=%v)", m.showPrev)
	}
	m, _ = statsKey(t, m, "p")
	if m.showPrev {
		t.Fatal("second p should hide the ghost")
	}
}

// e opens the export dialog; the filename extension picks the format
// and esc closes without exporting.
func TestExportModal(t *testing.T) {
	var gotFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFormat = r.URL.Query().Get("format")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := newStatsModel(t, srv.URL)
	m, _ = statsKey(t, m, "e")
	if !m.exportBox.open {
		t.Fatal("e should open the export dialog")
	}
	view := m.View().Content
	if !strings.Contains(view, "export analytics") {
		t.Fatal("export dialog not rendered")
	}
	if !strings.Contains(view, "overview") {
		t.Fatal("the dashboard should stay visible behind the dialog")
	}
	name := m.exportBox.name.Value()
	if !strings.HasPrefix(name, "spoo-stats-all-") || !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("default name = %q", name)
	}

	// a bad extension errors inline and keeps the dialog open
	m.exportBox.name.SetValue("report.pdf")
	m, _ = statsKey(t, m, "enter")
	if !m.exportBox.open || m.exportBox.err == "" {
		t.Fatalf("bad extension should error (open=%v err=%q)", m.exportBox.open, m.exportBox.err)
	}

	// .json drives the format param
	m.exportBox.name.SetValue("report.json")
	m, cmd := statsKey(t, m, "enter")
	if m.exportBox.open || cmd == nil {
		t.Fatal("enter with a valid name should close and export")
	}
	if msg := cmd(); msg != nil {
		if done, ok := msg.(exportDoneMsg); ok && done.err != nil {
			t.Fatalf("export failed: %v", done.err)
		}
	}
	if gotFormat != "json" {
		t.Fatalf("format = %q, want json", gotFormat)
	}
	os.Remove(filepath.Join(mustGetwd(t), "report.json"))

	// esc just closes
	m, _ = statsKey(t, m, "e")
	m, _ = statsKey(t, m, "esc")
	if m.exportBox.open {
		t.Fatal("esc should close the dialog")
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

// the time chart is focusable like any panel; t turns it into a
// date/clicks/unique table on the dashboard and in focus mode alike.
func TestTimeChartFocusAndTable(t *testing.T) {
	m := newStatsModel(t, "http://unused.invalid")
	m, _ = statsKey(t, m, "t") // focus starts on the time chart
	if !m.tableOn["time"] {
		t.Fatal("t did not toggle the time table on the dashboard")
	}
	view := m.View().Content
	for _, want := range []string{"· table", "2026-06-02", "unique"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard time table missing %q", want)
		}
	}

	m, _ = statsKey(t, m, "t") // back to the chart
	m, _ = statsKey(t, m, "f") // promote the time chart
	if !m.focusMode || m.focusItem != 0 {
		t.Fatalf("focusMode=%v item=%d, want time chart promoted", m.focusMode, m.focusItem)
	}
	m, _ = statsKey(t, m, "t")
	if !m.tableOn["time"] {
		t.Fatal("t did not toggle the time table in focus mode")
	}
	view = m.View().Content
	for _, want := range []string{"· table", "2026-06-02", "unique"} {
		if !strings.Contains(view, want) {
			t.Fatalf("focus-mode time table missing %q", want)
		}
	}
}
