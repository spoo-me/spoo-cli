package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/tui"
)

func newStatsCmd() *cobra.Command {
	var from, to, tz string
	var plain bool
	cmd := &cobra.Command{
		Use:   "stats [short-code]",
		Short: "Interactive analytics dashboard",
		Long: `Interactive analytics dashboard.

On a terminal this opens a live dashboard: time chart, browser/OS/
country/city/referrer panels, drill-down filtering, range cycling, and
a clicks/unique toggle. Piped, with --json, with --plain, or with a
custom --from/--to range it prints a static report instead.

Logged in without a short code, aggregates across all your links.
With a short code, shows that link — public stats work without login.`,
		Example: `  spoo stats             # dashboard for all your links
  spoo stats launch      # dashboard for one link
  spoo stats --plain     # static report
  spoo stats launch --from 2026-01-01 --to 2026-02-01
  spoo stats launch --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			var target string
			if len(args) == 1 {
				target = args[0]
			}
			scope := "all"
			if _, err := d.store.Load(); errors.Is(err, auth.ErrNotLoggedIn) {
				if target == "" {
					return fmt.Errorf("not logged in — pass a short code for public stats, or run `spoo auth login`")
				}
				scope = "anon"
			}
			asJSON, _ := cmd.Flags().GetBool("json")

			customRange := from != "" || to != ""
			if !asJSON && !plain && !customRange && stdoutIsTerminal(cmd) {
				model := tui.NewStats(d.client, target, scope, tz)
				final, err := tea.NewProgram(model).Run()
				if err != nil {
					return err
				}
				if m, ok := final.(tui.StatsModel); ok && m.FetchErr() != nil {
					return m.FetchErr()
				}
				return nil
			}

			// static path: the API's implicit default is only 7 days;
			// use the widest window unless the user narrows it
			if from == "" && to == "" {
				from = timeNow().UTC().AddDate(0, 0, -api.MaxRangeDays).Format(time.RFC3339)
			}
			res, err := d.client.Stats(cmd.Context(), api.StatsQuery{
				Scope:     scope,
				ShortCode: target,
				StartDate: from,
				EndDate:   to,
				Timezone:  tz,
				GroupBy:   []string{"time", "browser", "os", "country", "referrer"},
			})
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintln(prettyOut(cmd), renderStats(res, target))
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date, ISO 8601 (static report; default: 90 days ago)")
	cmd.Flags().StringVar(&to, "to", "", "end date, ISO 8601 (static report; default: now)")
	cmd.Flags().StringVar(&tz, "tz", "", "IANA timezone for time buckets (default UTC)")
	cmd.Flags().BoolVar(&plain, "plain", false, "print the static report instead of the dashboard")
	return cmd
}
