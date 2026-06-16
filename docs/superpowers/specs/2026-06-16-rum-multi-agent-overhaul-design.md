# Rum Multi-Agent Enhancement Sweep — Design Spec

Date: 2026-06-16
Branch: `feat/engine-api-cli-frontend-overhaul`
Status: Approved — implementation in progress

## Goal

Improve the Rum download manager across its four subsystems (download engine,
HTTP API, CLI/TUI, desktop frontend) using five scoped agents working in a
shared tree with disjoint path ownership. Deliver one PR with one commit per
area, an improvements doc, and GUI + CLI installers for Linux and Windows.

## Architecture (as-found)

Rum is a Wails desktop download manager.

- **Root module** `github.com/tiredbooy/Rum` — Wails app (`app.go`, `main.go`,
  `tray.go`) + React/TS frontend.
- **Backend module** `github.com/tiredbooy/Rum/backend`:
  - `internal/pkg/download` — engine: `JobManager` (semaphore-bounded parallel
    jobs), single-stream range download + resume, SSE progress publishing.
  - `internal/pkg/api` — Gin HTTP API (`/api/v1/...`), SSE streaming.
  - `internal/pkg/tui` + `cmd/rum` — Bubbletea TUI and CLI entrypoint.
  - `internal/pkg/{file-system,queue,config,utils,format}` — supporting.
- **Frontend** `frontend/src` — React + TS + Tailwind + shadcn/ui, TanStack
  Query, Zustand stores, SSE progress hooks.

## Confirmed problems

### Download engine (`backend/internal/pkg/download`)
1. `download.go:~93` speed limiter uses `SpeedLimitKB * 102` (should be `*1024`);
   per-job override branch is dead code. Bandwidth limiting is broken.
2. No segmented/multi-connection downloads — single stream even when server
   supports `Range`. README's "parallel downloads" is only parallel jobs.
3. `Options.MaxRetries` parsed/stored but never used — no retry/backoff.
4. `downloadRequest.go` sets `Accept-Encoding` manually, disabling Go's
   transparent decompression and making `Content-Length`/resume unreliable.
5. `manager.go` progress `int(downloaded/total*100)` divides by `total` that may
   be `-1`/`0` → NaN/garbage.
6. `manager.go` `CreateJobsFromURLs` nested HEAD goroutine leaks on hang.
7. `manager.go` `DeleteJobsByFilter` resets `m.urls` to empty without
   repopulating; can't filter "failed".
8. `manager.go:530` `fmt.Errorf("%w", err.Error())` — wrong type (vet error).
9. `Transport.MaxConnsPerHost: 2` throttles throughput.

### API (`backend/internal/pkg/api`)
- Inconsistent/unversioned error shapes, no pagination/filtering on list, no
  request validation envelope, no graceful shutdown, permissive CORS, no
  per-job speed-limit endpoint.

### CLI/TUI (`backend/internal/pkg/tui`, `cmd/rum`)
- Hand-rolled `flag` parsing, blocking `fmt.Scanln` prompts (no non-interactive
  mode), thin `--version/--help`, no subcommands.

### Frontend (`frontend/src`)
- Duplicate `_lib`/`lib` dirs, SSE reconnection robustness, missing
  error/empty/loading states, a11y gaps, progress/ETA tied to buggy backend
  field.

## The five agents — disjoint path ownership

| # | Agent | Owns (edits only here) | Mandate |
|---|-------|------------------------|---------|
| 1 | Frontend | `frontend/**` | UX/robustness: SSE reconnect, error/empty/loading states, a11y, dedupe `_lib`/`lib`, resilient progress/ETA. |
| 2 | Download engine | `backend/internal/pkg/download/**` | Owns shared types. Segmented multi-connection downloads (fallback to single-stream), retry+backoff, fix speed limiter/progress/gzip/leak/DeleteJobsByFilter/vet. Additive, backward-compatible exported APIs only. |
| 3 | API | `backend/internal/pkg/api/**` | go-api-design: consistent error envelope, pagination/filtering, validation, graceful shutdown, tighten CORS, per-job speed-limit endpoint. |
| 4 | CLI/TUI | `backend/internal/pkg/tui/**`, `backend/cmd/rum/**` | Subcommands, non-interactive flags, `--version/--help`, TUI resilience. |
| 5 | Supporting | `backend/internal/pkg/{file-system,queue,config,utils,format}/**` | go-concurrency: priority download queue, config validation, filesystem safety (atomic writes, path-traversal guards), util correctness. Additive, backward-compatible exported APIs only. |

### Coordination rules
- Agents edit **only** their owned paths. They may **read** any package.
- If an agent needs a change outside its scope, it **records a "requested
  upstream change"** in its report rather than editing — reconciled at integration.
- Agents 2 and 5 keep existing **exported** signatures backward-compatible
  (additive changes only) so consumers (3, 4) keep compiling.
- Integration gate: `go build ./... && go vet ./...` (backend) + frontend build.

## New features (ambitious, in scope)
- Segmented multi-connection downloads with graceful single-stream fallback.
- Auto-retry + resume with exponential backoff.
- Priority download queue (currently-underused `queue/` package).
- Per-job + global speed limits wired engine → API → frontend.
- Graceful shutdown + structured errors (API).
- Non-interactive CLI for scripting.

## Delivery
- One branch, one commit per area (frontend, engine, api, cli, supporting, docs,
  installers). **No `Co-Authored-By: Claude` trailer.**
- Verify push access to `tiredbooy/Rum`; once granted, push and open one PR.
- Improvements doc in `docs/`.
- Installer scripts: GUI (Wails) + CLI for Linux and Windows.

## Testing / verification
- Backend: `go build ./...`, `go vet ./...`, `go test ./...`, targeted unit tests
  for new engine logic (segmenting, retry, speed limiter math).
- Frontend: typecheck + build.
- Manual smoke where feasible.
