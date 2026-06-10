package api

import (
	"context"
	"net/http"
	"time"
)

type InspectResult struct {
	ShortURL    string `json:"short_url"`
	Status      int    `json:"status"`
	Destination string `json:"destination,omitempty"`
}

// Inspect resolves where a short code points without recording a click:
// the backend skips click tracking on HEAD requests, and redirects are
// not followed so the destination never gets hit either.
func (c *Client) Inspect(ctx context.Context, shortCode string) (*InspectResult, error) {
	noFollow := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	u := c.base + "/" + shortCode
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "spoo-cli")
	resp, err := noFollow.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return &InspectResult{
		ShortURL:    u,
		Status:      resp.StatusCode,
		Destination: resp.Header.Get("Location"),
	}, nil
}
