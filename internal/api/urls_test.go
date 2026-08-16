package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListURLsBuildsQueryAndFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("page") != "2" || q.Get("pageSize") != "50" || q.Get("sortBy") != "total_clicks" {
			t.Errorf("unexpected query: %v", q)
		}
		var filter map[string]any
		if err := json.Unmarshal([]byte(q.Get("filter")), &filter); err != nil {
			t.Errorf("filter not JSON: %v", err)
		}
		if filter["search"] != "launch" || filter["status"] != "ACTIVE" {
			t.Errorf("unexpected filter: %v", filter)
		}
		w.Write([]byte(`{"items":[{"id":"a1","alias":"launch","long_url":"https://x.com","total_clicks":42,"status":"ACTIVE","password_set":false}],"page":2,"pageSize":50,"total":51,"hasNext":false}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	page, err := c.ListURLs(context.Background(), ListURLsOptions{
		Page: 2, PageSize: 50, SortBy: "total_clicks", Search: "launch", Status: "ACTIVE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].TotalClicks != 42 || page.Items[0].LongURL != "https://x.com" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestResolveAliasUsesAPIHostAndEscapes(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`{"id":"65f0abc123","alias":"🚀","long_url":"https://x.com","status":"ACTIVE"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	u, err := c.ResolveAlias(context.Background(), "🚀")
	if err != nil {
		t.Fatal(err)
	}
	// httptest serves on 127.0.0.1, and the emoji alias must arrive
	// percent-encoded
	if gotPath != "/api/v1/urls/127.0.0.1/%F0%9F%9A%80" {
		t.Fatalf("path = %q", gotPath)
	}
	if u.ID != "65f0abc123" {
		t.Fatalf("id = %q", u.ID)
	}
}

func TestResolveAliasNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"URL not found","code":"not_found"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	_, err := c.ResolveAlias(context.Background(), "nope")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want IsNotFound", err)
	}
}

func TestUpdateURLSendsPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/urls/abc123" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "INACTIVE" {
			t.Errorf("unexpected body: %v", body)
		}
		w.Write([]byte(`{"id":"abc123","alias":"x","status":"INACTIVE","password_set":false,"updated_at":1781524800}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.UpdateURL(context.Background(), "abc123", map[string]any{"status": "INACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "INACTIVE" {
		t.Fatalf("status = %q", res.Status)
	}
}

func TestDeleteURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/urls/abc123" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"message":"deleted","id":"abc123"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	if err := c.DeleteURL(context.Background(), "abc123"); err != nil {
		t.Fatal(err)
	}
}
