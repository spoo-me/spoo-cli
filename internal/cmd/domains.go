package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newDomainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Manage custom domains",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsList(cmd)
		},
	}
	cmd.AddCommand(newDomainsAddCmd(), newDomainsVerifyCmd(), newDomainsConfigCmd(), newDomainsRemoveCmd())
	return cmd
}

func runDomainsList(cmd *cobra.Command) error {
	d, err := newDeps()
	if err != nil {
		return err
	}
	page, err := d.client.ListDomains(cmd.Context())
	if err != nil {
		return err
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(page)
	}
	if len(page.Items) == 0 {
		fmt.Fprintln(prettyOut(cmd), ui.Dim.Render("no custom domains — add one with `spoo domains add links.your.com`"))
		return nil
	}
	// cells stay unstyled: ANSI codes would skew tabwriter's column math
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDOMAIN\tSTATUS\tCREATED")
	for _, dom := range page.Items {
		created := dom.CreatedAt
		if len(created) >= 10 {
			created = created[:10]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", dom.ID, dom.FQDN, dom.Status, created)
	}
	return w.Flush()
}

func newDomainsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <fqdn>",
		Short: "Register a custom domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			dom, err := d.client.CreateDomain(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			out := prettyOut(cmd)
			fmt.Fprintln(out, ui.OK.Render("✓ registered ")+ui.Title.Render(dom.FQDN)+
				ui.Dim.Render(" ("+dom.Status+")"))
			fmt.Fprintln(out, "\nAdd these DNS records at your registrar:")
			printDNSRecords(cmd, dom.DNSRecords)
			fmt.Fprintln(out, ui.Dim.Render("\nthen run: spoo domains verify "+dom.FQDN))
			return nil
		},
	}
}

func printDNSRecords(cmd *cobra.Command, records []api.DNSRecord) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "  TYPE\tNAME\tVALUE")
	for _, r := range records {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", r.Type, r.Name, r.Value)
	}
	w.Flush()
}

// resolveDomain accepts either a domain id or an fqdn and returns the domain.
func resolveDomain(cmd *cobra.Command, d *deps, ref string) (*api.Domain, error) {
	page, err := d.client.ListDomains(cmd.Context())
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		if page.Items[i].ID == ref || strings.EqualFold(page.Items[i].FQDN, ref) {
			return &page.Items[i], nil
		}
	}
	return nil, fmt.Errorf("no domain matching %q — see `spoo domains`", ref)
}

func newDomainsVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <fqdn-or-id>",
		Short: "Check DNS and activate a pending domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			dom, err := resolveDomain(cmd, d, args[0])
			if err != nil {
				return err
			}
			verified, err := d.client.VerifyDomain(cmd.Context(), dom.ID)
			if err != nil {
				return err
			}
			out := prettyOut(cmd)
			if strings.EqualFold(verified.Status, "ACTIVE") {
				fmt.Fprintln(out, ui.OK.Render("✓ "+verified.FQDN+" is active"))
				return nil
			}
			fmt.Fprintln(out, ui.Dim.Render(verified.FQDN+" is still "+verified.Status+
				" — DNS can take a while to propagate; expected records:"))
			printDNSRecords(cmd, verified.DNSRecords)
			return nil
		},
	}
}

func newDomainsConfigCmd() *cobra.Command {
	var rootRedirect, notFound, robots string
	cmd := &cobra.Command{
		Use:   "config <fqdn-or-id>",
		Short: "View or set a domain's routing (apex redirect, 404 fallback, robots.txt)",
		Long: `View or set a custom domain's per-domain routing.

Run bare to print the current config. Pass a flag to change one field;
pass it empty (e.g. --root-redirect "") to clear it. Omitted flags are
left untouched.

  root-redirect       where GET / on the domain sends visitors (302)
  not-found-redirect  fallback for any path that doesn't match an alias
  robots-txt          override the served /robots.txt body`,
		Example: `  spoo domains config links.acme.com
  spoo domains config links.acme.com --root-redirect https://acme.com
  spoo domains config links.acme.com --not-found-redirect https://acme.com/404
  spoo domains config links.acme.com --root-redirect ""   # clear it`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			dom, err := resolveDomain(cmd, d, args[0])
			if err != nil {
				return err
			}

			fields := map[string]any{}
			if cmd.Flags().Changed("root-redirect") {
				fields["root_redirect"] = nilIfEmpty(rootRedirect)
			}
			if cmd.Flags().Changed("not-found-redirect") {
				fields["not_found_redirect"] = nilIfEmpty(notFound)
			}
			if cmd.Flags().Changed("robots-txt") {
				fields["custom_robots_txt"] = nilIfEmpty(robots)
			}

			// no flags → just show the current config
			if len(fields) == 0 {
				return showDomainConfig(cmd, dom)
			}
			updated, err := d.client.UpdateDomain(cmd.Context(), dom.ID, fields)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(updated)
			}
			fmt.Fprintln(prettyOut(cmd), ui.OK.Render("✓ updated routing for ")+ui.Title.Render(updated.FQDN))
			return showDomainConfig(cmd, updated)
		},
	}
	cmd.Flags().StringVar(&rootRedirect, "root-redirect", "", "destination for GET / (302); empty clears")
	cmd.Flags().StringVar(&notFound, "not-found-redirect", "", "fallback for unmatched paths; empty clears")
	cmd.Flags().StringVar(&robots, "robots-txt", "", "override /robots.txt body; empty clears")
	return cmd
}

// showDomainConfig prints a domain's routing as a key/value block.
func showDomainConfig(cmd *cobra.Command, dom *api.Domain) error {
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(dom)
	}
	out := prettyOut(cmd)
	fmt.Fprintln(out, ui.Title.Render(dom.FQDN)+ui.Dim.Render("  ("+dom.Status+")"))
	row := func(label, value string) {
		if value == "" {
			value = ui.Dim.Render("not set")
		}
		fmt.Fprintf(out, "  %s %s\n", ui.Dim.Render(fmt.Sprintf("%-18s", label)), value)
	}
	row("root redirect", dom.RootRedirect)
	row("not-found redirect", dom.NotFoundRedirect)
	return nil
}

// nilIfEmpty maps "" to a nil interface so the JSON body carries null
// (the backend's "clear this field" signal) rather than an empty string.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func newDomainsRemoveCmd() *cobra.Command {
	var cascade, yes bool
	cmd := &cobra.Command{
		Use:   "remove <fqdn-or-id>",
		Short: "Revoke a custom domain",
		Long: `Revoke a custom domain: it stops serving immediately.

Without --cascade its links stay in your account (they just stop
resolving); with --cascade the links are deleted too.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("revoking a domain stops it serving immediately — re-run with --yes to confirm")
			}
			dom, err := resolveDomain(cmd, d, args[0])
			if err != nil {
				return err
			}
			res, err := d.client.RevokeDomain(cmd.Context(), dom.ID, cascade)
			if err != nil {
				return err
			}
			msg := ui.OK.Render("✓ revoked ") + res.FQDN
			if cascade {
				msg += ui.Dim.Render(fmt.Sprintf(" (%d links deleted)", res.URLsDeleted))
			}
			fmt.Fprintln(prettyOut(cmd), msg)
			return nil
		},
	}
	cmd.Flags().BoolVar(&cascade, "cascade", false, "also delete all links on this domain")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the revocation")
	return cmd
}
