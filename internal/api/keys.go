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
	Token       string   `json:"token,omitempty"` // full token, present only on create
}

type CreateKeyRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expires_at,omitempty"` // ISO 8601 or epoch seconds
}

// CreateKey mints a new API key. Requires a device-flow (JWT) session;
// the backend refuses key creation authenticated by another API key.
func (c *Client) CreateKey(ctx context.Context, req CreateKeyRequest) (*APIKey, error) {
	var out APIKey
	if err := c.do(ctx, http.MethodPost, "/api/v1/keys", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
