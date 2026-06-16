# Rum — Reliability, Security & Features Overhaul (Multi-Agent Coordination Contract)

**Date:** 2026-06-16
**Role of this document:** This is the *bridge* between 7 worker agents. It is the single
source of truth for **what each agent owns**, **what interfaces it must expose**, and **what
interfaces it may consume** from siblings. Agents run in parallel and cannot talk to each other,
so every cross-agent dependency is pinned to an exact Go/TS symbol name here. The orchestrator
("bridge", agent 8) wires the agreed integration points into the hot shared files and runs the
final build/vet/race/test + integration review.

## Goal

Make the **engine more reliable, more secure, and produce better errors**, and add **new user-facing
features** in the engine and the frontend. This is additive: do not regress existing behavior.

## Hard rules for every worker agent

1. **Stay inside your owned paths.** Listed per-agent below. Do not edit another agent's files.
2. **Do NOT edit these hot shared files** (the orchestrator wires integration into them):
   `backend/internal/pkg/download/manager.go`, `segmented.go`, `download.go`,
   `downloadRequest.go`, `job.go`, `runProgram.go`;
   `frontend/src/App.tsx`, `main.tsx`, `router.tsx`.
   Instead, expose exported functions/types in your **own new files**; the orchestrator calls them.
3. **Self-contained new files preferred.** Add `*_test.go` for every new Go file with logic.
4. **Compile clean.** Run `gofmt -w`, `go build ./...`, `go vet ./...` for your package
   (from `backend/`). Frontend agents run `npx tsc --noEmit` from `frontend/`.
5. **No new heavyweight deps.** Backend may use stdlib + already-present modules
   (`golang.org/x/sync`, `golang.org/x/time`, `google/uuid`). Frontend may use already-present deps
   only — note `sonner` (toasts) and `framer-motion` are **already installed**.
6. **Follow existing idiom** (mutex-guarded `Job`, `dto.ErrorResponse` envelope, shadcn/Tailwind, RTL-aware).

## Shared interface registry (the contract)

These are the ONLY cross-agent touch points. Names are frozen; implement/consume exactly.

### Engine error taxonomy — OWNED BY B3, CONSUMED BY B1/B2/B4
- Package `download`, new file `errors.go`.
- Sentinel errors (wrap with `%w`): `ErrNotFound`, `ErrInvalidURL`, `ErrUnsupportedScheme`,
  `ErrInsufficientDiskSpace`, `ErrChecksumMismatch`, `ErrConflict`, `ErrTooLarge`, `ErrRemoteChanged`.
- `type CodedError struct { Code string; Kind ErrKind; Err error }` implementing `error` + `Unwrap`.
- `func Classify(err error) (code string, httpStatus int)` — maps any engine error to a stable
  machine code + HTTP status. Codes reuse `dto` codes where possible (string values must match
  `dto.CodeValidation="validation_error"`, `dto.CodeNotFound="not_found"`, `dto.CodeConflict="conflict"`,
  `dto.CodeInternal="internal_error"`, `dto.CodeUnsupported="unsupported"`, `dto.CodePayloadLarge="payload_too_large"`).
  New codes B3 may add: `"insufficient_disk_space"`, `"checksum_mismatch"`, `"remote_changed"`.

### Secure HTTP building blocks — OWNED BY B2, CONSUMED BY orchestrator (wired into `NewDownloader`)
- Package `download`, new file `security.go` (+ `httpclient.go` if needed).
- `func ValidateURL(raw string) error` — scheme allowlist (http/https only) → wraps `ErrUnsupportedScheme`/`ErrInvalidURL`.
- `func SecureTransport(base *http.Transport) *http.Transport` — sets `TLSClientConfig.MinVersion = tls.VersionTLS12`, sane timeouts; returns a transport the orchestrator installs on the shared client.
- `func RedirectPolicy(req *http.Request, via []*http.Request) error` — caps redirects (≤10), strips `Authorization`/cookies on cross-host redirect; assignable to `http.Client.CheckRedirect`.
- B2 must NOT edit `downloadRequest.go`; orchestrator wires these three in.

### Integrity helpers — OWNED BY B1, CONSUMED BY orchestrator (wired into segmented finalize)
- Package `download`, new files `integrity.go`, `preflight.go`.
- `func PreflightDiskSpace(dir string, needBytes int64) error` — wraps `ErrInsufficientDiskSpace`.
- `func VerifyChecksum(path, algo, expected string) error` — algo in {"sha256","md5"}; wraps `ErrChecksumMismatch`; empty expected = no-op (returns nil).
- `func ValidateResume(remoteETag, remoteLastMod, savedETag, savedLastMod string, remoteSize, savedSize int64) bool` — returns true if a saved partial is still safe to resume.

