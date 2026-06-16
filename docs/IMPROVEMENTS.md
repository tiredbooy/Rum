# Rum — Multi-Agent Enhancement Report

This document records the enhancement sweep performed across Rum's four
subsystems by five scoped agents working in parallel (one frontend, four
backend), plus the orchestrator-level integration that wired their work
together. For each area it states **what changed**, **the issue it fixes**, and
**why that path was chosen**.

- Branch: `feat/engine-api-cli-frontend-overhaul`
- Design spec: [`docs/superpowers/specs/2026-06-16-rum-multi-agent-overhaul-design.md`](superpowers/specs/2026-06-16-rum-multi-agent-overhaul-design.md)
- Verification: backend `go build ./...`, `go vet ./...`, and `go test -race ./...`
  all green; CLI binary builds and reports `--version`; the root Wails module
  builds; frontend `npm run build` passes.

## Method

The work was decomposed by **path ownership** so five agents could edit a single
shared working tree without colliding: each agent could *read* anything but only
*edit* files inside its assigned directories. Low-level packages (engine,
supporting libs) were required to keep their **exported** APIs
backward-compatible (additive changes only) so their consumers (API, CLI) kept
compiling. Cross-scope needs were reported back as "requested upstream changes"
and applied by the orchestrator during integration. The integration gate was
`go build ./... && go vet ./... && go test -race ./...`.

---

## Agent 1 — Frontend (`frontend/src/**`)

**Live progress over SSE was fragile.** `useAllProgressStream` only logged on
`onerror`, so when the backend closed the `SubscribeAll` channel the stream
could wedge and the UI silently stopped updating.
- *Fix:* reconnect with capped exponential backoff (1s → 15s), reset on
  `onopen`, and a guarded teardown (`cancelled` flag + `clearTimeout`) so no
  reconnect fires after unmount.
- *Why:* the failure mode is a server-initiated close, which needs an explicit
  fresh `EventSource` rather than relying on the browser's implicit retry.

**Download lists didn't update live.** The old handler wrote only to the
per-job *detail* cache and bailed when no detail entry existed, so list views
(which read `useDownloads`) dropped events.
- *Fix:* `applyPatch` now seeds the detail cache if absent and also patches every
  cached list query via `getQueriesData`.
- *Why:* lists and detail are separate query keys; both must be patched for the
  UI to reflect progress everywhere it's shown.

**Bogus progress / unknown size rendered as garbage.** The backend can emit a
progress value computed from an unknown total size.
- *Fix:* clamp `progress` to `[0,100]`; treat `total_size <= 0` as "Unknown
  size" and render `—` for the percentage; surface the error reason on failed
  downloads.
- *Why:* the frontend should be defensive regardless of backend values (and the
  backend now also guards this — defense in depth).

**Missing error/empty states & a11y.** A failed list fetch left a blank screen;
icon-only buttons had no accessible label.
- *Fix:* dedicated error state with a "Try again" button; corrected empty-guard;
  `aria-label` on every icon-only action button.

*Deliberately not done:* consolidating `_lib` vs `lib`. `lib/` holds the
shadcn-owned `utils.ts` (`cn`) imported by ~59 generated components; merging
would fight the shadcn generator for no real benefit.

---

## Agent 2 — Download engine (`backend/internal/pkg/download/**`)

**Headline feature — segmented (multi-connection) downloads.** The README
advertised "parallel downloads" but only ran parallel *jobs*; each file used a
single stream.
- *Fix:* `DownloadSegmented` splits a range-capable file into N contiguous
  segments and downloads them concurrently, each writing at its absolute offset
  via `os.File.WriteAt`. Concurrency is bounded with `errgroup.SetLimit`,
  cancellable via context (so pause still works), and resumable through a
  `.rumparts` JSON sidecar that records per-segment completion (written
  atomically). It falls back to the existing single-stream path when the server
  doesn't support ranges, the size is unknown, the file is small, or
  connections == 1. New additive `Options.Connections` (0/1 = single stream,
  default 4); exposed on the CLI as `-c`.
- *Why:* `WriteAt` avoids seek collisions between writers; an errgroup gives
  bounded, cancellable fan-out per go-concurrency; a sidecar makes resume robust
  without changing the on-disk file format.

**Bug fixes:**
- *Speed limiter* multiplied KB by **102** (typo for 1024) and had a dead
  per-job override — it throttled to ~10% of the configured rate. Fixed the math,
  made the override actually take precedence, and clamped burst so a buffer-sized
  `WaitN` can't stall.
- *`MaxRetries` was never used.* Added exponential backoff + jitter that resumes
  from the current offset and retries only transient faults (network resets,
  timeouts, 5xx, 429, 408) — never 4xx or context cancellation.
- *Manual `Accept-Encoding`* disabled net/http's transparent decompression and
  corrupted Content-Length/resume offsets. Removed it so offsets always map to
  raw file positions.
- *Progress divide-by-(0/-1)* produced NaN/garbage. Added `progressPercent`:
  `-1` for indeterminate (total ≤ 0), clamped `[0,100]` otherwise.
- *Goroutine leak* in `CreateJobsFromURLs`: a HEAD goroutine outlived its
  `time.After` timeout. Replaced with `errgroup` + a per-request context timeout
  so the HTTP call itself returns.
- *`DeleteJobsByFilter`* left the URL dedup index empty (breaking dedup) and
  couldn't filter failed jobs. It now repopulates the index and supports
  `error`/`failed`.
- *`fmt.Errorf("%w", err.Error())`* passed a string to `%w` (vet error) — fixed.
  Raised `MaxConnsPerHost` from 2 → 16 for segmented throughput.

