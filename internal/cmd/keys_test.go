package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKeysCreateShowsTokenOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"k1","name":"ci","scopes":["shorten:create"],"token_prefix":"abc12345","token":"spoo_secret_token"}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"keys", "create", "--name", "ci", "--scopes", "shorten:create"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "spoo_secret_token") || !strings.Contains(out.String(), "cannot be shown again") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestKeysCreateRequiresNameAndScopes(t *testing.T) {
	pointDepsAt(t, "http://unused.invalid")
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"keys", "create"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("err = %v, want --name guard", err)
	}
}

func TestKeysListTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"keys":[{"id":"k1","name":"ci","scopes":["shorten:create"],"token_prefix":"abc12345","created_at":1750000000,"revoked":false}]}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"keys"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "abc12345") || !strings.Contains(out.String(), "shorten:create") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
