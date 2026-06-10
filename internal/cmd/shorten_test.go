package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/config"
)

// pointDepsAt redirects the command dependency factory at a test server.
func pointDepsAt(t *testing.T, srvURL string) {
	t.Helper()
	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	store := auth.NewStore(t.TempDir())
	orig := newDeps
	newDeps = func() (*deps, error) {
		return &deps{client: api.New(srvURL, store), store: store, cfg: config.Config{APIBase: srvURL}}, nil
	}
	t.Cleanup(func() { newDeps = orig })
}

func TestShortenCommandJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"x","short_url":"https://spoo.me/abc","alias":"abc","long_url":"https://example.com","status":"ACTIVE"}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"shorten", "https://example.com", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var res api.ShortURL
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if res.ShortURL != "https://spoo.me/abc" {
		t.Fatalf("short_url = %q", res.ShortURL)
	}
}

func TestShortenCommandPipedBulk(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"x","short_url":"https://spoo.me/l` + string(rune('0'+n)) + `","alias":"a","long_url":"y","status":"ACTIVE"}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader("https://one.com\n\nhttps://two.com\n"))
	root.SetArgs([]string{"shorten"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || n != 2 {
		t.Fatalf("want 2 links (server saw %d):\n%s", n, out.String())
	}
}

func TestShortenCommandAPIErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"alias already taken","code":"CONFLICT_ERROR"}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"shorten", "https://example.com", "--alias", "taken"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "alias already taken") {
		t.Fatalf("err = %v, want alias-taken message", err)
	}
}
