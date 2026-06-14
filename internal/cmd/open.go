package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "open <short-code>",
		Short:             "Open a short link in your browser",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAlias,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			u := strings.TrimRight(d.cfg.APIBase, "/") + "/" + args[0]
			fmt.Fprintln(prettyOut(cmd), ui.Dim.Render("opening "+u))
			return browser.OpenURL(u)
		},
	}
}

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "inspect <short-code>",
		Short:             "See where a short link points without counting a click",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAlias,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			res, err := d.client.Inspect(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			switch {
			case res.Destination != "":
				out := prettyOut(cmd)
				fmt.Fprintln(out, ui.Title.Render(res.ShortURL))
				fmt.Fprintln(out, ui.Dim.Render("→ ")+res.Destination)
			case res.Status == http.StatusNotFound:
				return fmt.Errorf("%s does not exist", res.ShortURL)
			case res.Status == http.StatusGone:
				return fmt.Errorf("%s has expired", res.ShortURL)
			case res.Status == http.StatusUnauthorized || res.Status == http.StatusForbidden:
				return fmt.Errorf("%s is password-protected or blocked", res.ShortURL)
			default:
				return fmt.Errorf("%s responded with HTTP %d", res.ShortURL, res.Status)
			}
			return nil
		},
	}
}
