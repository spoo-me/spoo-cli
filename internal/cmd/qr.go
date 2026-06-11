package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newQRCmd() *cobra.Command {
	var invert bool
	cmd := &cobra.Command{
		Use:   "qr <short-code or url>",
		Short: "Render a link as a scannable QR code",
		Long: `Render a link as a QR code right in the terminal.

A bare argument is treated as one of your short codes; anything with a
scheme is encoded as-is. The default contrast suits dark terminals —
pass --invert on light ones if your scanner struggles.`,
		Example: `  spoo qr launch
  spoo qr https://spoo.me/launch
  spoo qr launch --invert`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			if !strings.Contains(target, "://") {
				d, err := newDeps()
				if err != nil {
					return err
				}
				target = strings.TrimRight(d.cfg.APIBase, "/") + "/" + target
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, ui.QR(target, invert))
			fmt.Fprintln(out, target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&invert, "invert", false, "dark modules on light (for light terminals)")
	return cmd
}
