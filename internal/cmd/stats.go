package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	spoo "github.com/spoo-me/spoo-go"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/tui/stats"
)

// resolveTarget maps a short code and login state onto a stats surface.
// Logged in with a code, the alias resolves to an owned link's url id on
// the given domain (empty means the system default — the SDK wants the
// namespace spelled out, so the default is applied here where it is
// visible policy). On a 404 with --domain set there is nowhere to fall
// back to — the public endpoint serves only default-domain links — so it
// errors instead of silently showing a different link's stats. Without
// --domain the code may still be someone else's public default-domain
// link, so it falls back to the public endpoint, announcing the switch
// on errOut.
func resolveTarget(ctx context.Context, client *spoo.Client, code, domain, defaultDomain string, loggedIn bool, errOut io.Writer) (stats.Target, error) {
	switch {
	case code == "":
		return stats.Target{Kind: stats.KindAccount}, nil
	case loggedIn:
		resolveDomain := domain
		if resolveDomain == "" {
			resolveDomain = defaultDomain
		}
		u, err := client.ResolveAlias(ctx, code, resolveDomain)
		if spoo.IsNotFound(err) {
			if domain != "" {
				return stats.Target{}, fmt.Errorf("%s on %s is not one of your links — public stats cover only %s links", code, domain, defaultDomain)
			}
			fmt.Fprintf(errOut, "note: %s isn't one of your links — showing public stats for %s/%s\n", code, defaultDomain, code)
			return stats.Target{Kind: stats.KindPublicLink, Alias: code}, nil
		}
		if err != nil {
			return stats.Target{}, err
		}
		return stats.Target{Kind: stats.KindOwnedLink, Alias: code, URLID: u.ID}, nil
	default:
		return stats.Target{Kind: stats.KindPublicLink, Alias: code}, nil
	}
}

func newStatsCmd() *cobra.Command {
	var from, to, tz, domain string
	var plain bool
	cmd := &cobra.Command{
		Use:   "stats [short-code]",
		Short: "Interactive analytics dashboard",
		Long: `Interactive analytics dashboard.

On a terminal this opens a live dashboard: time chart, browser/OS/
country/city/referrer panels, drill-down filtering, and a clicks/unique
toggle. Press T to type a time range — trailing windows (7d, 24h, 4h,
5m, 2w), relative ranges (now - 2w to now - 1w), or absolute dates
(2026-01-01 to 2026-02-15). Piped, with --json, with --plain, or with
a custom --from/--to range it prints a static report instead.

Logged in without a short code, aggregates across all your links.
With a short code, shows that link — public stats work without login.`,
		Example: `  spoo stats             # dashboard for all your links
  spoo stats launch      # dashboard for one link
  spoo stats --plain     # static report
  spoo stats launch --from 2026-01-01 --to 2026-02-01
  spoo stats launch --json`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeAlias,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			var code string
			if len(args) == 1 {
				code = args[0]
			}
			_, loadErr := d.store.Load()
			loggedIn := !errors.Is(loadErr, auth.ErrNotLoggedIn)
			if !loggedIn && code == "" {
				return fmt.Errorf("not logged in — pass a short code for public stats, or run `spoo auth login`")
			}
			defaultDomain := apiHost(d.cfg.APIBase)
			if domain != "" && !loggedIn {
				return fmt.Errorf("--domain requires login — public stats cover only %s links", defaultDomain)
			}
			target, err := resolveTarget(cmd.Context(), d.client, code, domain, defaultDomain, loggedIn, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")

			fromT, err := parseDate(from)
			if err != nil {
				return err
			}
			toT, err := parseDate(to)
			if err != nil {
				return err
			}

			customRange := !fromT.IsZero() || !toT.IsZero()
			if !asJSON && !plain && !customRange && stdoutIsTerminal(cmd) {
				model := stats.New(d.client, target, loggedIn, tz)
				final, err := tea.NewProgram(model).Run()
				if err != nil {
					return err
				}
				if m, ok := final.(stats.Model); ok && m.FetchErr() != nil {
					return m.FetchErr()
				}
				return nil
			}

			// static path: the API's implicit default is only 7 days;
			// use the widest window unless the user narrows it
			if fromT.IsZero() && toT.IsZero() {
				fromT = timeNow().UTC().AddDate(0, 0, -spoo.MaxRangeDays)
			}
			q := spoo.StatsQuery{
				StartDate: fromT,
				EndDate:   toT,
				Timezone:  tz,
				GroupBy:   []string{"time", "browser", "os", "country", "referrer"},
			}
			var res *spoo.StatsResponse
			var link *spoo.PublicLinkFacts
			switch target.Kind {
			case stats.KindOwnedLink:
				res, err = d.client.LinkStats(cmd.Context(), target.URLID, q)
			case stats.KindPublicLink:
				// no group_by here — the public endpoint returns every
				// dimension in one response, alongside the link facts
				var public *spoo.PublicStatsResult
				public, err = d.client.PublicStats(cmd.Context(), code, spoo.PublicStatsQuery{
					StartDate: fromT, EndDate: toT, Timezone: tz,
				})
				if public != nil {
					res, link = &public.Stats, &public.Link
				}
			default:
				res, err = d.client.Stats(cmd.Context(), q)
			}
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintln(prettyOut(cmd), renderStats(res, code, link))
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date, ISO 8601 (static report; default: 90 days ago)")
	cmd.Flags().StringVar(&to, "to", "", "end date, ISO 8601 (static report; default: now)")
	cmd.Flags().StringVar(&tz, "tz", "", "IANA timezone for time buckets (default UTC)")
	cmd.Flags().BoolVar(&plain, "plain", false, "print the static report instead of the dashboard")
	cmd.Flags().StringVar(&domain, "domain", "", "the link is on one of your custom domains")
	flagComp(cmd, "domain", completeDomain)
	return cmd
}
