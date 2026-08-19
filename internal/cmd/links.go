package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	spoo "github.com/spoo-me/spoo-go"

	"github.com/spoo-me/spoo-cli/internal/tui/links"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newLinksCmd() *cobra.Command {
	var opts spoo.ListURLsOptions
	cmd := &cobra.Command{
		Use:   "links",
		Short: "Browse and manage your links",
		Long: `Browse and manage your links.

On a terminal this opens an interactive browser (navigate, open, copy,
toggle, delete). Piped or with --json it prints the list and exits.`,
		Example: `  spoo links
  spoo links --search launch
  spoo links --json | jq '.items[].alias'
  spoo links delete 64f0c2...
  spoo links update 64f0c2... --status inactive`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			// accept hyphenated flag values (total-clicks) for the API's
			// snake_case fields, and case-insensitive status
			opts.SortBy = strings.ReplaceAll(opts.SortBy, "-", "_")
			opts.Status = normalizeStatus(opts.Status)
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON || !stdoutIsTerminal(cmd) {
				return printLinksList(cmd, d, opts, asJSON)
			}
			model := links.New(d.client, d.cfg.APIBase, opts, browser.OpenURL, clipboard.WriteAll)
			final, err := tea.NewProgram(model).Run()
			if err != nil {
				return err
			}
			if m, ok := final.(links.Model); ok && m.Err() != nil {
				return m.Err()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Search, "search", "", "filter by alias or destination")
	cmd.Flags().StringVar(&opts.Status, "status", "", "filter by status (active, inactive, blocked, expired)")
	cmd.Flags().StringVar(&opts.Domain, "domain", "", "filter by custom domain")
	cmd.Flags().IntVar(&opts.Page, "page", 1, "page number")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 20, "items per page (max 100)")
	cmd.Flags().StringVar(&opts.SortBy, "sort", "total_clicks", "sort by: total_clicks, created_at, last_click")
	fixed(cmd, "status", "active", "inactive", "blocked", "expired")
	fixed(cmd, "sort", "total_clicks", "created_at", "last_click")
	flagComp(cmd, "domain", completeDomain)
	cmd.AddCommand(newLinksDeleteCmd(), newLinksUpdateCmd())
	return cmd
}

func printLinksList(cmd *cobra.Command, d *deps, opts spoo.ListURLsOptions, asJSON bool) error {
	page, err := d.client.ListURLs(cmd.Context(), opts)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(page)
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tALIAS\tDESTINATION\tCLICKS\tSTATUS\tCREATED")
	for _, it := range page.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			it.ID, it.Alias, truncate(it.LongURL, 60), it.TotalClicks, it.Status, day(it.CreatedAt))
	}
	return w.Flush()
}

func newLinksDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:               "delete <id>",
		Short:             "Permanently delete a link",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeLinkID,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("deleting a link is irreversible — re-run with --yes to confirm")
			}
			if err := d.client.DeleteURL(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintln(prettyOut(cmd), ui.OK.Render("✓ deleted ")+args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the deletion")
	return cmd
}

func newLinksUpdateCmd() *cobra.Command {
	var (
		longURL, alias, password, expires, status string
		maxClicks                                 int
	)
	cmd := &cobra.Command{
		Use:               "update <id>",
		Short:             "Update a link's properties",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeLinkID,
		Example: `  spoo links update 64f0c2... --status inactive
  spoo links update 64f0c2... --long-url https://new-destination.com
  spoo links update 64f0c2... --max-clicks 0   # remove the click limit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			// the API's PATCH is tri-state: an omitted field keeps the
			// current setting, null clears it, a value replaces it. A
			// flag the user didn't pass stays omitted; the "remove"
			// spellings (--max-clicks 0, empty --password/--expires)
			// map to null.
			var params spoo.UpdateURLParams
			changed := 0
			if cmd.Flags().Changed("long-url") {
				params.LongURL = longURL
				changed++
			}
			if cmd.Flags().Changed("alias") {
				params.Alias = alias
				changed++
			}
			if cmd.Flags().Changed("password") {
				params.Password = spoo.Set(password)
				if password == "" {
					params.Password = spoo.Null[string]()
				}
				changed++
			}
			if cmd.Flags().Changed("max-clicks") {
				params.MaxClicks = spoo.Set(maxClicks)
				if maxClicks == 0 {
					params.MaxClicks = spoo.Null[int]()
				}
				changed++
			}
			if cmd.Flags().Changed("expires") {
				exp, err := spoo.ParseExpiry(expires, timeNow())
				if err != nil {
					return err
				}
				params.ExpireAfter = spoo.Set(exp)
				if exp.IsZero() {
					params.ExpireAfter = spoo.Null[time.Time]()
				}
				changed++
			}
			if cmd.Flags().Changed("status") {
				params.Status = normalizeStatus(status)
				changed++
			}
			if changed == 0 {
				return fmt.Errorf("nothing to update — pass at least one flag")
			}
			res, err := d.client.UpdateURL(cmd.Context(), args[0], params)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Fprintln(prettyOut(cmd), ui.OK.Render("✓ updated ")+res.Alias+
				ui.Dim.Render(" ("+res.Status+")"))
			return nil
		},
	}
	cmd.Flags().StringVar(&longURL, "long-url", "", "new destination URL")
	cmd.Flags().StringVar(&alias, "alias", "", "new alias")
	cmd.Flags().StringVar(&password, "password", "", "new password (empty removes it)")
	cmd.Flags().IntVar(&maxClicks, "max-clicks", 0, "click limit (0 removes it)")
	cmd.Flags().StringVar(&expires, "expires", "", "expiry: ISO 8601, epoch, or duration like 72h (empty removes it)")
	cmd.Flags().StringVar(&status, "status", "", "active or inactive")
	fixed(cmd, "status", "active", "inactive")
	return cmd
}
