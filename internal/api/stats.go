package api

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// StatsQuery parameterizes the authed stats endpoints. The account
// endpoint accepts every field; the per-link endpoint rejects a
// short_code group_by (the link is already picked by the path).
type StatsQuery struct {
	StartDate string
	EndDate   string
	GroupBy   []string // time, browser, os, country, city, referrer; account-only: short_code
	Timezone  string   // IANA name
	// Filters narrows results server-side; keys are the filterable
	// dimensions (browser, os, country, city, referrer, short_code).
	Filters map[string][]string
}

func (q StatsQuery) values() url.Values {
	v := url.Values{}
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
	for _, dim := range slices.Sorted(maps.Keys(q.Filters)) {
		if vals := q.Filters[dim]; len(vals) > 0 {
			v.Set(dim, strings.Join(vals, ","))
		}
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
// The wire still carries a legacy "scope" key — tolerated, never read.
type StatsResponse struct {
	URLID           string                      `json:"url_id"` // per-link responses echo the link
	Alias           string                      `json:"alias"`
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

// Stats aggregates clicks across every link the account owns.
// Auth is required — anonymous stats live on PublicStats.
func (c *Client) Stats(ctx context.Context, q StatsQuery) (*StatsResponse, error) {
	var out StatsResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/stats", q.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// LinkStats returns stats for one owned link by its url id (resolve an
// alias with ResolveAlias first). Unknown and foreign ids both 404.
func (c *Client) LinkStats(ctx context.Context, urlID string, q StatsQuery) (*StatsResponse, error) {
	var out StatsResponse
	path := "/api/v1/stats/links/" + url.PathEscape(urlID)
	if err := c.do(ctx, http.MethodGet, path, q.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PublicStats returns anyone's per-link stats without auth. The
// endpoint takes only a date range and timezone — no group_by — and
// answers with every dimension at once; private links 404 and
// password-protected ones 401. The {generation, link, stats} envelope
// is unwrapped to the standard stats wire.
func (c *Client) PublicStats(ctx context.Context, shortCode, startDate, endDate, timezone string) (*StatsResponse, error) {
	v := url.Values{}
	if startDate != "" {
		v.Set("start_date", startDate)
	}
	if endDate != "" {
		v.Set("end_date", endDate)
	}
	if timezone != "" {
		v.Set("timezone", timezone)
	}
	var out struct {
		Stats StatsResponse `json:"stats"`
	}
	path := "/api/v1/public/stats/" + url.PathEscape(shortCode)
	if err := c.do(ctx, http.MethodGet, path, v, nil, &out); err != nil {
		return nil, err
	}
	return &out.Stats, nil
}
