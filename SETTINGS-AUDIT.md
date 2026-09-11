# Settings audit — end-to-end fix

Untracked working note. Written 2026-09-07. Read this cold; it assumes nothing.

**What this was.** Reported bug: the download-path "choose folder" button did nothing.
Scope grew to auditing every setting in the app for the whole chain — control renders
the stored value → editing updates state → saved through the API → persisted to disk →
read back on reload → **actually consumed by the code it claims to control**.

Nothing here is committed. All changes are in the working tree.

---

## 1. How to build and run

```bash
cd /home/tehranspeaker/Videos/Rum
go build -tags "desktop,production,webkit2_41" -o ./Rum .
./Rum
```

**The tag set matters.** `desktop,production` are required — Wails' `app_default_unix.go`
is selected without them and `CreateApp` returns
`"Wails applications will not build without the correct build tags"`, so the binary
compiles and links but the app refuses to start. `webkit2_41` is needed because this
machine has webkit2gtk **4.1**, not 4.0.

`wails build` fails on this machine with the system Go (1.27), and that is **not** caused
by anything in this audit — it fails identically on a pristine checkout of HEAD:

```
internal error: package "context" without types was imported from "github.com/tiredbooy/Rum"
```

Wails v2.12.0 pins `golang.org/x/tools v0.30.0`, which predates Go 1.27's export-data
format. **Pinning the toolchain to the version in `go.mod` fixes it** — this produces the
full packaged build:

```bash
GOTOOLCHAIN=go1.25.7 wails build -clean -tags webkit2_41   # -> build/bin/Rum, 25 MiB, links libwebkit2gtk-4.1
```

The plain `go build` line above is the equivalent for a quick unpackaged binary. See §12
for why this matters to the installers.

Quit any running Rum first — Wails' single-instance lock refocuses the existing window
instead of starting the new binary:

```bash
pkill -f '.local/bin/Rum'
```

Useful paths:

| | |
|---|---|
| Settings | `~/.config/rum/settings.json` |
| App log | `~/.config/rum/logs/rum.log` |
| Verbose trace (only when Logging = Debug) | `~/.config/rum/logs/debug.log` |
| Autostart entry (Linux) | `~/.config/autostart/rum.desktop` |

---

## 2. The original bug

The button **was** wired, and a picked path **would** have reached the mutation. The
defect was that every failure mode was invisible and indistinguishable from a cancel.

`frontend/src/_lib/wails.ts` caught **every** rejection from the Wails binding and
returned `null` — the same value a user cancelling the dialog produces. Callers did
`if (dir) commit(dir)`, so a picker that could not open, a binding that rejected, and a
deliberate cancel all produced identical silence. The Go side logged nothing either.
Separately, `CategoryManager` froze picker availability in `useRef(hasChooseDir())` at
first render, so the Browse button could be permanently absent for a session; and the
frontend never imported the generated bindings at all — it hand-reimplemented them
against `window.go`.

Fixed: `chooseDir()` returns a discriminated `picked | cancelled | unavailable | failed`
result; `useFolderPicker` toasts the failure and keeps the text field as an always-usable
fallback; `App.ChooseDir` logs and returns a readable error; the frontend now calls the
generated `wailsjs/go/main/App` module.

---

## 3. Per-setting table

Beyond the picker, **eleven settings were cosmetic** — they saved, flashed "Saved",
reloaded correctly, and were then ignored by the engine until restart, or forever.

