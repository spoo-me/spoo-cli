package cmd

import (
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"
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

func normalizeStatus(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// prettyOut wraps stdout in a color-profile writer: full color on
// capable terminals, downsampled on basic ones, stripped when piped
// or when NO_COLOR is set. All styled output must go through this.
func prettyOut(cmd *cobra.Command) io.Writer {
	return colorprofile.NewWriter(cmd.OutOrStdout(), nil)
}
