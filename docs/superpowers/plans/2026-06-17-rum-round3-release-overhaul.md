# Rum Round 3 — Production Release Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Rum as a releasable cross-platform desktop download manager: four new/finished engine features, full UI coverage, a production-hardened Wails layer, and a 3-OS prebuilt-release pipeline.

**Architecture:** Keep the existing HTTP/Gin API model (serves the desktop frontend; CLI/TUI use the engine directly) and harden it. Four path-owned agents (Engine `backend/**`, Frontend `frontend/**`, Desktop `app.go|main.go|tray.go|wails.json`, Release `installers|.github|build-*|docs`) build in parallel against the Frozen Interface Contract below; the orchestrator wires cross-scope integration, runs the gate, commits per-area, and pushes to `dev`.

**Tech Stack:** Go 1.25 (backend module + root Wails module), Wails v2, Gin, `golang.org/x/time/rate`, `golang.org/x/sync/errgroup`; React 19 + TypeScript + Vite + Tailwind + shadcn/ui + TanStack Query + Zustand; GitHub Actions + nfpm + Inno Setup + AppImage.

## Global Constraints

These apply to **every task** — copied verbatim from the spec (`docs/superpowers/specs/2026-06-17-rum-round3-release-overhaul-design.md`):

- **Branch:** all work commits to `dev`. **No `Co-Authored-By: Claude` trailer** in any commit message.
- **Backward compatibility:** Agent A only makes **additive** changes to exported Go symbols (new fields/funcs/routes). Never change an existing exported signature or JSON field — the root module, CLI, and TUI must keep compiling, and the existing frontend must keep working.
- **Path ownership:** each agent edits ONLY its owned paths (table in spec §"four agents"). Out-of-scope needs are recorded as a "requested upstream change" in the agent's final report, applied by the orchestrator at integration. **`wails.json` is owned by Agent C**; Agent D routes any `nfpm` changes through the orchestrator.
- **Go version floor:** backend `go 1.25.7`, root `go 1.25.7`. Don't add heavyweight deps; prefer stdlib + already-present `golang.org/x/{time,sync}`.
- **Platforms:** Linux + Windows + macOS. macOS is **best-effort, unsigned, untested** — never claim it is verified.
- **Verification gate (must pass before "done"):**
  - backend: `cd backend && go build ./... && go vet ./... && go test ./...` (`-race` on `download`, `scheduler`, `queue`)
  - frontend: `cd frontend && npm ci && npx tsc --noEmit && npm run build`
  - root: `go build ./...`; `wails build -tags webkit2_41` if WebKit2GTK present (else component builds + note).
- **Tests are TDD:** write the failing test first, watch it fail, implement, watch it pass, commit.

## Frozen Interface Contract (reconciled with real code)

**REST (Agent A provides → B consumes), additive:**
| Method | Path | Body / Query |
|--------|------|--------------|
| POST | `/api/v1/downloads/batch` | `{ "urls": string[], "options"?: {dest_path?, speed_limit?, max_retries?, group_folder?, category?, checksum?, checksum_algo?, start_at?} }` → `{ "created": DownloadResponse[], "errors": {url,error}[] }` |
| GET | `/api/v1/settings/schedule` | → `{ "scheduled_start_enabled": bool, "rules": SpeedRule[] }` |
| PUT | `/api/v1/settings/schedule` | `{ "scheduled_start_enabled": bool, "rules": SpeedRule[] }` |
| GET | `/api/v1/settings/categories` | → `{ "enabled": bool, "rules": CategoryRule[] }` |
| PUT | `/api/v1/settings/categories` | `{ "enabled": bool, "rules": CategoryRule[] }` |
| POST | `/api/v1/downloads` | **extended additive optional:** `checksum`, `checksum_algo`, `start_at` (RFC3339), `category` |

**Go additive symbols (Agent A):**
```go
// config/setting.go — EXTEND existing SpeedRule (do NOT add a new BandwidthWindow type)
type SpeedRule struct {
    StartHour int   `json:"start_hour"`     // existing, 0-23
    EndHour   int   `json:"end_hour"`       // existing, 0-23
    LimitKBps int   `json:"limit_kbps"`     // existing, 0 = unlimited
    Days      []int `json:"days,omitempty"` // NEW: 0=Sun..6=Sat; empty = every day
}

// config/setting.go — NEW type + NEW Setting fields (all additive)
type CategoryRule struct {
    Name       string   `json:"name"`
    Extensions []string `json:"extensions"` // e.g. [".mp4",".mkv"]
    DestDir    string   `json:"dest_dir"`   // abs, or relative to OutDir
}
// added to Setting:
//   ScheduledStartEnabled bool          `json:"scheduled_start_enabled"`
//   EnableCategories      bool          `json:"enable_categories"`
//   Categories            []CategoryRule `json:"categories,omitempty"`
//   LaunchOnStartup       bool          `json:"launch_on_startup"`   // OS login autostart
//   MinimizeToTray        bool          `json:"minimize_to_tray"`
//   CloseToTray           bool          `json:"close_to_tray"`
//   EnableClipboardWatch  bool          `json:"enable_clipboard_watch"`
//   WindowState           WindowState   `json:"window_state"`
// type WindowState struct { W,H,X,Y int; Maximized bool } with json tags w,h,x,y,maximized

// download/structs.go — Options additive (Checksum/ChecksumAlgo already exist)
//   StartAt    time.Time
//   Categorize bool
//   Category   string

// download — new shared primitives
type SpeedGovernor struct { /* atomic *rate.Limiter, SetLimitKBps(int), Wrap(ctx,io.ReadCloser) io.ReadCloser */ }
func NewSpeedGovernor(kbps int) *SpeedGovernor
// JobManager gains: SetSpeedLimit(kbps int); adopts *scheduler.Scheduler as its concurrency gate.

// file-system/categorize.go
func CategorizeFile(path string, rules []CategoryRule) (dest string, err error) // NOTE: rules typed via a small local mirror to avoid import cycle — see Task A5.

// cmd/server — safe-port handshake
func Listen() (baseURL string, err error) // binds 127.0.0.1, prefers :8080, falls back to :0
func Serve()                              // runs the bound server (blocking; call in goroutine)
func APIBase() string                     // returns resolved base, e.g. "http://127.0.0.1:8080"
```

