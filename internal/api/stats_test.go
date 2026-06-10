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
		if q.Get("group_by") != "time,browser" || q.Get("scope") != "all" || q.Get("timezone") != "UTC" {
			t.Errorf("unexpected query: %v", q)
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
		Scope: "all", GroupBy: []string{"time", "browser"}, Timezone: "UTC",
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

func TestExportReturnsFilenameAndBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/export" || r.URL.Query().Get("format") != "xlsx" {
			t.Errorf("unexpected request: %s %v", r.URL.Path, r.URL.Query())
		}
		w.Header().Set("Content-Disposition", `attachment; filename="stats-launch.xlsx"`)
		w.Write([]byte("FAKEXLSX"))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	name, data, err := c.Export(context.Background(), StatsQuery{ShortCode: "launch"}, "xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if name != "stats-launch.xlsx" || string(data) != "FAKEXLSX" {
		t.Fatalf("name=%q data=%q", name, data)
	}
}