### Scheduler / queue — OWNED BY B5, CONSUMED BY orchestrator (optional wiring)
- New package `backend/internal/pkg/scheduler`. `New(maxConcurrent int)` returning a struct with
  `Submit(id string, priority queue.Priority)`, `Next(ctx) (id string, ok bool)`, `Done(id)`, `SetMax(n)`.
  Backed by `queue.PriorityQueue`. Must be leak-free and `-race` clean. This is opt-in; orchestrator
  decides whether to route `StartAllJobs` through it. B5 also owns `backend/internal/pkg/queue/`.

### API surface additions — OWNED BY B4
- Owns ALL of `backend/internal/pkg/api/` (handlers, dto, middlewares, routes) EXCEPT it must keep
  `dto` error codes stable. New endpoints: `POST /downloads/:id/retry`, `PATCH /downloads/:id/priority`.
- New middleware files: `requestid.go`, `recovery.go`, `ratelimit.go` (token-bucket via `golang.org/x/time/rate`), wired in `setupMiddlewares.go`.
- SSE heartbeat in `handlers/stream.go`. Update `writeServiceError` in `response.go` to use
  `download.Classify` (consume B3) — guard so it still compiles if called before integration by
  falling back to current string matching.
- Handlers call new manager methods `RetryJob(ctx, id)` / `SetJobPriority(id, p)` — the orchestrator
  adds these thin methods to `manager.go`. B4: assume they exist with those signatures.

### Frontend shared primitives — OWNED BY F2, CONSUMED BY F1
- `frontend/src/lib/toast.ts` — re-export wrapper around `sonner`: `export const toast` with
  `.success/.error/.info/.loading`. F1 imports from `@/lib/toast`.
- `frontend/src/components/ui/sonner.tsx` (Toaster) + `error-boundary.tsx` + `connection-banner.tsx`.
- F2 owns `frontend/src/components/`, `frontend/src/features/layout/`, `frontend/src/index.css`.
- F1 owns `frontend/src/features/dashboard/` (new feature components), `frontend/src/hooks/`
  (new hooks only — do not break existing), and may add `frontend/src/_lib/services` calls.
- Orchestrator mounts `<Toaster/>` and `<ErrorBoundary/>` in `App.tsx`.

## Agent roster & ownership

| Agent | Theme | Owned paths (new files unless noted) |
|------|-------|------|
| **B1** | Integrity & resume safety | `download/integrity.go`, `preflight.go`, `resume_validate.go` (+tests) |
| **B2** | Security hardening (SSRF/TLS/scheme/redirect) | `download/security.go`, `httpclient.go` (+tests) |
| **B3** | Structured errors + slog logging | `download/errors.go` (+test); new pkg `backend/internal/pkg/logging/` |
| **B4** | API features + middleware + SSE | all of `backend/internal/pkg/api/**` |
| **B5** | Concurrency scheduler + priority queue | new pkg `backend/internal/pkg/scheduler/`; `backend/internal/pkg/queue/**` |
| **F1** | New frontend features (retry, priority, checksum badge, speed sparkline, toasts, shortcuts) | `frontend/src/features/dashboard/**` (new), `frontend/src/hooks/**` (new) |
| **F2** | Reliability UX + a11y + toasts/error-boundary/theme contrast | `frontend/src/components/**`, `frontend/src/features/layout/**`, `frontend/src/index.css` |

## Integration points the orchestrator will wire (do not do these yourselves)
1. `NewDownloader`: install `SecureTransport` + `RedirectPolicy`; call `ValidateURL` at job creation.
2. Segmented finalize: `PreflightDiskSpace` before allocating, `VerifyChecksum` after assembling.
3. `manager.go`: add `RetryJob`, `SetJobPriority`; optionally route `StartAllJobs` via `scheduler`.
4. `response.go`/`writeServiceError`: use `download.Classify`.
5. `App.tsx`: mount `<Toaster/>` + `<ErrorBoundary/>`.

## Verification gate (orchestrator, after all agents land)
`cd backend && gofmt -l . && go build ./... && go vet ./... && go test -race ./...`;
`cd frontend && npx tsc --noEmit && npm run build`; then `wails build -tags webkit2_41`.
