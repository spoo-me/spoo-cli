package tui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/zalando/go-keyring"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
)

func newLinksModelWithPage(t *testing.T, srvURL string) LinksModel {
	t.Helper()
	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	client := api.New(srvURL, auth.NewStore(t.TempDir()))
	m := NewLinks(client, srvURL, api.ListURLsOptions{}, func(string) error { return nil }, func(string) error { return nil })

	page := &api.URLPage{
		Items: []api.URLItem{
			{ID: "id-first", Alias: "first", LongURL: "https://a.com", Status: "ACTIVE"},
			{ID: "id-second", Alias: "second", LongURL: "https://b.com", Status: "ACTIVE"},
			{ID: "id-third", Alias: "third", LongURL: "https://c.com", Status: "ACTIVE"},
		},
		Page: 1, PageSize: 20, Total: 3,
	}
	next, _ := m.Update(pageMsg{page: page})
	return next.(LinksModel)
}

func pressKey(t *testing.T, m LinksModel, key rune) (LinksModel, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	return next.(LinksModel), cmd
}

func pressEnter(t *testing.T, m LinksModel) (LinksModel, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return next.(LinksModel), cmd
}

// Regression: the table's default keymap binds 'd' to half-page-down.
// If the keypress leaks into the table, the cursor moves between the
// arming press and the confirming press, deleting the wrong row.
func TestDeleteRequiresTypedAlias(t *testing.T) {
	var deleted string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = r.URL.Path
		}
		w.Write([]byte(`{"message":"ok","id":"x"}`))
	}))
	defer srv.Close()

	m := newLinksModelWithPage(t, srv.URL)
	m, _ = pressKey(t, m, 'd') // open the delete dialog for "first"
	if !m.confirm.open || m.confirm.tag != "delete" || m.confirm.tagID != "id-first" {
		t.Fatalf("delete dialog not armed for the selected row: %+v", m.confirm)
	}

	// pressing enter before typing the alias must NOT delete
	m, cmd := pressEnter(t, m)
	if !m.confirm.open || cmd != nil {
		t.Fatal("enter without the typed alias should not confirm")
	}

	for _, ch := range "first" {
		m, _ = pressKey(t, m, ch)
	}
	m, cmd = pressEnter(t, m)
	if m.confirm.open || cmd == nil {
		t.Fatal("typing the alias then enter should confirm and delete")
	}
	if msg, ok := cmd().(actionMsg); !ok || msg.err != nil {
		t.Fatalf("delete failed: %+v", msg)
	}
	if deleted != "/api/v1/urls/id-first" {
		t.Fatalf("deleted %q, want /api/v1/urls/id-first", deleted)
	}
}

func TestDeleteDialogEscCancels(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	m, _ = pressKey(t, m, 'd')
	if !m.confirm.open {
		t.Fatal("expected the delete dialog after d")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(LinksModel)
	if m.confirm.open {
		t.Fatal("esc should close the delete dialog without deleting")
	}
}

// Flags passed on the command line must reach the TUI's fetch query.
func TestFetchCarriesOptions(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = map[string]string{
			"sortBy":   r.URL.Query().Get("sortBy"),
			"pageSize": r.URL.Query().Get("pageSize"),
			"filter":   r.URL.Query().Get("filter"),
		}
		w.Write([]byte(`{"items":[],"page":1,"pageSize":50,"total":0,"hasNext":false}`))
	}))
	defer srv.Close()

	keyring.MockInit()
	client := api.New(srv.URL, auth.NewStore(t.TempDir()))
	m := NewLinks(client, srv.URL, api.ListURLsOptions{
		SortBy: "last_click", PageSize: 50, Status: "INACTIVE", Search: "demo",
	}, nil, nil)
	m.Init()() // run the initial fetch command
	if got["sortBy"] != "last_click" || got["pageSize"] != "50" {
		t.Fatalf("query = %v", got)
	}
	for _, want := range []string{"INACTIVE", "demo"} {
		if !strings.Contains(got["filter"], want) {
			t.Fatalf("filter %q missing %q", got["filter"], want)
		}
	}
}

func TestDefaultSortIsTotalClicks(t *testing.T) {
	keyring.MockInit()
	client := api.New("http://unused.invalid", auth.NewStore(t.TempDir()))
	m := NewLinks(client, "http://unused.invalid", api.ListURLsOptions{}, nil, nil)
	if m.opts.SortBy != "total_clicks" {
		t.Fatalf("default sort = %q, want total_clicks", m.opts.SortBy)
	}
}

