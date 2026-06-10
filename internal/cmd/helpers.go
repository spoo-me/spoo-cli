package cmd

import (
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"
)

// timeNow is a seam for tests that need deterministic expiry math.
var timeNow = time.Now

func normalizeStatus(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// prettyOut wraps stdout in a color-profile writer: full color on
// capable terminals, downsampled on basic ones, stripped when piped
// or when NO_COLOR is set. All styled output must go through this.
func prettyOut(cmd *cobra.Command) io.Writer {
	return colorprofile.NewWriter(cmd.OutOrStdout(), nil)
}
