package api

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// Export downloads stats in the given format (json, csv, xlsx, xml).
// Returns the server-suggested filename and the file contents.
// csv arrives as a ZIP archive (one CSV per dimension).
func (c *Client) Export(ctx context.Context, q StatsQuery, format string) (string, []byte, error) {
	v := q.values()
	v.Set("format", format)
	resp, err := c.request(ctx, http.MethodGet, "/api/v1/export", v, nil)
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
