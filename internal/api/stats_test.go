package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsQueryAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("group_by") != "time,browser" || q.Get("timezone") != "UTC" {
			t.Errorf("unexpected query: %v", q)
		}
		if q.Has("scope") {
			t.Errorf("scope param must not be sent: %v", q)
		}
		w.Write([]byte(`{
			"scope": "all",
			"summary": {"total_clicks": 100, "unique_clicks": 60, "avg_redirection_time": 0.12},
			"metrics": {
				"clicks_by_browser": [{"browser": "Chrome", "clicks": 70, "clicks_percentage": 70.0}],
				"clicks_by_time": [{"date": "2026-06-01", "clicks": 10}]
			},
			"computed_metrics": {"unique_click_rate": 0.6}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.Stats(context.Background(), StatsQuery{
		GroupBy: []string{"time", "browser"}, Timezone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.TotalClicks != 100 || res.Summary.UniqueClicks != 60 {
		t.Fatalf("summary = %+v", res.Summary)
	}
	points := res.Metrics["clicks_by_browser"]
	if len(points) != 1 || points[0]["browser"] != "Chrome" {
		t.Fatalf("metrics = %+v", res.Metrics)
	}
	if res.ComputedMetrics["unique_click_rate"] != 0.6 {
		t.Fatalf("computed = %+v", res.ComputedMetrics)
	}
}

func TestLinkStatsHitsPerLinkPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stats/links/65f0abc123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if q := r.URL.Query(); q.Has("scope") || q.Has("short_code") {
			t.Errorf("legacy params must not be sent: %v", q)
		}
		w.Write([]byte(`{
			"url_id": "65f0abc123", "alias": "launch", "scope": "all",
			"summary": {"total_clicks": 42, "unique_clicks": 30},
			"metrics": {}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.LinkStats(context.Background(), "65f0abc123", StatsQuery{GroupBy: []string{"time"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.URLID != "65f0abc123" || res.Alias != "launch" || res.Summary.TotalClicks != 42 {
		t.Fatalf("res = %+v", res)
	}
}

func TestPublicStatsUnwrapsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/stats/launch" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("start_date") != "2026-01-01T00:00:00Z" || q.Get("timezone") != "UTC" {
			t.Errorf("unexpected query: %v", q)
		}
		if q.Has("group_by") || q.Has("scope") {
			t.Errorf("public endpoint takes only a range and timezone: %v", q)
		}
		w.Write([]byte(`{
			"generation": "v2",
			"link": {"alias": "launch", "domain": "spoo.me"},
			"stats": {
				"scope": "anon",
				"summary": {"total_clicks": 9, "unique_clicks": 5},
				"metrics": {"clicks_by_browser": [{"browser": "Chrome", "clicks": 9}]}
			}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	res, err := c.PublicStats(context.Background(), "launch", "2026-01-01T00:00:00Z", "", "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.TotalClicks != 9 || len(res.Metrics["clicks_by_browser"]) != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestExportReturnsFilenameAndBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/export" || r.URL.Query().Get("format") != "xlsx" {
			t.Errorf("unexpected request: %s %v", r.URL.Path, r.URL.Query())
		}
		if r.URL.Query().Has("scope") {
			t.Errorf("scope param must not be sent: %v", r.URL.Query())
		}
		w.Header().Set("Content-Disposition", `attachment; filename="stats-launch.xlsx"`)
		w.Write([]byte("FAKEXLSX"))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	name, data, err := c.Export(context.Background(), StatsQuery{}, "xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if name != "stats-launch.xlsx" || string(data) != "FAKEXLSX" {
		t.Fatalf("name=%q data=%q", name, data)
	}
}

func TestExportLinkHitsPerLinkPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/export/links/65f0abc123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Disposition", `attachment; filename="stats-launch.zip"`)
		w.Write([]byte("FAKEZIP"))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	name, data, err := c.ExportLink(context.Background(), "65f0abc123", StatsQuery{}, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if name != "stats-launch.zip" || string(data) != "FAKEZIP" {
		t.Fatalf("name=%q data=%q", name, data)
	}
}
