package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// URLItem is a row from GET /api/v1/urls. The envelope is camelCase
// (pageSize, hasNext) but items are snake_case; expire_after is a Unix
// timestamp — see UrlListItem in the backend's schemas/dto/responses/url.py.
type URLItem struct {
	ID           string `json:"id"`
	Alias        string `json:"alias"`
	LongURL      string `json:"long_url"`
	CreatedAt    string `json:"created_at"`
	LastClick    string `json:"last_click"`
	TotalClicks  int    `json:"total_clicks"`
	Status       string `json:"status"`
	PasswordSet  bool   `json:"password_set"`
	MaxClicks    *int   `json:"max_clicks"`
	ExpireAfter  *int64 `json:"expire_after"` // Unix seconds, null when unset
	PrivateStats bool   `json:"private_stats"`
	BlockBots    bool   `json:"block_bots"`
	Domain       string `json:"domain"`
}

type URLPage struct {
	Items    []URLItem `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
	HasNext  bool      `json:"hasNext"`
}

type ListURLsOptions struct {
	Page      int
	PageSize  int
	SortBy    string // created_at | last_click | total_clicks
	SortOrder string // ascending | descending
	Search    string
	Status    string // ACTIVE | INACTIVE | BLOCKED | EXPIRED
	Domain    string
}

func (c *Client) ListURLs(ctx context.Context, opts ListURLsOptions) (*URLPage, error) {
	q := url.Values{}
	if opts.Page > 0 {
		q.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(opts.PageSize))
	}
	if opts.SortBy != "" {
		q.Set("sortBy", opts.SortBy)
	}
	if opts.SortOrder != "" {
		q.Set("sortOrder", opts.SortOrder)
	}
	if opts.Domain != "" {
		q.Set("domain", opts.Domain)
	}
	filter := map[string]any{}
	if opts.Search != "" {
		filter["search"] = opts.Search
	}
	if opts.Status != "" {
		filter["status"] = opts.Status
	}
	if len(filter) > 0 {
		data, err := json.Marshal(filter)
		if err != nil {
			return nil, err
		}
		q.Set("filter", string(data))
	}
	var out URLPage
	if err := c.do(ctx, http.MethodGet, "/api/v1/urls", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateURL patches the given fields (snake_case keys per the API:
// long_url, alias, password, max_clicks, expire_after, status, ...).
func (c *Client) UpdateURL(ctx context.Context, id string, fields map[string]any) (*ShortURL, error) {
	var out ShortURL
	if err := c.do(ctx, http.MethodPatch, "/api/v1/urls/"+id, nil, fields, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteURL(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/urls/"+id, nil, nil, nil)
}
