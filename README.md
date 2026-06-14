<div align="center">
<pre>
          :+#%%#*=:
        -%@@@@@@@@@#.
       #@@@@@@@@@@@@@.
     .%@@@@@@@@@@@@@@+
    :@@@@@@@@@@@@@@@@+     <b>spoo</b>
  .+@@@@@@@@@@@@@@@@@.     the spoo.me CLI
.*@@@@@@@@@@@@@@@@@@+
+@@@@@@@@@@@@@@@@@@#
    =@@@@@@@@@@@@@%.
     *%%*+=*@@@@@#.
            :##*-
</pre>
</div>

<p align="center">Shorten, browse, and analyze your spoo.me links without leaving the terminal 🚀</p>

<p align="center">
    <a href="#-features"><kbd>🔥 Features</kbd></a>
    <a href="#-installation"><kbd>📦 Installation</kbd></a>
    <a href="#-quick-start"><kbd>🚀 Quick Start</kbd></a>
    <a href="#-commands"><kbd>📋 Commands</kbd></a>
    <a href="#-contributing"><kbd>🤝 Contributing</kbd></a>
</p>

<p align="center">
<a href="https://spoo.me"><img src="https://img.shields.io/badge/spoo.me-6a5cf4?logo=https://spoo.me/static/images/favicon.png" alt="spoo.me"></a>
<img src="https://img.shields.io/badge/Go-1.26-6a5cf4?logo=go&logoColor=white" alt="Go">
<a href="https://spoo.me/discord"><img src="https://img.shields.io/discord/1192388005206433892?logo=discord" alt="Discord"></a>
<a href="https://github.com/spoo-me/spoo-cli/blob/main/LICENSE"><img src="https://img.shields.io/static/v1.svg?style=flat&label=License&message=Apache%202.0&colorA=363a4f&colorB=b7bdf8" alt="License"></a>
</p>

# 🔥 Features

- `No-Account Shortening` - Create short links without signing in ⚡
- `Interactive Shorten` - Custom alias with live availability check, password, max clicks, expiry, bot blocking, and `--qr` 🎛️
- `Links Browser (TUI)` - Navigate, open, copy, edit, archive, and delete every link with a live analytics sidebar ✏️
- `Analytics Dashboard` - Terminal charts: traffic over time with a previous-period overlay, browser/OS/country/city/referrer panels, drill-down filtering, range expressions, and full mouse support 📊
- `QR Codes` - Render any link as a scannable QR code right in the terminal 📱
- `Custom Domains` - Add, verify, configure (apex redirect, 404 fallback, robots.txt), and remove your own domains 🌐
- `Export` - Download click data as JSON, CSV, XLSX, or XML 📤
- `API Keys` - Create, list, and revoke keys for scripting 🔑
- `Device-Flow Auth` - No keys to paste — sign in through the spoo.me device flow, sessions refresh for 30 days 🪪
- `Pipe-Aware` - `--json` on every command, plain output when piped, `NO_COLOR` honored, self-host friendly 🧰

# 📦 Installation

### With Go

```bash
go install github.com/spoo-me/spoo-cli/cmd/spoo@latest
```

### Prebuilt binary

Grab a binary for macOS, Linux, or Windows from the [releases page](https://github.com/spoo-me/spoo-cli/releases). Homebrew and Scoop packages land with the first tagged release.

> [!TIP]
> Point the CLI at a self-hosted instance any time with `export SPOO_API_URL=http://localhost:8000`.

# 🚀 Quick Start

```bash
spoo shorten https://example.com/very/long/path   # works without an account
spoo auth login                                   # browser one-click authorization
spoo shorten                                      # interactive form
cat urls.txt | spoo shorten                       # bulk
spoo links                                        # interactive link browser (TUI)
spoo stats launch                                 # charts in your terminal
```

# 📋 Commands

| Command | What it does |
|---|---|
| `spoo shorten [url]` | Create a link — flags for alias, password, max clicks, expiry, bot blocking, custom domain, and `--qr` to print a scannable code. Interactive form with live alias availability when run bare. |
| `spoo links` | Interactive TUI browser: navigate, open, copy, edit (`e`, pre-filled form), archive/activate (`t`), delete (`d`, type-the-alias confirm), QR (`Q`). `--json` or piping prints the list instead. `?` shows all keys. |
| `spoo links update/delete <id>` | Scriptable link management (`delete` requires `--yes`). |
| `spoo stats [code]` | Interactive analytics dashboard: time chart with previous-period overlay (`p`), browser/OS/country/city/referrer panels, drill-down filtering (enter or click any row), link switcher (`g`), range expressions (`T` — `7d`, `4h`, `now - 2w to now - 1w`, `2026-01-01 to 2026-02-15`), clicks↔unique toggle (`u`), full mouse support. Piped or `--plain` prints a static report; public stats work logged out. |
| `spoo export [code]` | Download analytics as JSON, CSV (zip), XLSX, or XML. |
| `spoo domains` | Custom domains: `add` (prints the DNS records to set), `verify`, `config` (apex redirect, 404 fallback, robots.txt), `remove`. |
| `spoo keys` | API keys: `create` (token shown once), list, `revoke`. |
| `spoo open <code>` | Open a short link in your browser. |
| `spoo inspect <code>` | See where a link points **without** counting a click. |
| `spoo qr <code>` | Render a link as a scannable QR code in the terminal (also `Q` in `spoo links`). |
| `spoo auth login/logout/status`, `spoo whoami` | Account session. |

> [!NOTE]
> `--json` on any command emits machine-readable output. Output is pipe-aware: styled on a terminal, plain when piped.

# 🔑 Authentication

`spoo auth login` uses spoo.me's connected-apps device flow: your browser opens, you approve once, and the CLI receives scoped tokens — it never sees your password. Sessions refresh themselves for 30 days. Manage or revoke access any time at <https://spoo.me/dashboard/apps>.

Headless? Pipe in an API key from <https://spoo.me/dashboard/keys>:

```bash
echo $SPOO_TOKEN | spoo auth login --with-token
```

# 🛠️ Development

```bash
go build ./...   # build
go test ./...    # test (no network, no real keychain)
```

Releases are cut by pushing a `v*` tag; [GoReleaser](https://goreleaser.com) builds and publishes every platform binary.

# 🤝 Contributing

Issues and pull requests are welcome. Found a bug or have an idea? [Open an issue](https://github.com/spoo-me/spoo-cli/issues) or join the conversation on [Discord](https://spoo.me/discord).

<p align="center">Part of the <a href="https://spoo.me">spoo.me</a> family · <a href="https://github.com/spoo-me/spoo-raycast">Raycast</a> · <a href="https://github.com/spoo-me/py_spoo_url">Python SDK</a> · <a href="https://github.com/spoo-me/spoo-rust">Rust</a></p>
