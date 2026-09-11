# Working-tree review

Untracked note, 2026-09-07. Nothing is committed or staged.

Two agents worked in this tree. **This review covers my work** (the settings
audit + the Go-toolchain download work); where a file is mostly the *installer*
agent's, that is called out so you do not read their lines as mine.

---

## 1. Overview

**43 tracked files modified (+7,752 / −2,796), 27 new files.**

| Area | Modified | New files | Whose |
|---|---|---|---|
| Settings backend (Go) | 5 files, +407/−32 | `reconnect.go` (185 ln) | mine |
| Settings API handlers (Go) | 3 files, +145/−41 | — | mine |
| Settings desktop shell (`app.go`, `desktop.go`) | 2 files, +93/−3 | — | mine |
| Settings frontend (TS/TSX) | 16 files, +793/−1,122 | 10 files, 809 ln | mine |
| New tests | — | 13 files, 2,028 ln | mine |
| Generated Wails bindings | 2 files, +7/−0 | `models.ts` (21 ln) | mine (regenerated) |
| `frontend/package-lock.json` | +573/−573 | — | mine |
| Installers / build scripts | 7 files, ~335 ln | — | **mine** (module-proxy + precedence) |
| Installers / build scripts | 10 files, ~4,750 ln | — | **installer agent's** |
| Docs / reports | `INSTALL.md`, `INSTALLER-HARDENING.md` | `SETTINGS-AUDIT.md` | shared |

Note the frontend is **net −329 lines**: nine settings cards were rewritten
smaller and four oversized files split out.

---

## 2. Settings backend (Go)

### `backend/internal/pkg/config/setting.go` (+200/−13)

The densest file. Six distinct changes.

**a) Absent-vs-false, the "upgraders silently ran unverified" bug** — `presentKeys`
(`:356`) + `defaultTrueBools` (`:691`, used `:734`).

```go
// A JSON-missing bool unmarshals to false, indistinguishable from a deliberate
// "off" — so the default-TRUE flags were left off and every upgrader silently
// ran with integrity verification and auto-resume disabled while the UI showed
// them enabled. presentKeys removes the ambiguity.
for key, field := range s.defaultTrueBools() {
    if !present[key] { *field = true }
}
```

**b) `settings_version` + `migrate()`** (`:21`, `:323`, `:328`) — see §7, this is
the behaviour change that touches existing installs.

**c) `ErrRecovered`** (`:289`) — a corrupt `settings.json` self-heals but the load
still returns an error; treating that as fatal made `GET /settings` answer 500
and the page claim it could not load settings it was holding.

**d) `start_on_launch` → deprecated alias.** `Update` redirects it to the field
that actually drives behaviour (`:503`), `Validate` mirrors them so they can never
disagree on disk (`:490`). The key is never dropped.

**e) `NormalizeProxy`** (`:249`) + enum validators — an unusable proxy was
persisted, shown in the UI, and silently ignored by the engine.

**f) `LogLevel` added to `SettingReq`** so the new UI control can write it.

### `backend/internal/pkg/download/manager.go` (+103/−0, no deletions)

**`ApplySettings`** (`:922`) is the core fix for "settings save but do nothing".
Before it, only `Connections` and four reliability options reached the running
engine; download folder, speed limit, parallel count, retries, silent, proxy,
SSRF guard and auto-organize were all restart-only.

```go
m.opt.Out          = s.OutDir          // was never updated -> wrong folder until restart
m.opt.Categorize   = s.EnableCategories // was NEVER set -> auto-organize was dead
...
m.sched.SetMax(parallel)                // existed, was never called
governor.SetLimitKBps(limit)            // honours an active bandwidth window
```

Transport is rebuilt **only** when the proxy or SSRF flag changes, so a routine
save does not throw away the connection pool.

Also `:622` / `:653` — two `DebugLog` lines on the *main* download path. Without
them, turning Logging to Debug and finishing a normal download produced an
**empty** log, because every pre-existing `DebugLog` call sits on an unusual
branch.

### `backend/internal/pkg/download/schedule.go` (+15/−6)

`EffectiveSpeedLimitKBps` (`:81`) exported so a save applies the same limit the
controller would; `startDueJobs` now gated on `ScheduledStartEnabled` (`:164`) —
that flag was previously read by nothing. See §7.

### `backend/internal/pkg/download/debug.go` (+61/−11)

