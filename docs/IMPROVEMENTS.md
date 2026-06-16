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

---

## Round 2 — Reliability, Security & New Features (2026-06-16)

A second sweep, decomposed across **seven scoped worker agents** (five backend,
two frontend) plus orchestrator-level integration. Same method as Round 1:
ownership by path, additive-only changes to shared exported APIs, and a frozen
**shared interface registry** so agents that could not talk to each other still
agreed on exact Go/TS symbol names. The contract lives at
[`docs/superpowers/specs/2026-06-16-rum-reliability-features-coordination.md`](superpowers/specs/2026-06-16-rum-reliability-features-coordination.md).

- Verification: backend `go build ./...`, `go vet ./...`, and
  `go test ./internal/pkg/download/ ./internal/pkg/api/... ./internal/pkg/scheduler/ ./internal/pkg/logging/`
  all green; frontend `npx tsc --noEmit` and `npm run build` pass; full
  `wails build -tags webkit2_41` succeeds.
- The integration points the orchestrator wired are listed under
  **Orchestrator integration** below.

### B1 — Integrity & resume safety (`download/integrity.go`, `preflight.go`, `resume_validate.go`)

- **What changed.** Added `VerifyChecksum(path, algo, expected string) error`
  (sha256/md5, empty `expected` is a no-op returning nil, wraps
  `ErrChecksumMismatch`), `PreflightDiskSpace(dir string, needBytes int64) error`
  (wraps `ErrInsufficientDiskSpace`, with platform-specific free-space probes in
  `preflight_unix.go`, `preflight_windows.go`, and a conservative
  `preflight_other.go` fallback selected by build tags), and
  `ValidateResume(remoteETag, remoteLastMod, savedETag, savedLastMod string, remoteSize, savedSize int64) bool`.
- **The issue / why it mattered.** A multi-segment download would happily
  allocate a sparse file larger than the remaining disk (ENOSPC mid-write,
  corrupt output), and there was no end-to-end integrity check, so a silently
  truncated or tampered file looked "complete". Resuming a partial against a
  changed remote could splice two different bytestreams together.
- **Why this path.** Free-space detection is inherently OS-specific
  (`statfs`/`statvfs` vs `GetDiskFreeSpaceEx`), so it is split by build tag with a
  safe fallback rather than pulling in a CGo dependency. Checksum verification is
  a post-assembly streaming hash (constant memory) and treats an empty expected
  value as opt-out so existing callers that don't supply a checksum are
  unaffected. `ValidateResume` is a pure boolean predicate (ETag/Last-Modified/
  size agreement) so it is trivially testable and has no I/O side effects.

### B2 — Security hardening (`download/security.go`)

- **What changed.** `ValidateURL(raw string) error` (scheme allowlist: http/https
  only, wrapping `ErrUnsupportedScheme` / `ErrInvalidURL`),
  `SecureTransport(base *http.Transport) *http.Transport` (sets
  `TLSClientConfig.MinVersion = tls.VersionTLS12` and stall timeouts),
  `RedirectPolicy(req, via)` (caps the chain at ≤10 and strips
  `Authorization`/cookies on a cross-host hop), and `IsPrivateHost(host string) bool`
  (an SSRF building block for private/loopback/link-local ranges).
- **The issue / why it mattered.** The client accepted any scheme the URL parser
  tolerated, had no explicit TLS floor, and followed redirects unboundedly while
  forwarding credentials across hosts — a credential-leak and SSRF surface for a
  tool that fetches arbitrary user-supplied URLs.
- **Why this path.** These are returned as composable primitives the orchestrator
  installs on the **shared** `http.Client`, rather than B2 editing the hot
  `downloadRequest.go` itself (forbidden by the contract). `IsPrivateHost` is
  provided but **not enforced by default**: a download manager legitimately needs
  to reach LAN/NAS hosts, so a blanket private-IP block would break real use; it
  is left as an opt-in hook (see Known limitations).

### B3 — Structured errors + slog logging (`download/errors.go`, new pkg `internal/pkg/logging/`)

- **What changed.** Eight sentinel errors (`ErrNotFound`, `ErrInvalidURL`,
  `ErrUnsupportedScheme`, `ErrInsufficientDiskSpace`, `ErrChecksumMismatch`,
  `ErrConflict`, `ErrTooLarge`, `ErrRemoteChanged`), a
  `CodedError{Code, Kind, Err}` type implementing `error` + `Unwrap`, and
  `Classify(err) (code string, httpStatus int)` that maps any engine error to a
  stable machine code + HTTP status. New `internal/pkg/logging` wraps `log/slog`
  with `Init/L/Debug/Info/Warn/Error` and level/JSON configuration. B3 also
  deleted the temporary sentinel stubs that B1/B2 had carried before
  integration.