| Setting | Was it broken | What was fixed | How verified |
|---|---|---|---|
| **Download folder** (`out_dir`) | Yes — persisted, but `m.opt.Out` was never updated, so new downloads kept using the old folder until restart | `JobManager.ApplySettings` pushes it live | `TestApplySettingsPushesEveryLiveOption`; live API — changed folder, downloaded, file landed in the new one |
| **Choose-folder button** | Yes — any failure was silently identical to a cancel | Discriminated result + toast + Go-side logging + use the generated bindings | 14 vitest cases in `wails.test.ts`; `TestChooseDirBeforeStartupReturnsAnError`. **Dialog itself needs a human — §5** |
| **Auto-organize** (`enable_categories`) | Yes — **completely dead**. `opt.Categorize` was only ever set for an explicit per-job category, so rules never moved a file | Threaded at startup, in `ApplySettings`, and pushed from `PUT /settings/categories` | Live API — enabled + default rules, downloaded `.mp4` landed in `dl/Videos/` instead of `dl/videos/` |
| **Proxy** | Yes — read once when the `Downloader` was built at boot; changes ignored until restart | `ApplySettings` rebuilds the transport when the proxy changes (and only then) | `TestApplySettingsRebuildsTransportOnlyOnProxyChange`; live — setting a proxy made the very next download route through it and fail |
| **Auto-resume on reconnect** | Yes — **read by zero lines of code**. A switch that changed nothing | New `ResumeController` (`backend/internal/pkg/download/reconnect.go`): re-queues network-failed jobs once their host answers; capped at 10 attempts; permanent failures (404, checksum) excluded | 6 tests in `reconnect_test.go` |
| **Scheduled start** (`scheduled_start_enabled`) | Yes — persisted and round-tripped; `startDueJobs` ran unconditionally | Now gates the controller; **plus a migration** so existing users keep today's behaviour — see §6 | `TestScheduledStartEnabledStartsDueJobs`, `…DisabledLeavesJobPending`, `…BandwidthWindowAppliesWithScheduledStartOff`, `upgrade_fixture_test.go` |
| **Watch clipboard for links** | Yes — Go emitted `clipboard:url`; **nothing in the frontend ever subscribed** | New `ClipboardLinkWatcher` → toast with an "Add" action that prefills the dialog | `onWailsEvent` subscribe/unsubscribe unit-tested. **Needs a human — §5** |
| **Launch on startup** | Yes — saved, but `SetAutostart` only ran at boot, so no effect until restart | New `watchAutostart` / `syncAutostart` applies within ~2s | `TestSyncAutostartAppliesAChangedPreference` actually creates/removes the `.desktop` entry in a temp XDG dir |
| **Parallel downloads** | Yes — `sched.SetMax` existed but was never called from the settings path | `ApplySettings` calls it | `TestApplySettingsPushesEveryLiveOption` asserts `sched.Max()` |
| **Retry attempts / Silent** | Yes — persisted, never pushed to the live manager | `ApplySettings` | Same test |
| **Speed limit** | Partly — applied, but only on the next 30 s controller tick | Applied immediately, honouring an active bandwidth window | `TestApplySettingsHonoursActiveBandwidthWindow`; live — saving a window dropped a download from 4 s to 23 ms |
| **Block private hosts** (SSRF guard) | Yes — backend field with **no UI at all** | Added to the Integrity card; transport rebuilds on change | `TestApplySettingsRebuildsTransportOnSSRFGuardChange`; curl round-trip |
| **Verify integrity / auto-resume on launch / keep partial on failure** | Yes — on any settings file written before those fields existed, a missing key read as `false`, so upgraders silently ran with them **off** while the UI drew them **on** | `presentKeys()` distinguishes "absent" from "explicitly false" | `TestMissingReliabilityKeysDefaultToOn`, `TestExplicitFalseReliabilityKeysArePreserved`; live API with a legacy JSON file |
| **All settings — corrupt file** | Yes — a corrupt `settings.json` self-healed but `GET /settings` still answered **500**, so the page said "Could not load preferences" for settings it was holding | `config.ErrRecovered` + a shared `loadSettings` helper across all six endpoints | 3 recovery tests; live — all four endpoints now 200 |
| **Logging** (`log_level`) | Yes — validated and persisted, but the slog logger it fed is called by **zero** lines (everything uses stdlib `log`, which has no levels), while `debug.log` was opened **unconditionally** | Now gates the verbose download trace; live via `ApplySettings`, no restart. Added a UI control (Normal / Debug) and two trace lines on the main download path — see §7 | `debug_test.go` (5 tests); live — Normal writes no file, Debug traces the download, back to Normal freezes it |
| **Retry attempts (duplicate)** | Cosmetic — two inputs for `max_retries` on one page | Single home in Integrity, next to backoff | tsc + build; automated check that every `NumField` is rendered by exactly one card |
| **Invalid values** | Yes — bad enums / proxy / colour were accepted with 200 then silently rewritten by `Validate()`, which looked like a control refusing to change | Per-field 400s with user-ready messages, rendered inline via `ApiError.fields` | 12 sub-tests + `api.test.ts`; curl sweep of 8 bad values |
| **Bad folder paths** | Yes — no validation; a bad path failed silently at download time | `validateWritableDir` (absolute, exists-or-creatable, writable) | 4 handler tests; live — a file-not-a-folder path returned an inline error and the input reverted |
| **Failed saves** | Yes — the local draft kept the value the server had rejected | `useSettingsPatch` reverts the draft and shows the field error inline | Observed live; `ApiError` plumbing unit-tested |
| **Form labels** | Yes — `SettingInput`/`SettingSelect` rendered `<Label>` with no `htmlFor`, unbound for screen readers | All controls bind label ↔ input with `aria-invalid` / `aria-describedby` | tsc + an accessibility snapshot |
| **Minimize / Close to tray** | Partly — saved and did nothing off-Windows; explained only in a long help sentence | New `App.Capabilities()` binding; the switches render **disabled** where there is no tray | `TestCapabilitiesReportsTrayAvailability` + `getCapabilities` vitest |
| **Post-download system action** | Partly — silently reset to "Nothing" on every restart, unexplained | Hint added: "Resets to Nothing when Rum restarts." | Live restart: `sleep` → `none`, as designed |
| **Accent colour** | Partly — committed only on blur, so leaving the page after using the OS colour picker lost it | 500 ms debounce + blur commit, with a duplicate-commit guard | tsc/build; presets verified live |
| Confirm-on-exit, theme, density, reduced motion, temp dir, keep-partial, file-conflict, auto-open-folder, completion sound, schedule rules, category rules | Already worked | — | curl round-trip of **all 24 PATCH fields** + restart re-read; conflict policy exercised end to end (rename → `conflict (1).bin`, skip → job refused) |

---

## 4. Files changed

Backend: `config/setting.go`, `api/handlers/{settingHandler,schedule,categories}.go`,
`download/{manager,schedule,debug}.go`, `download/reconnect.go` (new), `cmd/server/main.go`.

Root/desktop: `app.go`, `desktop.go`.

Frontend: `_lib/wails.ts`, `_lib/services/api/api.ts`,
`_lib/services/queries/settings.queries.ts`, `_lib/types/setting-types.ts`, `App.tsx`,
`pages/Settings.tsx`, all nine cards under `features/settings/`, plus new
`AccentPicker.tsx`, `CategoryRuleRow.tsx`, `SpeedWindowRow.tsx`, `SettingPathInput.tsx`,
`useFolderPicker.ts`, `useDesktopCapabilities.ts`, `numeric-bounds.ts`, `proxy.ts`,
`category-defaults.ts`, and `features/clipboard/ClipboardLinkWatcher.tsx`.

