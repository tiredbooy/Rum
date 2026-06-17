# Rum Round 3 — Production Release Overhaul: Design Spec

Date: 2026-06-17
Branch: `dev` (work committed and pushed here)
Status: Approved — implementation pending plan

## Goal

Take Rum from "two enhancement rounds done" to **releasable**. Deliver, across
four path-owned agents working in parallel against a frozen interface contract:

1. Four new/finished engine capabilities (reliability follow-ups; scheduled +
   bandwidth-window downloads; clipboard/link capture; categories &
   auto-organize).
2. Full frontend coverage of those features (built with `ui-ux-pro-max`), plus
   mounting the built-but-unused `ConnectionBanner`.
3. A production-hardened desktop/Wails layer (system tray + minimize/close-to-tray,
   native notifications, autostart + window-state persistence, native
   folder-picker, safe localhost API port).
4. A 3-OS prebuilt-release pipeline (Linux/Windows/macOS) plus polished
   from-source installers and rewritten install docs.

Commits land one-per-area on `dev` with **no `Co-Authored-By: Claude` trailer**.

## Decisions (from brainstorming)

- **Platforms:** Linux + Windows + macOS. macOS builds are **best-effort,
  unsigned, and untested** in this environment (no Mac); CI still produces an
  unsigned `.dmg` and docs explain the Gatekeeper workaround.
- **Features:** all four selected.
- **Distribution:** both prebuilt CI releases **and** polished from-source scripts.
- **Release scope:** prepare everything and **push to `dev`** (no PR required, no
  Claude co-author trailer).
- **Architecture:** keep the existing HTTP/Gin API (serves frontend + CLI + TUI);
  harden it rather than rewrite onto native Wails bindings.

## Architecture (as-found)

Rum is a Wails desktop download manager.

- **Root module** `github.com/tiredbooy/Rum` — Wails app: `main.go` embeds
  `frontend/dist`, launches `go server.Start()` (Gin) on hardcoded `:8080`
  (all interfaces), binds only a dead `Greet`. `tray.go` defines `startTray` but
  **it is never called**. `OnBeforeClose` quits (or confirms) — no tray/minimize.
- **Backend module** `github.com/tiredbooy/Rum/backend`:
  - `internal/pkg/download` — engine: `JobManager`, single-stream + segmented
    multi-connection download, retry/backoff, integrity (`integrity.go`,
    `preflight*.go`, `resume_validate.go`), security (`security.go`), structured
    errors (`errors.go`), SSE progress.
  - `internal/pkg/api` — Gin API (`/api/v1/...`), middleware, SSE.
  - `internal/pkg/scheduler` — **built but not routed**: priority-aware
    concurrency gate over `queue.PriorityQueue`.
  - `internal/pkg/{queue,config,file-system,utils,format,logging,tui}`,
    `cmd/{rum,server}`.
- **Frontend** `frontend/src` — React + TS + Tailwind + shadcn/ui, TanStack
  Query, Zustand, SSE hooks. `useClipBoardUrl` hook and `ConnectionBanner`
  component already exist (banner is **not mounted**).

### Confirmed gaps this round closes

- Tray is dead code; no minimize/close-to-tray; no native notifications;
  no autostart; window state not persisted.
- API binds `:8080` on **all interfaces** (LAN exposure) and hard-fails if the
  port is taken (no fallback).
- Single-file download path skips `PreflightDiskSpace`/`VerifyChecksum`.
- `scheduler` package is unrouted; concurrency is the plain parallelism limit.
- No checksum-entry UI; `ConnectionBanner` unmounted.
- No CI/CD, no prebuilt binaries; install is build-from-source only; no macOS;
  `README.md` is the stock Wails template.

## The four agents — disjoint path ownership

| # | Agent | Owns (edits only here) | Mandate |
|---|-------|------------------------|---------|
| A | Engine | `backend/**` | All 4 backend features + reliability wiring + new API routes + config fields. Backward-compatible (additive) exported APIs. |
| B | Frontend | `frontend/**` | UI for every feature via `ui-ux-pro-max`; mount `ConnectionBanner`; read API base from `App.GetApiBase()`. |
| C | Desktop/Wails | `app.go`, `main.go`, `tray.go`, `wails.json`, root `*.go` (not backend) | Tray + min/close-to-tray, native notifications, autostart + window state, native folder-picker, safe port handshake, app metadata. |
| D | Release/Install/Docs | `installers/**`, `.github/workflows/**`, `build-*.sh`, `installer.iss`, `Rum.desktop`, `INSTALL.md`, `README.md`, `docs/**` | CI release matrix (3 OS) + polished scripts + rewritten docs + improvements writeup. |

### Coordination rules (proven in Rounds 1–2)

