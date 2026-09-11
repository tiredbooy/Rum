# Rum installers

Self-contained installer scripts. Run them from a checkout; they resolve the
repo root from their own path, so the working directory does not matter.

| App | Linux | macOS | Windows |
|-----|-------|-------|---------|
| **CLI** (`rum`) | `installers/cli/install-linux.sh` | `installers/cli/install-macos.sh` | `installers/cli/install-windows.ps1` |
| **GUI** (Wails desktop) | `installers/gui/install-linux.sh` | `installers/gui/install-macos.sh` | `installers/gui/install-windows.ps1` |

Full Linux uninstall (GUI + CLI + menu + autostart; optional `--purge` of
`~/.config/rum` only): `installers/uninstall-linux.sh`.

Linux CLI from-source has been run in containers (debian/fedora/arch/alpine/
openSUSE). **macOS and Windows from-source were not executed** in that work
(PowerShell was parser-checked only). Treat those platforms as best-effort.

## What is automatic vs not

**Automatic when the script can:** OS/distro/pkgmgr/arch/root/sudo/shell-rc
detection; install Go / Node / Wails CLI; retries with backoff; atomic
stage-then-`mv`; temp-dir cleanup on `EXIT` / `INT` / `TERM`; idempotent
re-runs; user-local `~/.local` when there is no root; PATH lines in
`.bashrc` / `.profile` (and the printed `export PATH=…` hint).

**Not automatic:**

- Linux **GUI** GTK3 + WebKit2GTK packages require **root or passwordless
  sudo**. Without that the script dies with the distro install command.
- **macOS Xcode Command Line Tools** need a one-time GUI dialog.
- Downloaded user files and `~/.config/rum` are **never** deleted on a
  normal uninstall (only `uninstall-linux.sh --purge` removes config).

## Behaviour

- **Non-interactive.** `--yes` / `-Yes` is implied when stdin is not a TTY.
- **Retries.** Network and package operations: default 5 attempts,
  exponential backoff + jitter. `RUM_RETRY_MAX` overrides. HTTP 4xx
  (curl 22) stops after 2 attempts.
- **Go tarball URLs.** `go.dev/dl` → `dl.google.com/go` → (unless
  `RUM_GO_SKIP_EXTRA_MIRRORS=1`) Aliyun `mirrors.aliyun.com/golang` →
  `RUM_GO_FALLBACK_DL` (default `https://go.devneeds.ir/`). `curl` then
  `wget`.
- **Module proxy.** `--mirror` uses the built-in Iranian chain: Runflare →
  Liara → ParsPack → DevNeeds → `proxy.golang.org`. A custom `--mirror=URL`
  is tried before that chain. Auto-selected if `proxy.golang.org` / `go.dev` /
  `github.com` look unreachable. GOPROXY lists use **`|`** between proxies
  (comma only advances on 404/410; 429 sticks). `GOSUMDB` stays enabled
  (default `sum.golang.org`) to authenticate downloaded modules.
- **Atomic install.** Build to a temp file in the destination directory,
  then `mv`. Traps remove the work dir; INT exits 130.
- **Logging.** `$XDG_STATE_HOME/rum/` or `~/.local/state/rum/` or `/tmp`;
  Windows `%LOCALAPPDATA%\Rum\logs`. `--verbose` streams full output.
  Tokens in logged commands are redacted.
- **Preflight.** Detection line, plan, log path; `--dry-run` stops there.

### Go version (CLI vs GUI)

CLI: any Go **≥** `go.mod` (currently **1.25.7**) is kept.

GUI / `build-windows.sh`: Wails **v2.12.0** cannot typecheck with Go 1.26+
(`golang.org/x/tools` v0.30.0). If the system Go is **newer** than
`GO_MIN`, the installer downloads **exactly** `go$GO_MIN` (tarball/zip)
and sets `GOTOOLCHAIN=local` before `wails build`.

Node ≥ **18**. Wails CLI version comes from the root `go.mod` (fallback
**v2.12.0**). Linux GUI tries WebKit 4.1 then 4.0 and passes
`-tags webkit2_41` when only 4.1 is present.

## CLI

```bash
# Linux
chmod +x installers/cli/install-linux.sh
./installers/cli/install-linux.sh                 # /usr/local/bin if writable, else ~/.local/bin
./installers/cli/install-linux.sh --prefix ~/.local --yes
./installers/cli/install-linux.sh --dry-run
./installers/cli/install-linux.sh --uninstall     # rum binary only
```

```bash
# macOS
chmod +x installers/cli/install-macos.sh
./installers/cli/install-macos.sh
```