**Desktop ↔ backend (Agent C):** `func (a *App) GetApiBase() string` (returns `server.APIBase()`), `func (a *App) ChooseDir() (string, error)` (native dir dialog). Frontend reads base via `window.go.main.App.GetApiBase()` with fallback.

**TS types (Agent B), in `_lib/types`:**
```ts
interface SpeedRule { startHour: number; endHour: number; limitKbps: number; days?: number[] }
interface ScheduleSettings { scheduledStartEnabled: boolean; rules: SpeedRule[] }
interface CategoryRule { name: string; extensions: string[]; destDir: string }
interface CategorySettings { enabled: boolean; rules: CategoryRule[] }
// DownloadReq extended: checksum?, checksumAlgo?, startAt?, category?
```

---

## File Structure Map

**Agent A — Engine (`backend/**`)**
- Create: `backend/internal/pkg/download/governor.go` (+ `governor_test.go`) — `SpeedGovernor`.
- Create: `backend/internal/pkg/download/schedule.go` (+ `schedule_test.go`) — `ScheduleController` + active-window math.
- Create: `backend/internal/pkg/file-system/categorize.go` (+ `categorize_test.go`) — `CategorizeFile`.
- Create: `backend/internal/pkg/api/handlers/schedule.go`, `categories.go`, `batch.go` — new handlers.
- Modify: `download/structs.go` (Options fields), `download/manager.go` (scheduler adoption, SetSpeedLimit, categorize/StartAt wiring), `download/download.go` (single-file preflight+checksum+governor), `download/segmented.go` (governor + categorize), `config/setting.go` (new fields/Update/Validate/defaults), `api/dto/*` (new DTOs + extended create), `api/routes/routes.go` (new routes), `cmd/server/main.go` (Listen/Serve/APIBase + mount controller).

**Agent B — Frontend (`frontend/**`)**
- Modify: `src/_lib/services/api/api.ts` (base URL resolution), `src/features/layout/Layout.tsx` (mount ConnectionBanner), `src/stores/download-progress-store.ts` (online flag), `src/_lib/types/*`, `src/features/settings/SettingForm.tsx` (+ new section components), `src/features/dashboard/create-download/*` (checksum/category/start-at/batch/clipboard), `src/features/dashboard/download-list/*` (category badge).
- Create: `src/features/settings/ScheduleEditor.tsx`, `CategoryManager.tsx`, `DesktopSettings.tsx`; `src/_lib/services/api/schedule-api.ts`, `categories-api.ts`; `src/_lib/services/queries/schedule.queries.ts`, `categories.queries.ts`; `src/_lib/wails.ts` (typed binding wrapper).

**Agent C — Desktop (`app.go`, `main.go`, `tray.go`, `wails.json`)**
- Modify all four. Create: `autostart_linux.go`, `autostart_windows.go`, `autostart_darwin.go` (build-tagged) at repo root.

**Agent D — Release (`installers/**`, `.github/workflows/**`, `build-*.sh`, `installer.iss`, `Rum.desktop`, `INSTALL.md`, `README.md`, `docs/**`)**
- Create: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `installers/gui/install-macos.sh`, `installers/cli/install-macos.sh`.
- Modify: existing install scripts, `INSTALL.md`, `README.md`, `docs/IMPROVEMENTS.md`.

---

# AGENT A — ENGINE

Skills to load first: `go-concurrency` (governor, schedule controller, scheduler adoption), `go-api-design` (new handlers/routes), `security-audit` (checksum/URL paths). All changes additive.

### Task A1: Safe-port handshake in the server

**Files:**
- Modify: `backend/cmd/server/main.go`
- Test: `backend/cmd/server/listen_test.go` (new)

**Interfaces:**
- Produces: `server.Listen() (baseURL string, err error)`, `server.Serve()`, `server.APIBase() string`.
- Consumed by: Agent C `main.go` (`go`-side) and `App.GetApiBase()`.

- [ ] **Step 1 — failing test.** `listen_test.go`:
```go
package server

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListenBindsLoopbackAndServes(t *testing.T) {
	base, err := Listen()
	if err != nil { t.Fatalf("Listen: %v", err) }
	if !strings.HasPrefix(base, "http://127.0.0.1:") { t.Fatalf("want loopback base, got %q", base) }
	if APIBase() != base { t.Fatalf("APIBase()=%q want %q", APIBase(), base) }
	go Serve()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(base + "/api/v1/downloads/stats"); err == nil { resp.Body.Close(); return }
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not answer on the bound port")
}
```
- [ ] **Step 2 — run, expect FAIL** (`Listen` undefined): `cd backend && go test ./cmd/server/ -run TestListenBinds -v`
- [ ] **Step 3 — implement.** Refactor `Start()` into `Listen()`/`Serve()`. `Listen()` builds the router (InitAPI + middlewares + routes as today), creates `net.Listener` via `net.Listen("tcp", "127.0.0.1:8080")`; on `EADDRINUSE` retry with `127.0.0.1:0`; store the listener + `apiBase = "http://" + ln.Addr().String()` in package vars guarded by a mutex; build `srv` (same timeouts as current). `Serve()` runs `srv.Serve(ln)` and keeps the existing signal-driven graceful shutdown. `APIBase()` returns the stored base. Keep a thin `Start()` that calls `Listen()` then `Serve()` so any existing caller still works.
- [ ] **Step 4 — run, expect PASS.**
- [ ] **Step 5 — commit** (no trailer): `git commit -am "feat(engine): bind API to loopback with dynamic-port fallback"`

### Task A2: Disk preflight + checksum on the single-file path

**Files:**
- Modify: `backend/internal/pkg/download/download.go`
- Test: `backend/internal/pkg/download/download_single_integrity_test.go` (new)

**Interfaces:**
- Consumes: `PreflightDiskSpace(dir string, needBytes int64) error`, `VerifyChecksum(path, algo, expected string) error`, `Options{Checksum, ChecksumAlgo}`.
- Produces: single-file downloads now fail with `ErrInsufficientDiskSpace` before allocating and `ErrChecksumMismatch` after assembling when a checksum is supplied.