// 's' cycles the sort field and refetches from page 1.
func TestSortKeyCyclesAndRefetches(t *testing.T) {
	var gotSort, gotPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSort = r.URL.Query().Get("sortBy")
		gotPage = r.URL.Query().Get("page")
		w.Write([]byte(`{"items":[],"page":1,"pageSize":20,"total":0,"hasNext":false}`))
	}))
	defer srv.Close()

	m := newLinksModelWithPage(t, srv.URL)
	m, cmd := pressKey(t, m, 's')
	if cmd == nil {
		t.Fatal("s produced no refetch")
	}
	cmd()
	if gotSort != "created_at" || gotPage != "1" {
		t.Fatalf("sortBy=%q page=%q, want created_at page 1", gotSort, gotPage)
	}
	if m.opts.SortBy != "created_at" {
		t.Fatalf("model sort = %q", m.opts.SortBy)
	}
}

// '/' opens the search box; typing + enter applies the search.
func TestSearchModeAppliesQuery(t *testing.T) {
	var gotFilter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFilter = r.URL.Query().Get("filter")
		w.Write([]byte(`{"items":[],"page":1,"pageSize":20,"total":0,"hasNext":false}`))
	}))
	defer srv.Close()

	m := newLinksModelWithPage(t, srv.URL)
	m, _ = pressKey(t, m, '/')
	if !m.searching {
		t.Fatal("/ did not enter search mode")
	}
	for _, r := range "abc" {
		m, _ = pressKey(t, m, r)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	if m.searching || m.opts.Search != "abc" {
		t.Fatalf("searching=%v search=%q, want applied abc", m.searching, m.opts.Search)
	}
	if cmd == nil {
		t.Fatal("enter produced no refetch")
	}
	cmd()
	if !strings.Contains(gotFilter, "abc") {
		t.Fatalf("filter = %q, want abc", gotFilter)
	}
}

// esc cancels search mode without changing the applied query.
func TestSearchModeEscCancels(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	m, _ = pressKey(t, m, '/')
	for _, r := range "xyz" {
		m, _ = pressKey(t, m, r)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(LinksModel)
	if m.searching || m.opts.Search != "" {
		t.Fatalf("searching=%v search=%q, want cancelled empty", m.searching, m.opts.Search)
	}
}

// enter opens the detail pane for the selected row; esc returns to the
// list without quitting.
func TestEnterOpensDetailAndEscCloses(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	if !m.showDetail {
		t.Fatal("enter did not open the detail pane")
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(LinksModel)
	if m.showDetail {
		t.Fatal("esc did not close the detail pane")
	}
	if cmd != nil {
		t.Fatal("esc with detail open must not quit")
	}
}

// the pane mirrors the selection: moving the cursor changes the detail.
func TestDetailFollowsSelection(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	m, _ = pressKey(t, m, 'j') // move down while the pane is open
	if got := m.selected().Alias; got != "second" {
		t.Fatalf("selected = %q, want second", got)
	}
	view := m.View().Content
	if !strings.Contains(view, "https://b.com") {
		t.Fatalf("detail did not follow selection:\n%s", view)
	}
}

// delete with the detail pane open targets the selected item.
func TestDetailDeleteTargetsSelectedItem(t *testing.T) {
	var deleted string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = r.URL.Path
		}
		w.Write([]byte(`{"items":[],"page":1,"pageSize":20,"total":0,"hasNext":false}`))
	}))
	defer srv.Close()

	m := newLinksModelWithPage(t, srv.URL)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // open detail pane
	m = next.(LinksModel)

	m, _ = pressKey(t, m, 'd')
	for _, ch := range "first" {
		m, _ = pressKey(t, m, ch)
	}
	_, cmd := pressEnter(t, m)
	if cmd == nil {
		t.Fatal("confirm produced no command")
	}
	cmd()
	if deleted != "/api/v1/urls/id-first" {
		t.Fatalf("deleted %q, want id-first", deleted)
	}
}

// wide terminals get the side-by-side layout; narrow ones full screen.
func TestSplitLayoutIsResponsive(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = next.(LinksModel)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	if !m.splitActive() {
		t.Fatal("140 cols with detail open should use the split layout")
	}
	view := m.View().Content
	// in the split, the table (alias column) and detail coexist
	if !strings.Contains(view, "second") || !strings.Contains(view, "destination") {
		t.Fatalf("split view missing table or detail:\n%s", view)
	}

	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = next.(LinksModel)
	if m.splitActive() {
		t.Fatal("80 cols must fall back to the full-screen pane")
	}
}

