package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	spoo "github.com/spoo-me/spoo-go"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newExportCmd() *cobra.Command {
	var format, output, from, to, domain string
	cmd := &cobra.Command{
		Use:   "export [short-code]",
		Short: "Export click analytics to a file",
		Long: `Export click analytics as json, csv, xlsx, or xml.

Requires login. Without a short code, exports across all your links;
with one, exports that link (it must be yours).

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
			if _, err := d.store.Load(); errors.Is(err, auth.ErrNotLoggedIn) {
				return fmt.Errorf("export requires login — run `spoo auth login`")
			}
			fromT, err := parseDate(from)
			if err != nil {
				return err
			}
			toT, err := parseDate(to)
			if err != nil {
				return err
			}
			q := spoo.StatsQuery{StartDate: fromT, EndDate: toT}
			var file *spoo.ExportFile
			if len(args) == 1 {
				resolveDomain := domain
				if resolveDomain == "" {
					resolveDomain = apiHost(d.cfg.APIBase)
				}
				u, err := d.client.ResolveAlias(cmd.Context(), args[0], resolveDomain)
				if spoo.IsNotFound(err) {
					where := args[0]
					if domain != "" {
						where += " on " + domain
					}
					return fmt.Errorf("%s is not one of your links — export covers only links you own", where)
				}
				if err != nil {
					return err
				}
				if file, err = d.client.ExportLink(cmd.Context(), u.ID, q, format); err != nil {
					return err
				}
			} else if file, err = d.client.Export(cmd.Context(), q, format); err != nil {
				return err
			}
			defer file.Body.Close()

			if output == "-" {
				_, err := io.Copy(cmd.OutOrStdout(), file.Body)
				return err
			}
			name := file.Filename
			if output != "" {
				name = output
			}
			out, err := os.Create(name)
			if err != nil {
				return err
			}
			written, err := io.Copy(out, file.Body)
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(prettyOut(cmd), ui.OK.Render("✓ exported ")+name+
				ui.Dim.Render(fmt.Sprintf(" (%d bytes)", written)))
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "json", "json, csv, xlsx, or xml")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file (default: server-suggested name; - for stdout)")
	cmd.Flags().StringVar(&from, "from", "", "start date (ISO 8601)")
	cmd.Flags().StringVar(&to, "to", "", "end date (ISO 8601)")
	cmd.Flags().StringVar(&domain, "domain", "", "the link is on one of your custom domains")
	fixed(cmd, "format", "json", "csv", "xlsx", "xml")
	flagComp(cmd, "domain", completeDomain)
	return cmd
}
