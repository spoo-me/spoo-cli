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
    spoo shorten                                      # interactive mode
    cat urls.txt | spoo shorten                       # bulk
    spoo whoami

`--json` on any command emits machine-readable output. Output is
pipe-aware: on a terminal you get styled output, piped you get bare
short URLs. Set `SPOO_API_URL` to target a self-hosted instance.

## Auth

`spoo auth login` uses spoo.me's connected-apps device flow: your browser
opens, you approve once, and the CLI receives scoped tokens — it never
sees your password. Manage or revoke access any time at
https://spoo.me/dashboard/apps.

Headless? `echo $SPOO_TOKEN | spoo auth login --with-token` with an API
key from https://spoo.me/dashboard/keys.

## Development

    go build ./...   # build
    go test ./...    # test

Releases are cut by pushing a `v*` tag; GoReleaser builds and publishes
all platform binaries.