// the detail view renders the fields the table truncates
func TestDetailViewShowsFullFields(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	view := m.View().Content
	for _, want := range []string{"https://a.com", "short url", "password", "clicks"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail view missing %q:\n%s", want, view)
		}
	}
}

// Rage-scrolling with the pane open must fire ZERO stats requests:
// each move only re-arms the debounce, and stale ticks are dropped.
func TestStatsDebounceDropsStaleTicks(t *testing.T) {
	var statsCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/stats") {
			statsCalls++
		}
		w.Write([]byte(`{"scope":"all","summary":{"total_clicks":1},"metrics":{}}`))
	}))
	defer srv.Close()

	m := newLinksModelWithPage(t, srv.URL)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // opens pane, arms seq 1
	m = next.(LinksModel)
	seq1 := m.statsSeq

	m, _ = pressKey(t, m, 'j') // arms seq 2
	m, _ = pressKey(t, m, 'j') // arms seq 3
	if m.statsSeq != seq1+2 {
		t.Fatalf("statsSeq = %d, want %d", m.statsSeq, seq1+2)
	}

	// stale ticks (seq 1 and 2) arrive — both must be dropped
	for _, seq := range []int{seq1, seq1 + 1} {
		next, cmd := m.Update(statsTickMsg{seq: seq})
		m = next.(LinksModel)
		if cmd != nil {
			t.Fatalf("stale tick seq=%d triggered a fetch", seq)
		}
	}
	if statsCalls != 0 {
		t.Fatalf("stats endpoint hit %d times during scroll, want 0", statsCalls)
	}

	// the current tick fetches exactly once, for the rested row
	next, cmd := m.Update(statsTickMsg{seq: m.statsSeq})
	m = next.(LinksModel)
	if cmd == nil {
		t.Fatal("current tick did not fetch")
	}
	msg := cmd()
	if statsCalls != 1 {
		t.Fatalf("stats calls = %d, want 1", statsCalls)
	}
	sm, ok := msg.(statsMsg)
	if !ok || sm.alias != "third" {
		t.Fatalf("fetched %+v, want stats for third (the rested row)", msg)
	}
}

// cached rows schedule nothing — revisiting is free.
func TestStatsCacheSkipsRefetch(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	m.stats["first"] = statsEntry{res: &api.StatsResponse{}}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	if cmd != nil {
		t.Fatal("opening the pane on a cached row scheduled a fetch")
	}
	if !m.showDetail {
		t.Fatal("pane did not open")
	}
}

// once stats land, the pane renders the analytics section.
func TestDetailRendersAnalytics(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)
	next, _ = m.Update(statsMsg{alias: "first", res: &api.StatsResponse{
		Summary: api.StatsSummary{TotalClicks: 10, UniqueClicks: 4, AvgRedirectionTime: 42},
		Metrics: map[string][]map[string]any{
			"clicks_by_time":    {{"time": "2026-06-01", "clicks": 10.0}},
			"clicks_by_browser": {{"browser": "Chrome", "clicks": 9.0}},
			"clicks_by_os":      {{"os": "Windows", "clicks": 6.0}},
			"clicks_by_country": {{"country": "IN", "clicks": 10.0}},
		},
	}})
	m = next.(LinksModel)
	view := m.View().Content
	for _, want := range []string{"analytics", "4 of 10 clicks", "Chrome", "(90%)", "Windows", "top country", "42ms"} {
		if !strings.Contains(view, want) {
			t.Fatalf("analytics section missing %q:\n%s", want, view)
		}
	}
}

// the sparkline covers the whole series: early activity must not be
// truncated off the left edge when there are more points than columns.
func TestMiniSparkDownsamplesWholeSeries(t *testing.T) {
	pts := make([]api.MetricPoint, 90)
	for i := range pts {
		pts[i] = api.MetricPoint{Label: "d", Value: 0}
	}
	pts[3].Value = 28 // old spike, far outside the last 30 columns
	got := kit.MiniSpark(pts, 30)
	if strings.Contains(got, "flat") {
		t.Fatalf("old spike was cut off: %q", got)
	}
	if !strings.ContainsRune(got, '█') {
		t.Fatalf("expected a full-height bucket for the spike: %q", got)
	}
}

