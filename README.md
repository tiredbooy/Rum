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
- 🪟 **System tray** — show/hide, start-all, pause-all, and quit; optional
  **minimize-to-tray** and **close-to-tray**.
- 🔔 **Native notifications** on download completion.
- 🚀 **Autostart on login** (Linux `.desktop`, Windows Run key, macOS LaunchAgent)
  and **window-state persistence**.
- 📁 **Native folder picker** for choosing destinations.
- 🔌 **Safe local API** — binds to `127.0.0.1` only (not your LAN) and falls back
  to a free port if 8080 is taken.

**Add downloads**
- ➕ Single or **batch** (paste many URLs at once), with optional checksum,
  category, and scheduled-start.

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
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

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

CI (`.github/workflows/ci.yml`) runs the same build/vet/test gate on every push
and PR. Pushing a `v*` tag triggers `.github/workflows/release.yml`, which builds
and publishes installers for all three OSes.

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

Issues and pull requests are welcome. Please make sure the verification gate is
green before submitting: `cd backend && go build ./... && go vet ./... && go test ./...`,
`cd frontend && npx tsc --noEmit && npm run build`, and `go build -tags webkit2_41 ./...`
at the repo root. The change log lives in [docs/IMPROVEMENTS.md](docs/IMPROVEMENTS.md).

## License

Rum is released under the [MIT License](LICENSE).
