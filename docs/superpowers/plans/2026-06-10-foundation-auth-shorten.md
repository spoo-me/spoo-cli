# spoo CLI — Foundation, Auth & Shorten Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the `spoo-me/spoo-cli` repo and ship a working `spoo` binary with device-auth login (connected-apps integration), credential storage, a typed API client with transparent token refresh, and a polished `shorten` command (flags, piped-stdin bulk mode, and an interactive form with live alias availability).

**Architecture:** Cobra commands wrapped by Fang v2 for styled help/errors/manpages/completions. A thin hand-written typed API client (`internal/api`) talks to the spoo.me FastAPI backend, loading credentials from a keyring-backed store (`internal/auth`) and transparently refreshing device JWTs on 401. The device-auth flow runs a fixed-port loopback HTTP server (`127.0.0.1:53682/callback`) because the backend validates `redirect_uri` by exact match against `config/apps.yaml`. TUI views (links browser, stats dashboard) are **out of scope** — they come in follow-up plans and will add Bubble Tea v2 on top of this foundation.

**Tech Stack:** Go 1.26, `github.com/spf13/cobra` v1.10.x, `charm.land/fang/v2` v2.0.x, `charm.land/lipgloss/v2` v2.0.x, `charm.land/huh/v2` v2.0.x, `github.com/zalando/go-keyring` v0.2.x, `github.com/pkg/browser`, `golang.org/x/term`.

**Repo:** `/Users/zingzy/spoo/spoo-cli`, module `github.com/spoo-me/spoo-cli`, binary `spoo` (built from `cmd/spoo`).

**Follow-up plans (not in this document):** links TUI (Bubble Tea v2 table over `GET /api/v1/urls`), stats dashboard (`GET /api/v1/stats`), domains & keys commands, export.

---

## Backend contract (verified against spoo-latest source)

- `POST /api/v1/shorten` — body `{long_url, alias?, password?, block_bots?, max_clicks?, expire_after?, private_stats?, domain?}` → 201 `{id, short_url, alias, long_url, created_at, status, domain, ...}`. Anonymous allowed (lower rate limits).
- `GET /api/v1/shorten/check-alias?alias=X&domain=Y` → `{available: bool, reason: "length"|"format"|"taken"|null}`. Rate limit 180/min authed — built for debounce.
- `GET /auth/device/login?app_id=spoo-cli&redirect_uri=R&state=S` — browser consent page; on approval 302s to `R?code=...&state=...`. `R` must **exactly match** an entry in `config/apps.yaml` `redirect_uris` (see `services/auth/device.py:69-71` in spoo-latest).
- `POST /auth/device/token` — body `{code}` → `{access_token, refresh_token, user}`. Code expires in 5 minutes, one-time.
- `POST /auth/device/refresh` — body `{refresh_token}` → `{access_token, refresh_token}` (rotation; old refresh token invalidated).
- `GET /auth/me` — `Authorization: Bearer <jwt|spoo_apikey>` → `{user: {id, email, email_verified, name, plan, ...}}`.
- Error envelope on 4xx/5xx: `{"error": str, "code": str, "detail": str?}`.
- Access token TTL 15 min; refresh token 30 days. API keys are `spoo_`-prefixed Bearer tokens.

**Server-side prerequisite (separate repo, `/Users/zingzy/spoo/spoo-latest`):** `config/apps.yaml` entry `spoo-cli` is `status: coming_soon` with no `redirect_uris`. Before the device flow works against a real server it needs:

```yaml
  spoo-cli:
    name: Spoo CLI
    icon: spoo-cli.svg
    description: Shorten links from your terminal
    verified: true
    status: live
    type: device_auth
    redirect_uris:
      - http://127.0.0.1:53682/callback
```

This is Task 0 and must land in spoo-latest via its own PR. All CLI tests in this plan mock the backend with `httptest`, so CLI development does not block on it.

---

## File Structure

```
spoo-cli/
├── go.mod / go.sum
├── .gitignore
├── .golangci.yml
├── .goreleaser.yaml
├── .github/workflows/ci.yml
├── LICENSE                          (Apache-2.0, same as spoo-latest)
├── README.md
├── cmd/spoo/main.go                 — entry point; fang.Execute, version vars
└── internal/
    ├── config/config.go             — API base URL (env override), config dir
    │   └── config_test.go
    ├── ui/ui.go                     — lipgloss styles shared by all commands
    ├── auth/
    │   ├── store.go                 — Credentials + keyring/file token store
    │   ├── store_test.go
    │   ├── device.go                — loopback device-auth flow
    │   └── device_test.go
    ├── api/
    │   ├── client.go                — base client, error envelope, 401→refresh→retry
    │   ├── client_test.go
    │   ├── shorten.go               — Shorten, CheckAlias
    │   ├── shorten_test.go
    │   ├── auth.go                  — ExchangeDeviceCode, Me
    │   └── auth_test.go
    └── cmd/
        ├── root.go                  — NewRootCmd, dependency factory
        ├── auth.go                  — spoo auth login|logout|status
        ├── whoami.go                — spoo whoami
        ├── shorten.go               — spoo shorten (flags, stdin, output)
        ├── shorten_form.go          — interactive huh form
        ├── expiry.go                — relative-duration → epoch helper
        ├── expiry_test.go
        └── shorten_test.go          — command-level test against httptest
```

One responsibility per file. `internal/cmd` depends on `internal/api`, `internal/auth`, `internal/config`, `internal/ui`; nothing depends back on `internal/cmd`.