- **The issue / why it mattered.** Errors were ad-hoc strings, so the API layer
  matched on substrings to pick a status code — brittle and leaky. There was no
  single taxonomy shared between the engine and the API.
- **Why this path.** Sentinels + `errors.Is/As` is idiomatic Go and lets every
  producer wrap with `%w` without a hard dependency on the API package. `Classify`
  deliberately reuses the `dto` code **string values**
  (`"validation_error"`, `"not_found"`, `"conflict"`, `"internal_error"`,
  `"unsupported"`, `"payload_too_large"`) and only adds three new ones
  (`"insufficient_disk_space"`, `"checksum_mismatch"`, `"remote_changed"`), so the
  engine and the HTTP envelope stay in lockstep without the engine importing the
  `dto` package. `slog` was chosen over a third-party logger to honor the
  no-new-heavyweight-deps rule.

### B4 — API features, middleware & SSE (`internal/pkg/api/**`)

- **What changed.** New middleware `requestid.go`, `recovery.go`, and
  `ratelimit.go` (token bucket via `golang.org/x/time/rate`), composed in
  `setupMiddlewares.go` in the order
  `Recovery → RequestID → Logger → CORS → RateLimit → gzip → handler`. New
  handlers `RetryDownload` and `SetDownloadPriority` on routes
  `POST /downloads/:id/retry` and `PATCH /downloads/:id/priority`
  (body `{ "priority": "low"|"normal"|"high" }`, validated by
  `dto.SetPriorityRequest`). SSE keep-alive (`: keep-alive` comment every 15s) in
  `stream.go`. `writeServiceError` in `response.go` now routes through
  `download.Classify`, falling back to the previous behavior only when Classify
  returns `internal_error`.
- **The issue / why it mattered.** A panic in any handler could crash the
  process; there was no request correlation ID for the logs; an abusive client
  could hammer the API unbounded; and the new retry/priority features had no HTTP
  surface.
- **Why this path.** Recovery is outermost so a panic anywhere downstream becomes
  a sanitized 500 rather than a process exit; RequestID runs early so both the
  logger and recovery can attach the ID. `PATCH` (not `PUT`) for priority because
  it is a partial mutation of one field. The Classify integration is guarded
  (`if code != dto.CodeInternal`) so `response.go` still compiled and behaved
  correctly even before B3's work landed — a deliberate decoupling so the agents
  could build independently.

### B5 — Concurrency scheduler (new pkg `internal/pkg/scheduler/`)

- **What changed.** A priority-aware concurrency gate: `New(maxConcurrent int)`
  returning a `*Scheduler` with `Submit(id string, p queue.Priority)`,
  `Next(ctx) (string, bool)`, `Done(id)`, and `SetMax(n)`, backed by the Round 1
  `queue.PriorityQueue`. It is leak-free and `-race` clean (verified by
  `scheduler_test.go`).
- **The issue / why it mattered.** Round 1 left a correct `PriorityQueue`
  primitive but nothing turned "a queue of work" into "at most N concurrent
  downloads, highest priority first". This package is that missing gate.
- **Why this path.** It is built as a **standalone, opt-in** component rather than
  being forced into `StartAllJobs`, exactly as the contract permits. The
  orchestrator instead achieved priority ordering in the existing
  `StartAllJobs` path with a lightweight `priorityRank` sort (see below), leaving
  the scheduler as a ready-to-adopt primitive without changing current runtime
  behavior. (It is therefore **available-only / not yet routed** — see Known
  limitations.)

### F1 — New frontend features (`features/dashboard/download-list/**`, `hooks/**`)

- **What changed.** `SpeedSparkline.tsx`, `PriorityBadge.tsx`, and
  `CardActionsMenu.tsx`; hooks `useSpeedHistory.ts` (rolling speed samples) and
  `useKeyboardShortcuts.ts`. Retry and priority are wired through the query layer
  as `useRetryDownload` / `useSetDownloadPriority` in `download.queries.ts`
  (priority change does an optimistic cache patch with rollback on error), with
  matching `retryDownload` / `setDownloadPriority` calls in `download-api.ts` and
  `DownloadPriority` added to `download-types.ts`. `DownloadCard` renders the
  sparkline + priority badge + actions menu; `Dashboard.tsx` registers keyboard
  shortcuts.
- **The issue / why it mattered.** The new backend retry/priority endpoints and
  the live speed data had no UI. Power users had no keyboard affordances.
