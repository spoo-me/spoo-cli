package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	huh "charm.land/huh/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
)

var aliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,16}$`)

// runShortenForm collects shorten options interactively. The alias field
// validates against the live check-alias endpoint (rate limit 180/min —
// the backend sizes it for interactive use).
func runShortenForm(ctx context.Context, client *api.Client, req *api.ShortenRequest) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Long URL").
				Placeholder("https://example.com/very/long/path").
				Value(&req.LongURL).
				Validate(func(s string) error {
					u, err := url.Parse(s)
					if err != nil || u.Scheme == "" || u.Host == "" {
						return errors.New("enter a full URL including https://")
					}
					return nil
				}),
			huh.NewInput().
				Title("Custom alias").
				Description("optional — leave empty for a random one").
				Value(&req.Alias).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if !aliasRe.MatchString(s) {
						return errors.New("3-16 chars: letters, numbers, - and _")
					}
					check, err := client.CheckAlias(ctx, s, req.Domain)
					if err != nil {
						return nil // network hiccup: let the server be the judge on submit
					}
					if !check.Available {
						return fmt.Errorf("not available (%s)", check.Reason)
					}
					return nil
				}),
			huh.NewInput().
				Title("Password").
				Description("optional — viewers must enter it before redirecting").
				EchoMode(huh.EchoModePassword).
				Value(&req.Password),
			huh.NewConfirm().
				Title("Block bots?").
				Value(&req.BlockBots),
		),
	)
	return form.Run()
}
