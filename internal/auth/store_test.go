package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	spoo "github.com/spoo-me/spoo-go"
	"github.com/spoo-me/spoo-go/option"
	"github.com/zalando/go-keyring"
)

func TestStoreRoundTripKeyring(t *testing.T) {
	keyring.MockInit() // in-memory keyring; never touch the real one in tests
	s := NewStore(t.TempDir())

	want := Credentials{Mode: ModeDevice, AccessToken: "at", RefreshToken: "rt"}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if *got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("after Clear, Load err = %v, want ErrNotLoggedIn", err)
	}
}

func TestStoreFileFallback(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keyring daemon"))
	dir := t.TempDir()
	s := NewStore(dir)

	want := Credentials{Mode: ModeAPIKey, APIKey: "spoo_abc123"}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "credentials.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("fallback file not written: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials file mode = %v, want 0600", info.Mode().Perm())
	}

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if *got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
}

// The store doubles as the SDK's TokenSource; not being logged in maps
// to anonymous credentials, never an error.
func TestTokenSourceMapsModes(t *testing.T) {
	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	s := NewStore(t.TempDir())
	ctx := context.Background()

	creds, err := s.Token(ctx)
	if err != nil || creds != (spoo.Credentials{}) {
		t.Fatalf("anonymous Token = %+v, %v; want zero creds, nil", creds, err)
	}

	if err := s.Save(Credentials{Mode: ModeAPIKey, APIKey: "spoo_k"}); err != nil {
		t.Fatal(err)
	}
	if creds, _ = s.Token(ctx); creds.APIKey != "spoo_k" || creds.AccessToken != "" {
		t.Fatalf("api-key Token = %+v", creds)
	}

	if err := s.Save(Credentials{Mode: ModeDevice, AccessToken: "at", RefreshToken: "rt"}); err != nil {
		t.Fatal(err)
	}
	if creds, _ = s.Token(ctx); creds.AccessToken != "at" || creds.RefreshToken != "rt" || creds.APIKey != "" {
		t.Fatalf("device Token = %+v", creds)
	}
}

// The SDK refreshes on 401 and persists the rotated pair through
// Update — the store must end up holding the new tokens.
func TestRefreshRotationPersistsThroughStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/device/refresh":
			w.Write([]byte(`{"access_token":"newAT","refresh_token":"newRT"}`))
		case "/auth/me":
			if r.Header.Get("Authorization") == "Bearer newAT" {
				w.Write([]byte(`{"user":{"id":"1","email":"a@b.c"}}`))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"token expired","code":"AUTHENTICATION_ERROR"}`))
		}
	}))
	defer srv.Close()

	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	s := NewStore(t.TempDir())
	if err := s.Save(Credentials{Mode: ModeDevice, AccessToken: "staleAT", RefreshToken: "oldRT"}); err != nil {
		t.Fatal(err)
	}
	client := spoo.NewClient(option.WithBaseURL(srv.URL), option.WithTokenSource(s))
	if _, err := client.Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeDevice || got.AccessToken != "newAT" || got.RefreshToken != "newRT" {
		t.Fatalf("store not updated after refresh: %+v", got)
	}
}

// A definitive refresh rejection surfaces as spoo.ErrSessionExpired,
// which the commands translate into "run `spoo auth login` again".
func TestDeadRefreshTokenIsSessionExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid refresh token","code":"AUTHENTICATION_ERROR"}`))
	}))
	defer srv.Close()

	keyring.MockInit()
	_ = keyring.Delete("spoo-cli", "credentials")
	s := NewStore(t.TempDir())
	if err := s.Save(Credentials{Mode: ModeDevice, AccessToken: "deadAT", RefreshToken: "deadRT"}); err != nil {
		t.Fatal(err)
	}
	client := spoo.NewClient(option.WithBaseURL(srv.URL), option.WithTokenSource(s))
	_, err := client.Me(context.Background())
	if !errors.Is(err, spoo.ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}

func TestLoadNotLoggedIn(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keyring daemon"))
	s := NewStore(t.TempDir())
	if _, err := s.Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("Load err = %v, want ErrNotLoggedIn", err)
	}
}
