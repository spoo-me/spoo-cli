package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	spoo "github.com/spoo-me/spoo-go"
)

func TestLinksListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/urls" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"items":[{"id":"a1","alias":"launch","long_url":"https://x.com","total_clicks":42,"status":"ACTIVE","created_at":"2026-06-01T00:00:00Z"}],"page":1,"pageSize":20,"total":1,"hasNext":false}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"links", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var page spoo.URLPage
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if len(page.Items) != 1 || page.Items[0].Alias != "launch" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestLinksListPlainTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"id":"a1","alias":"launch","long_url":"https://x.com","total_clicks":42,"status":"ACTIVE","created_at":"2026-06-01T00:00:00Z"}],"page":1,"pageSize":20,"total":1,"hasNext":false}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out) // bytes.Buffer is not a TTY → plain table path
	root.SetErr(&out)
	root.SetArgs([]string{"links"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "launch") || !strings.Contains(out.String(), "ALIAS") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestLinksDeleteRequiresYes(t *testing.T) {
	pointDepsAt(t, "http://unused.invalid")
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"links", "delete", "a1"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("err = %v, want --yes guard", err)
	}
}

func TestLinksUpdateStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/urls/a1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "INACTIVE" {
			t.Errorf("body = %v", body)
		}
		w.Write([]byte(`{"id":"a1","alias":"x","status":"INACTIVE","password_set":false,"updated_at":1781524800}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"links", "update", "a1", "--status", "inactive"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

// an explicitly empty --alias or --long-url used to serialize to an
// empty PATCH body and report success having changed nothing.
func TestLinksUpdateRejectsEmptyValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request should be sent, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	for flag, wantErr := range map[string]string{
		"--alias":    "--alias cannot be empty",
		"--long-url": "--long-url cannot be empty",
	} {
		root := NewRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		root.SetArgs([]string{"links", "update", "a1", flag, ""})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("%s \"\": err = %v, want %q", flag, err, wantErr)
		}
	}
}
