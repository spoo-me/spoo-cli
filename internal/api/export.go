package api

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
)

// Export downloads account-wide stats in the given format (json, csv,
// xlsx, xml). Auth is required — anonymous export no longer exists.
// Returns the server-suggested filename and the file contents.
// csv arrives as a ZIP archive (one CSV per dimension).
func (c *Client) Export(ctx context.Context, q StatsQuery, format string) (string, []byte, error) {
	return c.export(ctx, "/api/v1/export", q, format)
}

// ExportLink downloads one owned link's stats by its url id (resolve
// an alias with ResolveAlias first). Unknown and foreign ids both 404.
func (c *Client) ExportLink(ctx context.Context, urlID string, q StatsQuery, format string) (string, []byte, error) {
	return c.export(ctx, "/api/v1/export/links/"+url.PathEscape(urlID), q, format)
}

func (c *Client) export(ctx context.Context, path string, q StatsQuery, format string) (string, []byte, error) {
	v := q.values()
	v.Set("format", format)
	resp, err := c.request(ctx, http.MethodGet, path, v, nil)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", nil, decode(resp, nil)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	filename := fmt.Sprintf("spoo-export.%s", format)
	if format == "csv" {
		filename = "spoo-export.zip"
	}
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		if name := params["filename"]; name != "" {
			filename = name
		}
	}
	return filename, data, nil
}