- [ ] **Step 1 — failing test** (httptest server serving a known body; call `DownloadSingleFile` with a wrong `Options.Checksum` and assert `errors.Is(err, ErrChecksumMismatch)`; second case with the correct sha256 asserts `nil` and file bytes match). Mirror the style already in `integrity_test.go`/`engine_test.go`.
- [ ] **Step 2 — run, expect FAIL.**
- [ ] **Step 3 — implement.** In `DownloadSingleFile`, after computing the output path and the remote `TotalSize` but before creating/truncating the file, call `PreflightDiskSpace(filepath.Dir(fullPath), remaining)` and return the error if non-nil. After the copy loop completes and the file is synced+closed (success path), call `VerifyChecksum(fullPath, opt.ChecksumAlgo, opt.Checksum)` and return its error (a no-op when `opt.Checksum == ""`). Place the checksum call so it runs once per successful download, not per retry attempt.
- [ ] **Step 4 — run, expect PASS** (`-race`).
- [ ] **Step 5 — commit:** `feat(engine): disk preflight and checksum verify on single-file downloads`

### Task A3: Adopt the priority scheduler as the concurrency gate

**Files:**
- Modify: `backend/internal/pkg/download/manager.go`
- Test: `backend/internal/pkg/download/manager_scheduler_test.go` (new)

**Interfaces:**
- Consumes: `scheduler.New(max)`, `(*Scheduler).Submit(id, queue.Priority)`, `.Next(ctx)`, `.Done(id)`, `.SetMax(n)`, `.Close()`, `scheduler.ParsePriority(string)`.
- Produces: `JobManager` runs at most `opt.Parallel` downloads, highest priority first; `(*JobManager).SetSpeedLimit(kbps int)` (used by A4); dispatcher is leak-free.

- [ ] **Step 1 — failing test.** Submit 5 jobs (mix of priorities) against a manager whose `opt.Parallel == 1` pointed at a slow httptest server; assert that at any instant `sched.Running() <= 1` and that a `high` job completes before a `low` job queued earlier. Assert no goroutine leak using `runtime.NumGoroutine()` before/after (with a settle delay) or `go test -race`.
- [ ] **Step 2 — run, expect FAIL.**
- [ ] **Step 3 — implement.** Add `sched *scheduler.Scheduler` to `JobManager`; construct in `NewJobManager` with `scheduler.New(opt.Parallel)`. Start ONE dispatcher goroutine (in `NewJobManager` or lazily on first start) that loops: `id, ok := m.sched.Next(m.dispatchCtx); if !ok { return }`; look up the job; if it's gone or no longer pending/paused, `m.sched.Done(id)` and continue; else launch `runDownload` in a goroutine whose defer calls `m.sched.Done(id)`. Replace the `sem` acquire/release in `StartJob`/`runDownload` with `m.sched.Submit(id, scheduler.ParsePriority(job.GetPriority()))`. `StartAllJobs` becomes: collect eligible jobs (still honoring the existing priority sort for deterministic Submit order) and Submit each. Add `SetSpeedLimit(kbps int)` storing under `mu` into `opt.SpeedLimit` (used live by the governor in A4) and `SetMax` passthrough when `MaxParallel` changes. Keep `PauseJob`/`PauseAllJobs`/cancellation working (cancel still calls the job's CancelFunc; a paused/cancelled job that was queued is skipped at dispatch). Close the scheduler + cancel `dispatchCtx` on a new `(*JobManager).Shutdown()` (wired by orchestrator in cmd/server shutdown).
- [ ] **Step 4 — run, expect PASS** (`-race`); also run the full `go test ./internal/pkg/download/ -race` to confirm existing tests still pass.
- [ ] **Step 5 — commit:** `feat(engine): route job concurrency through the priority scheduler`

### Task A4: SpeedGovernor + live bandwidth windows + per-job StartAt

**Files:**
- Create: `backend/internal/pkg/download/governor.go`, `governor_test.go`, `schedule.go`, `schedule_test.go`
- Modify: `download/download.go`, `download/segmented.go` (use the governor), `download/structs.go` (Options.StartAt), `config/setting.go` (SpeedRule.Days, ScheduledStartEnabled, Update/Validate), `api/handlers/schedule.go` (new), `api/routes/routes.go`
- Test: as above + `config/setting_test.go` additions

**Interfaces:**
- Produces: `NewSpeedGovernor(kbps int) *SpeedGovernor` with `SetLimitKBps(int)` and `Wrap(ctx, io.ReadCloser) io.ReadCloser`; `NewScheduleController(...)` ticking the governor + starting due `StartAt` jobs; `activeLimitKBps(rules []config.SpeedRule, now time.Time, fallback int) int` (pure, tested).
- Consumes: `(*rate.Limiter).SetLimit/SetBurst`, `JobManager.SetSpeedLimit`.

- [ ] **Step 1 — failing tests.**
  - `governor_test.go`: `NewSpeedGovernor(0)` Wrap returns the reader unchanged (unlimited); a governor at 100 KBps throttles ~N bytes to ≥ expected duration; `SetLimitKBps` mid-stream changes the effective rate.
  - `schedule_test.go`: table-test `activeLimitKBps` — a rule `{StartHour:0,EndHour:8,LimitKBps:500}` returns 500 at 03:00 and the fallback at 12:00; a rule with `Days:[1]` (Mon) only applies on Monday; overlapping rules pick the most restrictive non-zero limit; wrap-around `{StartHour:22,EndHour:6}` matches 23:00 and 02:00.
- [ ] **Step 2 — run, expect FAIL.**
- [ ] **Step 3 — implement.**
  - `governor.go`: struct holding `atomic.Pointer[rate.Limiter]` (nil = unlimited). `SetLimitKBps(kb)`: if `kb<=0` store nil; else build/replace a limiter (`rate.NewLimiter(rate.Limit(kb*1024), burst)`), reuse `speedLimitBytesPerSec`/burst logic from `ratelimit_math.go`. `Wrap(ctx, rc)`: if current limiter nil return `rc`; else return a reader that loads the current limiter pointer each `Read` and `WaitN`s (so live `SetLimitKBps` is honored mid-download).
  - `download.go` + `segmented.go`: replace per-download `newSpeedLimiter(...)` usage with the manager's shared `*SpeedGovernor` (`opt.Governor` field, set by the manager) — global shared budget across concurrent downloads (document this as intended). Fallback: if `opt.Governor == nil`, keep the existing per-download limiter so engine/CLI callers without a governor are unaffected.
  - `config/setting.go`: add `Days []int` to `SpeedRule`; add `ScheduledStartEnabled bool`; clamp `Days` entries to 0..6 in `Validate`; handle them in `Update` (schedule handled by dedicated handler, but keep `Validate` correct).
  - `structs.go`: add `StartAt time.Time` and `Governor *SpeedGovernor` to `Options`.
  - `schedule.go`: `ScheduleController` with a ticker (e.g. 30s) goroutine, context-cancelable (leak-free), that each tick computes `activeLimitKBps(setting.BandwidthSchedule, time.Now(), setting.SpeedLimitKB)` and calls `manager.SetSpeedLimit(...)` + `governor.SetLimitKBps(...)`, and (if `ScheduledStartEnabled`) starts any pending job whose `StartAt` is non-zero and `<= now`.
  - `api/handlers/schedule.go`: `GetSchedule`/`PutSchedule` reading/writing `Setting.BandwidthSchedule` + `ScheduledStartEnabled` via load → mutate → `Save()`; routes `GET|PUT /settings/schedule`.
