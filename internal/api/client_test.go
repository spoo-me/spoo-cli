package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/spoo-me/spoo-cli/internal/auth"
)

func newTestStore(t *testing.T, c *auth.Credentials) *auth.Store {
	t.Helper()
	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	s := auth.NewStore(t.TempDir())
	if c != nil {
		if err := s.Save(*c); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestDoSendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	store := newTestStore(t, &auth.Credentials{Mode: auth.ModeDevice, AccessToken: "tok123", RefreshToken: "rt"})
	c := New(srv.URL, store)
	if err := c.do(context.Background(), http.MethodGet, "/auth/me", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok123" {
		t.Fatalf("Authorization = %q, want Bearer tok123", gotAuth)
	}
}

func TestDoSendsClientHeader(t *testing.T) {
	var gotClient string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClient = r.Header.Get("X-Spoo-Client")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	if err := c.do(context.Background(), http.MethodGet, "/auth/me", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotClient != "cli/dev" {
		t.Fatalf("X-Spoo-Client = %q, want cli/dev", gotClient)
	}
}

func TestClientHeaderRejectsMalformedVersion(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()
	for version, want := range map[string]string{
		"1.2.3":                  "cli/1.2.3",
		"0.2.0-SNAPSHOT-697203b": "cli", // >16 chars
		"1.0+meta":               "cli", // invalid charset
		"":                       "cli",
	} {
		Version = version
		if got := clientHeader(); got != want {
			t.Errorf("clientHeader() with Version=%q = %q, want %q", version, got, want)
		}
	}
}

func TestDoParsesErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"alias already taken","code":"CONFLICT_ERROR","detail":"try another"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	err := c.do(context.Background(), http.MethodPost, "/api/v1/shorten", nil, map[string]string{}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != 409 || apiErr.Code != "CONFLICT_ERROR" || apiErr.Message != "alias already taken" {
		t.Fatalf("unexpected APIError: %+v", apiErr)
	}
}

func TestDoRefreshesOn401AndRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/device/refresh":
			w.Write([]byte(`{"access_token":"newAT","refresh_token":"newRT"}`))
		case "/auth/me":
			if r.Header.Get("Authorization") == "Bearer newAT" {
				w.Write([]byte(`{"user":{"id":"1"}}`))
				return
			}
			calls.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"token expired","code":"AUTHENTICATION_ERROR"}`))
		}
	}))
	defer srv.Close()

	store := newTestStore(t, &auth.Credentials{Mode: auth.ModeDevice, AccessToken: "staleAT", RefreshToken: "oldRT"})
	c := New(srv.URL, store)
	if err := c.do(context.Background(), http.MethodGet, "/auth/me", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected exactly one 401 before refresh, got %d", calls.Load())
	}
	// rotated tokens must be persisted
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "newAT" || got.RefreshToken != "newRT" {
		t.Fatalf("store not updated after refresh: %+v", got)
	}
}

func TestClientHeaderStrippedOnCrossOriginRedirect(t *testing.T) {
	gotClient := "unset"
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClient = r.Header.Get("X-Spoo-Client")
		w.Write([]byte(`{}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/final", http.StatusFound)
	}))
	defer redirector.Close()

	c := New(redirector.URL, newTestStore(t, nil))
	if err := c.do(context.Background(), http.MethodGet, "/start", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotClient != "" {
		t.Fatalf("X-Spoo-Client forwarded cross-origin = %q, want empty", gotClient)
	}
}

func TestClientHeaderKeptOnSameHostRedirect(t *testing.T) {
	gotClient := "unset"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		gotClient = r.Header.Get("X-Spoo-Client")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	if err := c.do(context.Background(), http.MethodGet, "/start", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotClient != "cli/dev" {
		t.Fatalf("X-Spoo-Client after same-host redirect = %q, want cli/dev", gotClient)
	}
}

// a password-protected link answers 401 too, but that's about the link,
// not the session — no token refresh, and an honest error instead of
// "session expired".
func TestDo401PasswordRequiredSkipsRefresh(t *testing.T) {
	var refreshCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/device/refresh" {
			refreshCalls.Add(1)
			w.Write([]byte(`{"access_token":"newAT","refresh_token":"newRT"}`))
			return
		}
		w.Header().Set("X-Error-Code", "password_required")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Password required","code":"password_required"}`))
	}))
	defer srv.Close()

	store := newTestStore(t, &auth.Credentials{Mode: auth.ModeDevice, AccessToken: "goodAT", RefreshToken: "goodRT"})
	c := New(srv.URL, store)
	err := c.do(context.Background(), http.MethodGet, "/api/v1/public/stats/secret", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "password protected") {
		t.Fatalf("err = %v, want a password-protected explanation", err)
	}
	if refreshCalls.Load() != 0 {
		t.Fatalf("refresh called %d times, want 0", refreshCalls.Load())
	}
	// the healthy session must survive untouched
	got, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got.AccessToken != "goodAT" || got.RefreshToken != "goodRT" {
		t.Fatalf("tokens rotated pointlessly: %+v", got)
	}
}
