package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the logged-in account",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhoami(cmd)
		},
	}
}

func runWhoami(cmd *cobra.Command) error {
	d, err := newDeps()
	if err != nil {
		return err
	}
	if _, err := d.store.Load(); errors.Is(err, auth.ErrNotLoggedIn) {
		fmt.Fprintln(cmd.OutOrStdout(), ui.Dim.Render("Not logged in. Run `spoo auth login` — anonymous shortening still works."))
		return nil
	}
	user, err := d.client.Me(cmd.Context())
	if err != nil {
		return err
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(user)
	}
	verified := ui.OK.Render("verified")
	if !user.EmailVerified {
		verified = ui.Err.Render("unverified")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n%s\n",
		ui.Title.Render(user.Email), verified, ui.Dim.Render("plan: "+user.Plan))
	return nil
}