---

### Task 0: Backend registry change (repo: spoo-latest)

**Files:**
- Modify: `/Users/zingzy/spoo/spoo-latest/config/apps.yaml` (the `spoo-cli` entry)

- [ ] **Step 1: Edit the registry entry**

Change the `spoo-cli` entry to:

```yaml
  spoo-cli:
    name: Spoo CLI
    icon: spoo-cli.svg
    description: Shorten links from your terminal
    verified: true
    status: live
    type: device_auth
    redirect_uris:
      - http://127.0.0.1:53682/callback
```

- [ ] **Step 2: Run the backend's existing device-auth tests**

Run: `cd /Users/zingzy/spoo/spoo-latest && uv run pytest tests/integration/test_device_auth.py -q`
Expected: PASS (registry change is data-only; existing tests cover the consent flow)

- [ ] **Step 3: Commit on a branch and open a PR**

```bash
cd /Users/zingzy/spoo/spoo-latest
git checkout -b feat/spoo-cli-live
git add config/apps.yaml
git commit -m "feat(apps): mark spoo-cli live with loopback redirect URI"
```

Do **not** merge until the CLI is ready to release; the CLI work below proceeds against mocks. Return to the spoo-cli repo for all remaining tasks.

---

### Task 1: Repo scaffold + root command under Fang

**Files:**
- Create: `go.mod` (via `go mod init`), `.gitignore`, `cmd/spoo/main.go`, `internal/cmd/root.go`, `LICENSE`

- [ ] **Step 1: Initialize the module and fetch dependencies**

```bash
cd /Users/zingzy/spoo/spoo-cli
go mod init github.com/spoo-me/spoo-cli
go get github.com/spf13/cobra@v1.10.2 charm.land/fang/v2@v2.0.1 charm.land/lipgloss/v2@v2.0.3 charm.land/huh/v2@v2.0.3 github.com/zalando/go-keyring@v0.2.8 github.com/pkg/browser@latest golang.org/x/term@latest
```

Expected: `go.mod` and `go.sum` created without errors.

- [ ] **Step 2: Add `.gitignore` and LICENSE**

`.gitignore`:

```gitignore
/spoo
/dist/
*.test
coverage.out
```

```bash
cp /Users/zingzy/spoo/spoo-latest/LICENSE LICENSE
```

- [ ] **Step 3: Write `internal/cmd/root.go`**

```go
package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd builds the spoo root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spoo",
		Short:         "Shorten links and manage your spoo.me account from the terminal",
		Long:          "spoo is the official command-line client for spoo.me —\nshorten links, browse analytics, and manage your account without leaving the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "output machine-readable JSON")
	return root
}
```

- [ ] **Step 4: Write `cmd/spoo/main.go`**

```go
package main

import (
	"context"
	"os"

	fang "charm.land/fang/v2"
	"github.com/spoo-me/spoo-cli/internal/cmd"
)

// Set by goreleaser via ldflags.
var (
	version = "dev"
	commit  = ""
)

func main() {
	opts := []fang.Option{
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt),
	}
	if commit != "" {
		opts = append(opts, fang.WithCommit(commit))
	}
	if err := fang.Execute(context.Background(), cmd.NewRootCmd(), opts...); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Build and verify styled help renders**

```bash
go mod tidy
go build -o spoo ./cmd/spoo && ./spoo --help
```

Expected: Fang-styled help page (colored USAGE/FLAGS sections), exit code 0. `./spoo --version` prints `dev`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .gitignore LICENSE cmd internal docs
git commit -m "feat: scaffold spoo CLI with cobra root command under fang"
```

---

### Task 2: Config package (API base URL + config dir)

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadDefaultAPIBase(t *testing.T) {
	t.Setenv("SPOO_API_URL", "")
	cfg := Load()
	if cfg.APIBase != "https://spoo.me" {
		t.Fatalf("APIBase = %q, want https://spoo.me", cfg.APIBase)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("SPOO_API_URL", "http://localhost:8000")
	cfg := Load()
	if cfg.APIBase != "http://localhost:8000" {
		t.Fatalf("APIBase = %q, want http://localhost:8000", cfg.APIBase)
	}
}

func TestDirCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp) // honored on linux; darwin uses HOME
	t.Setenv("HOME", tmp)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Fatal("empty config dir")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `undefined: Load`

- [ ] **Step 3: Write `internal/config/config.go`**

```go
package config

import (
	"os"
	"path/filepath"
)

// DefaultAPIBase is the production spoo.me endpoint.
const DefaultAPIBase = "https://spoo.me"

type Config struct {
	APIBase string
}

// Load resolves configuration. SPOO_API_URL overrides the default,
// which keeps the CLI pointable at a local dev server.
func Load() Config {
	cfg := Config{APIBase: DefaultAPIBase}
	if v := os.Getenv("SPOO_API_URL"); v != "" {
		cfg.APIBase = v
	}
	return cfg
}

// Dir returns the spoo config directory, creating it if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "spoo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): API base URL with env override and config dir"
```

---

### Task 3: Shared UI styles

**Files:**
- Create: `internal/ui/ui.go`

No test — this file is pure style constants; it gets exercised by command tests later.

- [ ] **Step 1: Write `internal/ui/ui.go`**

