package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func newShortenCmd() *cobra.Command {
	var (
		req     api.ShortenRequest
		expires string
	)
	cmd := &cobra.Command{
		Use:   "shorten [url]",
		Short: "Create a short link",
		Long: `Create a short link.

With a URL argument, shortens it directly. With input piped on stdin,
shortens every non-empty line (one short URL per line out). With no
argument on a terminal, opens an interactive form.`,
		Example: `  spoo shorten https://example.com/very/long/path
  spoo shorten https://example.com --alias launch --expires 72h
  cat urls.txt | spoo shorten
  spoo shorten`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := newDeps()
			if err != nil {
				return err
			}
			if req.ExpireAfter, err = parseExpiry(expires, time.Now()); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")

			switch {
			case len(args) == 1:
				req.LongURL = args[0]
				return shortenOne(cmd, d, req, asJSON)
			case !stdinIsTerminal(cmd):
				return shortenLines(cmd, d, req, asJSON)
			default:
				if err := runShortenForm(cmd.Context(), d.client, &req); err != nil {
					return err
				}
				return shortenOne(cmd, d, req, asJSON)
			}
		},
	}
	cmd.Flags().StringVarP(&req.Alias, "alias", "a", "", "custom alias (3-16 chars, a-z 0-9 - _)")
	cmd.Flags().StringVarP(&req.Password, "password", "p", "", "password-protect the link")
	cmd.Flags().IntVar(&req.MaxClicks, "max-clicks", 0, "auto-expire after N clicks")
	cmd.Flags().StringVar(&expires, "expires", "", "expiry: ISO 8601, epoch seconds, or duration like 72h")
	cmd.Flags().BoolVar(&req.BlockBots, "block-bots", false, "block known bot user agents")
	cmd.Flags().BoolVar(&req.PrivateStats, "private-stats", false, "make stats private (requires login)")
	cmd.Flags().StringVar(&req.Domain, "domain", "", "use one of your custom domains")
	return cmd
}

func stdinIsTerminal(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func stdoutIsTerminal(cmd *cobra.Command) bool {
	f, ok := cmd.OutOrStdout().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func shortenOne(cmd *cobra.Command, d *deps, req api.ShortenRequest, asJSON bool) error {
	res, err := d.client.Shorten(cmd.Context(), req)
	if err != nil {
		return err
	}
	return printShortURL(cmd, res, asJSON)
}

// shortenLines shortens each non-empty stdin line with the same flag
// options. Sequential on purpose: authed accounts get 60 req/min.
func shortenLines(cmd *cobra.Command, d *deps, base api.ShortenRequest, asJSON bool) error {
	var results []*api.ShortURL
	scanner := bufio.NewScanner(cmd.InOrStdin())
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		req := base
		req.LongURL = line
		req.Alias = "" // aliases are unique; never reuse one across bulk input
		res, err := d.client.Shorten(cmd.Context(), req)
		if err != nil {
			return fmt.Errorf("shortening %q: %w", line, err)
		}
		if asJSON {
			results = append(results, res)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), res.ShortURL)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	return nil
}

func printShortURL(cmd *cobra.Command, res *api.ShortURL, asJSON bool) error {
	out := cmd.OutOrStdout()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if !stdoutIsTerminal(cmd) {
		_, err := io.WriteString(out, res.ShortURL+"\n")
		return err
	}
	body := ui.OK.Render("✓ Link created") + "\n\n" +
		ui.Title.Render(res.ShortURL) + "\n" +
		ui.Dim.Render("→ "+truncate(res.LongURL, 60))
	fmt.Fprintln(prettyOut(cmd), ui.Box.Render(body))
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
