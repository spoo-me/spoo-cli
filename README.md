# spoo

The official command-line client for [spoo.me](https://spoo.me) — shorten
links, browse analytics, and manage your account without leaving the terminal.

## Install

With Go:

    go install github.com/spoo-me/spoo-cli/cmd/spoo@latest

Or grab a prebuilt binary for macOS, Linux, or Windows from the
[releases page](https://github.com/spoo-me/spoo-cli/releases).
Homebrew and Scoop packages are coming with the first tagged release.

## Quick start

    spoo shorten https://example.com/very/long/path   # works without an account
    spoo auth login                                   # browser one-click authorization
    spoo shorten                                      # interactive form
    cat urls.txt | spoo shorten                       # bulk
    spoo links                                        # interactive link browser (TUI)
    spoo stats launch                                 # charts in your terminal

## Commands

| Command | What it does |
|---|---|
| `spoo shorten [url]` | Create a link — flags for alias, password, max clicks, expiry, bot blocking, custom domain. Interactive form with live alias availability when run bare. |
| `spoo links` | Interactive TUI browser: navigate, open, copy, toggle status, delete. `--json` or piping prints the list instead. |
| `spoo links update/delete <id>` | Scriptable link management (`delete` requires `--yes`). |
| `spoo stats [code]` | Click analytics: summary, click sparkline, browser/OS/country/referrer charts. Public stats work logged out. |
| `spoo export [code]` | Download analytics as json, csv (zip), xlsx, or xml. |
| `spoo domains` | Custom domains: `add` (prints the DNS records to set), `verify`, `remove`. |
| `spoo keys` | API keys: `create` (token shown once), list, `revoke`. |
| `spoo open <code>` | Open a short link in your browser. |
| `spoo inspect <code>` | See where a link points **without** counting a click. |
| `spoo auth login/logout/status`, `spoo whoami` | Account session. |

`--json` on any command emits machine-readable output. Output is
pipe-aware: styled on a terminal, plain when piped, and `NO_COLOR` is
honored. Set `SPOO_API_URL` to target a self-hosted instance.

## Auth

`spoo auth login` uses spoo.me's connected-apps device flow: your browser
opens, you approve once, and the CLI receives scoped tokens — it never
sees your password. Sessions refresh themselves for 30 days. Manage or
revoke access any time at https://spoo.me/dashboard/apps.

Headless? `echo $SPOO_TOKEN | spoo auth login --with-token` with an API
key from https://spoo.me/dashboard/keys.

## Development

    go build ./...   # build
    go test ./...    # test (no network, no real keychain)

Releases are cut by pushing a `v*` tag; GoReleaser builds and publishes
all platform binaries.