Generated bindings: `frontend/wailsjs/go/main/App.{js,d.ts}`, `frontend/wailsjs/go/models.ts`.

Tests: 57 Go test functions across 10 new files; 28 new frontend tests (77 total).

---

## 5. Manual checklist — six things automation cannot confirm

Build and launch per §1, and keep `tail -f ~/.config/rum/logs/rum.log` open.

### 1. Folder picker — the original bug
1. Settings → **General** → **Save location** → click **Browse**.
2. **Works:** native folder chooser opens. Pick a folder → the field fills, a green
   **Saved** pill shows for 2 s.
3. Confirm: `python3 -c "import json;print(json.load(open('$HOME/.config/rum/settings.json'))['out_dir'])"`
4. Click **Browse** again, press **Cancel** → nothing happens, no error. Correct.
5. **Broken:** a red toast appears. Then
   `grep 'folder picker' ~/.config/rum/logs/rum.log` → `desktop: folder picker failed: <reason>`.
   If instead you see `desktop: ChooseDir called before startup`, the window was not
   ready — reopen Settings and retry.
6. **Broken differently:** no **Browse** button at all → the Wails bindings did not load;
   the text field still works.
7. Repeat for **Storage → Temp directory → Browse**, and the folder icon on any
   **Auto-organize** rule (click **Add rule** first if the list is empty).

### 2. Clipboard watcher
1. Settings → **Desktop** → **Watch clipboard for links** ON.
2. Copy any URL, e.g. `https://example.com/file.zip`.
3. **Works:** within ~1.5 s a toast **"Link copied"** with an **Add** button; clicking it
   opens the download dialog prefilled.
4. Copy the same URL again → no second toast (de-duped). Correct.
5. Switch off, copy another URL → no toast.
6. **Broken:** confirm the setting saved —
   `python3 -c "import json;print(json.load(open('$HOME/.config/rum/settings.json'))['enable_clipboard_watch'])"`
   must be `True`. If it is and there is still no toast, the Go watcher is emitting but
   the UI is not receiving.

### 3. Open folder after finish
1. Settings → **Post-download** → **Open folder after finish** ON.
2. Complete any small download.
3. **Works:** the file manager opens on the download folder.
4. **Broken:** nothing opens, and nothing is logged — `filesystem.OpenFolder` has no
   logging. Add some if this fails.
5. Switch it back off.

### 4. System action
Only test **Close App** — the other two power off or suspend the machine.
1. Settings → **Post-download** → **System action** → **Close App**.
2. Complete a small download. **Works:** Rum exits by itself.
3. Relaunch → **System action** must read **Nothing**. That reset is deliberate (a
   persisted shutdown must never fire on the next launch) and the hint says so.
4. **Broken:** it still reads **Close App** after a restart.

### 5. Confirm on exit
1. Settings → **General** → **Confirm on exit** ON. Close the window.
   **Works:** "Are you sure you want to quit?" — No keeps it open, Yes quits.
2. Relaunch, turn it **OFF**, close the window **without restarting**.
   **Works:** quits immediately, no dialog. Step 2 is the real proof (live re-read).

### 6. Launch on startup
1. Settings → **Desktop** → **Launch on startup** ON.
2. Within ~2 s, without restarting: `ls ~/.config/autostart/rum.desktop` exists.
3. Switch OFF → within ~2 s it is gone.
4. **Broken:** `grep autostart ~/.config/rum/logs/rum.log` →
   `desktop: apply autostart (enable=true): <reason>`.

**Not testable here:** *Minimize to tray* / *Close to tray* are Windows-only in this
build; both render **disabled** with "Not available on this platform." That is correct
on Linux/macOS.

**Bonus (10 s):** Settings → **General** → **Logging** → **Debug**, run one download,
then `cat ~/.config/rum/logs/debug.log` — expect a `start job=… connections=… limit=…` /
`end job=… status=completed` pair. Set back to **Normal**; the file stops growing.

---

## 6. `settings_version` migration — who it affects

**Why it exists.** Making `scheduled_start_enabled` a real gate would otherwise have
silently changed behaviour for every existing user. The old `Save()` always wrote the
whole struct, so the key is **never absent** in a real upgrader's file — it is present
and set to `false` (the old default), while the controller started due jobs regardless.
Honouring it as-is would have quietly stopped scheduled downloads from starting.

**What it does.** New `settings_version` field on `config.Setting`.

- **Absent or `0`** (any file written by a build before this work): `migrate()` sets
  `ScheduledStartEnabled = true`, adopting the behaviour that build actually had, then
  stamps version `1`.
- **`>= 1`**: no migration; the stored value is the user's choice and is respected.
- Fresh installs are written at version `1` by `setDefaults()`, so the migration never
  runs on them.
- `Validate()` stamps the current version, so anything this build writes is current.
  `migrate()` runs **before** `Validate()` on the load path, so the stamp never masks a
  pending migration.

**Net effect:** every existing user keeps exactly the behaviour they had. From here on
the switch means what its label says. Covered by `upgrade_fixture_test.go`, which uses a
byte-accurate reproduction of an old build's `settings.json` rather than an assumption
about which keys are present.

Adding a future behaviour change to a persisted setting? Bump `currentSettingsVersion`
and add a branch to `migrate()`.