*Tests:* speed-limiter math, retry transient-vs-permanent decision + backoff cap,
progress guard, segment range computation, and end-to-end segmented
download/resume/fallback via `httptest`, all under `-race`.

---

## Agent 3 — HTTP API (`backend/internal/pkg/api/**`)

Guided by the go-api-design conventions (thin handlers, consistent error
envelope, capped bodies/pages, honest status codes).

- **Consistent error envelope** `{error, code, fields}` with honest status
  mapping (404/400/409/500); full errors logged with a request ID, sanitized
  messages returned. *Why:* never leak internal error strings; keep a stable
  client contract. The `error` key is preserved so the existing frontend keeps
  working.
- **Request validation** (absolute http(s) URLs, non-negative params), 1 MiB
  body cap via `http.MaxBytesReader`, field-level errors.
- **Opt-in pagination/filtering** on `GET /downloads` (`?status&limit&offset`,
  capped page size). *Why:* the existing frontend relies on the bare-array
  response, so pagination only engages when the client asks — additive, no
  breakage.
- **SSE hardening:** immediate flush, 15s keep-alive (detects dead clients,
  beats proxy idle-timeouts), disconnect detection via request context, and a
  `defer Unsubscribe` so subscriptions can't leak.
- **CORS** moved off wildcard `*` to a configurable allowlist
  (`RUM_CORS_ORIGINS`, with localhost/Tauri dev defaults). Recovery middleware
  made outermost; request-ID middleware added.
- **New** `PUT /settings/speed-limit`. **Graceful shutdown + server timeouts**
  in `cmd/server` (wired at integration), with `WriteTimeout=0` to preserve the
  long-lived SSE streams.

---

## Agent 4 — CLI / TUI (`backend/internal/pkg/tui/**`, `backend/cmd/rum/**`)

- **Non-interactive mode.** The bulk-download path blocked on `fmt.Scanln`
  prompts, breaking scripts/CI. Added `--yes`/`--non-interactive` (and
  `RUM_NONINTERACTIVE`); the engine's `RunProgram` now skips the prompt in that
  mode or when `--group` is already given (wired at integration). *Why:* a
  download manager must be scriptable; an env flag threads cleanly through to the
  engine without restructuring it.
- **CLI ergonomics:** `--version`/`--help` with rich usage and examples; single
  `runDownloads` helper; fixed an always-false empty-jobs guard. *Decision:* kept
  stdlib `flag` rather than adopting cobra, because the real parser lives in the
  engine's `RunProgram`; pulling in cobra would duplicate or invasively
  restructure shared engine code for marginal gain.
- **TUI race & UX:** fixed a genuine data race on `Job.CancelFunc` (now stored
  under lock; the engine got `Get/SetCancelFunc` accessors at integration);
  wired `download.SetQuitFunc` so a post-download "close" action quits the TUI
  cleanly; started a previously-dead scroll tick; avoided a progress send-storm
  when total size is unknown; corrected a garbled batch-counter display and show
  "unknown" ETA / `--` percent for unknown-size downloads.

---

## Agent 5 — Supporting packages (`file-system`, `queue`, `config`, `utils`, `format`)

- **`queue`:** implemented a real `PriorityQueue` (heap-backed, priority levels
  with FIFO within a level, bounded, context-aware, leak-free) with `-race`
  tests. *Why:* the package was effectively empty scaffolding; the engine now has
  an adoptable, correct scheduling primitive.
- **`file-system`:** `AtomicWriteFile` (temp + fsync + rename) so a crash can't
  corrupt `jobs.json`/`settings.json`; `SanitizeFileName`/`SafeJoin`
  path-traversal guards (now applied by the engine in `PrepareOutputPath`);
  removed a process-killing `log.Fatal` in `getHomeDir`. *Why:* metadata writes
  and remote-derived filenames are crash- and security-sensitive choke points.
- **`config`:** `Setting.Validate()` clamps speed limit / parallelism / retries
  to sane ranges and validates enums; `LoadSettingMetadata` recovers to defaults
  on missing/corrupt files instead of crashing; saves are atomic.
- **`utils`:** robust `ConvertSizeToInt` for Content-Length parsing; fixed
  `GetUserAgentForQueue` which used division instead of modulo and panicked on
  most inputs.
- **`format`:** hardened `CleanFileName`/`ExtractFileNameFromURL` against
  unsafe/host-leaking names; fixed `FormatRemainingTime` seconds math.

---

## Orchestrator integration (cross-scope wiring)

Changes that crossed agent boundaries, applied after all agents finished and the
full tree was rebuilt:

1. **Graceful shutdown + timeouts** in `cmd/server/main.go` (requested by the API
   agent; outside its scope).
2. **Non-interactive guard** in `download/runProgram.go` honoring
   `RUM_NONINTERACTIVE`, plus the new `-c` connections flag (requested by the CLI
   agent; lives in the engine package).
3. **Race-safe `Job.CancelFunc`** accessors in `download/job.go`, adopted across
   `manager.go` (requested by the CLI agent).
4. **Path-traversal sanitization** wired into `download.go`'s `PrepareOutputPath`
   using the supporting agent's `SanitizeFileName`.
5. **`go.sum` reconciliation** for the new `golang.org/x/sync` dependency in both
   the backend module and the root Wails module.

## Notes / follow-ups

- Segmented downloads are **on by default** (4 connections for range-capable
  files > 4 MiB); set `Connections: 1` / `-c 1` for legacy single-stream.
- A multi-segment download leaves a `*.rumparts` sidecar next to the output while
  in progress; it is removed on success and ignored unless `{URL,TotalSize}`
  match exactly.
- Live per-job speed limits and adopting the new `PriorityQueue` inside the
  engine are natural follow-ups (the primitives now exist).