```powershell
# Windows
.\installers\cli\install-windows.ps1              # %LOCALAPPDATA%\Programs\Rum
.\installers\cli\install-windows.ps1 -DryRun
.\installers\cli\install-windows.ps1 -Uninstall
```

**Auto-installed:** Go ≥ 1.25.7 (CLI does not pin an older toolchain).

## GUI

```bash
# Linux — default prefix ~/.local (bin + icons + .desktop)
chmod +x installers/gui/install-linux.sh
./installers/gui/install-linux.sh
./installers/gui/install-linux.sh --prefix /usr/local
./installers/gui/install-linux.sh --uninstall     # this prefix's binary/icon/desktop
```

```bash
# macOS — /Applications/Rum.app, else ~/Applications
chmod +x installers/gui/install-macos.sh
./installers/gui/install-macos.sh
./installers/gui/install-macos.sh --uninstall
```

```powershell
# Windows — Inno Setup if iscc.exe is on PATH, else per-user + shortcuts
.\installers\gui\install-windows.ps1
.\installers\gui\install-windows.ps1 -Installer   # fail if iscc.exe is missing
.\installers\gui\install-windows.ps1 -Uninstall
```

**Auto-installed when possible:** exact `go.mod` Go (see above), Node 18+,
Wails CLI. **Needs root on Linux:** GTK3 + WebKit2GTK. **Needs you on
macOS:** Xcode CLT dialog. **Needs you on Windows:** WebView2 if winget/
choco/scoop cannot install it (usually preinstalled on Win10/11).

## Flags

Every bash installer (`cli` / `gui` linux+macOS) accepts:

| Flag | Meaning |
|------|---------|
| `--prefix DIR` | CLI: `DIR/bin`. GUI Linux: `DIR/bin` + icons + `.desktop` (default `~/.local`). GUI macOS: `DIR/Rum.app` (default `/Applications` or `~/Applications`). |
| `--yes`, `-y` | Non-interactive (default when stdin is not a TTY) |
| `--mirror` | Built-in Iranian module-proxy fallback chain |
| `--mirror=URL` | Custom module proxy before the built-in fallback chain |
| `--uninstall` | Remove what **this** script installed (not a full wipe) |
| `--verbose`, `-v` | Stream full command output |
| `--dry-run` | Detect, print plan, exit |
| `-h`, `--help` | Usage from the script header |

PowerShell: `-Prefix`, `-Mirror <url>`, `-Uninstall`, `-Yes`, `-DryRun`.
`[CmdletBinding()]` also provides `-Verbose`. GUI Windows adds `-Installer`.

`uninstall-linux.sh` also has `--purge` (delete `~/.config/rum` only;
without a TTY this **requires** `--yes` or it exits 3 and deletes
nothing). It always looks under `--prefix`, `/usr/local`, `~/.local`,
`$HOME/bin`, plus a dpkg `.deb` (removed via apt/dpkg, never by hand).
It does **not** delete downloaded files. Second run on a clean tree
skips missing paths and exits 0 (proven in `rum-test-uninstall-alpine`).

## Convenience wrappers

- `./build-linux.sh` → `installers/gui/install-linux.sh` (all flags forwarded)
- `./backend/install.sh` → CLI linux or macOS installer
- `.\backend\install.ps1` → `installers\cli\install-windows.ps1`
- `./backend/cmd/rum/build.sh` — rebuild CLI into `--prefix` (default
  `~/.local`); does **not** auto-install Go
- `./build-windows.sh` — cross-compile Windows GUI; Docker +
  `amake/innosetup` for `Rum-Setup.exe`. `--skip-installer` stops at
  `Rum.exe`.

## Env (optional)

| Variable | Role |
|----------|------|
| `RUM_RETRY_MAX` | Retry count (default 5) |
| `RUM_CURL_CONNECT_TIMEOUT` | curl/wget connect timeout |
| `RUM_GO_DL_BASE` | default `https://go.dev/dl` |
| `RUM_GO_GOOGLE_DL` | default `https://dl.google.com/go` |
| `RUM_GO_FALLBACK_DL` | last tarball URL (default the built-in proxy host) |
| `RUM_GO_SKIP_EXTRA_MIRRORS=1` | skip Aliyun and the fallback tarball URL |
| `RUM_GO_SUMDB` | checksum database (default `sum.golang.org`) |

## Notes

- Official Go/Node archives are checksum-verified when the checksum file
  is reachable; if it is not, the next URL is still tried.
- Logs never print tokens, passwords, or API keys.
- The built-in Go module chain uses Runflare, Liara, ParsPack, then the
  existing DevNeeds endpoint. Only use `RUM_GO_SUMDB=off` when you explicitly
  accept unauthenticated module downloads.
