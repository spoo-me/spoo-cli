package cmd

import (
	"context"
	"net/http"
	"time"
)

// InspectResult is what a HEAD probe of a short link revealed.
type InspectResult struct {
	ShortURL    string `json:"short_url"`
	Status      int    `json:"status"`
	Destination string `json:"destination,omitempty"`
}

// inspectLink resolves where a short code points without recording a
// click: the backend skips click tracking on HEAD requests, and
// redirects are not followed so the destination never gets hit either.
// This probes the redirect edge, not the API, so it stays a plain HTTP
// call in the CLI instead of an SDK method.
func inspectLink(ctx context.Context, base, shortCode string) (*InspectResult, error) {
	noFollow := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	u := base + "/" + shortCode
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "spoo-cli")
	req.Header.Set("X-Spoo-Client", clientTag())
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
