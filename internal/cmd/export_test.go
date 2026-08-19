package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportRequiresLogin(t *testing.T) {
	pointDepsAt(t, "http://unused.invalid")
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"export", "launch"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "requires login") {
		t.Fatalf("err = %v, want a login requirement", err)
	}
}

func TestExportAccountWide(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Query().Has("scope") {
			t.Errorf("scope param must not be sent: %v", r.URL.Query())
		}
		w.Write([]byte(`{"export":"ok"}`))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"export", "-o", "-"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/export" {
		t.Fatalf("path = %q, want /api/v1/export", gotPath)
	}
	if !strings.Contains(out.String(), `{"export":"ok"}`) {
		t.Fatalf("stdout = %q", out.String())
	}
}

// exporting one link resolves the alias first and uses the per-link
// export endpoint.
func TestExportOwnedLink(t *testing.T) {
	var paths []string
	var gotURLID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/api/v1/urls/") {
			w.Write([]byte(`{"id":"65f0abc123","alias":"launch","long_url":"https://x.com","status":"ACTIVE"}`))
			return
		}
		gotURLID = r.URL.Query().Get("url_id")
		w.Write([]byte(`{"export":"ok"}`))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"export", "launch", "-o", "-"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{"/api/v1/urls/127.0.0.1/launch", "/api/v1/export/links/65f0abc123"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	if gotURLID != "" {
		t.Fatalf("url_id = %q, want no query param on the per-link route", gotURLID)
	}
}

// anonymous export died with the scope param; a foreign code has no
// export surface at all, so the error must say so instead of retrying.
func TestExportForeignCodeErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"URL not found","code":"not_found"}`))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"export", "launch"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "links you own") {
		t.Fatalf("err = %v, want an ownership explanation", err)
	}
}

// --domain must reach the resolve path, replacing the API host.
func TestExportDomainFlagResolvesOnThatDomain(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/api/v1/urls/") {
			w.Write([]byte(`{"id":"65f0abc123","alias":"promo","long_url":"https://x.com","status":"ACTIVE"}`))
			return
		}
		w.Write([]byte(`{"export":"ok"}`))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"export", "promo", "--domain", "links.example.com", "-o", "-"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{"/api/v1/urls/links.example.com/promo", "/api/v1/export/links/65f0abc123"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestSafeFilename(t *testing.T) {
	for _, tt := range []struct {
		name, format, want string
	}{
		{"stats.json", "json", "stats.json"},
		{"../../evil.json", "json", "evil.json"},
		{"/etc/passwd", "json", "passwd"},
		{".", "json", "spoo-export.json"},
		{"..", "json", "spoo-export.json"},
		{"", "json", "spoo-export.json"},
		{"", "csv", "spoo-export.zip"},
		{"/", "xlsx", "spoo-export.xlsx"},
	} {
		if got := safeFilename(tt.name, tt.format); got != tt.want {
			t.Errorf("safeFilename(%q, %q) = %q, want %q", tt.name, tt.format, got, tt.want)
		}
	}
}

// a hostile Content-Disposition must never steer the write outside the
// working directory: the SDK strips the name and the CLI guards again
// before os.Create.
func TestExportServerFilenameStaysInWorkingDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="../../evil.json"`)
		w.Write([]byte(`{"export":"ok"}`))
	}))
	defer srv.Close()
	pointDepsAtLoggedIn(t, srv.URL)
	dir := t.TempDir()
	t.Chdir(dir)

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"export"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one exported file in cwd, got %v", entries)
	}
	name := entries[0].Name()
	if name != filepath.Base(name) || strings.Contains(name, "..") {
		t.Fatalf("exported name %q escaped sanitization", name)
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "..", "evil.json")); err == nil {
		t.Fatal("export escaped the working directory")
	}
}
