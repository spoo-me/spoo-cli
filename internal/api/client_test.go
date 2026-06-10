package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
