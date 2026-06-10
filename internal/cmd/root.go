package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd builds the spoo root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spoo",
		Short:         "Shorten links and manage your spoo.me account from the terminal",
		Long:          "spoo is the official command-line client for spoo.me —\nshorten links, browse analytics, and manage your account without leaving the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "output machine-readable JSON")
	return root
}
