package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type APIKey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
	CreatedAt   int64    `json:"created_at"`
	ExpiresAt   int64    `json:"expires_at"`
	Revoked     bool     `json:"revoked"`
	TokenPrefix string   `json:"token_prefix"`
}

func (c *Client) ListKeys(ctx context.Context) ([]APIKey, error) {
	var out struct {
		Keys []APIKey `json:"keys"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/keys", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Keys, nil
}

// DeleteKey removes a key. With revoke=true it is soft-revoked (kept in
// the list, unusable); with revoke=false the record is hard-deleted.
func (c *Client) DeleteKey(ctx context.Context, id string, revoke bool) error {
	q := url.Values{"revoke": {strconv.FormatBool(revoke)}}
	return c.do(ctx, http.MethodDelete, "/api/v1/keys/"+id, q, nil, nil)
}
