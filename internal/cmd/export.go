package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newExportCmd() *cobra.Command {
	var format, output, from, to string
	cmd := &cobra.Command{
		Use:   "export [short-code]",
		Short: "Export click analytics to a file",
		Long: `Export click analytics as json, csv, xlsx, or xml.

csv arrives as a ZIP archive with one CSV per dimension; xlsx is a
workbook with one sheet per dimension.`,
		Example: `  spoo export --format xlsx
  spoo export launch --format csv -o launch-stats.zip
  spoo export launch --format json -o -   # write to stdout`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeAlias,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "json", "csv", "xlsx", "xml":
			default:
				return fmt.Errorf("--format must be one of json, csv, xlsx, xml (got %q)", format)
			}
			d, err := newDeps()
			if err != nil {
				return err
			}
			q := api.StatsQuery{StartDate: from, EndDate: to}
			if len(args) == 1 {
				q.ShortCode = args[0]
			}
			if _, err := d.store.Load(); errors.Is(err, auth.ErrNotLoggedIn) {
				if q.ShortCode == "" {
					return fmt.Errorf("not logged in — pass a short code for public stats, or run `spoo auth login`")
				}
				q.Scope = "anon"
			} else {
				q.Scope = "all"
			}
			name, data, err := d.client.Export(cmd.Context(), q, format)
			if err != nil {
				return err
			}
			if output == "-" {
				_, err := cmd.OutOrStdout().Write(data)
				return err
			}
			if output != "" {
				name = output
			}
			if err := os.WriteFile(name, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintln(prettyOut(cmd), ui.OK.Render("✓ exported ")+name+
				ui.Dim.Render(fmt.Sprintf(" (%d bytes)", len(data))))
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "json", "json, csv, xlsx, or xml")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file (default: server-suggested name; - for stdout)")
	cmd.Flags().StringVar(&from, "from", "", "start date (ISO 8601)")
	cmd.Flags().StringVar(&to, "to", "", "end date (ISO 8601)")
	fixed(cmd, "format", "json", "csv", "xlsx", "xml")
	return cmd
}
