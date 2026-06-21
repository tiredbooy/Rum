# Rum — Publish-Readiness Roadmap

Generated 2026-06-21 from an 18-lens multi-agent audit (backend correctness/concurrency/security/reliability/API/quality, frontend UX/a11y/perf/quality/i18n, product feature-gaps, and release infra). 156 raw findings, deduped below.

**Verdict: close, but not shippable yet.** The download *engine* is genuinely strong (`go vet` clean, `-race` tests pass, careful segmented resume + fsync durability). What blocks a public launch is a cluster of (1) data-loss / broken-core-flow bugs, (2) an unauthenticated local-API + dead security guards, and (3) missing release infrastructure (the CI/release pipeline was deleted, the LICENSE is corrupted, the lockfile points at a private mirror).

Legend: 🔴 P0 = must fix before publish · 🟠 P1 = needed for a strong v1 · 🟢 P2 = post-launch / net-new.

---

## 🔴 P0 — Publish blockers

### A. Data loss & broken core flows
| # | Issue | Evidence | Fix | Effort |
|---|-------|----------|-----|--------|
| A1 | **`DeleteJobFromDisk` inverted filter wipes ALL history on every single delete** | `manager.go` delete path keeps only the deleted job | Invert to `if item.ID != jobID { keep }`, or delete from the map and `SaveJobsToDisk` like other mutations. Add a 3-jobs-delete-1 regression test. | trivial |
| A2 | **`saveToDisk` ranges the jobs map without the lock → fatal `concurrent map iteration and map write` crash** | called from unlocked completion/repair/priority paths + a bare TUI goroutine | `RLock` + snapshot map before serialize (or require callers hold the lock); run manager tests with `-race` in CI | small |
| A3 | **Background download goroutines have no panic recovery — one panic crashes the whole app** | `runDownload` and other long-lived goroutines | `defer recover()` helper that logs + marks the one job failed | small |
| A4 | **"From URL" add tab is a dead flow** — fetched/selected URLs never reach the draft the dialog saves | `LoadFromUrlForm` never calls `updateDraft` (vs `BulkDownloadForm.tsx:44`) | wire selection → `updateDraft({urls,dest_path})`; add integration test | medium |
| A5 | **Global file-conflict policy (overwrite/skip/rename) not applied to normal downloads — existing files silently overwritten** | policy persisted but `PrepareOutputPath` ignores it | apply `resolveConflict` in `PrepareOutputPath`/before final move; test each policy | medium |
| A6 | **Per-download "Scheduled start" ignored when AutoStart is on (the default)** | `CreateDownload` auto-starts even with a future `StartAt` | skip auto-start when `StartAt` is future; `startDueJobs` honors per-job `StartAt` | small |

### B. Security
| # | Issue | Evidence | Fix | Effort |
|---|-------|----------|-----|--------|
| B1 | **Local API has zero authentication** — any local process (or a web page via DNS-rebind/CSRF to 127.0.0.1) can drive every endpoint | no auth middleware on `/api/v1` | mint a random per-session bearer token at startup, hand to frontend via `GetApiBase()`, require it in middleware (constant-time compare) + strict Host-header allowlist | medium |
| B2 | **Unauthenticated `PATCH /settings` can arm machine shutdown/sleep/close** on next download completion | post-download power actions driven by persisted settings | gate behind B1 token; require explicit recent in-UI opt-in; don't silently persist a destructive action across sessions | small |
| B3 | **Arbitrary download/categorize destination** via unvalidated `OutDir`/`TempDir`/absolute category `DestDir` | settings accept any absolute path | validate/confine destinations; warn on system paths | medium |
| B4 | **Proxy setting is a dead feature** — exposed in UI + persisted, never applied to any transport | `config.Proxy` never read by the dialer | set `Transport.Proxy = http.ProxyURL(u)` (http/https/socks5 via `x/net/proxy`); validate scheme. **Near-mandatory for the Iran/Persian audience.** | small |
| B5 | **SSRF guard `IsPrivateHost` is dead code** — downloads to loopback/private/cloud-metadata hosts unrestricted; redirects not re-validated | guard written, never wired to `DialContext` | wire default-on SSRF check at dial time (re-check post-DNS to defeat rebinding), settings toggle for LAN use | medium |

### C. Release infrastructure
| # | Issue | Fix | Effort |
|---|-------|-----|--------|
| C1 | **All CI/CD + the release pipeline were deleted last commit** → every advertised prebuilt installer (AppImage/.deb/.exe/.dmg) is now unproduced | restore `.github/workflows`: lint (golangci-lint, eslint, tsc) + `go test`/`vitest` + cross-platform `wails build` matrix + tag→release upload | medium |
| C2 | **Frontend lockfile pins all 573 deps to a private mirror `npm.devneeds.ir`** — `npm ci` 403s off-LAN (only the deleted workflow rewrote it) | repoint lockfile to the public npm registry; verify clean `npm ci` | small |
| C3 | **LICENSE file is corrupted into legally-invalid text** | restore the intended OSI license verbatim (confirm which: MIT/Apache-2.0/GPL) | trivial |
| C4 | **README & INSTALL.md advertise a Releases/installer path that can't exist** (CI gone) | fix after C1, or soften docs to build-from-source until releases ship | small |
| C5 | **No persistent log file in the shipped GUI** — logs go to a stderr nobody can read | redirect stdlib log + slog to a rotating file under `os.UserConfigDir()/rum/logs/` (lumberjack) in `main.go` before serve | medium |
| C6 | **`InitLogFile` has an inverted error check → debug log is a permanent no-op** | fix the condition; covers C5's dev side | trivial |
| C7 | **No single source of truth for version** (1.0.0 vs 0.1.1) and ldflags inject a nonexistent variable | one version var, injected via ldflags that actually matches; surface in UI/About | small |
| C8 | **`Rum.desktop` ships hardcoded developer paths** — `Exec`/icon broken on any other machine | use install-relative paths / proper desktop-entry templating | trivial |
| C9 | **`.env` is git-tracked** | untrack, add to `.gitignore`, rotate anything sensitive | trivial |