---

## 7. The two legacy fields — final status

### `start_on_launch` → deprecated read-back alias
It duplicated `auto_resume_on_launch` and **no code ever read it**.
`auto_resume_on_launch` is the keeper — it is what `server.Listen()` acts on and what the
UI shows.

- The key is **never dropped** from `settings.json`.
- `Validate()` mirrors `AutoResumeOnLaunch` into it on every load and save, so the two can
  never disagree on disk. The canonical field always wins: nothing ever acted on
  `StartOnLaunch`, so its stored value only ever reflected an old default.
- `Update()` redirects a PATCH carrying `start_on_launch` onto `AutoResumeOnLaunch`,
  applied *first* so an explicit `auto_resume_on_launch` in the same body wins.
- `TestNothingOutsideConfigReadsTheDeprecatedField` walks all non-test `.go` files and
  fails if `StartOnLaunch` / `start_on_launch` appears outside `config/setting.go`.

### `log_level` → a control that does something
It was worse than "no UI". The slog logger it fed is called by **zero** lines in the
codebase (everything logs through stdlib `log`, which has no levels), while
`download.DebugLog` — a real verbose trace — was opened **unconditionally**, so every user
accumulated a `debug.log` forever and the level controlled nothing.

- `log_level` now gates that trace: `debug` opens it, anything else keeps it closed.
- `SetDebugLogging` is mutex-guarded (the settings API opens/closes it while download
  goroutines write) and is called from `ApplySettings`, so **it applies live — no restart,
  and therefore no restart caveat in the UI**.
- Two trace lines were added on the *main* download path. Without them, turning Debug on
  and finishing a normal download produced an **empty** log — every pre-existing
  `DebugLog` call sits on an unusual branch (resume, categorize failure, changed remote).
  The trace now answers the first question a maintainer has:
  ```
  13:05:13: start job=b78ae197… url=… out=… connections=8 limit=0kB/s retries=3 verify=true tempdir="" categorize=false
  13:05:13: end   job=b78ae197… status=completed 100000/100000 bytes
  ```
- UI: a **Logging** select in the General card — Normal / Debug. Only `debug` changes
  behaviour, so a legacy `warn`/`error` config displays as Normal (accurate — it logs
  identically) and is not rewritten until the user picks something. The endpoint accepts
  and round-trips all four canonical levels; an unknown level returns a `log_level` field
  error.

---

## 8. `frontend/package-lock.json`

**The finding.** All 573 `resolved` URLs pointed at
`https://registry.npmjs.org/repository/npm/...` — a leaked Nexus-proxy path that 404s. Both
`npm ci` and `npm install` failed against the committed file. (The lockfile was a mix: 573
broken, 79 already correct.)

Nothing in the environment reintroduces it — the effective registry is
`https://registry.npmjs.org/`; there is no `frontend/.npmrc` or repo-root `.npmrc`;
`~/.npmrc` holds only `prefix=`; no `/etc/npmrc`, no `npm_config_*` env vars, no scoped
overrides. `frontend/.env` holds only `VITE_API_URL` and npm never reads it. The bad path
was baked into the committed lockfile alone.

**What changed.** A pure textual replacement of the prefix
`https://registry.npmjs.org/repository/npm/` → `https://registry.npmjs.org/`, rather than
regenerating, so nothing else could drift. Verified before writing: parsed both versions
and compared everything except `resolved` — identical; **0 version changes, 0 integrity-hash
changes**, same package set, `lockfileVersion: 3` unchanged. Byte delta is exactly
−8595 = 573 × the 15 characters of `repository/npm/`. `git diff` shows
`573 + resolved / 573 - resolved` and nothing else.

**Proof from clean:**
```
rm -rf frontend/node_modules ~/.npm/_cacache
cd frontend && npm ci      # added 540 packages in 56s, exit 0
```
No registry flag, no `--force`. `npm ci` did not rewrite the lockfile. Against that fresh
install: `tsc --noEmit`, `npm run build`, `npm test` (77 tests) all pass.

Unrelated environment note: npm here blocks install scripts by policy (`esbuild`, `msw`
postinstall). esbuild still works — its binary ships as the `@esbuild/linux-x64` optional
dependency.

---

## 9. Self-review findings

Re-read of the whole diff hunting for regressions *introduced*, not bugs fixed.

| Location | What was wrong | Fixed |
|---|---|---|
| `download/schedule.go:164` + `config/setting.go:659` | Gating `startDueJobs` silently changed behaviour for existing users; the "default true when absent" mitigation protected nobody because the key is never absent | Yes — versioned migration, §6 |
| `features/settings/AccentPicker.tsx:56` | Debounce and `onBlur` could both commit the same colour before the save landed → identical PATCH twice | Yes — `committed` ref (`:44`) |
| `_lib/services/queries/settings.queries.ts:106` | `patch` depended on the whole `mutation` object, which TanStack v5 recreates each render → new identity every render; harmless today, a render loop for whoever puts it in a dep array | Yes — depend on the stable `mutate` (`:85`) |

The migration **caught itself**: adding it failed `TestScheduledStartDisabledLeavesJobPending`,
because that fixture saved a `Setting` that had never been through `setDefaults`, writing
`settings_version: 0`. The migration was right; the root cause was fixed (`Validate()` now
stamps the version) rather than the test.

Checked, no regression found:

