package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate spoo with your spoo.me account",
	}
	cmd.AddCommand(newLoginCmd(), newLogoutCmd(), newStatusCmd())
	return cmd
}

func newLoginCmd() *cobra.Command {
	var withToken bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in via your browser (or --with-token for an API key)",
		Long: `Log in to spoo.me.

By default this opens your browser for a one-click authorization using
spoo.me's connected-apps device flow. The CLI never sees your password.

For headless environments (CI, servers), create an API key on
https://spoo.me/dashboard/keys and pipe it in:

  echo $SPOO_TOKEN | spoo auth login --with-token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if withToken {
				return loginWithToken(cmd, d)
			}
			return loginWithBrowser(cmd, d)
		},
	}
	cmd.Flags().BoolVar(&withToken, "with-token", false, "read a spoo_ API key from stdin")
	return cmd
}

func loginWithBrowser(cmd *cobra.Command, d *deps) error {
	flow := &auth.DeviceFlow{
		APIBase:     d.cfg.APIBase,
		OpenBrowser: browser.OpenURL,
		Out:         cmd.ErrOrStderr(),
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	code, err := flow.Run(ctx)
	if err != nil {
		return err
	}
	tokens, err := d.client.ExchangeDeviceCode(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	if err := d.store.Save(auth.Credentials{
		Mode:         auth.ModeDevice,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), ui.OK.Render("✓ Logged in as ")+ui.Title.Render(tokens.User.Email))
	return nil
}

func loginWithToken(cmd *cobra.Command, d *deps) error {
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return fmt.Errorf("no token on stdin — usage: echo $SPOO_TOKEN | spoo auth login --with-token")
	}
	token := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(token, "spoo_") {
		return fmt.Errorf("that doesn't look like a spoo API key (expected spoo_ prefix)")
	}
	if err := d.store.Save(auth.Credentials{Mode: auth.ModeAPIKey, APIKey: token}); err != nil {
		return err
	}
	// validate immediately so a bad key fails loudly now, not later
	user, err := d.client.Me(cmd.Context())
	if err != nil {
		_ = d.store.Clear()
		return fmt.Errorf("API key rejected: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), ui.OK.Render("✓ Logged in as ")+ui.Title.Render(user.Email))
	return nil
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if err := d.store.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.Dim.Render("Logged out."))
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhoami(cmd)
		},
	}
}