`debug.log` was opened unconditionally, so everyone accumulated one forever and
`log_level` controlled nothing. Now gated on `log_level=debug`, applied live via
`ApplySettings`, and mutex-guarded because the settings API can close the file
while download goroutines write to it.

### `backend/internal/pkg/download/reconnect.go` (new, 185 ln)

`auto_resume_on_reconnect` was **read by zero lines of code**. New
`ResumeController` re-queues network-failed jobs once their host answers, capped
at 10 attempts, permanent failures (404, checksum) excluded.

### `backend/internal/pkg/api/handlers/` (+145/−41 over 3 files)

- `loadSettings` (`settingHandler.go:25`) — shared by all six settings endpoints,
  fixes the corrupt-file 500.
- `validateSettingReq` — per-field 400s so the form can show a message inline;
  invalid enums used to return 200 and get silently rewritten.
- `validateWritableDir` (`:207`) — a bad download folder failed silently at
  download time; now rejected at save time.
- `applyDownloadOptions` → `GlobalManager.ApplySettings` (`:136`).
- `schedule.go` / `categories.go` push to the live engine after saving.

### `backend/cmd/server/main.go` (+28/−2)

`Categorize: setting.EnableCategories` at startup, the `ResumeController` wired
in and stopped on shutdown, and `InitLogging` now honours `log_level` instead of
hard-coding `"info"`.

### `app.go` (+45/−3), `desktop.go` (+48/−0)

`ChooseDir` logs and returns a readable error; new `Capabilities()` binding so the
UI can disable tray switches that do nothing off-Windows; `watchAutostart` /
`syncAutostart` applies "Launch on startup" within ~2 s instead of at next launch.

---

## 3. Settings frontend (TS/TSX)

Net **−329 lines**. The bulk of the deletions are the nine settings cards being
rewritten against shared controls rather than each hand-rolling its own row,
flash-state and patch call.

### The reported bug — `frontend/src/_lib/wails.ts` (+99/−20)

`chooseDir` caught **every** rejection and returned `null`, the same value a
cancel returns — so a picker that could not open was indistinguishable from a
button that does nothing, and nothing was logged.

```ts
export type ChooseDirResult =              // :104
  | { status: "picked"; path: string }
  | { status: "cancelled" }
  | { status: "unavailable" }
  | { status: "failed"; message: string };
```

Also now imports the generated binding (`:12`) instead of re-implementing it
against `window.go`.

### `useFolderPicker.ts` (new) — availability is state, not frozen

`CategoryManager` had `useRef(hasChooseDir())`, so if the shell injected bindings
after first render the Browse button was gone for the session.

### `settings.queries.ts` (+83/−10)

`useSettingsPatch` (`:57`) centralises the save flow every card repeated. On
failure it puts the message **inline on the offending field** (`:94`, via the new
`ApiError.fields`) and reverts the local draft, which previously kept showing a
value the server had rejected. `:85` depends on the stable `mutate`, not the
whole mutation object.

### `api.ts` (+41/−2)

New `ApiError` carrying `{error, code, fields}`. Previously `request` threw a bare
`Error` and every per-field message the backend sent was discarded.

### Cards — largest first

| File | Δ | Why |
|---|---|---|
| `CategoryManager.tsx` | +34/−218 | row extracted to `CategoryRuleRow.tsx`, defaults to `category-defaults.ts`; frozen-picker fix |
| `AppearanceSettings.tsx` | +45/−196 | accent picker extracted to `AccentPicker.tsx` (+ debounce, so a colour picked from the OS dialog is not lost) |
| `ScheduleEditor.tsx` | +26/−162 | window row extracted to `SpeedWindowRow.tsx`; scheduled-start help text now matches real behaviour |
| `IntegritySettings.tsx` | +106/−153 | `max_retries` moved here (was duplicated), new `block_private_hosts` toggle |
| `controls.tsx` | +101/−44 | labels now bound with `htmlFor` (they were unbound — screen readers had no name), plus inline error + `aria-describedby` |
| `DownloadSettings.tsx` | +74/−86 | bounds moved to `numeric-bounds.ts`, inline range messages |
| `DesktopSettings.tsx` | +41/−53 | tray toggles disabled where there is no tray, via `useDesktopCapabilities` |
| `StorageSettings.tsx` | +42/−103 | shared `SettingPathInput` |
| `GeneralSettings.tsx` | +54/−50 | shared path input + new Logging control |
| `PostDownloadSettings.tsx` | +24/−19 | proxy validation; "resets on restart" hint |

