package cmd

import (
	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/config"
)

// deps bundles everything a command needs. Factory is a package var so
// command tests can swap in a client pointed at httptest.
type deps struct {
	client *api.Client
	store  *auth.Store
	cfg    config.Config
}

var newDeps = func() (*deps, error) {
	cfg := config.Load()
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	store := auth.NewStore(dir)
	return &deps{client: api.New(cfg.APIBase, store), store: store, cfg: cfg}, nil
}

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
	root.AddCommand(
		newAuthCmd(), newWhoamiCmd(), newShortenCmd(),
		newLinksCmd(), newStatsCmd(), newExportCmd(),
		newDomainsCmd(), newKeysCmd(),
		newOpenCmd(), newInspectCmd(), newQRCmd(),
	)
	return root
}