- **Written-but-unread / read-but-unwritten.** Reflected over all 32 `Setting` fields —
  every one has a consumer except the deprecated alias (guarded by a test) and
  `reduced_motion`, consumed in TS by `ThemeProvider`. Reverse direction: all **30 fields
  the UI PATCHes** map to a real `SettingReq` json tag, so none can flash "Saved" and
  persist nothing. Every `NumField` is rendered by exactly one card.
- **Error envelope.** `ApiError extends Error`, so all 16 `err instanceof Error ? err.message`
  call sites are unaffected; nothing reads a flattened shape. Verified nothing still calls
  `useUpdateSettings` directly, which would now fail silently. Only delta: a non-JSON error
  body yields `"HTTP 500"` instead of `"Request failed"`.
- **New mutex in `debug.go`.** `debugMu` is never taken while holding `m.mu` — both
  `DebugLog` calls in `runDownload` sit outside the lock, and `SetDebugLogging` runs after
  `m.mu.Unlock()` in `ApplySettings`. `go test -race` on download/config/handlers is clean.
- **Hooks.** `useFolderPicker` cannot loop (same-value `setState` bails out);
  `ClipboardLinkWatcher` subscribes once and cleans up; `useDesktopCapabilities` is
  `[]`-guarded with an `alive` flag; draft-revert closures capture the last-known-server
  value, which is what they should revert to.

### Generated Wails bindings — verified by regeneration

`frontend/wailsjs/go/main/App.{js,d.ts}` and `models.ts` were hand-edited to add
`Capabilities`. The wails CLI **is** available (`~/go/bin/wails`, just not on `PATH`), so
verification was done the strong way:

```bash
PATH="$HOME/go/bin:$PATH" wails generate module -tags webkit2_41
```