**New** (10 files, 809 ln): `AccentPicker`, `CategoryRuleRow`, `SpeedWindowRow`,
`SettingPathInput`, `useFolderPicker`, `useDesktopCapabilities`, `numeric-bounds`,
`proxy`, `category-defaults`, `ClipboardLinkWatcher`.

`ClipboardLinkWatcher` is the missing half of "Watch clipboard for links": Go
emitted a `clipboard:url` event that **nothing in the frontend subscribed to**.

---

## 4. New tests (13 files, 2,028 ln)

| File | Covers |
|---|---|
| `config/upgrade_fixture_test.go` | byte-accurate old-build `settings.json` — the migration, both legacy fields, disabled flags not resurrected |
| `config/defaults_backfill_test.go` | absent-vs-explicit-false, proxy normalisation, corrupt-file recovery |
| `config/legacy_fields_test.go` | `start_on_launch` alias; a source scan asserting nothing outside `config` reads it |
| `download/apply_settings_test.go` | every live-tunable option reaches the engine; transport rebuild only on proxy/SSRF change |
| `download/reconnect_test.go` | requeue on reconnect, gated by the setting, 404s ignored, attempt cap |
| `download/scheduled_start_toggle_test.go` | the toggle actually gates; bandwidth windows independent of it |
| `download/debug_test.go` | log level gates the trace, live toggle, idempotent |
| `handlers/settings_validation_test.go` | per-field 400s, unusable dirs, corrupt-file recovery |
| `autostart_sync_test.go` | really creates/removes the `.desktop` entry |
| `wails_bindings_test.go` | generated bindings match the Go methods by reflection |
| 3 frontend `.test.ts` | `chooseDir` outcomes, `ApiError` fields, proxy/bounds validation |

Go: 57 new test functions. Frontend: 49 → 77 tests.

---

## 5. Installers / build scripts

**Read this split carefully.** Of ~5,085 changed lines here, **~335 are mine**;
the rest is the installer agent's hardening, which I only reviewed.

**Mine** — the same two changes in all seven download paths
(`installers/gui/install-{linux,macos}.sh`, `installers/cli/install-{linux,macos}.sh`,
`build-windows.sh`, both `install-windows.ps1`):

1. **Module-proxy fallback.** When every archive host fails, fetch
   `golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>` through `GOPROXY`. The `go`
   command does the download, not curl, so it is checksum-database verified — a
   raw fetch would be unverifiable, because the sha256 published on go.dev is for
   the `.tar.gz` and cannot validate the module zip.
2. **Verified route preferred over unverified.** If the checksum index is
   unreachable, try the module proxy *before* installing an unchecked archive.
   Guarded by a named `go_can_bootstrap_module_proxy` — a machine with no Go at
   all cannot use that route, and preferring it there would deadlock the
   installer. `RUM_REQUIRE_VERIFIED_GO=1` makes the unverified last resort fatal.

Also mine, earlier: the `Sync-SessionPath` PATH-ordering fix in
`install-windows.ps1` (machine PATH was re-prepended after the pinned Go, which
would have handed Wails the wrong toolchain).

**Not mine:** `installers/uninstall-linux.sh`, `installers/README.md`,
`backend/install.sh`, `backend/install.ps1`, `backend/cmd/rum/build.sh`,
`build-linux.sh`, and the bulk of the seven files above.

---

## 6. Mechanical / low-signal changes — collapsed

| What | Size | Why it is noise |
|---|---|---|
| `frontend/package-lock.json` | +573/−573 | **Only `resolved` URLs.** Every one pointed at a leaked Nexus path `registry.npmjs.org/repository/npm/…` that 404s, so `npm ci` failed for everyone. Pure prefix substitution: 0 version changes, 0 integrity-hash changes, `lockfileVersion` unchanged; byte delta exactly −8,595 = 573 × the 15 chars removed. `git diff` shows `573 + resolved / 573 − resolved` and nothing else. |
| `frontend/wailsjs/go/main/App.{js,d.ts}` + `models.ts` | +7 / 21 new | Generated. Adds the `Capabilities` binding. **Regenerated with the real `wails generate module`** — `App.js` and `App.d.ts` came back byte-identical to what I hand-wrote; `models.ts` differed only in trailing whitespace. See §7. |
| `pages/Settings.tsx` | +2/−6 | Deleted an empty `Props` interface. |
| `App.tsx` | +2/−0 | Mounts `ClipboardLinkWatcher`. |
| Card rewrites | ~700 deleted lines | Duplicated flash/patch/row markup replaced by shared controls. Behaviour-preserving except where noted in §3. |