- Agents edit **only** owned paths; may **read** anything.
- Out-of-scope needs are recorded as "requested upstream change" in the agent's
  report and applied by the orchestrator at integration — never edited directly.
- Agent A keeps existing **exported** Go signatures backward-compatible
  (additive only) so the root module + CLI/TUI keep compiling.
- **`wails.json` is owned by Agent C** (app metadata + window opts). The `nfpm`
  block inside it is needed by Agent D's `.deb` job; D records any `nfpm` changes
  as a "requested upstream change" and the orchestrator applies them to
  `wails.json` at integration. D never edits `wails.json` directly.
- All four code against the **Frozen Interface Contract** below.

## Frozen Interface Contract

### REST routes (A provides → B consumes)

| Method | Path | Body / Query | Purpose |
|--------|------|--------------|---------|
| POST | `/api/v1/downloads/batch` | `{ urls: string[], options?: CreateOptions }` | Batch / clipboard add. Each URL `ValidateURL`-checked; returns created jobs + per-URL errors. |
| GET | `/api/v1/settings/schedule` | — | Current `ScheduleSettings`. |
| PUT | `/api/v1/settings/schedule` | `ScheduleSettings` | Replace bandwidth windows + scheduled-start config. |
| GET | `/api/v1/settings/categories` | — | Current category rules + toggle. |
| PUT | `/api/v1/settings/categories` | `{ enabled: bool, rules: CategoryRule[] }` | Replace category rules. |
| POST | `/api/v1/downloads` | **extended, additive optional:** `checksum`, `checksum_algo`, `start_at`, `category` | Existing create gains optional fields. |

Existing routes (`retry`, `priority`, `speed-limit`, lists, streams) unchanged.

### Go symbols (A — additive only)

```go
// download/structs.go (Options additive)
type Options struct {
    // ...existing (incl. Checksum, ChecksumAlgo)...
    StartAt    time.Time // zero = start immediately
    Categorize bool
    Category   string    // explicit category override; "" = auto-detect
}

// config (additive)
type BandwidthWindow struct {
    Days    []int  // 0=Sun..6=Sat; empty = every day
    Start   string // "HH:MM" local
    End     string // "HH:MM" local
    LimitKB int    // 0 = unlimited
}
type CategoryRule struct {
    Name       string   // "Video", "Audio", ...
    Extensions []string // [".mp4", ".mkv"]
    DestDir    string   // absolute or relative-to-OutDir
}
type Setting struct {
    // ...existing...
    BandwidthSchedule    []BandwidthWindow
    Categories           []CategoryRule
    EnableCategories     bool
    EnableClipboardWatch bool
    AutoStart            bool
    MinimizeToTray       bool
    CloseToTray          bool
    WindowState          struct{ W, H, X, Y int; Maximized bool }
}

// file-system
func CategorizeFile(path string, rules []CategoryRule) (dest string, err error)

// engine
// - StartAllJobs routes through scheduler.Scheduler (priority-aware concurrency).
// - ScheduleController: leak-free ticker; computes active bandwidth limit and
//   honors per-job StartAt. Mounted in cmd/server by the orchestrator.
```

`config.Setting` gains desktop-owned fields (AutoStart/MinimizeToTray/
CloseToTray/WindowState) because settings live in the backend module that the
root module imports; Agent A adds them, Agent C reads/writes them via the API or
the config package.

### Desktop ↔ backend safe-port handshake (A + C + B)

- `server.Start()` binds `127.0.0.1`, prefers port **8080**, falls back to a
  dynamic free port (`127.0.0.1:0`) if taken; records the actual base URL.
- Agent C exposes a Wails-bound method `func (a *App) GetApiBase() string`.
- Agent B's API client resolves base URL via `window.go.main.App.GetApiBase()`
  when available, else falls back to `http://127.0.0.1:8080` (preserves
  `wails dev` / browser dev).

### TS types (B)

```ts
interface BandwidthWindow { days: number[]; start: string; end: string; limitKB: number }
interface CategoryRule { name: string; extensions: string[]; destDir: string }
interface ScheduleSettings { windows: BandwidthWindow[]; scheduledStartEnabled: boolean }
// DownloadRequest extended: checksum?, checksumAlgo?, startAt?, category?
```

## Agent A — Engine details

Skills: `go-concurrency`, `go-api-design`, `security-audit`.

1. **Reliability follow-ups:** wire `PreflightDiskSpace` + `VerifyChecksum` into
   `DownloadSingleFile`; route `StartAllJobs` through `scheduler.Scheduler` so
   concurrency is priority-aware (preserve existing behavior/tests); accept
   `checksum`/`checksum_algo` on create and thread to `Options`.