- [ ] **Step 4 — run, expect PASS** (`-race` on download).
- [ ] **Step 5 — commit:** `feat(engine): live bandwidth windows, speed governor and scheduled-start`

### Task A5: Categories & auto-organize

**Files:**
- Create: `backend/internal/pkg/file-system/categorize.go`, `categorize_test.go`, `backend/internal/pkg/api/handlers/categories.go`
- Modify: `config/setting.go` (CategoryRule, EnableCategories, Categories, Update/Validate/defaults), `download/download.go` + `download/segmented.go` (apply on finalize), `download/structs.go` (Categorize, Category — already added in contract), `api/routes/routes.go`
- Test: `categorize_test.go`, finalize wiring covered by an engine test

**Interfaces:**
- Produces: `filesystem.CategorizeFile(path string, rules []filesystem.CategoryRule) (dest string, err error)` and a `filesystem.CategoryRule{Name, Extensions, DestDir}` mirror type (to avoid a `config`→`filesystem` import cycle; `config.CategoryRule` is converted at the call site in the engine). `GET|PUT /settings/categories`.
- Consumes: `SafeJoin`, `SanitizeFileName`, `CreateGroupFolder`.

- [ ] **Step 1 — failing test.** `categorize_test.go`: given rules `[{Video,[.mp4],"Videos"}]` and a file `a.mp4` under a temp base, `CategorizeFile` returns `<base>/Videos/a.mp4` and the function (or caller) creates the dir; an extension with no matching rule returns the original path unchanged; path-traversal in `DestDir` is neutralized via `SafeJoin`.
- [ ] **Step 2 — run, expect FAIL.**
- [ ] **Step 3 — implement.** `categorize.go`: match the file's lowercased extension against each rule's `Extensions`; on match compute `dest` via `SafeJoin(resolveBase(rule.DestDir), filepath.Base(path))` (absolute `DestDir` used as-is after sanitization; relative resolved against the file's current dir/OutDir); `CreateGroupFolder(dir)`; return dest (no move here — pure path resolution + dir creation, the engine performs the rename so it can honor `FileConflict`). In `download.go`/`segmented.go` success paths, when `opt.Categorize`, resolve dest and `os.Rename` the finished file (atomic same-filesystem; fall back to copy+remove across filesystems), respecting existing conflict handling. `config/setting.go`: add `CategoryRule`, `EnableCategories`, `Categories`; merge in `Update`; in `Validate` drop rules with empty `Name`/`Extensions`. `categories.go` handler: `GET|PUT /settings/categories` via load→mutate→Save.
- [ ] **Step 4 — run, expect PASS.**
- [ ] **Step 5 — commit:** `feat(engine): category-based auto-organize and settings endpoints`

### Task A6: Batch create endpoint + extended create DTO

**Files:**
- Create: `backend/internal/pkg/api/handlers/batch.go`
- Modify: `api/dto/*` (extend `CreateDownloadRequest` with `Checksum`,`ChecksumAlgo`,`StartAt`,`Category`; add `BatchCreateRequest`/`BatchCreateResponse`), `api/handlers/*` create handler (thread new fields into `Options`), `api/routes/routes.go`, `download/manager.go` (`CreateJobsFromURLs` already validates; add a variant or pass per-job options through)
- Test: `backend/internal/pkg/api/handlers/batch_test.go` (httptest via the gin engine)

**Interfaces:**
- Produces: `POST /downloads/batch` → `{created:[], errors:[{url,error}]}`; create accepts checksum/algo/start_at/category (additive optional, existing clients unaffected).
- Consumes: `GlobalManager.CreateJobsFromURLs`, `dto.*`, `download.Classify`.

- [ ] **Step 1 — failing test.** POST `/api/v1/downloads/batch` with two valid URLs + one bad scheme → 207/200 body has `created` length 2 and `errors` length 1 with the bad URL; assert the existing `POST /downloads` still works with NO new fields (backward-compat).
- [ ] **Step 2 — run, expect FAIL.**
- [ ] **Step 3 — implement.** Extend `CreateDownloadRequest` with the four optional fields (json `checksum`,`checksum_algo`,`start_at`,`category`; parse `start_at` RFC3339 → `time.Time`). Thread them into the `Options`/job creation path used by `CreateDownload`. Add `BatchCreateRequest{URLs []string; Options ...}`; `batch.go` validates each URL with `download.ValidateURL`, creates jobs for the valid ones, returns per-URL errors for the rest using the standard envelope codes via `download.Classify`. Register the route.
- [ ] **Step 4 — run, expect PASS.**
- [ ] **Step 5 — commit:** `feat(engine): batch create endpoint and checksum/category/start-at on create`

### Task A7: Desktop-facing settings fields

**Files:**
- Modify: `config/setting.go` (add `LaunchOnStartup`,`MinimizeToTray`,`CloseToTray`,`EnableClipboardWatch`,`WindowState` + `WindowState` type; `SettingReq` pointers; `Update`; `Validate`; `setDefaults`/`applyMissingDefaults`)
- Test: `config/setting_test.go` additions

**Interfaces:**
- Produces: the new persisted settings fields + their `PATCH /settings` handling.
- Consumed by: Agent C (reads via config / API) and Agent B (toggles).

- [ ] **Step 1 — failing test.** Marshal a `Setting` with the new fields set, `Save()`, reload via `LoadSettingMetadata()`, assert round-trip; `Update` with a `SettingReq` toggling `MinimizeToTray` persists it; defaults: `CloseToTray=false`, `MinimizeToTray=false`, `LaunchOnStartup=false`, `EnableClipboardWatch=false`.
- [ ] **Step 2 — run, expect FAIL.**
- [ ] **Step 3 — implement.** Add the fields + `WindowState struct{W,H,X,Y int `json:"..."`; Maximized bool `json:"maximized"`}`; extend `SettingReq` with the matching `*bool`/`*WindowState`; merge in `Update`; set defaults; no clamping needed beyond booleans (WindowState validated lazily by Agent C).
- [ ] **Step 4 — run, expect PASS.**
- [ ] **Step 5 — commit:** `feat(engine): persist desktop preferences (tray, autostart, window state)`

**Agent A final report must list:** any "requested upstream change" for the orchestrator (e.g. mounting `ScheduleController` and calling `JobManager.Shutdown()` in `cmd/server`), and the exact resolved route/symbol names so B/C can finalize.

---

# AGENT B — FRONTEND

Skill to load first: **`ui-ux-pro-max`** (use it to plan/build/review every new UI surface; match the existing shadcn/ui + Tailwind look). Build against the Frozen Interface Contract; if an endpoint isn't live yet, code to the contract and verify at integration. Verification for each task: `cd frontend && npx tsc --noEmit && npm run build`.

Conventions (confirmed): API client `src/_lib/services/api/api.ts` (`API_URL` + `request<T>()`); per-domain api modules in `src/_lib/services/api/`; TanStack Query hooks in `src/_lib/services/queries/`; types in `src/_lib/types/`; feature UI under `src/features/<feature>/`; Settings UI is `src/features/settings/SettingForm.tsx`; SSE in `src/hooks/useAllProgressStream.tsx`; `ConnectionBanner` is `src/components/connection-banner.tsx` (props `{online, message?, className?}`) and is **not mounted**.

### Task B1: API base via Wails binding + mount ConnectionBanner

**Files:**
- Create: `src/_lib/wails.ts`
- Modify: `src/_lib/services/api/api.ts`, `src/stores/download-progress-store.ts`, `src/features/layout/Layout.tsx`, `src/hooks/useAllProgressStream.tsx`

**Interfaces:**
- Consumes: `window.go.main.App.GetApiBase()` (Agent C). Online state from the SSE hook's `onopen`/`onerror`.
- Produces: a single `apiBase()` resolver used by `request()` and SSE.

- [ ] **Step 1.** `wails.ts`: `export async function getApiBase(): Promise<string>` that calls `window.go?.main?.App?.GetApiBase?.()` inside try/catch and falls back to `import.meta.env.VITE_API_URL ?? "http://127.0.0.1:8080"`. Export a memoized resolved value + a synchronous `apiBaseSync()` (resolved once at boot) so existing synchronous `${API_URL}` usages keep working.
- [ ] **Step 2.** `api.ts`: resolve the base from `wails.ts` at module init (top-level `await` is fine in Vite ESM, or resolve in `main.tsx` before render and store). Keep `request<T>()` shape identical.
- [ ] **Step 3.** `download-progress-store.ts`: add `online: boolean` + `setOnline(b)`. In `useAllProgressStream.tsx`, set `online=true` on `onopen`, `online=false` on `onerror`/close.
- [ ] **Step 4.** `Layout.tsx`: render `<ConnectionBanner online={online} />` at the top of the main panel (read `online` from the store).
- [ ] **Step 5.** Verify (`tsc --noEmit && build`), then commit: `feat(frontend): resolve API base from Wails binding and mount connection banner`

### Task B2: Bandwidth-schedule editor + scheduled-start toggle

**Files:**
- Create: `src/_lib/services/api/schedule-api.ts`, `src/_lib/services/queries/schedule.queries.ts`, `src/features/settings/ScheduleEditor.tsx`
- Modify: `src/_lib/types/setting-types.ts` (add `SpeedRule`, `ScheduleSettings`), `src/features/settings/SettingForm.tsx` (add a "Bandwidth schedule" section rendering `<ScheduleEditor/>`)

**Interfaces:** `GET|PUT /settings/schedule` ⇄ `ScheduleSettings { scheduledStartEnabled, rules: SpeedRule[] }`, `SpeedRule { startHour, endHour, limitKbps, days? }`.

- [ ] **Step 1.** Types + api module (`getSchedule()`, `putSchedule(s)`) + queries (`useSchedule()`, `useUpdateSchedule()` with optimistic cache update + rollback, matching the existing priority pattern).
- [ ] **Step 2.** `ScheduleEditor.tsx` (built with `ui-ux-pro-max`): a list of rules; each row = start-hour select (0–23), end-hour select, limit input (KB/s, 0 = unlimited), optional day-of-week multiselect; add/remove row; a top toggle for `scheduledStartEnabled`; Save calls `useUpdateSchedule`. Empty state + helper text ("e.g. throttle to 500 KB/s during 09:00–18:00"). Accessible labels on every control.
- [ ] **Step 3.** Wire into `SettingForm.tsx` as a new collapsible section.
- [ ] **Step 4.** Verify + commit: `feat(frontend): bandwidth-schedule editor and scheduled-start toggle`

### Task B3: Category manager

**Files:**
- Create: `src/_lib/services/api/categories-api.ts`, `src/_lib/services/queries/categories.queries.ts`, `src/features/settings/CategoryManager.tsx`
- Modify: `setting-types.ts` (`CategoryRule`, `CategorySettings`), `SettingForm.tsx`

**Interfaces:** `GET|PUT /settings/categories` ⇄ `CategorySettings { enabled, rules: CategoryRule[] }`.

- [ ] **Step 1.** Types + api + queries (optimistic, like B2).
- [ ] **Step 2.** `CategoryManager.tsx` (`ui-ux-pro-max`): master toggle `enabled`; rule rows (name, extensions as a tag/chips input, destination dir with a "Browse" button that calls `window.go.main.App.ChooseDir()` when available else a text input); add/remove; sensible default rules offered (Video/Audio/Documents/Archives/Images) as a "Reset to defaults" affordance. Validation: non-empty name + ≥1 extension before Save.
- [ ] **Step 3.** Wire into `SettingForm.tsx`.
- [ ] **Step 4.** Verify + commit: `feat(frontend): category auto-organize manager`

### Task B4: Desktop preference toggles

**Files:**
- Create: `src/features/settings/DesktopSettings.tsx`
- Modify: `setting-types.ts` (`Setting`/`SettingReq` gain `launch_on_startup`,`minimize_to_tray`,`close_to_tray`,`enable_clipboard_watch`), `SettingForm.tsx`

**Interfaces:** existing `PATCH /settings` (already supports partial updates via `useUpdateSettings`). Add the new boolean fields to the TS `Setting`/`SettingReq` shapes.

- [ ] **Step 1.** Extend the TS settings types with the four booleans (snake_case to match the Go json tags).
- [ ] **Step 2.** `DesktopSettings.tsx` (`ui-ux-pro-max`): switches for Launch on startup, Minimize to tray, Close to tray (with helper "keep running in the tray when the window is closed"), and Watch clipboard for links (with a privacy note). Each switch persists on change via `useUpdateSettings` (onBlur/onChange pattern already used in `SettingForm`).
- [ ] **Step 3.** Wire into `SettingForm.tsx` under a "Desktop" section.
- [ ] **Step 4.** Verify + commit: `feat(frontend): desktop preference toggles`

### Task B5: Add-download enhancements (checksum, category, start-at, batch, clipboard)

**Files:**
- Modify: `src/_lib/types/download-types.ts` (extend `DownloadReq`), `src/_lib/services/api/download-api.ts` (add `createBatch()`), `src/_lib/services/queries/download.queries.ts` (add `useCreateBatch()`), the create-download UI under `src/features/dashboard/create-download/*`
- Use: `src/hooks/useClipBoardUrl.tsx` (`useClipboardUrl()` → `{clipboardUrl, pasteValidUrl, readRawText}`)

**Interfaces:** `POST /downloads/batch`; extended `POST /downloads` fields `checksum`,`checksumAlgo`,`startAt`,`category`.

- [ ] **Step 1.** Extend `DownloadReq` with optional `checksum?`, `checksumAlgo?: "sha256"|"md5"`, `startAt?: string`, `category?: string`. Add `createBatch(urls, options)` to the api module and `useCreateBatch()` query.
- [ ] **Step 2.** In the add-download dialog (`ui-ux-pro-max`): an "Advanced" disclosure with checksum input + algo select, a category select (populated from `useCategories()`), and an optional schedule-start datetime. A "Paste from clipboard" button (uses `useClipboardUrl`) that prefills the URL when a valid link is detected; a "Batch" tab with a textarea (one URL per line) wired to `useCreateBatch()` and a per-URL result summary.
- [ ] **Step 3.** Verify + commit: `feat(frontend): checksum, category, scheduled-start and batch/clipboard add`

### Task B6: Category badge on download cards + types

**Files:**
- Modify: `download-types.ts` (`Download` gains optional `category?`), the download card under `src/features/dashboard/download-list/*`

- [ ] **Step 1.** Add `category?: string` to the `Download` type (if the backend echoes it; otherwise derive from extension client-side as a fallback).
- [ ] **Step 2.** Render a small category badge (reuse the `Badge` ui component / pattern from `PriorityBadge`) on each card when present.
- [ ] **Step 3.** Verify + commit: `feat(frontend): show category badge on download cards`

**Agent B final report:** list any contract mismatches found (field names/shapes) as requested upstream changes.

---

# AGENT C — DESKTOP / WAILS

Skill to load first: `go-concurrency` (tray + clipboard goroutines must be leak-free). Owns `app.go`, `main.go`, `tray.go`, `wails.json`, and new root-level build-tagged `autostart_*.go`. Reads settings via the `config` package (already imported by the root module through `cmd/server`). Verification: `go build ./...` at repo root, and `wails build -tags webkit2_41` if WebKit present.

Current state to build on: `main.go` runs `go server.Start()` and binds only `Greet`; `tray.go` defines `startTray(ctx)` **but it is never called**; `OnBeforeClose` quits or confirms.

### Task C1: App bindings — GetApiBase + ChooseDir, and the safe-port wiring

**Files:** Modify `app.go`, `main.go`.

**Interfaces:**
- Consumes: `server.Listen()`, `server.Serve()`, `server.APIBase()` (Agent A / Task A1).
- Produces: `func (a *App) GetApiBase() string`, `func (a *App) ChooseDir() (string, error)`; both added to `Bind`.

- [ ] **Step 1.** In `main.go`, replace `go server.Start()` with `base, err := server.Listen()` (handle err) then `go server.Serve()`; keep `server.SetQuitFunc(wailsApp.Quit)`. Store `base` where `App` can read it (e.g. `app.apiBase = base`).
- [ ] **Step 2.** In `app.go`, add `apiBase string` to `App`; `GetApiBase() string` returns it (fallback to `server.APIBase()`); `ChooseDir() (string, error)` calls `runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title:"Choose download folder"})`. Add both to `Bind: []interface{}{app}` (already binds `app`; the new exported methods are auto-bound). Remove the dead `Greet` if unused by the frontend (confirm first; else leave it).
- [ ] **Step 3.** Build (`go build ./...`); commit: `feat(desktop): expose GetApiBase and native folder picker; wire safe port`

### Task C2: Wire the system tray with quick actions

**Files:** Modify `tray.go`, `main.go`.

- [ ] **Step 1.** In `main.go` `OnStartup` (after `a.ctx` is set), call `startTray(ctx)`. (Currently never called.)
- [ ] **Step 2.** In `tray.go`, expand the menu: **Show Window** (`runtime.WindowShow`), **Pause all** (HTTP `POST {apiBase}/api/v1/downloads/pause-all` via `net/http`), **Start all** (`/downloads/start-all`), separator, **Quit** (set `isQuitting=true` then `runtime.Quit`). Keep the menu event loop in its own goroutine (as today) but ensure it exits cleanly on quit (select on a `done` channel) — no leaked goroutine.
- [ ] **Step 3.** Build; commit: `feat(desktop): system tray with show/start-all/pause-all/quit`

### Task C3: Minimize-to-tray / close-to-tray

**Files:** Modify `main.go`.

**Interfaces:** Consumes `config.Setting{MinimizeToTray, CloseToTray}` (Task A7).

- [ ] **Step 1.** Load the latest setting in `OnBeforeClose` (re-read via `config.Setting.LoadSettingMetadata()` so a runtime toggle is honored). If `CloseToTray` and not `isQuitting`: `runtime.WindowHide(ctx)` and `return true` (prevent close). Else keep the existing confirm/quit logic.
- [ ] **Step 2.** For minimize-to-tray: if `MinimizeToTray`, hook `runtime.WindowIsMinimised`/the minimise event to `WindowHide`. (If Wails v2 lacks a minimise callback, document that close-to-tray is the supported path and minimise-to-tray is best-effort.)
- [ ] **Step 3.** Build; manual smoke if WebKit present; commit: `feat(desktop): close-to-tray and minimize-to-tray honoring settings`

### Task C4: Native completion notifications

**Files:** Modify `main.go` (+ a small `notify.go` at root if cleaner).

- [ ] **Step 1.** Subscribe to download completion: simplest reliable approach — a lightweight poller goroutine (context-cancelable, leak-free) that hits `{apiBase}/api/v1/downloads/stats` or `/downloads?status=completed` every few seconds and notifies on newly-completed job ids (track a seen-set). (Avoids reaching into engine internals from the root module.)
- [ ] **Step 2.** Show the OS notification via Wails (`runtime.Notification` if available in v2) or the already-present `github.com/gen2brain/beeep` dep: `beeep.Notify("Rum", "<file> finished", iconPath)`. Respect `Setting.Silent`.
- [ ] **Step 3.** Build; commit: `feat(desktop): native notifications on download completion`

### Task C5: Autostart-on-login + window-state persistence + clipboard watcher

**Files:** Create `autostart_linux.go`, `autostart_windows.go`, `autostart_darwin.go` (build-tagged); modify `main.go`.

**Interfaces:** Consumes `config.Setting{LaunchOnStartup, EnableClipboardWatch, WindowState}` (Task A7).

- [ ] **Step 1 — autostart.** Per-OS `SetAutostart(enable bool) error`:
  - Linux: write/remove `~/.config/autostart/rum.desktop` (reuse the existing `Rum.desktop` content, `Exec=` the installed binary).
  - Windows: set/delete `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\Rum` via `golang.org/x/sys/windows/registry` (already an indirect dep) — value = exe path.
  - macOS: write/remove `~/Library/LaunchAgents/ir.tiredbooy.rum.plist`.
  On startup, reconcile the OS state to `Setting.LaunchOnStartup`.
- [ ] **Step 2 — window state.** On `OnStartup`, if `Setting.WindowState` is non-zero, `runtime.WindowSetSize`/`WindowSetPosition`/`WindowMaximise`. On `OnBeforeClose` (and periodically), read current geometry (`runtime.WindowGetSize`/`Position`) and persist into `Setting.WindowState` via `Save()`.
- [ ] **Step 3 — clipboard watcher (opt-in).** If `Setting.EnableClipboardWatch`, a context-cancelable goroutine polls `runtime.ClipboardGetText(ctx)` every ~1.5s; on a newly-seen valid http(s) URL, emit a Wails event (`runtime.EventsEmit(ctx, "clipboard:url", url)`) the frontend listens for. Leak-free; stops on quit.
- [ ] **Step 4.** Build; commit: `feat(desktop): autostart, window-state persistence and opt-in clipboard watcher`

### Task C6: Real app metadata

**Files:** Modify `wails.json`.

- [ ] **Step 1.** Fix metadata: correct `author.email`, `nfpm.homepage` (currently `https:/tiredbooy.ir` — missing a slash), ensure `productVersion` is injected at build (Agent D may pass `-ldflags`); keep window opts. Verify JSON parses.
- [ ] **Step 2.** Commit: `chore(desktop): correct app metadata in wails.json`

**Agent C final report:** confirm which Wails v2 runtime calls exist in the pinned version (`runtime.Notification`, minimise events, `ClipboardGetText`) and note any that had to degrade to best-effort; list requested upstream changes (e.g. if it needs a settings field beyond Task A7).

---

# AGENT D — RELEASE / INSTALL / DOCS

No special skill required; follow the repo's existing installer conventions (`installers/README.md`). Owns `installers/**`, `.github/workflows/**`, `build-*.sh`, `installer.iss`, `Rum.desktop`, `INSTALL.md`, `README.md`, `docs/**`. Verification: `actionlint` if available (else careful YAML review); shellcheck the scripts; the real proof is a tag push (orchestrator decides when).

### Task D1: CI — build/vet/test on every push & PR

**Files:** Create `.github/workflows/ci.yml`.

- [ ] **Step 1.** Workflow `on: [push, pull_request]` with a Linux job: checkout, setup-go 1.25.x, install WebKit deps (`libgtk-3-dev libwebkit2gtk-4.1-dev`), `cd backend && go build ./... && go vet ./... && go test ./...`; setup-node 20, `cd frontend && npm ci && npx tsc --noEmit && npm run build`; root `go build ./...`.
- [ ] **Step 2.** Validate YAML (`actionlint .github/workflows/ci.yml` if installed). Commit: `ci: build, vet and test on push and PR`

### Task D2: CI — tagged release matrix (Linux/Windows/macOS)

**Files:** Create `.github/workflows/release.yml`.

**Interfaces:** triggered by `on: push: tags: ['v*']`. Uses `wailsapp/wails` CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`).

- [ ] **Step 1.** Three matrix jobs:
  - **linux** (ubuntu): install WebKit deps; `wails build -tags webkit2_41 -platform linux/amd64`; package **`.deb`** via the `nfpm` config in `wails.json` (`wails build -nsis`? no — use `nfpmConfig`; or run `nfpm pkg`); build an **AppImage** (linuxdeploy or appimagetool over `build/bin/Rum`). Upload artifacts.
  - **windows** (windows-latest): `wails build -platform windows/amd64`; if Inno Setup present (`choco install innosetup`), compile `installer.iss` → `Rum-Setup.exe`; else upload the raw `.exe`.
  - **macos** (macos-latest): `wails build -platform darwin/universal`; create an **unsigned `.dmg`** (`create-dmg` or `hdiutil`); clearly label the artifact "unsigned".
- [ ] **Step 2.** A final job (`softprops/action-gh-release`) attaches all artifacts to the GitHub Release for the tag, with release notes pointing at `docs/IMPROVEMENTS.md`.
- [ ] **Step 3.** Validate YAML; commit: `ci: cross-platform tagged release pipeline (deb/AppImage/exe/dmg)`

### Task D3: Polished from-source install scripts + macOS

**Files:** Modify `installers/{gui,cli}/install-linux.sh`, `install-windows.ps1`; create `installers/{gui,cli}/install-macos.sh`.

- [ ] **Step 1.** Linux/Windows scripts: add clearer dependency pre-checks (detect missing `go`/`node`/WebKit and print the exact distro install command already in `INSTALL.md`); keep existing `--uninstall`/`--mirror`/`--prefix` flags; fail with actionable messages.
- [ ] **Step 2.** `install-macos.sh` (gui + cli): check for Xcode CLT + Homebrew `go node`; `wails build` (gui) / `go build ./backend/cmd/rum` (cli); install the `.app` into `/Applications` (gui) or the binary into `/usr/local/bin` (cli); `--uninstall`.
- [ ] **Step 3.** `shellcheck` the bash; commit: `installers: clearer dependency checks and macOS install scripts`

### Task D4: Documentation — INSTALL rewrite, README, improvements

**Files:** Modify `INSTALL.md`, `README.md`, `docs/IMPROVEMENTS.md`.

- [ ] **Step 1.** `INSTALL.md`: lead with **"Download a prebuilt installer from Releases"** (table: Linux `.AppImage`/`.deb`, Windows `.exe`, macOS `.dmg` + the unsigned/Gatekeeper note `xattr -dr com.apple.quarantine /Applications/Rum.app`); keep build-from-source as the second path.
- [ ] **Step 2.** `README.md`: replace the stock Wails template with a real project README (what Rum is, features incl. the new ones, screenshots placeholder, install link to `INSTALL.md`, dev quickstart `wails dev`, architecture one-paragraph, license).
- [ ] **Step 3.** `docs/IMPROVEMENTS.md`: append a **"Round 3 — Production Release Overhaul"** section documenting every change (what / why / how), mirroring the Round 1–2 style, and update the "Known limitations" list (mark resolved items: single-file integrity ✓, scheduler routed ✓, ConnectionBanner mounted ✓, checksum UI ✓).
- [ ] **Step 4.** Commit: `docs: prebuilt-install guide, real README, round-3 improvements writeup`

**Agent D final report:** note any workflow step that can only be proven on a real tag push (e.g. macOS dmg), and any `nfpm`/`wails.json` change it needs the orchestrator to apply.

---

# ORCHESTRATOR — INTEGRATION (done by the lead, after agents finish)

These cross-scope edits touch files multiple agents read but only the orchestrator writes, plus the final gate and delivery.

### Task I1: Mount the schedule controller + scheduler shutdown in cmd/server

**Files:** Modify `backend/cmd/server/main.go` (the `Listen()`/`Serve()` refactor from A1 lives here).

- [ ] Build the `*download.SpeedGovernor` from `setting.SpeedLimitKB`, attach it to `opt.Governor`. After `handlers.InitAPI(opt)`, get the manager (`handlers.GlobalManager`), start `download.NewScheduleController(manager, governor, &setting).Start(ctx)`. On shutdown, `controller.Stop()` then `manager.Shutdown()` (which cancels the dispatcher context and closes the scheduler — `sched` is unexported, so shutdown is encapsulated in this one exported method). Verify `go build ./... && go vet ./...`.

### Task I2: Confirm the safe-port handshake end-to-end

- [ ] With A1 + C1 done: `main.go` calls `server.Listen()`/`Serve()`; `App.GetApiBase()` returns the base; frontend `wails.ts` consumes it. Manually confirm (or via a smoke test) the frontend reaches the API on a non-8080 port when 8080 is occupied.

### Task I3: Reconcile modules + full verification gate

- [ ] `cd backend && go mod tidy`; root `go mod tidy`; ensure `go.sum` updated in both modules for any new deps (`golang.org/x/sys/windows/registry` for Windows autostart is build-tagged — confirm it doesn't break Linux build).
- [ ] Run the full gate:
  - `cd backend && go build ./... && go vet ./... && go test ./... && go test -race ./internal/pkg/download/ ./internal/pkg/scheduler/ ./internal/pkg/queue/`
  - `cd frontend && npm ci && npx tsc --noEmit && npm run build`
  - root `go build ./...`; attempt `wails build -tags webkit2_41` (if WebKit present); record result.
- [ ] Fix any integration breakage (contract mismatches reported by agents).

### Task I4: Commit per area + push to dev

- [ ] Ensure each area is a clean commit (`feat(engine)`, `feat(frontend)`, `feat(desktop)`, `ci`/`installers`, `docs`) — **no `Co-Authored-By: Claude` trailer** (Global Constraints).
- [ ] `git push origin dev`. Report the pushed commit range. Do **not** tag a release (Release scope = "prepare and push to dev"; the tag-triggered pipeline is ready but the user pulls the trigger).

---

## Self-Review (completed by plan author)

**1. Spec coverage** — every spec section maps to a task:
- Reliability follow-ups → A2 (single-file integrity), A3 (scheduler routed), B1 (ConnectionBanner mounted), B5 (checksum UI). ✓
- Scheduled + bandwidth windows → A4 (governor/controller/SpeedRule.Days) + B2. ✓
- Clipboard/link capture → A6 (batch endpoint) + B5 (clipboard/batch UI) + C5 (clipboard watcher). ✓
- Categories & auto-organize → A5 + B3 + B6. ✓
- Desktop hardening (tray/min-close-to-tray/notifications/autostart/window-state/folder-picker/safe-port/metadata) → C1–C6 + A1 + A7. ✓
- Release pipeline + scripts + docs (3 OS) → D1–D4. ✓
- Delivery (per-area commits, no trailer, push to dev) → I4. ✓
- Verification gate → I3. ✓

**2. Placeholder scan** — no "TBD"/"add error handling"/"write tests for the above" left; each backend task has concrete test intent + implementation steps; UI/CI tasks have concrete acceptance and exact files. (UI JSX is generated by Agent B via `ui-ux-pro-max` against the specified props/behavior — intentional, not a placeholder.)

**3. Type consistency** — names align across tasks: `SpeedRule{StartHour,EndHour,LimitKBps,Days}` (extends existing), `CategoryRule{Name,Extensions,DestDir}`, `Options{StartAt,Categorize,Category,Governor}`, `server.Listen/Serve/APIBase`, `App.GetApiBase/ChooseDir`, settings JSON tags (snake_case) match between Go (A7) and TS (B4). The bandwidth feature reuses `config.SpeedRule` (NOT a new `BandwidthWindow`) — corrected from the spec's tentative name; noted in the contract.

**Risk-ordered execution note:** A1 (port) and A7 (settings fields) unblock C and B; A3 (scheduler adoption) is the riskiest engine change — implement and `-race` it before A4 builds on the manager. Recommended order: A1 → A7 → A2 → A3 → A4 → A5 → A6, with B/C/D proceeding in parallel against the contract once A1+A7 land.