```go
// Package ui holds the lipgloss styles shared by every spoo command,
// so CLI output and future TUI views render with one visual language.
package ui

import lipgloss "charm.land/lipgloss/v2"

var (
	Accent  = lipgloss.Color("#A78BFA") // spoo violet
	Success = lipgloss.Color("#34D399")
	Danger  = lipgloss.Color("#F87171")
	Muted   = lipgloss.Color("#9CA3AF")

	Title   = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	OK      = lipgloss.NewStyle().Bold(true).Foreground(Success)
	Err     = lipgloss.NewStyle().Bold(true).Foreground(Danger)
	Dim     = lipgloss.NewStyle().Foreground(Muted)
	Box     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Accent).Padding(0, 2)
	KeyHint = lipgloss.NewStyle().Foreground(Muted).Italic(true)
)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): shared lipgloss style palette"
```

---

### Task 4: Credential store (keyring with file fallback)

**Files:**
- Create: `internal/auth/store.go`
- Test: `internal/auth/store_test.go`

Design: credentials are one JSON blob stored under keyring service `spoo-cli`, account `credentials`. If the OS keyring is unavailable (headless Linux, CI), fall back to `<configdir>/credentials.json` with mode 0600. Two auth modes: `device` (JWT pair from the connected-apps flow) and `api_key` (a `spoo_...` token for non-interactive use).

- [ ] **Step 1: Write the failing tests**

`internal/auth/store_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -v`
Expected: FAIL — `undefined: NewStore`

- [ ] **Step 3: Write `internal/auth/store.go`**

```go
// Package auth manages spoo CLI credentials and the device-auth login flow.
package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "spoo-cli"
	keyringAccount = "credentials"
	credsFileName  = "credentials.json"
)

type Mode string

const (
	ModeDevice Mode = "device"  // JWT pair from the connected-apps device flow
	ModeAPIKey Mode = "api_key" // spoo_... API key (non-interactive / CI)
)

type Credentials struct {
	Mode         Mode   `json:"mode"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
}

var ErrNotLoggedIn = errors.New("not logged in — run `spoo auth login`")

// Store persists credentials in the OS keyring, falling back to a
// 0600 file in the config dir when no keyring is available.
type Store struct {
	dir string
}

func NewStore(configDir string) *Store { return &Store{dir: configDir} }

func (s *Store) file() string { return filepath.Join(s.dir, credsFileName) }

func (s *Store) Save(c Credentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, keyringAccount, string(data)); err == nil {
		return nil
	}
	return os.WriteFile(s.file(), data, 0o600)
}

func (s *Store) Load() (*Credentials, error) {
	if raw, err := keyring.Get(keyringService, keyringAccount); err == nil {
		return unmarshalCreds([]byte(raw))
	}
	data, err := os.ReadFile(s.file())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		return nil, err
	}
	return unmarshalCreds(data)
}

func (s *Store) Clear() error {
	_ = keyring.Delete(keyringService, keyringAccount)
	if err := os.Remove(s.file()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func unmarshalCreds(data []byte) (*Credentials, error) {
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -v`
Expected: PASS (3 tests)

Note: `keyring.MockInit()` persists for the test binary's lifetime — every later test package that touches the store must call it too, or macOS tests will hit the real Keychain.

- [ ] **Step 5: Commit**

```bash
git add internal/auth
git commit -m "feat(auth): credential store with keyring and 0600 file fallback"
```

---

### Task 5: API client core (error envelope, auth header, 401→refresh→retry)

**Files:**
- Create: `internal/api/client.go`
- Test: `internal/api/client_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/api/client_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -v`
Expected: FAIL — `undefined: New`

- [ ] **Step 3: Write `internal/api/client.go`**

```go
// Package api is a typed client for the spoo.me HTTP API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spoo-me/spoo-cli/internal/auth"
)

type Client struct {
	base  string
	http  *http.Client
	store *auth.Store
}

func New(base string, store *auth.Store) *Client {
	return &Client{
		base:  strings.TrimRight(base, "/"),
		http:  &http.Client{Timeout: 30 * time.Second},
		store: store,
	}
}

// APIError mirrors the backend's error envelope {error, code, detail}.
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"error"`
	Detail  string `json:"detail"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s (%s)", e.Message, e.Detail)
	}
	return e.Message
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	creds, err := c.store.Load()
	if err != nil && !errors.Is(err, auth.ErrNotLoggedIn) {
		return err
	}
	resp, err := c.send(ctx, method, path, query, body, creds)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized && creds != nil &&
		creds.Mode == auth.ModeDevice && creds.RefreshToken != "" {
		resp.Body.Close()
		if creds, err = c.refreshTokens(ctx, creds); err != nil {
			return err
		}
		if resp, err = c.send(ctx, method, path, query, body, creds); err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	return decode(resp, out)
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body any, creds *auth.Credentials) (*http.Response, error) {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "spoo-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if creds != nil {
		switch creds.Mode {
		case auth.ModeAPIKey:
			req.Header.Set("Authorization", "Bearer "+creds.APIKey)
		case auth.ModeDevice:
			req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		}
	}
	return c.http.Do(req)
}

