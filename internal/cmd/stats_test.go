package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/config"
)

const statsBody = `{
	"scope": "anon",
	"summary": {"total_clicks": 100, "unique_clicks": 60, "first_click": "2026-05-01T10:00:00Z", "last_click": "2026-06-01T10:00:00Z", "avg_redirection_time": 0.12},
	"metrics": {
		"clicks_by_browser": [
			{"browser": "Chrome", "clicks": 70.0},
			{"browser": "Firefox", "clicks": 30.0}
		],
		"clicks_by_time": [
			{"date": "2026-05-01", "clicks": 40.0},
			{"date": "2026-05-02", "clicks": 60.0}
		]
	}
}`

func TestStatsAnonymousRequiresShortCode(t *testing.T) {
	pointDepsAt(t, "http://unused.invalid")
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"stats"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "short code") {
		t.Fatalf("err = %v, want short-code guidance", err)
	}
}

func TestStatsRendersChartsForAnonCode(t *testing.T) {
	var gotScope, gotCode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotScope = r.URL.Query().Get("scope")
		gotCode = r.URL.Query().Get("short_code")
		w.Write([]byte(statsBody))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats", "launch"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotScope != "anon" || gotCode != "launch" {
		t.Fatalf("scope=%q code=%q, want anon/launch", gotScope, gotCode)
	}
	text := out.String()
	for _, want := range []string{"100 clicks", "60 unique", "Chrome", "Browsers", "Clicks over time"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
}

func TestStatsUsesAllScopeWhenLoggedIn(t *testing.T) {
	var gotScope string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotScope = r.URL.Query().Get("scope")
		w.Write([]byte(statsBody))
	}))
	defer srv.Close()

	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	store := auth.NewStore(t.TempDir())
	if err := store.Save(auth.Credentials{Mode: auth.ModeAPIKey, APIKey: "spoo_k"}); err != nil {
		t.Fatal(err)
	}
	orig := newDeps
	newDeps = func() (*deps, error) {
		return &deps{client: api.New(srv.URL, store), store: store, cfg: config.Config{APIBase: srv.URL}}, nil
	}
	t.Cleanup(func() { newDeps = orig })

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotScope != "all" {
		t.Fatalf("scope = %q, want all", gotScope)
	}
}
