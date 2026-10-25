<div align="center">

# Rum

**A fast, modern, cross-platform download manager.**

Segmented multi-connection downloads, a priority scheduler, bandwidth scheduling,
auto-organizing categories, checksum verification, clipboard capture, a system
tray, and native notifications — in a clean desktop app *and* a scriptable CLI.

[Install](#install) · [Features](#features) · [Quickstart](#development-quickstart) · [Architecture](#architecture) · [License](#license)

</div>

---

## What is Rum?

Rum is a download manager built in **Go** with a **React + TypeScript** UI,
packaged as a native desktop app via [Wails](https://wails.io). It ships in two
forms from one codebase:

- **Desktop app** — a native window (GTK/WebKit on Linux, WebView2 on Windows,
  WebKit on macOS) for Linux, Windows, and macOS.
- **CLI / TUI** (`rum`) — a terminal download manager driving the same engine,
  for scripts, servers, and keyboard-first users.

The download engine splits range-capable files into multiple connections,
resumes interrupted transfers, retries transient failures with backoff, and
verifies integrity — exposed over a small local HTTP API that the UI, CLI, and
TUI all share.

## Features

**Download engine**
- ⚡ **Segmented (multi-connection) downloads** — range-capable files are split
  into N concurrent segments for higher throughput, with a `.rumparts` sidecar
  for robust resume; automatic fallback to single-stream when unsupported.
- 🔁 **Smart retry** — exponential backoff with jitter that resumes from the
  current offset and only retries transient faults (resets, timeouts, 5xx, 429).
- 🪪 **Priority scheduler** — concurrency is gated by a priority-aware scheduler,
  so high-priority downloads run first within your connection budget.
- 🎚️ **Bandwidth scheduling** — global speed governor with time-of-day/day-of-week
  windows (e.g. throttle to 500 KB/s during work hours), plus **scheduled start**
  for individual downloads.
- 🗂️ **Categories & auto-organize** — finished files are routed into folders by
  extension rules (Video/Audio/Documents/Archives/Images, all customizable).
- 🔐 **Checksum verification** — verify a download against an expected
  SHA-256/MD5; disk-space preflight before allocating.
- 🛡️ **Hardened networking** — http(s)-only scheme allowlist, TLS floor, bounded
  redirects that strip credentials across hosts.

**Desktop experience**
- 📋 **Clipboard capture** — opt-in watcher that offers to add http(s) links you
  copy.
- 🪟 **System tray** *(Windows)* — show/hide, start-all, pause-all, and quit;
  optional **minimize-to-tray** and **close-to-tray**. On Linux/macOS these fall
  back to normal window behavior (a Wails v2 GTK/AppKit app can't host a second
  tray event loop in-process; native tray support arrives with Wails v3).
- 🔔 **Native notifications** on download completion.
- 🚀 **Autostart on login** (Linux `.desktop`, Windows Run key, macOS LaunchAgent)
  and **window-state persistence**.
- 📁 **Native folder picker** for choosing destinations.
- 🔌 **Safe local API** — binds to `127.0.0.1` only (not your LAN) and falls back
  to a free port if 8080 is taken.

**Add downloads**
- ➕ Single or **batch** (paste many URLs at once), with optional checksum,
  category, and scheduled-start.
- 🌐 **Browser extension** (Chrome, Edge, Brave, Firefox): right-click a link
  or send big browser downloads to Rum automatically.
  See **[browser-extension/README.md](browser-extension/README.md)**.

## Install

**The easy way — grab a prebuilt installer from the
[Releases page](https://github.com/tiredbooy/Rum/releases/latest):**

| OS | File |
|----|------|
| Linux | `.AppImage` (portable) or `.deb` |
| Windows | `Rum-Setup.exe` |
| macOS | `.dmg` (unsigned — see note) |

> macOS builds are **unsigned**; after installing, run once:
> `xattr -dr com.apple.quarantine /Applications/Rum.app`

Prefer to build it yourself, or want the CLI? Full step-by-step instructions for
all three OSes (prebuilt **and** from source) are in **[INSTALL.md](INSTALL.md)**.

## Development quickstart

Requirements: **Go 1.25+**, **Node.js 20+**, the **Wails CLI**, and platform
WebView dependencies (GTK3 + WebKit2GTK 4.1 on Linux; WebView2 on Windows; Xcode
CLT on macOS). See <https://wails.io/docs/gettingstarted/installation>.

```bash
# install the Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0

# run the desktop app with hot-reload (frontend + Go)
wails dev
# on modern Linux (WebKit2GTK 4.1):
wails dev -tags webkit2_41

# production build → build/bin/Rum
wails build -tags webkit2_41
```

Work on the pieces individually:

```bash
# backend engine / API / CLI (a standalone Go module)
cd backend && go build ./... && go vet ./... && go test ./...
go run ./cmd/rum --help          # the CLI

# frontend (Vite/React/TS)
cd frontend && npm ci && npm run dev
```

If you only need a quick unpackaged binary, `go build` works too, but the tag
set matters: `go build -tags "desktop,production,webkit2_41" -o Rum .`. Without
`desktop,production` it links but refuses to start ("Wails applications will not
build without the correct build tags"); `wails build` adds them for you. Drop
`webkit2_41` if you only have WebKit2GTK 4.0. Any Go at or above the `go.mod`
version works, including 1.26/1.27; the installers set `GOTOOLCHAIN=local`.

## Data directory

Rum stores jobs, settings, the extension token and logs in
`<user config dir>/rum` (`~/.config/rum` on Linux). Set `RUM_CONFIG_DIR` to use
another folder, e.g. for a portable install or a throwaway profile.

## Update check

On startup (at most once a day, Settings > Updates > "Check for updates") Rum
reads `https://api.github.com/repos/tiredbooy/Rum/releases/latest` through its
own proxy setting with an 8 s timeout per source. If GitHub is unreachable it
tries each mirror in turn. A mirror is a URL prefix: Rum requests
`<mirror>/https://api.github.com/...`. Rum only shows a notice; it never
installs or replaces itself. "Download" saves the installer for your system to
your save location and checks it against the release's `SHA256SUMS`; a file that
does not match is deleted.

Mirrors: the built-in default is `https://ghproxy.net/`
(`updater.DefaultMirrors` in `backend/internal/pkg/updater/checker.go`). To use
your own, set `update_mirrors` in `settings.json` (or `PATCH /api/v1/settings`),
for example `"update_mirrors": ["https://my-proxy.example/"]`. Up to 5, http(s)
only; an empty list means the built-in default. A mirror can see and alter what
it relays, so the checksum file protects against corruption, not a hostile
mirror; Rum rebuilds every release and asset URL from the tag itself instead of
trusting the mirror's payload.

The running version comes from `handlers.AppVersion`; set it at build time with
`-ldflags "-X github.com/tiredbooy/Rum/backend/internal/pkg/api/handlers.AppVersion=1.2.3"`.

## Architecture

Rum keeps one engine behind a small local HTTP API and reuses it everywhere. The
**backend** Go module (`backend/`) holds the download engine
(`internal/pkg/download`), a priority queue + scheduler, config, filesystem and
security helpers, and a Gin HTTP API (`internal/pkg/api`) exposing
`/api/v1/...`; `cmd/rum` is the CLI/TUI and `cmd/server` is the API server. The
**root** module is the Wails desktop shell: it embeds the built frontend, starts
the API bound to `127.0.0.1`, exposes the resolved base URL and a native folder
picker to the UI, and adds desktop concerns (tray, notifications, autostart,
window state, clipboard watcher). The **frontend** (`frontend/`) is a
React + TypeScript + Tailwind + shadcn/ui app using TanStack Query and Zustand,
with live progress over Server-Sent Events. The desktop UI, the CLI, and the TUI
therefore all drive the exact same engine.

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for
the verification gate and the repo conventions.

## License

Rum is released under the [MIT License](LICENSE).
