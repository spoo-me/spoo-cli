package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const twoLinksJSON = `{"items":[
	{"id":"a1","alias":"launch","long_url":"https://launch.example.com/x","status":"ACTIVE"},
	{"id":"b2","alias":"promo","long_url":"https://promo.example.com/y","status":"ACTIVE"}
],"page":1,"pageSize":100,"total":2,"hasNext":false}`

// complete drives cobra's hidden __complete command and returns its raw
// output (candidate lines + a trailing :directive line).
func complete(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"__complete"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("__complete %v: %v", args, err)
	}
	return out.String()
}

func TestCompleteAliasSuggestsLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/urls" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(twoLinksJSON))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	// every alias command should complete the first positional with aliases
	for _, sub := range []string{"open", "inspect", "qr", "stats", "export"} {
		out := complete(t, sub, "")
		for _, want := range []string{"launch", "promo", "https://launch.example.com/x"} {
			if !strings.Contains(out, want) {
				t.Fatalf("%s completion missing %q:\n%s", sub, want, out)
			}
		}
	}

	// the prefix narrows the candidates
	out := complete(t, "open", "la")
	if !strings.Contains(out, "launch") || strings.Contains(out, "promo") {
		t.Fatalf("prefix 'la' should yield only launch:\n%s", out)
	}

	// a satisfied positional offers nothing (and never file completion)
	if out := complete(t, "open", "launch", ""); strings.Contains(out, "promo") {
		t.Fatalf("second arg should not complete:\n%s", out)
	}
}

func TestCompleteLinkIDDescribedByAlias(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(twoLinksJSON))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	for _, sub := range []string{"delete", "update"} {
		out := complete(t, "links", sub, "")
		if !strings.Contains(out, "a1\tlaunch") || !strings.Contains(out, "b2\tpromo") {
			t.Fatalf("links %s should complete ids described by alias:\n%s", sub, out)
		}
	}
}

func TestCompleteKeyIDDescribedByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/keys" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"keys":[
			{"id":"k1","name":"ci-bot","revoked":false},
			{"id":"k2","name":"old","revoked":true}
		]}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	out := complete(t, "keys", "revoke", "")
	if !strings.Contains(out, "k1\tci-bot") {
		t.Fatalf("revoke should complete live key ids by name:\n%s", out)
	}
	if strings.Contains(out, "k2") {
		t.Fatalf("revoked keys should be omitted:\n%s", out)
	}
}

func TestCompleteFixedFlags(t *testing.T) {
	pointDepsAt(t, "http://unused.invalid")
	out := complete(t, "export", "--format", "")
	for _, want := range []string{"json", "csv", "xlsx", "xml"} {
		if !strings.Contains(out, want) {
			t.Fatalf("--format completion missing %q:\n%s", want, out)
		}
	}
	if out := complete(t, "links", "--status", ""); !strings.Contains(out, "blocked") {
		t.Fatalf("--status completion missing values:\n%s", out)
	}
}

func TestCompleteScopesCommaAware(t *testing.T) {
	pointDepsAt(t, "http://unused.invalid") // fixed list; no API call

	// bare: offers the full scope set
	out := complete(t, "keys", "create", "--scopes", "")
	for _, want := range []string{"shorten:create", "stats:read", "admin:all"} {
		if !strings.Contains(out, want) {
			t.Fatalf("--scopes missing %q:\n%s", want, out)
		}
	}
	// mid-list: keeps the typed prefix, drops the already-chosen scope,
	// and narrows by the partial after the last comma
	out = complete(t, "keys", "create", "--scopes", "shorten:create,sta")
	if !strings.Contains(out, "shorten:create,stats:read") {
		t.Fatalf("comma-aware completion should append stats:read:\n%s", out)
	}
	if strings.Contains(out, "shorten:create,shorten:create") {
		t.Fatalf("already-chosen scope should not be re-offered:\n%s", out)
	}
}

func TestCompleteDomainFromLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"items":[
			{"id":"a","alias":"x","long_url":"https://a","domain":"go.acme.io"},
			{"id":"b","alias":"y","long_url":"https://b","domain":"go.acme.io"},
			{"id":"c","alias":"z","long_url":"https://c","domain":"l.example.com"}
		],"page":1,"pageSize":100,"total":3,"hasNext":false}`))
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	out := complete(t, "shorten", "--domain", "")
	if !strings.Contains(out, "go.acme.io") || !strings.Contains(out, "l.example.com") {
		t.Fatalf("--domain should list distinct link domains:\n%s", out)
	}
	if strings.Count(out, "go.acme.io") != 1 {
		t.Fatalf("domains should be de-duplicated:\n%s", out)
	}
}

func TestCompleteIsBestEffortOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	// a failing API must not error or suggest anything — just no candidates,
	// and crucially no fallback to file completion (directive :4 = NoFileComp)
	out := complete(t, "open", "")
	if strings.Contains(out, "launch") {
		t.Fatalf("errored fetch should yield no candidates:\n%s", out)
	}
	if !strings.Contains(out, ":4") {
		t.Fatalf("expected NoFileComp directive (:4):\n%s", out)
	}
}
