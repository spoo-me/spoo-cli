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

// Regression: the table's default keymap binds 'd' to half-page-down.
// If the keypress leaks into the table, the cursor moves between the
// arming press and the confirming press, deleting the wrong row.
func TestDeleteConfirmTargetsSameRow(t *testing.T) {
	var deleted string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = r.URL.Path
		}
		w.Write([]byte(`{"message":"ok","id":"x"}`))
	}))
	defer srv.Close()

	m := newLinksModelWithPage(t, srv.URL)
	if m.tbl.Cursor() != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.tbl.Cursor())
	}

	m, _ = pressKey(t, m, 'd') // arm confirmation
	if m.pending != "id-first" {
		t.Fatalf("pending = %q, want id-first", m.pending)
	}
	if m.tbl.Cursor() != 0 {
		t.Fatalf("cursor moved to %d after arming delete — key leaked into the table", m.tbl.Cursor())
	}

	m, cmd := pressKey(t, m, 'd') // confirm
	if cmd == nil {
		t.Fatal("confirming press returned no command")
	}
	if msg, ok := cmd().(actionMsg); !ok || msg.err != nil {
		t.Fatalf("delete failed: %+v", msg)
	}
	if deleted != "/api/v1/urls/id-first" {
		t.Fatalf("deleted %q, want /api/v1/urls/id-first", deleted)
	}
}

func TestOtherKeyCancelsPendingDelete(t *testing.T) {
	m := newLinksModelWithPage(t, "http://unused.invalid")
	m, _ = pressKey(t, m, 'd')
	if m.pending == "" {
		t.Fatal("expected pending delete after first d")
	}
	m, _ = pressKey(t, m, 'r') // any other action cancels
	if m.pending != "" {
		t.Fatalf("pending = %q, want cleared", m.pending)
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

// d-d with the pane open deletes the selected item.
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
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(LinksModel)

	m, _ = pressKey(t, m, 'd') // arm
	_, cmd := pressKey(t, m, 'd')
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
