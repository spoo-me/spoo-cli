package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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

func TestLoadNotLoggedIn(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keyring daemon"))
	s := NewStore(t.TempDir())
	if _, err := s.Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("Load err = %v, want ErrNotLoggedIn", err)
	}
}