Result: `App.js` and `App.d.ts` came back **byte-identical** to the hand-written versions;
`models.ts` differed only in trailing whitespace (the generator's output is now in place).
The generator also flipped `frontend/wailsjs/runtime/*` from mode 644 → 755 with zero content
change; restored to 644 to keep the diff clean.

`wails_bindings_test.go` reflects over `*App` and asserts every exported method has a
correctly-dispatching binding, that no declaration names a method that no longer exists,
and that `models.ts` matches `DesktopCapabilities`'s JSON tags in both directions. Proved
non-vacuous by injecting a fake export — it failed with the right message — then restoring.

---

## 10. Verification gates

All green, in one run, under the real build tags — final pass re-run after the installer
agent's GOTOOLCHAIN edits landed, so this reflects **both agents' changes present**:

| Gate | |
|---|---|
| `cd backend && go build ./...` | PASS |
| `cd backend && go vet ./...` | PASS |
| `cd backend && go test ./... -count=1` | PASS |
| `go build -tags desktop,production,webkit2_41 ./...` | PASS |
| `go vet -tags desktop,production,webkit2_41 ./...` | PASS |
| `go test -tags desktop,production,webkit2_41 -count=1 ./...` | PASS |
| `cd frontend && tsc --noEmit` | PASS |
| `cd frontend && npm run build` | PASS |
| `cd frontend && npm test` | PASS (77) |
| `go test -race` (download, config, handlers) | PASS |
| `bash -n` on all 9 installer / build shell scripts (their files) | PASS |

Real binary: `/home/tehranspeaker/Videos/Rum/Rum`, 37,152,952 bytes (36 MiB), ELF 64-bit
dynamically linked, 144 shared-object deps including `libwebkit2gtk-4.1`, `libgtk-3`,
`libsoup-3.0`, `libjavascriptcoregtk-4.1`; 415 symbols from the Wails linux desktop
frontend. Compiled and linked only — never launched.

---

## 11. Known gaps / not verified

1. **The six items in §5 were never executed.** A no-browser constraint applied for most
   of this work and the desktop app was never launched. Everything *around* each one is
   tested; the last hop is not.
2. **`wails build` does not work here** (§1). Only the `go build` path is proven. Packaging
   (icons, `.desktop` file, nfpm/deb) is therefore unverified.
3. **The frontend has no DOM test environment** — vitest runs with `environment: "node"`,
   and `@testing-library/react` / jsdom are not installed. No React component renders in
   any test. Component behaviour was verified by reading the code and, before the
   no-browser rule, by driving the real UI once. Adding a DOM environment would require
   touching `package.json` + the lockfile.
4. **`ClipboardLinkWatcher` has no test.** The event plumbing (`onWailsEvent`) is unit
   tested; the component is not (see 3).
5. **`filesystem.OpenFolder` has no logging**, so "Open folder after finish" failing is
   silent. Worth adding.
6. **Live verification used an out-of-tree API harness**, not the desktop shell — a small
   `main` calling `server.Start()` with an isolated `XDG_CONFIG_HOME`. That exercises the
   full HTTP + engine + disk path but not the Wails layer.
7. **The `debug.log` trace is sparse.** Two lines per download were added on the main path;
   the pre-existing `DebugLog` calls only fire on resume / categorize-failure / changed-remote
   branches. More trace points would make bug reports more useful.
8. **`ScheduleEditor` still uses `NativeSelect`** (a styled raw `<select>`) for the hour
   pickers, against the repo's design-system-first rule. It works and is keyboard
   accessible; replacing it with the Radix `Select` was judged out of scope.
9. **Post-download `shutdown` / `sleep` were never exercised** — they would power off or
   suspend the machine. Only the disarm-on-restart behaviour is unit tested.
10. **Two files sit in the 150–250 line "watch" band** (`CategoryRuleRow.tsx` 165,
    `SpeedWindowRow.tsx` 170). Under the hard limit, worth an eye if they grow.

---

## 12. Read-only review of the install / build scripts

Reviewed at the orchestrator's request because these scripts build and ship the app.
**Nothing here was edited** — they belong to another agent.

### The good news: build tags are correct everywhere

Every GUI path shells out to `wails build`, and the Wails CLI supplies `desktop,production`
itself. So none of these scripts can produce the "compiles but refuses to start" binary
that §1 warns about — that failure only bites someone hand-rolling `go build` with a
partial tag set.

| File:line | Verdict |
|---|---|
| `installers/gui/install-linux.sh:1023` `wails build -clean -tags "$WAILS_TAGS"` | Correct |
| `installers/gui/install-macos.sh:619` `wails build -clean -platform darwin/universal` | Correct (webkit tag is Linux-only) |
| `installers/gui/install-windows.ps1:568` `wails build -platform windows/amd64 -clean` | Correct (WebView2, no webkit tag) |
| `build-windows.sh:438` `wails build -platform windows/amd64 -clean` | Correct |
| `build-linux.sh:19` | Thin delegator to the Linux GUI installer — nothing to get wrong |
| `backend/cmd/rum/build.sh:175` `go build ... ./cmd/rum` | Correct — this is the **CLI/TUI**, not the Wails GUI; it needs no tags, no frontend and no webkit |

### WebKit 4.0 vs 4.1 — handled correctly

`installers/gui/install-linux.sh:808-813`:

```bash
wails_tags() {
  if command -v pkg-config && pkg-config --exists webkit2gtk-4.1 \
     && ! pkg-config --exists webkit2gtk-4.0; then printf '%s' "webkit2_41"; fi
}
```

All four cases are right: 4.1-only → tag; 4.0-only → no tag; both → no tag (builds against
4.0, which exists); neither → no tag, and the preflight at `:750` already dies with a
friendly message. Ordering is also correct — `ensure_build_deps` (`:964`) installs the
webkit dev package *before* `WAILS_TAGS` is evaluated (`:967`), so detection never runs
against a not-yet-installed library.

Two things I suspected and cleared:
- `:747-749` `pkg-config --exists X && have_wk=1` under `set -Eeuo pipefail` — **safe**;
  a command before `&&` is exempt from `set -e`. Verified empirically.
- Tag-detection ordering — **safe**, see above.

### Frontend build and the lockfile

All three GUI installers plus `build-windows.sh` run `npm ci` first with an `npm install`
fallback (`install-linux.sh:979`, `install-macos.sh:599`, `install-windows.ps1:546`,
`build-windows.sh:418`). Before §8 they were silently taking the `npm install` fallback on
every run, because `npm ci` could not succeed against the broken lockfile. They now get the
fast, reproducible path. They do not run `npm run build` themselves — correct, `wails build`
compiles the frontend (its "Compiling frontend" step).

### The real problem: `wails build` is broken on modern Go

**This is the one worth relaying.**

`installers/gui/install-linux.sh:411` / `install-macos.sh:326` / `build-windows.sh:234`
all gate on `version_ge "$cur" "$GO_MIN"` where `GO_MIN` comes from `go.mod` (`1.25.7`) —
so **any** newer system Go is accepted, including 1.26/1.27. But Wails v2.12.0 pins
`golang.org/x/tools v0.30.0`, which cannot read Go 1.27's export data, and `wails build`
dies before compiling:

```
internal error: package "context" without types was imported from "github.com/tiredbooy/Rum"
```

Reproduced on a pristine `git archive HEAD` checkout, so it is not caused by this audit.
Effect: a user who already has a current Go gets a failed install; a user with no Go gets
the pinned 1.25.7 tarball and is fine. The retry-with-mirror fallback does not help — this
is not a network error.

**Exact one-line change** (each script, immediately before its `wails build`) — **this was
relayed to the installer agent and has since been applied; see the verification below**:

```bash
export GOTOOLCHAIN="go${GO_MIN}"          # installers/gui/install-linux.sh, install-macos.sh, build-windows.sh
```
```powershell
$env:GOTOOLCHAIN = "go$GoMin"             # installers/gui/install-windows.ps1
```

Alternative if pinning is unwanted: bump `github.com/wailsapp/wails/v2` to a release whose
`x/tools` understands Go 1.27.

### Verification of the applied fix (2026-09-07 14:05, independent, read-only)

The installer agent applied all four. Verified without editing their files.

| File:line | Landed | Correct? |
|---|---|---|
| `installers/gui/install-linux.sh:1019` | `export GOTOOLCHAIN="go${GO_MIN}"` | Yes — line 1019, before the build subshell at 1020-1041 |
| `installers/gui/install-macos.sh:616` | `export GOTOOLCHAIN="go${GO_MIN}"` | Yes — before the subshell at 617-634 |
| `build-windows.sh:435` | `export GOTOOLCHAIN="go${GO_MIN}"` | Yes — before the subshell at 436-449 |
| `installers/gui/install-windows.ps1:565` | `$env:GOTOOLCHAIN = "go$GoMin"` | Yes — before `wails build` at 569 |

Checks performed:

- **Placement** — in all four the assignment precedes the build. The three shell scripts put
  it *outside* the `( … )` subshell, so it is inherited by the build and by the
  retry-with-mirror path inside the same subshell. Correct.
- **No hardcoded literal** — all four interpolate the value the script already derives from
  `go.mod` (`GO_MIN` via `read_go_min`, `$GoMin` via `Get-GoMin`), so it follows `go.mod`
  automatically. Resolves to `go1.25.7` today; that toolchain exists upstream (HTTP 302 on
  `go.dev/dl/go1.25.7.linux-amd64.tar.gz`).
- **Scope** — `$GoMin` is assigned at `install-windows.ps1:494` inside the same top-level
  `try` block (opens at 481) as line 565, so it is in scope. Not, as a shallow read
  suggests, inside `Add-AppShortcut` (that function ends at 474).
- **No harmful leak** — the export is top-level and persists for the remainder of each
  script, but nothing after the build runs another Go toolchain: `install-linux.sh` has no
  further `go build`/`go install` after line 1041 (only file installation), and
  `build-windows.sh` only runs Docker/Inno Setup afterwards. The CLI build
  (`backend/cmd/rum/build.sh`) is a separate script and is untouched.
- **Syntax** — `bash -n` clean on `install-linux.sh`, `install-macos.sh`, `build-windows.sh`
  and `build-linux.sh`; `shellcheck` reports nothing on any of the three changed lines.
  **The `.ps1` could not be syntax-checked — neither `pwsh` nor `powershell` is installed
  on this machine.** It was verified by reading only.

**In-situ proof.** Reproducing `install-linux.sh:1019-1024` exactly — deriving `GO_MIN` from
`go.mod`, exporting `GOTOOLCHAIN`, computing `WAILS_TAGS` with the same `wails_tags()` logic,
then running its build line:

```
GOTOOLCHAIN=go1.25.7   WAILS_TAGS=webkit2_41
wails build -clean -tags "webkit2_41"      -> exit 0, build/bin/Rum, 25 MiB,
                                              links libwebkit2gtk-4.1 + libgtk-3
```

A/B against the same command with the fix removed (`GOTOOLCHAIN=local`, i.e. the system Go
1.27 the scripts previously used):

```
-> exit 1
   internal error: package "context" without types was imported from "github.com/tiredbooy/Rum"
```

So the export is causally responsible for the fix. **The fix landed correctly in all four
places and works.**

### Clean-machine toolchain download — VERIFIED, with one blocking caveat

Previously listed as unexercised. Now tested in a throwaway `archlinux:latest` container
(`docker run --rm --memory=2g`, nothing on the host touched), which had **Go 1.27.0** from
the distro and **zero** toolchains in its module cache — i.e. exactly the situation the fix
has to survive.

**Result: the download path WORKS on the default proxy.**

```
GOPROXY=https://proxy.golang.org,direct  GOTOOLCHAIN=go1.25.7  go version
  -> go: downloading go1.25.7 (linux/amd64)
     go version go1.25.7 linux/amd64
  17 seconds, 275 MB cached as golang.org/toolchain@v0.0.1-go1.25.7.linux-amd64
```

Toolchains are ordinary modules (`proxy.golang.org` serves
`golang.org/toolchain/@v/v0.0.1-go1.25.7.linux-amd64.info` with HTTP 200), so `GOPROXY`
does govern them. `GOPROXY=off` fails cleanly with `toolchain not available`.

### RESOLVED: `GOSUMDB=off` blocker — fixed and verified 2026-09-07 14:34

**The blocker (now historical).** Setting `GOTOOLCHAIN=go${GO_MIN}` failed whenever
`GOSUMDB=off` was in effect — which `configure_goproxy` sets on every mirror path:

```
go: download go1.25.7: golang.org/toolchain@v0.0.1-go1.25.7.linux-amd64:
    verifying module: checksum database disabled by GOSUMDB=off
```

Measured across a matrix (fresh cache each, default proxy, only the sumdb var changing):
sumdb on → OK; `GOSUMDB=off` → FAIL; and `+GOFLAGS=-mod=mod`, `+GOPRIVATE`, `+GONOSUMDB`,
`+GONOSUMCHECK`, `GOPROXY=direct` → all FAIL. It failed even against a cache already
populated by a successful sumdb-on run, so it was not merely a first-download problem.

**The fix that landed.** The installer agent replaced the toolchain switch with an exact
pin: a new `pin_wails_go()` installs `go$GO_MIN` via the existing `install_go_tarball` when
the system Go is *newer* than `GO_MIN`, and the build then runs under `GOTOOLCHAIN=local`.

| File | Pin fn | Called from | `GOTOOLCHAIN=local` | PATH prepend |
|---|---|---|---|---|
| `installers/gui/install-linux.sh` | `:425` | `ensure_go` `:667 :692 :709` | `:1057` | `:609` |
| `installers/gui/install-macos.sh` | `:337` | `:482 :492 :497` | `:642` | `:424` |
| `build-windows.sh` | `:245` | `:351 :359 :364` | `:467` | `:343` |
| `installers/gui/install-windows.ps1` | `Install-RequiredGo:403-408` | `:540` | `:584` | see defect below |

Checked specifically for the easy half-fix (exporting `GOTOOLCHAIN=local` but leaving the
newer-Go path untouched): **not present** — `pin_wails_go` is invoked from *all three*
`ensure_go` exit paths, so a newer system Go really does get the exact tarball. The
`GOTOOLCHAIN=local` export is after the pin, not before. `GOPROXY` chains were also switched
to `mirror|https://proxy.golang.org,direct` — pipe for the flaky mirror, comma for the
well-behaved public proxy, which is the correct use of each separator.

**Proof (throwaway `archlinux:latest` container, `--rm --memory=2g`, system Go 1.27.0, empty
module cache):**

```
1. blocker reproduced   GOSUMDB=off + GOTOOLCHAIN=go1.25.7   -> exit 1, "checksum database disabled"
2. pinned go1.25.7 tree, GOROOT set, PATH prepended
3. GOTOOLCHAIN=local + GOSUMDB=off + mirror-first GOPROXY
     go version   -> go version go1.25.7 linux/amd64      (exit 0)
     go build     -> exit 0, binary runs
     extra toolchain downloads triggered: 0  (the sumdb is never consulted)
```

**Verdict: RESOLVED for `install-linux.sh`, `install-macos.sh` and `build-windows.sh`.**
`bash -n` clean on all four shell scripts; `shellcheck` reports no errors.

### STILL BROKEN on Windows: `Sync-SessionPath` puts the wrong Go first

`installers/gui/install-windows.ps1` — the pin logic is right but the PATH ordering defeats
it. `Install-GoZip` correctly prepends the pinned Go (`$env:Path = "$bin;$env:Path"`) and
puts it at the front of the *User* PATH, then calls:

```powershell
function Sync-SessionPath {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user    = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = @($machine, $user, $env:Path) -join ";"    # <-- Machine wins
}
```

`Install-RequiredGo` calls `Sync-SessionPath` again right after `Install-GoZip` (`:406-407`),
so the machine PATH is re-prepended after the pin. Go installed machine-wide — which is what
`winget install GoLang.Go` and the MSI do, putting `C:\Program Files\Go\bin` on the Machine
PATH — therefore resolves **before** the pinned `%LOCALAPPDATA%\Go\bin`. With
`GOTOOLCHAIN=local` that hands Wails the *newer* Go and the original
`internal error: package "context" without types` returns.

Demonstrated as a negative control in the same container: re-ordering PATH so the system Go
comes first makes `go` report 1.27.0 again while `GOTOOLCHAIN=local` is set.

**file:line and corrected line — for the owning agent, not changed here:**

`installers/gui/install-windows.ps1`, inside `Sync-SessionPath`:

```powershell
# current (machine PATH wins, defeats the pin):
$env:Path = @($machine, $user, $env:Path) -join ";"
# corrected (session-prepended dirs stay first):
$env:Path = @($env:Path, $machine, $user) -join ";"
```

The `.ps1` parses clean (0 syntax errors, checked with
`System.Management.Automation.Language.Parser` in the local `mcr.microsoft.com/powershell`
image) — this is a logic defect, not a syntax one. It could not be tested functionally: no
Windows host available.

### Go toolchain download on a restricted network — RESOLVED

Google's download hosts are blocked here (`go.dev/dl` 404, `dl.google.com/go` 404
for every Go version, `storage.googleapis.com/golang` 403). Two things fix it:

