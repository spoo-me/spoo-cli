package main

import (
	"context"
	"os"

	fang "charm.land/fang/v2"

	"github.com/spoo-me/spoo-cli/internal/cmd"
)

// Set by goreleaser via ldflags.
var (
	version = "dev"
	commit  = ""
)

func main() {
	opts := []fang.Option{
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt),
	}
	if commit != "" {
		opts = append(opts, fang.WithCommit(commit))
	}
	if err := fang.Execute(context.Background(), cmd.NewRootCmd(), opts...); err != nil {
		os.Exit(1)
	}
}
