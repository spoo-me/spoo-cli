package tui

import (
	"net/http"
	"net/http/httptest"
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
	m := NewLinks(client, srvURL, "", func(string) error { return nil }, func(string) error { return nil })

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