1. **`mirrors.aliyun.com/golang/` works** and serves the byte-correct release —
   its sha256 matches the official index exactly
   (`12e6d6a191091ae27dc31f6efc630e3a3b8ba409baf3573d955b196fdf086005`). This
   corrects an earlier claim in this file that the tarball route "dies here": it
   was measured before that mirror was in the URL list.
2. **A module-proxy fallback was added** to every Go download path
   (`install_go_from_module_proxy` in the five shell installers,
   `Install-GoFromModuleProxy` in the two PowerShell ones). It resolves
   `golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>` through `GOPROXY`, which is
   reachable here (HTTP 206) when the tarball hosts are not.

The fetch is performed **by the `go` command**, not by curl, so it is verified
against the checksum database — a raw download would be unverifiable, because the
sha256 published on go.dev is for the `.tar.gz` and cannot validate the module
zip (`12e6d6a1…` vs `43a6a446…`). `GOSUMDB` is forced on for that call so a mirror
path's `GOSUMDB=off` cannot disable the only check the route has. The module
layout was verified rather than assumed — entries are prefixed
`golang.org/toolchain@v0.0.1-go…/`, not `go/`, and arrive read-only — and is
normalised so the rest of the install flow is identical for both routes.

Verified end-to-end in a container on this network, driving the real
`install_go_tarball`: with tarball hosts reachable it installs from Aliyun; with
all of them forced unreachable it falls through to the module proxy and reports
`Installed go version go1.25.7 linux/amd64`.

**Verified routes are now preferred over unverified ones.** Precedence in every
Go download path, strongest first:

| # | Route | Verified by |
|---|---|---|
| 1 | archive + known sha256 | official checksum from `go.dev/dl/?mode=json` |
| 2 | module proxy | checksum database (`sum.golang.org`) |
| 3 | archive with no sha256 | **nothing** — last resort, loud warning |

When the checksum index is unreachable, the installer now tries the module proxy
*before* installing an unchecked archive — that route is verified or it fails, so
it can never be the silently-worse choice. The one exception is a machine with no
Go at all: the module route needs an existing `go` to drive the download, so
there the archive is the only option; that is an explicit named guard
(`go_can_bootstrap_module_proxy`), and the warning says so. `RUM_REQUIRE_VERIFIED_GO=1`
makes the unverified last resort fatal for anyone who wants that.

All five branches (verified archive; index-down → module proxy; bootstrap with no
Go → unverified archive + warning; the same with `RUM_REQUIRE_VERIFIED_GO=1` →
refuses; all archive hosts down → module proxy) were exercised against the real
script in a container on this network. Details and transcripts in
`INSTALLER-HARDENING.md` entries 29-31.