// Q pops a QR dialog for the selected link; c copies, anything closes.
func TestQRDialog(t *testing.T) {
	var copied string
	m := newLinksModelWithPage(t, "http://unused.invalid")
	m.copyText = func(s string) error { copied = s; return nil }

	m, _ = pressKey(t, m, 'Q')
	if m.qrURL == "" {
		t.Fatal("Q should open the QR dialog for the selected link")
	}
	view := m.View().Content
	if !strings.Contains(view, "▄") || !strings.Contains(view, "first") {
		t.Fatal("QR dialog should render the code and the URL")
	}
	m, _ = pressKey(t, m, 'c')
	if m.qrURL != "" || !strings.Contains(copied, "/first") {
		t.Fatalf("c should copy and close (copied=%q)", copied)
	}

	m, _ = pressKey(t, m, 'Q')
	m, _ = pressKey(t, m, 'x')
	if m.qrURL != "" {
		t.Fatal("any key should dismiss the QR dialog")
	}
}

// The edit form diffs against the original and only PATCHes real changes.
func TestEditFormChanges(t *testing.T) {
	max := 100
	it := api.URLItem{
		ID: "id-x", Alias: "launch", LongURL: "https://old.com",
		Status: "ACTIVE", MaxClicks: &max,
	}
	e, _ := newEditForm().show(it)

	// no edits → no changes
	if ch, _ := e.changes(); len(ch) != 0 {
		t.Fatalf("unchanged form yields %v, want empty", ch)
	}

	// edit destination, alias, max-clicks, status; leave password blank
	e.inputs[fDest].SetValue("https://new.com")
	e.inputs[fAlias].SetValue("promo")
	e.inputs[fMaxClicks].SetValue("0")
	e.status = "inactive"
	ch, err := e.changes()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"long_url": "https://new.com", "alias": "promo",
		"max_clicks": 0, "status": "INACTIVE", // API enum is upper-case
	}
	if len(ch) != len(want) {
		t.Fatalf("changes = %v, want %v", ch, want)
	}
	for k, v := range want {
		if fmt.Sprintf("%v", ch[k]) != fmt.Sprintf("%v", v) {
			t.Fatalf("changes[%q] = %v, want %v", k, ch[k], v)
		}
	}
	if _, ok := ch["password"]; ok {
		t.Fatal("blank password must not be sent")
	}
}

// 'e' opens the editor pre-filled with the selected link.
func TestEditKeyOpensPrefilledForm(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	m, cmd := pressKey(t, m, 'e')
	if !m.edit.open || cmd == nil {
		t.Fatal("e should open the edit form")
	}
	if m.edit.inputs[fDest].Value() != "https://a.com" || m.edit.inputs[fAlias].Value() != "first" {
		t.Fatalf("form not pre-filled: dest=%q alias=%q", m.edit.inputs[fDest].Value(), m.edit.inputs[fAlias].Value())
	}
}

// the edit dialog: esc cancels, the status toggle flips, enter saves
// the upper-cased status, and the form is boxed.
func TestEditFormInteraction(t *testing.T) {
	var patchedStatus string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			b, _ := io.ReadAll(r.Body)
			patchedStatus = string(b)
		}
		w.Write([]byte(`{"message":"ok","id":"id-first"}`))
	}))
	defer srv.Close()

	// esc cancels without saving
	m := newLinksModelWithPage(t, srv.URL)
	m.width, m.height = 120, 40
	m.relayout()
	m, _ = pressKey(t, m, 'e')
	if v := m.View().Content; !strings.Contains(v, "edit first") || !strings.Contains(v, "╮") {
		t.Fatal("edit form should be titled and boxed")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(LinksModel)
	if m.edit.open || m.confirm.open {
		t.Fatal("esc should close the edit form with no save dialog")
	}

	// reopen, jump to the status field, flip it, save → confirm → PATCH
	m, _ = pressKey(t, m, 'e')
	for i := 0; i < statusField; i++ {
		next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m = next.(LinksModel)
	}
	if m.edit.focus != statusField {
		t.Fatalf("focus = %d, want the status field", m.edit.focus)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // toggle active→inactive
	m = next.(LinksModel)
	if m.edit.status != "inactive" {
		t.Fatalf("status toggle = %q, want inactive", m.edit.status)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // save
	m = next.(LinksModel)
	if !m.confirm.open || m.confirm.tag != "save" {
		t.Fatalf("enter should stage a save confirmation: %+v", m.confirm)
	}
	m, cmd := pressEnter(t, m) // confirm
	if cmd == nil {
		t.Fatal("confirm produced no PATCH")
	}
	cmd()
	if !strings.Contains(patchedStatus, `"INACTIVE"`) {
		t.Fatalf("PATCH body = %q, want upper-case INACTIVE", patchedStatus)
	}
}
