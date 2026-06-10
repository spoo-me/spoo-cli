package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDomainReturnsDNSRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/custom-domains" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"d1","fqdn":"links.acme.com","status":"PENDING","dns_records":[{"type":"CNAME","name":"links.acme.com","value":"spoo.me"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	d, err := c.CreateDomain(context.Background(), "links.acme.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "PENDING" || len(d.DNSRecords) != 1 || d.DNSRecords[0].Type != "CNAME" {
		t.Fatalf("unexpected domain: %+v", d)
	}
}

func TestRevokeDomainCascade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Query().Get("cascade") != "true" {
			t.Errorf("unexpected request: %s cascade=%s", r.Method, r.URL.Query().Get("cascade"))
		}
		w.Write([]byte(`{"id":"d1","fqdn":"links.acme.com","cascade":true,"urls_deleted":7}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.RevokeDomain(context.Background(), "d1", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.URLsDeleted != 7 {
		t.Fatalf("urls_deleted = %d", res.URLsDeleted)
	}
}
