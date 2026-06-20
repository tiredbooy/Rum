# Rum — Reliability & UX Overhaul (2026-06-20)

Design doc + roadmap for the "make downloads bulletproof and the app addictive" pass.
This is the shared contract for the parallel builder agents.

## Context

Rum is a Wails v2 desktop download manager: Go engine + HTTP API (Gin) + SSE, React 19 / TS /
Vite / Tailwind / shadcn frontend. Multi-connection (segmented) range downloads, pause/resume via a
`*.rumparts` sidecar, priority scheduler, bandwidth windows, categories.

A user downloaded a 39 GB ZIP across several days (pause/resume in chunks) and got a **corrupted
header** on extraction. Top priority is making segmented + paused/resumed downloads provably
reliable, then a set of UX upgrades.

## Goals (this pass)

1. **Eliminate silent corruption** in segmented / paused / resumed downloads.
2. **Verify + smart-repair** so an already-finished (possibly corrupt) file can be checked and
   only its bad/missing segments re-fetched.
3. **Buttery progress** (interpolated, never lurching) + per-download live speed sparkline.
4. **Bulk multi-select + Delete All + batch actions**, drag-&-drop add, drag-to-reorder queue.
5. **Rich desktop notifications + optional completion sound.**
6. **Four new settings groups:** always-verify integrity, auto-retry/resume policy, theme
   accent/density/reduced-motion, temp dir + keep-partial-on-failure.
7. A **roadmap** for the bigger "addictive" features (built deliberately next).

Non-goals this pass: browser extension, account sync, plugin system, torrents (see roadmap).

---

## Root-cause analysis: the corruption

Verified by reading `backend/internal/pkg/download/segmented.go` end-to-end. The offset math is
correct (contiguous non-overlapping segments, absolute `WriteAt`); the "short write" theory is a
red herring — `*os.File.WriteAt` returns an error on a short write per the `io.WriterAt` contract.
The real causes:

### C1 — No resume revalidation across sessions (prime suspect)
The sidecar (`partsMeta`, `segmented.go:130-150`) stores only `TotalSize` + `URL`. Resume re-issues
`Range: bytes=start-end` (`segmented.go:381`) with **no `If-Range`** and no validator check. If the
remote bytes changed between sessions (CDN re-shard, re-upload, mirror swap, redirect drift), resume
staples **new-version bytes onto old-version bytes** — same total size, perfect-looking progress,
silently corrupt archive. This is the classic multi-day-resume corruption.

### C2 — Unsafe durability ordering on crash/power-loss
- Sidecar `Done` counters are snapshotted every 2 s (`segmented.go:267-292`) with **no fsync of file
  data first**, and the sidecar write itself is **not fsync'd** (`segmented.go:152-162`). After a hard
  kill/power-loss the metadata can claim more progress than is durably on disk → resume skips a hole.
- **Pause never fsyncs**: `outFile.Sync()` sits *after* `if err != nil { return err }`
  (`segmented.go:319-325`), so a paused download's tail lives only in page cache.

### C3 — No integrity verification for normal downloads
`VerifyChecksum` runs only when the user supplied a checksum (`segmented.go:330`). For a random ZIP
there is none, so completion is declared, the sidecar deleted, and nobody notices. A plain size check
can't catch it either: `Truncate(totalSize)` makes the file **sparse**, so unwritten holes read back
as zeros and the size still matches.

---

## Engine design (Task 2 — Go, `backend/internal/pkg/download`)

All changes TDD'd with a local `httptest` range server that can simulate: 206 partial content,
changed `ETag`/`Last-Modified`, mid-stream disconnect, and a forced "hole".

### E1 — Validator-aware, revalidating resume
- Extend `partsMeta` with `ETag`, `LastModified`, `ContentLength`, and a small `Version` int.
- On first segment open, capture `ETag`/`Last-Modified` (from the initial HEAD/GET used to size the
  file; thread it through `Job`/`Options` or capture on the first segment response).
