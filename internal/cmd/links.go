package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/links"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newLinksCmd() *cobra.Command {
	var opts api.ListURLsOptions
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
	cmd.AddCommand(newLinksDeleteCmd(), newLinksUpdateCmd())
	return cmd
}

func printLinksList(cmd *cobra.Command, d *deps, opts api.ListURLsOptions, asJSON bool) error {
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
		created := it.CreatedAt
		if len(created) >= 10 {
			created = created[:10]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			it.ID, it.Alias, truncate(it.LongURL, 60), it.TotalClicks, it.Status, created)
	}
	return w.Flush()
}

func newLinksDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a link",
		Args:  cobra.ExactArgs(1),
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
		Use:   "update <id>",
		Short: "Update a link's properties",
		Args:  cobra.ExactArgs(1),
		Example: `  spoo links update 64f0c2... --status inactive
  spoo links update 64f0c2... --long-url https://new-destination.com
  spoo links update 64f0c2... --max-clicks 0   # remove the click limit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			fields := map[string]any{}
			if cmd.Flags().Changed("long-url") {
				fields["long_url"] = longURL
			}
			if cmd.Flags().Changed("alias") {
				fields["alias"] = alias
			}
			if cmd.Flags().Changed("password") {
				fields["password"] = password
			}
			if cmd.Flags().Changed("max-clicks") {
				fields["max_clicks"] = maxClicks
			}
			if cmd.Flags().Changed("expires") {
				exp, err := parseExpiry(expires, timeNow())
				if err != nil {
					return err
				}
				fields["expire_after"] = exp
			}
			if cmd.Flags().Changed("status") {
				fields["status"] = normalizeStatus(status)
			}
			if len(fields) == 0 {
				return fmt.Errorf("nothing to update — pass at least one flag")
			}
			res, err := d.client.UpdateURL(cmd.Context(), args[0], fields)
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
	cmd.Flags().StringVar(&password, "password", "", "new password")
	cmd.Flags().IntVar(&maxClicks, "max-clicks", 0, "click limit (0 removes it)")
	cmd.Flags().StringVar(&expires, "expires", "", "expiry: ISO 8601, epoch, or duration like 72h")
	cmd.Flags().StringVar(&status, "status", "", "active or inactive")
	return cmd
}
