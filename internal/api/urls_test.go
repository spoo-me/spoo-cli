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
		w.Write([]byte(`{"items":[{"id":"a1","alias":"launch","longUrl":"https://x.com","totalClicks":42,"status":"ACTIVE"}],"page":2,"pageSize":50,"total":51,"hasNext":false}`))
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
		w.Write([]byte(`{"id":"abc123","short_url":"https://spoo.me/x","alias":"x","status":"INACTIVE"}`))
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
