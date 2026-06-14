package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// domainListBody is the canned ListDomains response used to resolve fqdn→id.
const domainListBody = `{"items":[{"id":"dom-1","fqdn":"links.acme.com","status":"ACTIVE","root_redirect":"https://acme.com","not_found_redirect":""}],"page":1,"pageSize":20,"total":1,"hasNext":false}`

// runDomainsConfig points deps at a server that records the PATCH body and
// runs `domains config` with the given args.
func runDomainsConfig(t *testing.T, args ...string) (patchBody map[string]any, sawPatch bool, out string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			sawPatch = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &patchBody)
			w.Write([]byte(`{"id":"dom-1","fqdn":"links.acme.com","status":"ACTIVE","root_redirect":"https://new.com","not_found_redirect":""}`))
			return
		}
		w.Write([]byte(domainListBody)) // GET /custom-domains
	}))
	defer srv.Close()
	pointDepsAt(t, srv.URL)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"domains", "config"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return patchBody, sawPatch, buf.String()
}

// Bare config shows current routing and never PATCHes.
func TestDomainsConfigShowsCurrent(t *testing.T) {
	_, sawPatch, out := runDomainsConfig(t, "links.acme.com")
	if sawPatch {
		t.Fatal("bare config must not send a PATCH")
	}
	for _, want := range []string{"links.acme.com", "root redirect", "https://acme.com", "not-found redirect", "not set"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

// A provided flag is sent; an omitted one is not (keeps current).
func TestDomainsConfigSetsOnlyProvided(t *testing.T) {
	body, sawPatch, _ := runDomainsConfig(t, "links.acme.com", "--root-redirect", "https://new.com")
	if !sawPatch {
		t.Fatal("expected a PATCH")
	}
	if body["root_redirect"] != "https://new.com" {
		t.Fatalf("root_redirect = %v, want https://new.com", body["root_redirect"])
	}
	if _, present := body["not_found_redirect"]; present {
		t.Fatal("omitted flag must not appear in the PATCH body")
	}
}

// An empty flag value clears the field (JSON null), distinct from omission.
func TestDomainsConfigEmptyClears(t *testing.T) {
	body, sawPatch, _ := runDomainsConfig(t, "links.acme.com", "--root-redirect", "")
	if !sawPatch {
		t.Fatal("expected a PATCH")
	}
	v, present := body["root_redirect"]
	if !present || v != nil {
		t.Fatalf("root_redirect = %v (present=%v), want explicit null", v, present)
	}
}
