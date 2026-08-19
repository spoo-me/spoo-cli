package cmd

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"
	spoo "github.com/spoo-me/spoo-go"
)

// apiHost is the hostname of the configured API base — the system
// default domain that short links live on unless --domain says otherwise.
func apiHost(base string) string {
	if u, err := url.Parse(base); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return base
}

// timeNow is a seam for tests that need deterministic expiry math.
var timeNow = time.Now

// dateLayouts are the shapes --from/--to accept: full RFC 3339 or a
// bare date.
var dateLayouts = []string{time.RFC3339, "2006-01-02"}

// parseDate reads a --from/--to flag; empty input means "not set".
func parseDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q (want ISO 8601, e.g. 2026-01-15)", raw)
}

// day renders a timestamp as YYYY-MM-DD, empty when unset.
func day(t spoo.Timestamp) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func normalizeStatus(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// prettyOut wraps stdout in a color-profile writer: full color on
// capable terminals, downsampled on basic ones, stripped when piped
// or when NO_COLOR is set. All styled output must go through this.
func prettyOut(cmd *cobra.Command) io.Writer {
	return colorprofile.NewWriter(cmd.OutOrStdout(), nil)
}
