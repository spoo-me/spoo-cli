package api

import (
	"context"
	"net/http"
	"net/url"
)

type ShortenRequest struct {
	LongURL      string `json:"long_url"`
	Alias        string `json:"alias,omitempty"`
	Password     string `json:"password,omitempty"`
	BlockBots    bool   `json:"block_bots,omitempty"`
	MaxClicks    int    `json:"max_clicks,omitempty"`
	ExpireAfter  string `json:"expire_after,omitempty"` // ISO 8601 or epoch seconds
	PrivateStats bool   `json:"private_stats,omitempty"`
	Domain       string `json:"domain,omitempty"`
}

type ShortURL struct {
	ID        string `json:"id"`
	ShortURL  string `json:"short_url"`
	Alias     string `json:"alias"`
	LongURL   string `json:"long_url"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status"`
	Domain    string `json:"domain"`
}

func (c *Client) Shorten(ctx context.Context, req ShortenRequest) (*ShortURL, error) {
	var out ShortURL
	if err := c.do(ctx, http.MethodPost, "/api/v1/shorten", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type AliasCheck struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

func (c *Client) CheckAlias(ctx context.Context, alias, domain string) (*AliasCheck, error) {
	q := url.Values{"alias": {alias}}
	if domain != "" {
		q.Set("domain", domain)
	}
	var out AliasCheck
	if err := c.do(ctx, http.MethodGet, "/api/v1/shorten/check-alias", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
