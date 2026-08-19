package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	spoo "github.com/spoo-me/spoo-go"
	"github.com/spoo-me/spoo-go/option"

	"github.com/spoo-me/spoo-cli/internal/auth"
	"github.com/spoo-me/spoo-cli/internal/config"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// deps bundles everything a command needs. Factory is a package var so
// command tests can swap in a client pointed at httptest.
type deps struct {
	client *spoo.Client
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
	client := spoo.NewClient(
		option.WithBaseURL(cfg.APIBase),
		option.WithTokenSource(store),
		option.WithClientTag(clientTag()),
	)
	return &deps{client: client, store: store, cfg: cfg}, nil
}

// NewRootCmd builds the spoo root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "spoo",
		Short:         "Shorten links and manage your spoo.me account from the terminal",
		Long:          ui.Banner() + "\n\nShorten links, browse analytics, and manage your account without leaving the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "output machine-readable JSON")
	root.AddCommand(
		newAuthCmd(), newWhoamiCmd(), newShortenCmd(),
		newLinksCmd(), newStatsCmd(), newExportCmd(),
		newOpenCmd(), newInspectCmd(), newQRCmd(),
	)
	humanizeErrors(root)
	return root
}

// humanizeErrors wraps every command's RunE so the SDK's sentinel
// conditions come out as CLI guidance instead of raw API messages. The
// SDK owns detection (errors.Is); the CLI owns the wording.
func humanizeErrors(cmd *cobra.Command) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			return humanize(run(c, args))
		}
	}
	for _, sub := range cmd.Commands() {
		humanizeErrors(sub)
	}
}

func humanize(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, spoo.ErrSessionExpired):
		return errors.New("session expired — run `spoo auth login` again")
	case errors.Is(err, spoo.ErrLinkPasswordProtected):
		return errors.New("this link's stats are password protected")
	}
	return err
}