- **Why this path.** Mutations live in the existing TanStack Query layer so they
  inherit cache invalidation and the shared `request` wrapper; priority uses an
  optimistic update because the perceived latency of a badge flip matters and the
  rollback path is cheap. Speed history is a hook (not component state) so the
  sparkline stays a pure presentational component. All user feedback goes through
  the F2 `@/lib/toast` surface, never `sonner` directly.

### F2 — Reliability UX, a11y & shared primitives (`components/**`, `features/layout/**`, `index.css`)

- **What changed.** `lib/toast.ts` (a thin, frozen re-export of `sonner`'s
  `toast` exposing at least `.success/.error/.info/.loading/.dismiss`),
  `components/error-boundary.tsx`, `components/connection-banner.tsx`, and a
  rewritten `components/ui/sonner.tsx` Toaster that resolves the active theme
  **without** `next-themes`. Accessibility fixes in `features/layout/Header.tsx`
  and contrast/focus tweaks in `index.css`.
- **The issue / why it mattered.** The generated `sonner.tsx` imported
  `next-themes`, which is **not installed**, so the toast surface would have
  failed to build. F1 also needed one stable toast import path rather than each
  feature reaching into `sonner`. There was no React error boundary, so a render
  exception blanked the whole app.
- **Why this path.** Removing the `next-themes` dependency (reading the theme
  directly) keeps the dependency set unchanged per the contract. `toast.ts` is a
  one-line re-export precisely so the contract surface is frozen and trivially
  stable for F1 to consume. The error boundary is a class component because React
  error boundaries must be class-based.

### Orchestrator integration (cross-scope wiring)

Applied after all seven agents finished, into the hot shared files the agents
were forbidden to touch:

1. **Security on the shared client** — `NewDownloader` (`downloadRequest.go`)
   installs `SecureTransport(baseTransport)` and `CheckRedirect: RedirectPolicy`;
   `ValidateURL` is called in `CreateJobsFromURLs` (`manager.go`).
2. **Integrity in the segmented path** — `segmented.go` calls
   `PreflightDiskSpace` before allocating the output and `VerifyChecksum` after
   assembling it, driven by new additive `Options.Checksum` /
   `Options.ChecksumAlgo` (`structs.go`).
3. **Priority on the job** — `Job.Priority` field with `GetPriority`/`SetPriority`
   accessors (`job.go`); `JobManager.RetryJob(ctx, id)` and
   `SetJobPriority(id, priority)` with `normalizePriority` / `priorityRank`
   helpers (`manager.go`); `StartAllJobs` now sorts eligible jobs by
   `priorityRank` so higher-priority downloads start first.
4. **Honest API errors** — `writeServiceError` (`response.go`) routes through
   `download.Classify`.
5. **Frontend shell** — `<Toaster/>` and `<ErrorBoundary/>` mounted in `App.tsx`.

### Known limitations / future work

- **Single-file path has no integrity wiring.** `PreflightDiskSpace` and
  `VerifyChecksum` are wired into the **segmented** finalize path only
  (`segmented.go`). `DownloadSingleFile` (`download.go`) — the fallback for
  non-range-capable servers, unknown sizes, small files, or `Connections == 1` —
  does **not** yet preflight disk space or verify a checksum. Threading the same
  two calls into that path is the obvious next step.
- **Scheduler is opt-in / not yet routing `StartAllJobs`.** The
  `internal/pkg/scheduler` package is complete and tested but is not imported by
  the engine. Priority ordering today comes from the `priorityRank` sort the
  orchestrator added to `StartAllJobs`, not from the scheduler's concurrency
  gate. Adopting the scheduler would give true priority-aware *concurrency*
  (currently the gate is the engine's existing parallelism limit).
- **SSRF guard is off by default.** `IsPrivateHost` exists but nothing rejects
  private/loopback targets, because reaching LAN hosts is a legitimate use case.
  Enforcing it should be a configurable policy, not a hard default.
- **No UI to supply expected checksums.** The engine accepts
  `Options.Checksum` / `Options.ChecksumAlgo`, but the frontend has no field to
  enter an expected hash, so checksum verification is currently only reachable
  programmatically / via the CLI.
- **`ConnectionBanner` is built but not mounted.** F2 provides
  `components/connection-banner.tsx`, but it is not yet rendered anywhere
  (analogous to the opt-in scheduler). The reconnect/offline UX it offers is a
  ready-to-wire follow-up.
- **`internal/pkg/logging` is available but not yet adopted** by the engine/API
  call sites, which still use the existing logging; migrating call sites onto the
  `slog` wrapper is a follow-up.
