# Installing Rum

Rum comes in two flavours — pick the one you want:

- **Rum (Desktop app)** — a normal window with buttons, like any other app. Most people want this.
- **Rum CLI** — the terminal version you run by typing commands. For people who like the keyboard.

There are two ways to get Rum. **Most people should use Path 1.**

1. **Download a prebuilt installer** (easiest — nothing to build). ⬇️
2. **Build it from source** (for the CLI, or if no prebuilt file fits your system).

---

## Path 1 — Download a prebuilt installer from Releases ⭐

Go to the **[Releases page](https://github.com/tiredbooy/Rum/releases/latest)**, open the
latest release, and download the file for your system from the table below. No
Go, Node, or build tools required.

| Your system | Download this | How to use it |
|-------------|---------------|---------------|
| **Linux** (any distro) | `Rum-*-x86_64.AppImage` | `chmod +x Rum-*.AppImage` then double-click or run it. Fully portable — no install needed. |
| **Linux** (Debian/Ubuntu/Mint) | `Rum_*_amd64.deb` | `sudo apt install ./Rum_*_amd64.deb` (or double-click in your software centre). Adds Rum to your app menu. |
| **Windows** 10/11 | `Rum-Setup-*.exe` | Double-click and click through the installer. Adds Start-Menu + Desktop shortcuts. (If only a raw `Rum-*.exe` is attached, just run it directly.) |
| **macOS** (Intel + Apple Silicon) | `Rum-*-macOS-unsigned.dmg` | Open the `.dmg` and drag **Rum** to **Applications**. See the note below. |

### ⚠️ macOS note — the app is unsigned

The macOS build is **unsigned** (the project has no Apple Developer signing
certificate). The first time you open it, macOS Gatekeeper may say the app
"cannot be opened because it is from an unidentified developer" or "is damaged".

After dragging Rum into **Applications**, run this **once** in Terminal to clear
the quarantine flag, then open Rum normally:

```bash
xattr -dr com.apple.quarantine /Applications/Rum.app
```

(Alternatively: right-click **Rum.app → Open → Open**, which only needs to be
done once.)

> The macOS build is **best-effort and untested** — it is produced
> automatically but not verified on a real Mac. If you hit issues, the
> build-from-source path (below) is the fallback.

---

## Path 2 — Build from source

The from-source scripts detect OS, CPU, package manager, and whether they
can use root; retry failed downloads; and install into `~/.local` when there
is no root. `--yes` is implied when stdin is not a TTY (CI, pipes).

**Automatic when possible:** Go, Node.js / npm, the Wails CLI, and (on Linux,
**as root or with passwordless sudo**) gcc, pkg-config, GTK3, and WebKit2GTK.

**Still needs you:**

- **Linux GUI:** GTK3 + WebKit2GTK packages cannot be installed without root.
  Run the GUI installer as root / with passwordless sudo, or install those
  packages yourself first.
- **macOS:** Xcode Command Line Tools need a one-time Apple dialog
  (`xcode-select --install`) if they are missing. The script cannot click it.
- **Windows from-source** and **macOS from-source** have not been executed
  end-to-end in the current hardening work. Treat them as best-effort.

Pass `--dry-run` to print the plan and exit. Re-running an installer is safe
(it skips tools that already meet the minimum and replaces the binary).

### 🐧 Linux

#### A) The Desktop app (the one with a window)

```bash
cd path/to/Rum
./installers/gui/install-linux.sh
```

That's it. The script prints a short plan (what's missing, what it will
install), then proceeds. Search for **Rum** in your apps menu when it finishes.

To remove it later: `./installers/gui/install-linux.sh --uninstall`

(The repo-root `./build-linux.sh` is the same installer.)

#### B) The CLI (terminal) version

```bash
cd path/to/Rum
./installers/cli/install-linux.sh
```

When it's done, open a **new** terminal and type `rum --help`.

To remove it later: `./installers/cli/install-linux.sh --uninstall`

### 🪟 Windows

> Open **PowerShell**: click Start, type `PowerShell`, click it. If a script is
> blocked the first time, paste this once and press Enter:
> `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`

#### A) The Desktop app

```powershell
cd C:\path\to\Rum
.\installers\gui\install-windows.ps1
```

The script installs Go / Node / Wails if they're missing (via winget, choco,
or scoop, otherwise an official zip), builds the app, and adds **Rum** to
your **Start Menu** and **Desktop**.

> With **Inno Setup** installed, the same script instead produces a classic
> `Rum-Setup.exe` you can share — others just double-click it.

To remove it later: `.\installers\gui\install-windows.ps1 -Uninstall`

#### B) The CLI version

```powershell
cd C:\path\to\Rum
.\installers\cli\install-windows.ps1
```

