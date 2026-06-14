package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeysList(cmd)
		},
	}
	cmd.AddCommand(newKeysCreateCmd(), newKeysRevokeCmd())
	return cmd
}

func runKeysList(cmd *cobra.Command) error {
	d, err := newDeps()
	if err != nil {
		return err
	}
	keys, err := d.client.ListKeys(cmd.Context())
	if err != nil {
		return err
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(keys)
	}
	if len(keys) == 0 {
		fmt.Fprintln(prettyOut(cmd), ui.Dim.Render("no API keys — create one with `spoo keys create --name my-key`"))
		return nil
	}
	// cells stay unstyled: ANSI codes would skew tabwriter's column math
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPREFIX\tNAME\tSCOPES\tCREATED\tSTATE")
	for _, k := range keys {
		state := "active"
		if k.Revoked {
			state = "revoked"
		}
		created := ""
		if k.CreatedAt > 0 {
			created = time.Unix(k.CreatedAt, 0).UTC().Format("2006-01-02")
		}
		fmt.Fprintf(w, "%s\t%s…\t%s\t%s\t%s\t%s\n",
			k.ID, k.TokenPrefix, k.Name, strings.Join(k.Scopes, ","), created, state)
	}
	return w.Flush()
}

func newKeysCreateCmd() *cobra.Command {
	var (
		name, description, expires string
		scopes                     []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API key",
		Long: `Create an API key.

Scopes: shorten:create, urls:read, urls:manage, stats:read,
domains:read, domains:manage, admin:all.

Requires a browser login (spoo auth login) — the API refuses key
creation authenticated by another API key. The token is shown ONCE.`,
		Example: `  spoo keys create --name ci --scopes shorten:create
  spoo keys create --name bot --scopes shorten:create,stats:read --expires 720h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if len(scopes) == 0 {
				return fmt.Errorf("--scopes is required (e.g. --scopes shorten:create,stats:read)")
			}
			d, err := newDeps()
			if err != nil {
				return err
			}
			exp, err := parseExpiry(expires, timeNow())
			if err != nil {
				return err
			}
			key, err := d.client.CreateKey(cmd.Context(), api.CreateKeyRequest{
				Name: name, Description: description, Scopes: scopes, ExpiresAt: exp,
			})
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(key)
			}
			body := ui.OK.Render("✓ key created: ") + key.Name + "\n\n" +
				ui.Title.Render(key.Token) + "\n\n" +
				ui.Err.Render("save it now — it cannot be shown again")
			fmt.Fprintln(prettyOut(cmd), ui.Box.Render(body))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "key name (required)")
	cmd.Flags().StringVar(&description, "description", "", "what this key is for")
	cmd.Flags().StringSliceVar(&scopes, "scopes", nil, "comma-separated scopes (required)")
	cmd.Flags().StringVar(&expires, "expires", "", "expiry: ISO 8601, epoch, or duration like 720h")
	flagComp(cmd, "scopes", completeScopes)
	return cmd
}

func newKeysRevokeCmd() *cobra.Command {
	var hard bool
	cmd := &cobra.Command{
		Use:               "revoke <id>",
		Short:             "Revoke an API key",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeKeyID,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if err := d.client.DeleteKey(cmd.Context(), args[0], !hard); err != nil {
				return err
			}
			action := "revoked"
			if hard {
				action = "deleted"
			}
			fmt.Fprintln(prettyOut(cmd), ui.OK.Render("✓ "+action+" ")+args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&hard, "delete", false, "hard-delete the record instead of revoking")
	return cmd
}
