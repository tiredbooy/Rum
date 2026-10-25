# Contributing

## Verification gate

Run all of these before sending a change:

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && npx tsc --noEmit && npm run build && npm test
# repo root (Wails shell); same tags as a real build
go build -tags "desktop,production,webkit2_41" ./...
go test -tags "desktop,production,webkit2_41" ./...
```

Drop `webkit2_41` if your system only has WebKit2GTK 4.0. For concurrency-touching
changes also run `go test -race` on `download`, `config` and `handlers`.

Go is run with `GOTOOLCHAIN=local` in the installers; do the same locally if your
Go is newer than `go.mod` and you want to avoid a toolchain download. The Wails CLI
and library versions must match: install the CLI at the version in the root
`go.mod` (`go install github.com/wailsapp/wails/v2/cmd/wails@<that version>`).

## Layout

- `backend/` - engine, scheduler, config, Gin API, CLI/TUI (own Go module).
- repo root - Wails shell (`app.go`, `desktop.go`); `build/` holds icons.
- `frontend/` - React/TS UI. `frontend/wailsjs/` is generated and committed.
- `installers/` - install scripts; see [installers/README.md](installers/README.md).

## Generated Wails bindings

After adding, removing or changing an exported method on `App`, regenerate:

```bash
wails generate module -tags webkit2_41
```

`wails_bindings_test.go` reflects over `*App` and fails when `frontend/wailsjs`
drifts from the Go side. The generator flips `frontend/wailsjs/runtime/*` to mode
755; restore 644 to keep the diff clean.

## Adding or changing a setting

A setting is only real if something consumes it. Wire the whole chain:

1. `backend/internal/pkg/config/setting.go`: field, default (`setDefaults`),
   validation, and the `SettingReq` field the API accepts.
2. Make it take effect without a restart: `download.JobManager.ApplySettings`
   pushes live-tunable options to the running engine (the transport is rebuilt
   only when the proxy or SSRF flag changes).
3. Frontend: render it in exactly one card under `features/settings/` and save
   through `useSettingsPatch`, so a rejected value reverts the draft and shows
   the server's per-field message inline.
4. Test the consumer, not just the round-trip through `settings.json`.

Rules that bite:

- A JSON-absent bool decodes to `false`. Default-true flags must be listed in
  `defaultTrueBools()` or upgraders silently get them off.
- To change the behaviour of an already-persisted setting, bump
  `currentSettingsVersion` and add a branch to `migrate()` so existing installs
  keep what they had (see `upgrade_fixture_test.go`).
- `start_on_launch` is a deprecated alias of `auto_resume_on_launch`; nothing
  outside `config` may read it (a test enforces this).
- A corrupt `settings.json` self-heals and returns `config.ErrRecovered`;
  handlers go through `loadSettings` so that is not a 500.
- Native pickers can fail or be cancelled. `chooseDir()` returns
  `picked | cancelled | unavailable | failed`; never collapse failures into
  "cancelled".

## Local state and debugging

| What | Where (Linux) |
|---|---|
| Settings | `~/.config/rum/settings.json` |
| App log | `~/.config/rum/logs/rum.log` |
| Verbose download trace | `~/.config/rum/logs/debug.log`, only when Settings > General > Logging is Debug (applies live) |
| Autostart entry | `~/.config/autostart/rum.desktop` |

Quit a running Rum before testing a new build: the single-instance lock refocuses
the old window instead of starting the new binary.

Not covered by automated tests, so check by hand when touching them: the native
folder dialog, the clipboard watcher toast, open-folder-after-finish, the
post-download system action, confirm-on-exit, and launch-on-startup. The frontend
test environment is `node` (no DOM), so component rendering is not tested.

## Frontend conventions

Match the existing components (shadcn/ui and the shared settings controls); no raw
`<select>`, `<dialog>` or `confirm()`. Extract at ~150 lines, split at ~250. Keep
labels bound to inputs and keep focus states.
