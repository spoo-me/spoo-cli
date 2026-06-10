// Package api is a typed client for the spoo.me HTTP API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spoo-me/spoo-cli/internal/auth"
)

type Client struct {
	base  string
	http  *http.Client
	store *auth.Store
}

func New(base string, store *auth.Store) *Client {
	return &Client{
		base:  strings.TrimRight(base, "/"),
		http:  &http.Client{Timeout: 30 * time.Second},
		store: store,
	}
}

// APIError mirrors the backend's error envelope {error, code, detail}.
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"error"`
	Detail  string `json:"detail"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s (%s)", e.Message, e.Detail)
	}
	return e.Message
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	creds, err := c.store.Load()
	if err != nil && !errors.Is(err, auth.ErrNotLoggedIn) {
		return err
	}
	resp, err := c.send(ctx, method, path, query, body, creds)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized && creds != nil &&
		creds.Mode == auth.ModeDevice && creds.RefreshToken != "" {
		resp.Body.Close()
		if creds, err = c.refreshTokens(ctx, creds); err != nil {
			return err
		}
		if resp, err = c.send(ctx, method, path, query, body, creds); err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	return decode(resp, out)
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body any, creds *auth.Credentials) (*http.Response, error) {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "spoo-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if creds != nil {
		switch creds.Mode {
		case auth.ModeAPIKey:
			req.Header.Set("Authorization", "Bearer "+creds.APIKey)
		case auth.ModeDevice:
			req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		}
	}
	return c.http.Do(req)
}

// refreshTokens exchanges the refresh token for a new pair and persists it.
// The backend rotates refresh tokens, so the stored pair must be replaced.
func (c *Client) refreshTokens(ctx context.Context, creds *auth.Credentials) (*auth.Credentials, error) {
	resp, err := c.send(ctx, http.MethodPost, "/auth/device/refresh", nil,
		map[string]string{"refresh_token": creds.RefreshToken}, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := decode(resp, &out); err != nil {
		return nil, fmt.Errorf("session expired — run `spoo auth login` again: %w", err)
	}
	updated := *creds
	updated.AccessToken = out.AccessToken
	updated.RefreshToken = out.RefreshToken
	if err := c.store.Save(updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func decode(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		apiErr := &APIError{Status: resp.StatusCode, Message: resp.Status}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = json.Unmarshal(data, apiErr)
		return apiErr
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