- Persist them in the sidecar.
- On resume, send `If-Range: <etag-or-lastmod>` together with `Range`. Per RFC 7233 the server
  returns `206` if the validator still matches, or `200` (full body) if it changed.
  - On `206`: safe — continue.
  - On `200` (validator changed): **abort the resume cleanly, discard the sidecar, truncate, and
    restart from zero** (do NOT write the full body into one segment's offset). Surface a clear,
    non-alarming status ("source changed, restarting").
- Also reject `200` in the existing defensive guard (already present) but now distinguish
  "validator changed" from "server ignored range".
- If the server sends no validators at all, record that fact; resume is then best-effort and the
  always-verify pass (E3) becomes the safety net.

### E2 — Correct durability ordering
- Establish the invariant **"never record progress you haven't made durable."** Before each sidecar
  snapshot: `outFile.Sync()` the data, *then* write the sidecar.
- Make the sidecar write durable: write tmp, `f.Sync()`, `rename`, then fsync the **parent dir**.
  Add a `fsyncDir(path)` helper.
- On pause/cancel: fsync data and flush a final sidecar **before** returning (move the sync above the
  `if err != nil` early return, or run it in a deferred finalizer that distinguishes pause from error).
- Defensive: check `WriteAt` returned `n == len(buf[:n])` and error on mismatch (belt-and-suspenders).

### E3 — Always-verify on completion (size + completeness; optional hash)
- After `g.Wait()` succeeds, before declaring done: verify every segment is `complete()` and the sum
  of segment sizes equals `totalSize`, and the on-disk file size equals `totalSize`. (Catches holes
  the size check alone would miss, since completeness is tracked per segment.)
- If the user supplied a checksum, keep verifying it. Add an **optional** `VerifyIntegrity` setting
  (E4) that, when on, computes a full-file hash after download and stores it in the sidecar/job for
  later re-verification (so a future "verify" doesn't need the server).
- On any verification failure: **keep the sidecar**, mark the job error with an actionable message,
  do NOT delete partial data.

### E4 — VerifyFile + RepairFile primitives (for verify + smart repair)
- `VerifyFile(ctx, job) (ok bool, badSegments []int, err error)`: recompute segment plan from
  `totalSize`/connections, optionally re-HEAD to confirm size/validator, and detect bad/missing
  segments. If a stored full-file hash exists, check it; otherwise re-validate by re-fetching segment
  checksums only when `--deep`/`force` is requested (cheap path: structural/size/sparse-hole check).
- `RepairFile(ctx, job, badSegments)`: re-open the file, re-fetch **only** the bad/missing byte ranges
  (reusing `downloadSegment`), re-run E3 verification. Reuses all the E1/E2 safety.
- These back the new API endpoints (Task 4).

### Engine acceptance tests
- Resume after simulated crash with unflushed tail → no hole (E2).
- Resume when server `ETag` changed → detected, clean restart, correct final bytes (E1).
- Completion with an injected hole → verification fails, sidecar kept (E3).
- `RepairFile` re-fetches only the bad segment and produces a byte-correct file (E4).
- All existing download tests still pass.

---

## Backend design (Task 4 — Go, `config` + `api`; owns the FE settings *type* contract)

### B1 — New settings (extend `Setting`, `SettingReq`, `Validate`, `setDefaults`, `applyMissingDefaults`)
- `VerifyIntegrity bool` (`verify_integrity`) — verify every finished download; default **true**
  (this is the bug fix; make safety the default).
- Auto-retry/resume policy:
  - `AutoResumeOnReconnect bool` (`auto_resume_on_reconnect`, default true)
  - `AutoResumeOnLaunch bool` (`auto_resume_on_launch`, default true)
  - `RetryBackoffSec int` (`retry_backoff_sec`, default reuse current backoff; clamp 1–60)
  - (`MaxRetries` already exists — surface it in the new UI group too.)
- Theme/appearance:
  - `AccentColor string` (`accent_color`, hex, validated; default "" = app default)
  - `UIDensity string` (`ui_density`: "comfortable"|"compact"; default "comfortable")
  - `ReducedMotion bool` (`reduced_motion`, default false)
- Temp/partial:
  - `TempDir string` (`temp_dir`, default "" = next to output)
  - `KeepPartialOnFailure bool` (`keep_partial_on_failure`, default true)
- Validate/clamp all; add to `setDefaults`/`applyMissingDefaults`. Keep JSON keys stable.

### B2 — Endpoints (Gin, `routes.go` + handlers)
- `POST /downloads/:id/verify` → engine `VerifyFile`; returns `{ ok, badSegments, checkedAt }`.
- `POST /downloads/:id/repair` → engine `RepairFile`; streams back to running state, repairs bad
  segments, re-verifies. Idempotent; safe to call on a completed job.
- Confirm `DELETE /downloads?status=all` cancels in-flight jobs, removes sidecars + partial files
  (honoring `KeepPartialOnFailure` only for failures, not explicit delete), and is atomic w.r.t. the
  scheduler. Add `status=completed|error|all` coverage + a guard so deleting a running job pauses/cancels it first.
- Wire `TempDir` into `PrepareOutputPath` (write `.part` under temp dir, move on finalize) and
  `KeepPartialOnFailure` into the error path.

### B3 — Frontend type/api contract (so Task 6 consumes, not guesses)
- Add the new fields to `frontend/src/_lib/types/setting-types.ts`.
- Add `verifyDownload(id)` / `repairDownload(id)` to `download-api.ts` + react-query mutations.

---

## Frontend design

### F1 — Buttery progress + sparkline (Task 3 — `features/dashboard/download-list`)
- New `useSmoothProgress(targetPct, isActive)` hook: holds a displayed value, on each animation frame
  eases toward the SSE target at a bounded rate (e.g. lerp factor or max %/frame), so the bar moves
  continuously between sparse SSE ticks and never jumps backward. Snap to 100 on complete; reset on
  restart. Respect `reduced_motion` (fall back to the current CSS transition).
- `DownloadProgress.tsx` consumes the hook for `width`. Keep the gradient; add a subtle indeterminate
  shimmer while `running` with 0 recent bytes.
- New `SpeedSparkline.tsx`: a small rolling chart of recent speed samples (ring buffer in the
  progress store / local state from SSE speed). Lightweight SVG/canvas, no heavy chart lib.
- Do **not** restructure `useAllProgressStream.tsx`; read from it.

### F2 — Bulk select + Delete All + drag/drop + reorder (Task 5 — `features/dashboard`)
- Selection store (new `useSelection` zustand): selected ids, shift-range, ctrl-toggle, select-all /
  select-by-filter, clear.
- Checkbox affordance on `DownloadCard` (appears on hover / when any selection active); row click +
  modifiers select.
- Bulk action bar (appears when selection non-empty): Delete (uses existing `deleteDownloads`/single
  delete loop), Pause, Resume, Retry, with a count + confirm for destructive actions.
- **Delete All** button in `Toolbar.tsx` (the explicitly requested feature) → confirm dialog →
  `deleteDownloads("all")`. "Clear completed" stays.
- Drag-&-drop: drop URLs (text/uri-list) or local files onto the window → open Add dialog prefilled /
  batch-add. Global drop overlay.
- Drag-to-reorder the queue within a status; on drop, persist order via the existing priority API
  (or a new lightweight order field if needed — prefer reusing priority).
- Owns only `features/dashboard/**` + the new selection store. Does NOT edit App.tsx/theme/notifications.

### F3 — Settings UI + notifications + theme (Task 6 — `features/settings`, theme, notifications; owns App wiring)
- New settings sections in `SettingForm.tsx` for B1's four groups, using existing form primitives;
  consume the types from B3. Color picker for accent, density toggle, reduced-motion switch.
- Verify/Repair buttons on completed `DownloadCard` overflow menu (calls B3 mutations) — coordinate
  with Task 5's card edits by only adding menu items (additive), or expose via a small shared hook.
- Theme provider: apply `accent_color` (CSS var), `ui_density` (class on root), `reduced_motion`
  (class + respects `prefers-reduced-motion`). Live-applies on settings change.
- Notifications service: subscribe to download status transitions (from the SSE/react-query cache);
  on `completed`/`error` fire a native notification (Wails/Notification API) honoring `silent`; play an
  optional completion sound (bundled short asset) gated by a setting. Mount once in App.
- Owns `App.tsx`, theme provider, notification service, `setting-types` UI usage.

---

## Verification strategy (Task 7)

- Engine/backend: `cd backend && go build ./... && go vet ./... && go test ./...`; root module
  `go build ./...`.
- Frontend: `cd frontend && npm run build` (tsc + vite) and `npm test` (vitest).
- Manual smoke (best-effort): start a download, pause mid-way, resume, confirm byte-correct; trigger
  verify/repair; Delete All; toggle each new setting.
- Commit to `dev`, push to `origin/dev`. **No Claude co-author line** (user instruction).

---

## Roadmap (deliberate, next passes)

**Reliability & power**
- Segment-level rolling hashes persisted in the sidecar for instant, server-free verify.
- Mirror / multi-source download (fetch one file from several URLs, fastest-segment wins).
- Smart connection auto-tuning per host (probe throughput, adapt segment count live).
- Checksum auto-discovery (sidecar `.sha256`/`.md5`, release-page parsing).
- Scheduler 2.0: per-category concurrency, time-of-day queues, "download when idle/on Wi-Fi".

**Capture & flow**
- Browser extension / native messaging capture; "copy link → auto-add" with rules.
- Clipboard-watch UX upgrade (toast to add detected links).
- Batch from page: paste a page URL, extract media/file links.
- Remote control: phone web UI to add/monitor downloads on the desktop engine.

**Delight & retention (the "addictive" layer)**
- Live global throughput dashboard (aggregate sparkline, today's data, streak/stats).
- Completion celebrations (subtle confetti/sound, "X GB saved this week").
- Themeable skins / accent presets, app icon badges, mini-player-style compact mode.
- Per-download notes/tags, search, smart folders.
- Achievements/stats ("fastest download", "most this month") — gamified but non-intrusive.

**Trust**
- Optional malware/hash reputation check on completion.
- Bandwidth/usage analytics local-only, exportable.