---

## 7. Look hardest at these

### Behaviour changes that affect EXISTING users

**1. `settings_version` migration — `config/setting.go:323-345`.** The one place
existing installs change. Making `scheduled_start_enabled` a real gate would
otherwise have silently stopped scheduled downloads for everyone: the old
`Save()` wrote the whole struct, so that key is **never absent** in a real
upgrader's file — it is present and `false`, while the controller started due
jobs regardless. A file at version 0 therefore adopts the behaviour it actually
had (`ScheduledStartEnabled = true`) and is stamped; version ≥ 1 is respected as
the user's choice. Covered by `upgrade_fixture_test.go` against a byte-accurate
old-build fixture, not an assumption about which keys are present.

**2. The default-true backfill — `config/setting.go:734`.** Upgraders whose file
predates `verify_integrity` / `auto_resume_*` / `keep_partial_on_failure` will
now find those **on** (they were silently off while the UI drew them on). Anyone
who explicitly turned one off keeps it off. This is a real change in what the
engine does for those users, and it is the intended fix.

**3. Verified-route preference in the installers.** When the checksum index is
unreachable the installer now downloads Go from a different host than before. Same
version, same result, different route — but worth knowing if you are diffing
install logs.

**4. `debug.log` is no longer written by default.** Previously created for
everyone; now only at `log_level=debug`. Existing files are left alone.

### Generated files I touched

`frontend/wailsjs/go/main/App.js`, `App.d.ts`, and the new `models.ts`. I
hand-wrote them first, then ran the real `wails generate module -tags webkit2_41`
and diffed: `App.js` and `App.d.ts` **byte-identical**, `models.ts` differing only
in trailing whitespace (generator output kept). `wails_bindings_test.go` now
reflects over `*App` and fails if they drift again — proven non-vacuous by
injecting a fake export and watching it fail.

### Still unverified

1. **Six manual UI checks** — folder-picker dialog, clipboard watcher, open-folder-
   after-finish, post-download system action, confirm-on-exit, launch-on-startup.
   Everything around each is tested; the last hop needs a human.
   Checklist in `SETTINGS-AUDIT.md` §5.
2. **No DOM test environment** — vitest runs `environment: "node"` and
   `@testing-library/react` is not installed, so no React component renders in any
   test. Adding one means touching `package.json` + the lockfile.
3. **Windows and macOS installer paths** were never executed — no host. Both
   `.ps1` files parse clean; the PATH-ordering fix in particular is reasoned, not
   run.
4. **`https://go.devneeds.ir/`** returned HTTP 429 for everything from both host
   and container, so whether that mirror serves toolchains is **unknown**. Not a
   toolchain-specific gap — plain modules 429 too.
5. **`wails build` needs `GOTOOLCHAIN` pinned** on Go ≥ 1.26 (Wails v2.12's
   `x/tools` cannot read newer export data). Fixed in the installers; if you build
   by hand use
   `go build -tags "desktop,production,webkit2_41"`. Omitting `desktop,production`
   produces a binary that links but refuses to start.

---

## 8. Gate status

All green in one run, with both agents' changes present:

| Gate | |
|---|---|
| backend `go build` / `vet` / `test` | PASS |
| backend `go test -race` (download, config, handlers) | PASS |
| root module, `-tags desktop,production,webkit2_41`, build/vet/test | PASS |
| frontend `tsc --noEmit` / `npm run build` / `npm test` (77) | PASS |
| `bash -n` on every installer/build script | PASS |
| `shellcheck -S error` on the five patched shell scripts | PASS |
| both `.ps1` parse clean (local PowerShell image) | PASS |
| `npm ci` from a clean cache + no `node_modules` | PASS |

Companion documents: `SETTINGS-AUDIT.md` (per-setting table, manual checklist,
migration detail, self-review), `INSTALLER-HARDENING.md` (installer agent's, with
my entries 29–31 on the toolchain work).