// refreshTokens exchanges the refresh token for a new pair and persists it.
// The backend rotates refresh tokens, so the stored pair must be replaced.
func (c *Client) refreshTokens(ctx context.Context, creds *auth.Credentials) (*auth.Credentials, error) {
	resp, err := c.send(ctx, http.MethodPost, "/auth/device/refresh", nil,
		map[string]string{"refresh_token": creds.RefreshToken}, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := decode(resp, &out); err != nil {
		return nil, fmt.Errorf("session expired — run `spoo auth login` again: %w", err)
	}
	updated := *creds
	updated.AccessToken = out.AccessToken
	updated.RefreshToken = out.RefreshToken
	if err := c.store.Save(updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func decode(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		apiErr := &APIError{Status: resp.StatusCode, Message: resp.Status}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = json.Unmarshal(data, apiErr)
		return apiErr
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/api
git commit -m "feat(api): base client with error envelope and transparent token refresh"
```

---

### Task 6: Typed endpoints — Shorten, CheckAlias, ExchangeDeviceCode, Me

**Files:**
- Create: `internal/api/shorten.go`, `internal/api/auth.go`
- Test: `internal/api/shorten_test.go`, `internal/api/auth_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/api/shorten_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShorten(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/shorten" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["long_url"] != "https://example.com" || req["alias"] != "mylink" {
			t.Errorf("unexpected body: %v", req)
		}
		if _, ok := req["password"]; ok {
			t.Error("empty optional fields must be omitted")
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"x","short_url":"https://spoo.me/mylink","alias":"mylink","long_url":"https://example.com","status":"ACTIVE"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.Shorten(context.Background(), ShortenRequest{LongURL: "https://example.com", Alias: "mylink"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ShortURL != "https://spoo.me/mylink" {
		t.Fatalf("ShortURL = %q", res.ShortURL)
	}
}

func TestCheckAlias(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/shorten/check-alias" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("alias") != "taken1" {
			t.Errorf("alias = %q", r.URL.Query().Get("alias"))
		}
		w.Write([]byte(`{"available":false,"reason":"taken"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.CheckAlias(context.Background(), "taken1", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Available || res.Reason != "taken" {
		t.Fatalf("unexpected: %+v", res)
	}
}
```

`internal/api/auth_test.go`:

```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/device/token" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"access_token":"at","refresh_token":"rt","user":{"id":"1","email":"a@b.c","email_verified":true,"name":"A","plan":"free"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	tok, err := c.ExchangeDeviceCode(context.Background(), "onetimecode")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at" || tok.User.Email != "a@b.c" {
		t.Fatalf("unexpected: %+v", tok)
	}
}

func TestMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"user":{"id":"1","email":"a@b.c","email_verified":true,"name":"A","plan":"free"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	u, err := c.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@b.c" || !u.EmailVerified {
		t.Fatalf("unexpected user: %+v", u)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -v`
Expected: FAIL — `undefined: ShortenRequest` etc.

- [ ] **Step 3: Write `internal/api/shorten.go`**

```go
package api

import (
	"context"
	"net/http"
	"net/url"
)

type ShortenRequest struct {
	LongURL      string `json:"long_url"`
	Alias        string `json:"alias,omitempty"`
	Password     string `json:"password,omitempty"`
	BlockBots    bool   `json:"block_bots,omitempty"`
	MaxClicks    int    `json:"max_clicks,omitempty"`
	ExpireAfter  string `json:"expire_after,omitempty"` // ISO 8601 or epoch seconds
	PrivateStats bool   `json:"private_stats,omitempty"`
	Domain       string `json:"domain,omitempty"`
}

type ShortURL struct {
	ID        string `json:"id"`
	ShortURL  string `json:"short_url"`
	Alias     string `json:"alias"`
	LongURL   string `json:"long_url"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status"`
	Domain    string `json:"domain"`
}

func (c *Client) Shorten(ctx context.Context, req ShortenRequest) (*ShortURL, error) {
	var out ShortURL
	if err := c.do(ctx, http.MethodPost, "/api/v1/shorten", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type AliasCheck struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

func (c *Client) CheckAlias(ctx context.Context, alias, domain string) (*AliasCheck, error) {
	q := url.Values{"alias": {alias}}
	if domain != "" {
		q.Set("domain", domain)
	}
	var out AliasCheck
	if err := c.do(ctx, http.MethodGet, "/api/v1/shorten/check-alias", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
```

- [ ] **Step 4: Write `internal/api/auth.go`**

```go
package api

import (
	"context"
	"net/http"
)

type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Plan          string `json:"plan"`
}

type DeviceTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

// ExchangeDeviceCode trades a one-time device-auth code for a JWT pair.
// The code is the credential — no prior auth is required.
func (c *Client) ExchangeDeviceCode(ctx context.Context, code string) (*DeviceTokens, error) {
	var out DeviceTokens
	if err := c.do(ctx, http.MethodPost, "/auth/device/token", nil, map[string]string{"code": code}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Me(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if err := c.do(ctx, http.MethodGet, "/auth/me", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.User, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/api/ -v`
Expected: PASS (7 tests total in package)

- [ ] **Step 6: Commit**

```bash
git add internal/api
git commit -m "feat(api): typed shorten, check-alias, device token, and me endpoints"
```

---

### Task 7: Device-auth loopback flow

**Files:**
- Create: `internal/auth/device.go`
- Test: `internal/auth/device_test.go`

Flow: listen on `127.0.0.1:53682` (fixed — backend allowlists this exact URI), open the browser to `/auth/device/login`, receive `code`+`state` on the callback, validate `state`, hand the code back to the caller. The caller (the `auth login` command) exchanges it via `Client.ExchangeDeviceCode`.

- [ ] **Step 1: Write the failing test**

`internal/auth/device_test.go`:

```go
package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Simulates the browser leg: the flow opens a URL; we parse state and
// redirect_uri out of it and hit the loopback callback like spoo.me would.
func TestDeviceFlowReturnsCode(t *testing.T) {
	flow := &DeviceFlow{
		APIBase: "https://spoo.example",
		Out:     io.Discard,
		OpenBrowser: func(authURL string) error {
			go func() {
				u, err := url.Parse(authURL)
				if err != nil {
					t.Error(err)
					return
				}
				q := u.Query()
				if q.Get("app_id") != "spoo-cli" {
					t.Errorf("app_id = %q", q.Get("app_id"))
				}
				cb := q.Get("redirect_uri")
				if !strings.HasPrefix(cb, "http://127.0.0.1:53682/callback") {
					t.Errorf("redirect_uri = %q", cb)
				}
				time.Sleep(50 * time.Millisecond) // let the server start
				resp, err := http.Get(fmt.Sprintf("%s?code=thecode&state=%s", cb, q.Get("state")))
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
			}()
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	code, err := flow.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if code != "thecode" {
		t.Fatalf("code = %q, want thecode", code)
	}
}

func TestDeviceFlowRejectsStateMismatch(t *testing.T) {
	flow := &DeviceFlow{
		APIBase: "https://spoo.example",
		Out:     io.Discard,
		OpenBrowser: func(authURL string) error {
			go func() {
				time.Sleep(50 * time.Millisecond)
				resp, err := http.Get("http://127.0.0.1:53682/callback?code=evil&state=wrong")
				if err == nil {
					resp.Body.Close()
				}
			}()
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := flow.Run(ctx); err == nil {
		t.Fatal("expected state-mismatch error, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run TestDeviceFlow -v`
Expected: FAIL — `undefined: DeviceFlow`

- [ ] **Step 3: Write `internal/auth/device.go`**

```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
)

const (
	// AppID matches the spoo-cli entry in the backend's config/apps.yaml.
	AppID = "spoo-cli"
	// CallbackAddr is fixed: the backend validates redirect_uri by exact
	// match against the registry, so a random port would be rejected.
	CallbackAddr = "127.0.0.1:53682"
	CallbackPath = "/callback"
)

const successHTML = `<!doctype html><html><body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;background:#0f0f14;color:#e5e5ea">
<div style="text-align:center"><h1 style="color:#a78bfa">✓ spoo CLI authorized</h1><p>You can close this tab and return to your terminal.</p></div>
</body></html>`

// DeviceFlow drives the spoo.me connected-apps device authorization:
// loopback server → browser consent → one-time code.
type DeviceFlow struct {
	APIBase     string
	OpenBrowser func(url string) error
	Out         io.Writer // progress messages (stderr)
}

// Run blocks until the consent callback delivers a code, the context
// expires, or the callback is invalid. Returns the one-time code.
func (f *DeviceFlow) Run(ctx context.Context) (string, error) {
	state, err := randomState()
	if err != nil {
		return "", err
	}

	ln, err := net.Listen("tcp", CallbackAddr)
	if err != nil {
		return "", fmt.Errorf("cannot listen on %s (is another spoo login running?): %w", CallbackAddr, err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != CallbackPath {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("authorization failed: state mismatch — please try again")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("authorization failed: callback carried no code")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, successHTML)
		codeCh <- code
	})}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	authURL := fmt.Sprintf("%s/auth/device/login?app_id=%s&redirect_uri=%s&state=%s",
		f.APIBase, AppID,
		url.QueryEscape("http://"+CallbackAddr+CallbackPath), state)

	fmt.Fprintln(f.Out, "Opening your browser to authorize spoo CLI…")
	fmt.Fprintf(f.Out, "If it doesn't open automatically, visit:\n\n  %s\n\n", authURL)
	if err := f.OpenBrowser(authURL); err != nil {
		fmt.Fprintf(f.Out, "(could not open browser: %v)\n", err)
	}

	select {
	case code := <-codeCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("login timed out: %w", ctx.Err())
	}
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -v`
Expected: PASS (5 tests in package). The two device tests bind the real port 53682, so they cannot run in parallel — they don't use `t.Parallel()`.

- [ ] **Step 5: Commit**

```bash
git add internal/auth
git commit -m "feat(auth): loopback device-auth flow with CSRF state check"
```

---

### Task 8: Commands — auth login/logout/status, whoami

**Files:**
- Create: `internal/cmd/auth.go`, `internal/cmd/whoami.go`
- Modify: `internal/cmd/root.go`

- [ ] **Step 1: Add the dependency factory to `internal/cmd/root.go`**

Replace the file contents with:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/config"
)

// deps bundles everything a command needs. Factory is a package var so
// command tests can swap in a client pointed at httptest.
type deps struct {
	client *api.Client
	store  *auth.Store
	cfg    config.Config
}

var newDeps = func() (*deps, error) {
	cfg := config.Load()
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	store := auth.NewStore(dir)
	return &deps{client: api.New(cfg.APIBase, store), store: store, cfg: cfg}, nil
}

// NewRootCmd builds the spoo root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spoo",
		Short:         "Shorten links and manage your spoo.me account from the terminal",
		Long:          "spoo is the official command-line client for spoo.me —\nshorten links, browse analytics, and manage your account without leaving the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "output machine-readable JSON")
	root.AddCommand(newAuthCmd(), newWhoamiCmd(), newShortenCmd())
	return root
}
```

(`newShortenCmd` arrives in Task 9; to keep this task compiling, add it in Step 2 as a stub and replace it in Task 9 — or implement Tasks 8 and 9 back-to-back before running. Preferred: add the stub.)

Stub to include now in `internal/cmd/shorten.go`:

```go
package cmd

import "github.com/spf13/cobra"

func newShortenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shorten [url]",
		Short: "Create a short link",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
```

- [ ] **Step 2: Write `internal/cmd/auth.go`**

```go
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate spoo with your spoo.me account",
	}
	cmd.AddCommand(newLoginCmd(), newLogoutCmd(), newStatusCmd())
	return cmd
}

func newLoginCmd() *cobra.Command {
	var withToken bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in via your browser (or --with-token for an API key)",
		Long: `Log in to spoo.me.

By default this opens your browser for a one-click authorization using
spoo.me's connected-apps device flow. The CLI never sees your password.

For headless environments (CI, servers), create an API key on
https://spoo.me/dashboard/keys and pipe it in:

  echo $SPOO_TOKEN | spoo auth login --with-token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if withToken {
				return loginWithToken(cmd, d)
			}
			return loginWithBrowser(cmd, d)
		},
	}
	cmd.Flags().BoolVar(&withToken, "with-token", false, "read a spoo_ API key from stdin")
	return cmd
}

func loginWithBrowser(cmd *cobra.Command, d *deps) error {
	flow := &auth.DeviceFlow{
		APIBase:     d.cfg.APIBase,
		OpenBrowser: browser.OpenURL,
		Out:         cmd.ErrOrStderr(),
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	code, err := flow.Run(ctx)
	if err != nil {
		return err
	}
	tokens, err := d.client.ExchangeDeviceCode(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	if err := d.store.Save(auth.Credentials{
		Mode:         auth.ModeDevice,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), ui.OK.Render("✓ Logged in as ")+ui.Title.Render(tokens.User.Email))
	return nil
}

func loginWithToken(cmd *cobra.Command, d *deps) error {
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return fmt.Errorf("no token on stdin — usage: echo $SPOO_TOKEN | spoo auth login --with-token")
	}
	token := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(token, "spoo_") {
		return fmt.Errorf("that doesn't look like a spoo API key (expected spoo_ prefix)")
	}
	if err := d.store.Save(auth.Credentials{Mode: auth.ModeAPIKey, APIKey: token}); err != nil {
		return err
	}
	// validate immediately so a bad key fails loudly now, not later
	user, err := d.client.Me(cmd.Context())
	if err != nil {
		_ = d.store.Clear()
		return fmt.Errorf("API key rejected: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), ui.OK.Render("✓ Logged in as ")+ui.Title.Render(user.Email))
	return nil
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if err := d.store.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.Dim.Render("Logged out."))
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhoami(cmd)
		},
	}
}
```

- [ ] **Step 3: Write `internal/cmd/whoami.go`**

```go
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the logged-in account",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhoami(cmd)
		},
	}
}

func runWhoami(cmd *cobra.Command) error {
	d, err := newDeps()
	if err != nil {
		return err
	}
	if _, err := d.store.Load(); errors.Is(err, auth.ErrNotLoggedIn) {
		fmt.Fprintln(cmd.OutOrStdout(), ui.Dim.Render("Not logged in. Run `spoo auth login` — anonymous shortening still works."))
		return nil
	}
	user, err := d.client.Me(cmd.Context())
	if err != nil {
		return err
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(user)
	}
	verified := ui.OK.Render("verified")
	if !user.EmailVerified {
		verified = ui.Err.Render("unverified")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n%s\n",
		ui.Title.Render(user.Email), verified, ui.Dim.Render("plan: "+user.Plan))
	return nil
}
```

- [ ] **Step 4: Build and smoke-test against nothing**

```bash
go build ./... && go run ./cmd/spoo auth --help && go run ./cmd/spoo whoami
```

Expected: auth help shows login/logout/status; `whoami` prints the dim "Not logged in" line (assuming no stored credentials) and exits 0.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd
git commit -m "feat(cmd): auth login/logout/status and whoami commands"
```

---

### Task 9: The shorten command (flags, piped stdin, pretty/plain/JSON output)

**Files:**
- Create: `internal/cmd/expiry.go`, `internal/cmd/expiry_test.go`
- Modify: `internal/cmd/shorten.go` (replace Task 8's stub)
- Test: `internal/cmd/shorten_test.go`

Behavior matrix:
- `spoo shorten https://x.com --alias hi` → one link, pretty box if stdout is a TTY, bare short URL otherwise (pipe-friendly), full JSON with `--json`.
- `cat urls.txt | spoo shorten` → one link per non-empty input line, bare short URL per line (JSON array with `--json`).
- `spoo shorten` on a TTY with no args → interactive form (Task 10).
- `--expires` accepts ISO 8601, epoch seconds, or a Go duration (`30m`, `72h`) converted to epoch.

- [ ] **Step 1: Write the failing expiry tests**

`internal/cmd/expiry_test.go`:

```go
package cmd

import (
	"strconv"
	"testing"
	"time"
)

func TestParseExpiryPassthroughISO(t *testing.T) {
	got, err := parseExpiry("2027-01-02T15:04:05Z", time.Now())
	if err != nil || got != "2027-01-02T15:04:05Z" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestParseExpiryDuration(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	got, err := parseExpiry("72h", now)
	if err != nil {
		t.Fatal(err)
	}
	want := strconv.FormatInt(now.Add(72*time.Hour).Unix(), 10)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseExpiryRejectsNegativeDuration(t *testing.T) {
	if _, err := parseExpiry("-5m", time.Now()); err == nil {
		t.Fatal("want error for negative duration")
	}
}

func TestParseExpiryEmpty(t *testing.T) {
	got, err := parseExpiry("", time.Now())
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmd/ -run TestParseExpiry -v`
Expected: FAIL — `undefined: parseExpiry`

- [ ] **Step 3: Write `internal/cmd/expiry.go`**

```go
package cmd

import (
	"fmt"
	"strconv"
	"time"
)

// parseExpiry normalizes --expires input. Durations ("30m", "72h")
// become epoch seconds relative to now; anything else passes through
// for the backend to validate (ISO 8601 or epoch).
func parseExpiry(raw string, now time.Time) (string, error) {
	if raw == "" {
		return "", nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if d <= 0 {
			return "", fmt.Errorf("--expires duration must be positive, got %q", raw)
		}
		return strconv.FormatInt(now.Add(d).Unix(), 10), nil
	}
	return raw, nil
}
```

- [ ] **Step 4: Run expiry tests to verify they pass**

Run: `go test ./internal/cmd/ -run TestParseExpiry -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Write the failing command test**

`internal/cmd/shorten_test.go`:

```go
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
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/cmd/ -run TestShortenCommand -v`
Expected: FAIL (stub command ignores args / lacks flags)

- [ ] **Step 7: Replace `internal/cmd/shorten.go` with the real implementation**

```go
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newShortenCmd() *cobra.Command {
	var (
		req     api.ShortenRequest
		expires string
	)
	cmd := &cobra.Command{
		Use:   "shorten [url]",
		Short: "Create a short link",
		Long: `Create a short link.

With a URL argument, shortens it directly. With input piped on stdin,
shortens every non-empty line (one short URL per line out). With no
argument on a terminal, opens an interactive form.`,
		Example: `  spoo shorten https://example.com/very/long/path
  spoo shorten https://example.com --alias launch --expires 72h
  cat urls.txt | spoo shorten
  spoo shorten`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if req.ExpireAfter, err = parseExpiry(expires, time.Now()); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")

			switch {
			case len(args) == 1:
				req.LongURL = args[0]
				return shortenOne(cmd, d, req, asJSON)
			case !stdinIsTerminal(cmd):
				return shortenLines(cmd, d, req, asJSON)
			default:
				if err := runShortenForm(cmd.Context(), d.client, &req); err != nil {
					return err
				}
				return shortenOne(cmd, d, req, asJSON)
			}
		},
	}
	cmd.Flags().StringVarP(&req.Alias, "alias", "a", "", "custom alias (3-16 chars, a-z 0-9 - _)")
	cmd.Flags().StringVarP(&req.Password, "password", "p", "", "password-protect the link")
	cmd.Flags().IntVar(&req.MaxClicks, "max-clicks", 0, "auto-expire after N clicks")
	cmd.Flags().StringVar(&expires, "expires", "", "expiry: ISO 8601, epoch seconds, or duration like 72h")
	cmd.Flags().BoolVar(&req.BlockBots, "block-bots", false, "block known bot user agents")
	cmd.Flags().BoolVar(&req.PrivateStats, "private-stats", false, "make stats private (requires login)")
	cmd.Flags().StringVar(&req.Domain, "domain", "", "use one of your custom domains")
	return cmd
}

func stdinIsTerminal(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func stdoutIsTerminal(cmd *cobra.Command) bool {
	f, ok := cmd.OutOrStdout().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func shortenOne(cmd *cobra.Command, d *deps, req api.ShortenRequest, asJSON bool) error {
	res, err := d.client.Shorten(cmd.Context(), req)
	if err != nil {
		return err
	}
	return printShortURL(cmd, res, asJSON)
}

// shortenLines shortens each non-empty stdin line with the same flag
// options. Sequential on purpose: authed accounts get 60 req/min.
func shortenLines(cmd *cobra.Command, d *deps, base api.ShortenRequest, asJSON bool) error {
	var results []*api.ShortURL
	scanner := bufio.NewScanner(cmd.InOrStdin())
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		req := base
		req.LongURL = line
		req.Alias = "" // aliases are unique; never reuse one across bulk input
		res, err := d.client.Shorten(cmd.Context(), req)
		if err != nil {
			return fmt.Errorf("shortening %q: %w", line, err)
		}
		if asJSON {
			results = append(results, res)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), res.ShortURL)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	return nil
}

func printShortURL(cmd *cobra.Command, res *api.ShortURL, asJSON bool) error {
	out := cmd.OutOrStdout()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if !stdoutIsTerminal(cmd) {
		_, err := io.WriteString(out, res.ShortURL+"\n")
		return err
	}
	body := ui.OK.Render("✓ Link created") + "\n\n" +
		ui.Title.Render(res.ShortURL) + "\n" +
		ui.Dim.Render("→ "+truncate(res.LongURL, 60))
	fmt.Fprintln(out, ui.Box.Render(body))
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

(`runShortenForm` is implemented in Task 10; add a temporary version in this step so the package compiles:)

```go
// in shorten_form.go — replaced by Task 10
package cmd

import (
	"context"
	"errors"

	"github.com/spoo-me/spoo-cli/internal/api"
)

func runShortenForm(ctx context.Context, client *api.Client, req *api.ShortenRequest) error {
	return errors.New("interactive mode not yet implemented — pass a URL: spoo shorten <url>")
}
```

- [ ] **Step 8: Run all command tests**

Run: `go test ./internal/cmd/ -v`
Expected: PASS (7 tests: 4 expiry + 3 shorten)

- [ ] **Step 9: Commit**

```bash
git add internal/cmd
git commit -m "feat(cmd): shorten command with flags, piped bulk mode, and tty-aware output"
```

---

### Task 10: Interactive shorten form (huh) with live alias check

**Files:**
- Modify: `internal/cmd/shorten_form.go` (replace the Task 9 stub)

Interactive TUI paths are exercised manually; the validation logic delegates to `Client.CheckAlias`, already unit-tested in Task 6.

- [ ] **Step 1: Replace `internal/cmd/shorten_form.go`**

```go
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	huh "charm.land/huh/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
)

var aliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,16}$`)

// runShortenForm collects shorten options interactively. The alias field
// validates against the live check-alias endpoint (rate limit 180/min —
// the backend sizes it for interactive use).
func runShortenForm(ctx context.Context, client *api.Client, req *api.ShortenRequest) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Long URL").
				Placeholder("https://example.com/very/long/path").
				Value(&req.LongURL).
				Validate(func(s string) error {
					u, err := url.Parse(s)
					if err != nil || u.Scheme == "" || u.Host == "" {
						return errors.New("enter a full URL including https://")
					}
					return nil
				}),
			huh.NewInput().
				Title("Custom alias").
				Description("optional — leave empty for a random one").
				Value(&req.Alias).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if !aliasRe.MatchString(s) {
						return errors.New("3-16 chars: letters, numbers, - and _")
					}
					check, err := client.CheckAlias(ctx, s, req.Domain)
					if err != nil {
						return nil // network hiccup: let the server be the judge on submit
					}
					if !check.Available {
						return fmt.Errorf("not available (%s)", check.Reason)
					}
					return nil
				}),
			huh.NewInput().
				Title("Password").
				Description("optional — viewers must enter it before redirecting").
				EchoMode(huh.EchoModePassword).
				Value(&req.Password),
			huh.NewConfirm().
				Title("Block bots?").
				Value(&req.BlockBots),
		),
	)
	return form.Run()
}
```

- [ ] **Step 2: Build and manually verify the form**

```bash
go build -o spoo ./cmd/spoo && ./spoo shorten
```

Expected: form renders with four fields; Esc/Ctrl-C aborts cleanly with a non-zero exit; submitting with an empty URL shows the inline validation error. (Full round-trip needs a backend — verify against a local spoo-latest with `SPOO_API_URL=http://localhost:8000 ./spoo shorten`.)

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: all packages PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cmd
git commit -m "feat(cmd): interactive shorten form with live alias availability"
```

---

### Task 11: CI, lint, and release tooling

**Files:**
- Create: `.golangci.yml`, `.goreleaser.yaml`, `.github/workflows/ci.yml`, `README.md`

- [ ] **Step 1: Write `.golangci.yml`**

```yaml
version: "2"
linters:
  default: standard
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - misspell
```

- [ ] **Step 2: Write `.goreleaser.yaml`**

```yaml
version: 2
project_name: spoo
builds:
  - id: spoo
    main: ./cmd/spoo
    binary: spoo
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.ShortCommit}}
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
checksum:
  name_template: checksums.txt
changelog:
  use: github-native
```

- [ ] **Step 3: Write `.github/workflows/ci.yml`**

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - run: go build ./...
      - run: go test ./... -race -coverprofile=coverage.out
      - uses: golangci/golangci-lint-action@v8
        with:
          version: latest
```

- [ ] **Step 4: Write `README.md`**

```markdown
# spoo

The official command-line client for [spoo.me](https://spoo.me) — shorten
links, browse analytics, and manage your account without leaving the terminal.

## Install

    go install github.com/spoo-me/spoo-cli/cmd/spoo@latest

## Quick start

    spoo shorten https://example.com/very/long/path   # works without an account
    spoo auth login                                   # browser one-click authorization
    spoo shorten                                      # interactive mode
    cat urls.txt | spoo shorten                       # bulk
    spoo whoami

`--json` on any command emits machine-readable output. Set `SPOO_API_URL`
to target a self-hosted instance.

## Auth

`spoo auth login` uses spoo.me's connected-apps device flow: your browser
opens, you approve once, and the CLI receives scoped tokens — it never
sees your password. Manage or revoke access any time at
https://spoo.me/dashboard/apps.

Headless? `echo $SPOO_TOKEN | spoo auth login --with-token` with an API
key from https://spoo.me/dashboard/keys.
```

- [ ] **Step 5: Verify lint passes locally (if golangci-lint installed) and tests still green**

```bash
go vet ./... && go test ./...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add .golangci.yml .goreleaser.yaml .github README.md
git commit -m "chore: CI, lint config, goreleaser, and README"
```

---

## Self-Review Checklist (run after all tasks)

1. **Spec coverage:** device-auth login ✓ (Tasks 7-8), API-key login ✓ (Task 8), credential storage ✓ (Task 4), transparent refresh ✓ (Task 5), shorten with all backend options ✓ (Task 9), interactive form + live alias check ✓ (Task 10), JSON/scriptable output ✓ (Task 9), connected-apps registry change ✓ (Task 0), release tooling ✓ (Task 11).
2. **Out of scope, deliberately:** links/stats/domains/keys commands, Bubble Tea TUI views, shell completion docs (fang provides `spoo completion` automatically), manual-paste login fallback (add when a user reports a broken loopback).
3. **Manual end-to-end test before release:** run spoo-latest locally with the Task 0 registry change, then `SPOO_API_URL=http://localhost:8000 ./spoo auth login` and `./spoo shorten https://example.com`.
