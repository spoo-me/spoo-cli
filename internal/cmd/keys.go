package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

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
	cmd.AddCommand(newKeysRevokeCmd())
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
		fmt.Fprintln(prettyOut(cmd), ui.Dim.Render("no API keys — create one at https://spoo.me/dashboard/keys"))
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