Open a **new** PowerShell window and type `rum --version`.

To remove it later: `.\installers\cli\install-windows.ps1 -Uninstall`

### 🍎 macOS (build from source)

Xcode Command Line Tools still need a one-time Apple dialog (`xcode-select
--install`) if they aren't already present — the script will tell you. Go and
Node are installed via Homebrew when available, otherwise from official
tarballs into `~/.local`.

```bash
cd path/to/Rum
./installers/gui/install-macos.sh      # Desktop app → /Applications (or ~/Applications)
# or
./installers/cli/install-macos.sh      # CLI → /usr/local/bin/rum (or ~/.local/bin)
```

The GUI script builds a universal app and copies **Rum.app** into
**/Applications**, clearing the Gatekeeper quarantine flag for you. Because the
build is unsigned, if macOS still refuses to open it run once:

```bash
xattr -dr com.apple.quarantine /Applications/Rum.app
```

To remove later: `./installers/gui/install-macos.sh --uninstall` (or
`./installers/cli/install-macos.sh --uninstall`).

---

## ❓ Common questions

**Which should I pick — prebuilt or from source?**
If a prebuilt file in the table fits your system, use it: nothing to install,
nothing to build. Build from source if you want the CLI, want to use your own
icon, or no prebuilt file matches your machine.

**Do the from-source scripts really do everything?**
They auto-detect your system, install what they can without prompting, build
Rum, and install it. Re-running is safe. They do **not** magically get root
for GTK/WebKit, and they cannot finish the macOS Xcode CLT dialog for you.
A timestamped log is written under `$XDG_STATE_HOME/rum/` (or
`~/.local/state/rum/`, `/tmp`, or `%LOCALAPPDATA%\Rum\logs` on Windows).
Pass `--verbose` / `-Verbose` to stream it; `--dry-run` prints the plan and
exits.

**Which Go version does the GUI use?**
The CLI accepts any Go at or above `go.mod` (currently 1.25.7). The GUI
installer, if it finds a *newer* system Go (1.26/1.27), installs **exactly**
the `go.mod` version via the official tarball/zip and sets `GOTOOLCHAIN=local`.
Wails v2.12.0 cannot typecheck against Go 1.27. The installer pins the
compatible toolchain directly and keeps module checksum verification enabled.

**Will it use my icon and app name?**
Yes. The app is named **Rum** and ships with the Rum icon. To use a different
icon, replace `build/appicon.png` (a square PNG, e.g. 512×512) and run the
installer again.

**Downloads fail with "403 Forbidden" or time out (restricted networks).**
The scripts retry with backoff, then try extra Go tarball URLs (including
Aliyun) and, if `proxy.golang.org` looks unreachable, an Iranian module-proxy
chain (Runflare → Liara → ParsPack → DevNeeds) before the official proxy.

If *every* Go tarball URL fails, the installer falls back to fetching the
toolchain through the Go module proxy instead
(`golang.org/toolchain@v0.0.1-go<version>.<os>-<arch>`), which reaches a
different host. That route needs an existing `go` on the machine to drive the
download, so it can rescue a too-new Go but not a machine with no Go at all;
the installer says which route it used. It is always checksum-verified against
`sum.golang.org` — set `RUM_GO_SUMDB` to point elsewhere if you run your own.

The installer prefers whichever route it can actually verify. If the official
checksum index is unreachable — so an archive download could not be checked — it
tries the module proxy first, because that route is verified against
`sum.golang.org` or it fails. It only installs an unverified archive when there is
no alternative (typically a machine with no Go at all, which cannot use the module
route), and it says so:

> `Go 1.25.7 obtained via the tarball but could NOT be verified: ...`

Set `RUM_REQUIRE_VERIFIED_GO=1` to refuse to install in that case instead of
warning. It is not the default because on restricted networks the verification
services are often unreachable too, and that would block installation entirely.

`--mirror` (no URL) or `-Mirror` forces the built-in Iranian proxy chain;
`--mirror=URL` / `-Mirror URL` tries your proxy first, then that chain. The
installer keeps `GOSUMDB=sum.golang.org` enabled so downloaded modules are
authenticated. You can pass any Go module proxy:

```bash
# Linux / macOS
./installers/gui/install-linux.sh --mirror
./installers/cli/install-linux.sh --mirror=https://your-proxy.example/
```

```powershell
# Windows
.\installers\gui\install-windows.ps1 -Mirror https://your-proxy.example/
```

**Something else went wrong.** Re-read the message the script printed — it names
exactly what's missing and the command to fix it. The last line always points
at the log file.

---

For the technical reference (flags like `--prefix`, `--yes`, `--dry-run`,
building installers, the release pipeline), see [`installers/README.md`](installers/README.md).
