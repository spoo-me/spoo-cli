package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type StatsQuery struct {
	ShortCode string
	Scope     string // "all" (authed, optional code) or "anon" (code required)
	StartDate string
	EndDate   string
	GroupBy   []string // time, browser, os, country, city, referrer, short_code
	Timezone  string   // IANA name
}

func (q StatsQuery) values() url.Values {
	v := url.Values{}
	if q.Scope != "" {
		v.Set("scope", q.Scope)
	}
	if q.ShortCode != "" {
		v.Set("short_code", q.ShortCode)
	}
	if q.StartDate != "" {
		v.Set("start_date", q.StartDate)
	}
	if q.EndDate != "" {
		v.Set("end_date", q.EndDate)
	}
	if len(q.GroupBy) > 0 {
		v.Set("group_by", strings.Join(q.GroupBy, ","))
	}
	if q.Timezone != "" {
		v.Set("timezone", q.Timezone)
	}
	return v
}

type StatsSummary struct {
	TotalClicks        int     `json:"total_clicks"`
	UniqueClicks       int     `json:"unique_clicks"`
	FirstClick         string  `json:"first_click"`
	LastClick          string  `json:"last_click"`
	AvgRedirectionTime float64 `json:"avg_redirection_time"`
}

type StatsTimeRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// StatsResponse keeps Metrics loosely typed: keys are dynamic
// ("clicks_by_browser", "unique_clicks_by_time", ...) and each point
// carries its dimension label under the dimension's own name.
type StatsResponse struct {
	Scope           string                      `json:"scope"`
	ShortCode       string                      `json:"short_code"`
	Summary         StatsSummary                `json:"summary"`
	TimeRange       StatsTimeRange              `json:"time_range"`
	Metrics         map[string][]map[string]any `json:"metrics"`
	ComputedMetrics map[string]float64          `json:"computed_metrics"`
	GeneratedAt     string                      `json:"generated_at"`
}

// MaxRangeDays is the widest window the stats endpoint accepts; without
// explicit dates it defaults to only the LAST 7 DAYS, so clients that
// want "all recent activity" should request this window explicitly.
const MaxRangeDays = 90

type MetricPoint struct {
	Label string
	Value float64
}

// Points extracts (label, value) pairs from the loosely typed metrics
// payload for one dimension/metric pair, e.g. ("browser", "clicks") →
// the "clicks_by_browser" series with labels from the "browser" key.
func (r *StatsResponse) Points(dimension, metric string) []MetricPoint {
	pts := r.Metrics[metric+"_by_"+dimension]
	out := make([]MetricPoint, 0, len(pts))
	for _, p := range pts {
		label, _ := p[dimension].(string)
		value, ok := p[metric].(float64)
		if label == "" || !ok {
			continue
		}
		out = append(out, MetricPoint{Label: label, Value: value})
	}
	return out
}

func (c *Client) Stats(ctx context.Context, q StatsQuery) (*StatsResponse, error) {
	var out StatsResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/stats", q.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
