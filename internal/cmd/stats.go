package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
)

func newStatsCmd() *cobra.Command {
	var from, to, tz string
	cmd := &cobra.Command{
		Use:   "stats [short-code]",
		Short: "Show click analytics",
		Long: `Show click analytics.

Logged in without a short code, aggregates across all your links.
With a short code, shows that link — public stats work without login.`,
		Example: `  spoo stats             # all your links (requires login)
  spoo stats launch      # one link
  spoo stats launch --from 2026-01-01 --tz Asia/Kolkata
  spoo stats launch --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			// the API's implicit default is only 7 days; use the widest
			// window unless the user narrows it themselves
			if from == "" && to == "" {
				from = timeNow().UTC().AddDate(0, 0, -api.MaxRangeDays).Format(time.RFC3339)
			}
			q := api.StatsQuery{
				StartDate: from,
				EndDate:   to,
				Timezone:  tz,
				GroupBy:   []string{"time", "browser", "os", "country", "referrer"},
			}
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
			res, err := d.client.Stats(cmd.Context(), q)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintln(prettyOut(cmd), renderStats(res, q.ShortCode))
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date, ISO 8601 (default: 90 days ago)")
	cmd.Flags().StringVar(&to, "to", "", "end date, ISO 8601 (default: now)")
	cmd.Flags().StringVar(&tz, "tz", "", "IANA timezone for time buckets (default UTC)")
	return cmd
}