2. **Scheduled + bandwidth windows:** add `BandwidthWindow` config; a leak-free
   `ScheduleController` ticker recomputes the active speed limit and starts jobs
   whose `StartAt` is due; `GET|PUT /settings/schedule`. `-race` tests for window
   selection + day matching + boundary times.
3. **Clipboard/link capture (server):** `POST /downloads/batch` parsing many
   URLs (reuse `getUrlsFromTxt` logic), each `ValidateURL`-checked; returns
   created + per-URL errors.
4. **Categories & auto-organize:** `CategorizeFile` applied in the finalize path
   of **both** single + segmented downloads (reuse `SafeJoin`/`SanitizeFileName`);
   `GET|PUT /settings/categories`.

## Agent B — Frontend details

Skill: `ui-ux-pro-max`.

- Settings: bandwidth-window editor, scheduled-start toggle, category manager,
  desktop toggles (autostart / minimize-to-tray / close-to-tray / clipboard
  watch — these PUT to settings).
- Add-download: optional checksum + algo fields, optional category select,
  optional start-at.
- Clipboard add affordance (build on `useClipBoardUrl`) + batch add-from-text
  (textarea → `POST /downloads/batch`).
- Category badges on download cards.
- **Mount `ConnectionBanner`**; API client reads base from `App.GetApiBase()`
  with fallback.
- Keep accessibility/empty/error-state quality from prior rounds.

## Agent C — Desktop/Wails details

Skills: `go-concurrency`.

- **Tray:** call `startTray`; menu = Show / Pause-all (hits API) / Quit; tray
  icon tooltip + title.
- **Minimize/close-to-tray:** `OnBeforeClose` + minimize handlers respect
  `CloseToTray`/`MinimizeToTray`; quitting still possible from tray + confirm.
- **Native notifications:** on download complete/failed (subscribe to engine
  events or poll stats); clickable → `WindowShow`.
- **Autostart:** toggle writes an OS autostart entry (Linux `.desktop` in
  autostart dir, Windows registry Run key, macOS LaunchAgent) gated by
  `AutoStart`.
- **Window state:** restore on startup from `WindowState`; persist on resize/move/close.
- **Native folder-picker:** bound `App.ChooseDir() (string, error)` using
  `runtime.OpenDirectoryDialog`; frontend uses it for download dirs.
- **Safe port:** consume the handshake; expose `GetApiBase()`.
- **Metadata:** real `wails.json` author/homepage/IDs; clipboard watcher emits a
  Wails event when `EnableClipboardWatch` (opt-in).

## Agent D — Release/Install/Docs details

- **CI** (`.github/workflows/release.yml`): on `v*` tag → matrix build:
  - Linux: `wails build` → **AppImage** + **`.deb`** (nfpm config already in
    `wails.json`).
  - Windows: `wails build` + **Inno Setup** → `Rum-Setup.exe`.
  - macOS: `wails build` → unsigned **`.dmg`** (best-effort).
  - Upload all to a GitHub Release. Plus a `ci.yml` running build/vet/test on PRs.
- **Scripts:** polish `installers/{cli,gui}/install-*.sh|ps1` (dep detection,
  clearer errors); add macOS install scripts.
- **Docs:** rewrite `INSTALL.md` to lead with "download the prebuilt installer",
  build-from-source second; replace stock `README.md`; Round-3 entry in
  `docs/IMPROVEMENTS.md`.

## Orchestrator integration (me)

1. Wire the safe-port handshake across A/C/B.
2. Mount `ScheduleController` in `cmd/server/main.go`.
3. Reconcile `go.sum` in both modules for any new deps.
4. Confirm contract symbols/routes/types align; resolve "requested upstream
   changes" from agent reports.
5. Run the verification gate; commit per area; push to `dev`.

## Testing / verification gate

- Backend: `cd backend && go build ./... && go vet ./... && go test ./...`
  (`-race` on `download`, `scheduler`, `queue`).
- Root + frontend: `cd frontend && npm ci && npx tsc --noEmit && npm run build`;
  root `go build ./...`; `wails build -tags webkit2_41` if WebKit2GTK present
  (else component builds + a noted caveat).
- CI YAML validated logically; real proof on push/tag.

## Delivery

- Commits (one per area, **no Claude co-author trailer**), pushed to `dev`:
  `feat(engine): ...`, `feat(frontend): ...`, `feat(desktop): ...`,
  `ci/installers: ...`, `docs: ...`.

## Risks / limitations

- **macOS unsigned/untested** — Gatekeeper warning; documented workaround; `.dmg`
  job best-effort.
- Full `wails build` needs WebKit2GTK in this env — fall back to component builds
  if absent and note it.
- New background goroutines (schedule controller, clipboard watch) must be
  leak-free — `-race` tests + opt-in gating.
- Autostart is OS-specific — implement per-OS with safe no-ops where unsupported.