---

## 🟠 P1 — Needed for a strong v1

**Reliability / engine**
- No idle/stall read timeout on the transfer body — a stalled server hangs a download forever (add a stall watchdog → retryable). *high*
- Single-stream (non-segmented) resume sends `Range` with no `If-Range` validator and stores no hash → silent byte-splice corruption if the source changed. *high*
- No filename-collision handling — distinct URLs with the same filename clobber each other (allocate `name (1).ext` like IDM/browsers). *high*

**API correctness**
- `CreateDownloadRequest` fields `dest_path/filename/group_folder/speed_limit/user_agent/referer/max_retries` are accepted + validated then **silently dropped** — thread them through the engine or reject them (the contract currently lies). *high*
- Global gzip middleware wraps the SSE stream, buffering/defeating real-time progress. *high*

**UX**
- No feedback (toast) on the most common actions: add / start / pause / resume / delete. *high*
- Single-card delete destroys the file with no confirmation or undo. *high*
- Add dialog never resets its draft — reopens pre-filled with the last download. *medium*
- Triple/duplicate completion notifications from 3 independent sources (Go poller + engine + frontend); engine one has a "Downlods" typo + wrong per-job body. *high*

**i18n / RTL** (high-impact for a Persian audience)
- RTL is structurally impossible — no `dir` attribute is ever set, `DirectionProvider` never mounted. Strings are hardcoded English; no language switcher; dates/numbers not Jalali/Persian-digit formatted. Adopt i18next (or react-intl), wire `dir`, switch the Persian calendar on. *high*
- Clipboard auto-capture is shipped but dead — Go emits `clipboard:url`, no frontend `EventsOn` listener. *high*

**Quality / hygiene**
- The structured-logging package (`pkg/logging`) has zero importers; 37 stdlib `log.*` + raw `fmt.Print` instead. Unify. *medium*
- 7 files not `gofmt`-clean; no ESLint actually configured (despite `@eslint/js` in devDeps). *small*
- `zod`/`@hookform/resolvers`/`axios` declared but unused → zero runtime validation of backend responses (`request<T>` blind-casts). Add zod parsing at the API boundary. *medium*
- TypeScript pinned to 4.x while consuming React 19 / TS-5.2+ type defs — survives only via `skipLibCheck`. Bump to TS 5.x. *small*
- Frontend list not virtualized though `react-virtual` is a dep; 161 KB tray PNG rendered at 8×8; no `manualChunks`; dead `motion/react`/animate-ui imports (would crash the build if wired). *medium*

---

## 🟢 P2 — Net-new features (competitive gaps)

Ranked by user impact for a download manager:
1. **Browser integration** — a "send to Rum" extension or capture flow (every competitor has this). *large*
2. **Video/stream grabbing (HLS/DASH) via yt-dlp** — IDM/FDM headline feature; 1000+ sites. *large*
3. **BitTorrent / magnet support** — FDM/Motrix/qBittorrent core. *large*
4. **Per-download speed limit & per-download destination/filename** (partly already in the DTO — see P1 silently-dropped fields). *medium*
5. **Cross-platform system tray + close-to-tray** (currently Windows-only; Linux/macOS are the stated targets). *medium*
6. **Auto-update** (Wails ships none) — at minimum an update-available check against GitHub Releases. *medium*
7. **First-run onboarding** — default download dir prompt, feature tour. *small*
8. **Import URL list from file / OS protocol handler (`rum://`) / right-click context menu.** *medium*

---

## ⚡ Quick wins (≤1 hr each, do first)
- A1 invert `DeleteJobFromDisk` (one line + test) — stops history wipe
- A3 panic-recovery defer in download goroutines
- B4 wire the proxy (small, high audience value)
- C3 restore LICENSE · C6 fix inverted log check · C8 fix `Rum.desktop` paths · C9 untrack `.env`
- `gofmt -w` the 7 dirty files

## Suggested sequence
1. **Stop-the-bleeding bugs:** A1, A2, A3 (data loss + crashes) — with `-race` tests.
2. **Security pass:** B1+B2 (auth + destructive-action gating), B4 (proxy), then B3/B5.
3. **Core-flow fixes:** A4, A5, A6 + the P1 API/UX correctness items.
4. **Release infra:** C1–C9 so installers can actually be produced and trusted.
5. **i18n/RTL + polish**, then **P2 features** for a standout v1.
