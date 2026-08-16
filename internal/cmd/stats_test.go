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

// statsBody is the standard stats wire; the stray "scope" key proves
// the decoder tolerates it (the response wire still carries one).
const statsBody = `{
	"scope": "anon",
	"summary": {"total_clicks": 100, "unique_clicks": 60, "first_click": "2026-05-01T10:00:00Z", "last_click": "2026-06-01T10:00:00Z", "avg_redirection_time": 0.12},
	"metrics": {
		"clicks_by_browser": [
			{"browser": "Chrome", "clicks": 70.0},
			{"browser": "Firefox", "clicks": 30.0}
		],
		"clicks_by_time": [
			{"time": "2026-05-01", "clicks": 40.0},
			{"time": "2026-05-02", "clicks": 60.0}
		]
	}
}`

const publicStatsBody = `{"generation": "v2", "link": {"alias": "launch"}, "stats": ` + statsBody + `}`

// pointDepsAtLoggedIn is pointDepsAt with stored credentials, so the
// command under test sees a logged-in session.
func pointDepsAtLoggedIn(t *testing.T, srvURL string) {
	t.Helper()
	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	store := auth.NewStore(t.TempDir())
	if err := store.Save(auth.Credentials{Mode: auth.ModeAPIKey, APIKey: "spoo_k"}); err != nil {
		t.Fatal(err)
	}
	orig := newDeps
	newDeps = func() (*deps, error) {
		return &deps{client: api.New(srvURL, store), store: store, cfg: config.Config{APIBase: srvURL}}, nil
	}
	t.Cleanup(func() { newDeps = orig })
}

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

// anonymous + code goes straight to the public endpoint.
func TestStatsAnonymousCodeUsesPublicEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if q := r.URL.Query(); q.Has("scope") || q.Has("group_by") {
			t.Errorf("public request must send no scope/group_by: %v", q)
		}
		w.Write([]byte(publicStatsBody))
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
	if gotPath != "/api/v1/public/stats/launch" {
		t.Fatalf("path = %q, want the public stats endpoint", gotPath)
	}
	text := out.String()
	for _, want := range []string{"100 clicks", "60 unique", "Chrome", "Browsers", "Clicks over time"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
}

// logged in without a code reads the account surface — and never
// sends the removed scope param.
func TestStatsAccountWideWhenLoggedIn(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Query().Has("scope") {
			t.Errorf("scope param must not be sent: %v", r.URL.Query())
		}
		w.Write([]byte(statsBody))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/stats" {
		t.Fatalf("path = %q, want /api/v1/stats", gotPath)
	}
}

// logged in + code resolves the alias to a url id, then reads the
// per-link endpoint.
func TestStatsResolvesOwnedLinkToPerLinkEndpoint(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/api/v1/urls/") {
			w.Write([]byte(`{"id":"65f0abc123","alias":"launch","long_url":"https://x.com","status":"ACTIVE"}`))
			return
		}
		w.Write([]byte(statsBody))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats", "launch"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{"/api/v1/urls/127.0.0.1/launch", "/api/v1/stats/links/65f0abc123"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

// a code that doesn't resolve may still be someone else's public link,
// so stats falls back to the public endpoint.
func TestStatsFallsBackToPublicForForeignCode(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/api/v1/urls/") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"URL not found","code":"not_found"}`))
			return
		}
		w.Write([]byte(publicStatsBody))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats", "launch"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[1] != "/api/v1/public/stats/launch" {
		t.Fatalf("paths = %v, want a public-stats fallback", paths)
	}
}
