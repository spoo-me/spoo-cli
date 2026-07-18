package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"keys":[{"id":"k1","name":"CI","scopes":["shorten:create"],"token_prefix":"abc12345","revoked":false}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	keys, err := c.ListKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].TokenPrefix != "abc12345" {
		t.Fatalf("keys = %+v", keys)
	}
}

func TestInspectDoesNotFollowRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD (no click tracking)", r.Method)
		}
		w.Header().Set("Location", "https://example.com/destination")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.Inspect(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusFound || res.Destination != "https://example.com/destination" {
		t.Fatalf("unexpected: %+v", res)
	}
}
