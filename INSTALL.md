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

> **One-time setup first.** Building Rum needs a few free tools present first.
> It's a copy-and-paste command, and you only do it **once**. Each section below
> tells you exactly what to paste. The installer scripts also detect anything
> missing and print the exact command for your system.

### 🐧 Linux

#### A) The Desktop app (the one with a window)

**Step 1 — Install the one-time tools.** Open your **Terminal** and paste the
line that matches your system:

- **Ubuntu / Linux Mint / Debian:**
  ```bash
  sudo apt update && sudo apt install -y golang nodejs npm build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
  ```
- **Fedora:**
  ```bash
  sudo dnf install -y golang nodejs npm gcc pkg-config gtk3-devel webkit2gtk4.1-devel
  ```
- **Arch / Manjaro / Garuda:**
  ```bash
  sudo pacman -S --needed go nodejs npm base-devel gtk3 webkit2gtk-4.1
  ```

**Step 2 — Install Rum.** Go into the Rum folder and run the installer:
```bash
cd path/to/Rum          # the folder you downloaded
./installers/gui/install-linux.sh
```

The script builds the app, installs it, and adds **Rum** (with its icon) to your
application menu. When it finishes, search for **Rum** in your apps menu. 🎉

To remove it later: `./installers/gui/install-linux.sh --uninstall`

#### B) The CLI (terminal) version

**Step 1 — One-time tool:** install Go (only Go is needed for the CLI):
- Ubuntu/Debian: `sudo apt install -y golang`
- Fedora: `sudo dnf install -y golang`
- Arch: `sudo pacman -S --needed go`

**Step 2 — Install:**
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

**Step 1 — Install the one-time tools** (download and click through each):
- **Go** — https://go.dev/dl/  (Windows installer → Next → Next → Finish)
- **Node.js** — https://nodejs.org  (the "LTS" version)

**Step 2 — Install Rum.**
```powershell
cd C:\path\to\Rum
.\installers\gui\install-windows.ps1
```
The script builds and installs the app, then adds **Rum** to your **Start Menu**
and **Desktop**. 🎉

> With **Inno Setup** installed, the same script instead produces a classic
> `Rum-Setup.exe` you can share — others just double-click it.

To remove it later: `.\installers\gui\install-windows.ps1 -Uninstall`

#### B) The CLI version

**Step 1 — One-time tool:** install **Go** from https://go.dev/dl/.

**Step 2 — Install:**
```powershell
cd C:\path\to\Rum
.\installers\cli\install-windows.ps1
```
Open a **new** PowerShell window and type `rum --version`.

To remove it later: `.\installers\cli\install-windows.ps1 -Uninstall`

### 🍎 macOS (build from source)

**Step 1 — One-time tools:**
```bash
xcode-select --install            # Apple's command-line tools (C toolchain)
brew install go node              # via Homebrew — https://brew.sh
```

**Step 2 — Install:**
```bash
cd path/to/Rum
./installers/gui/install-macos.sh      # Desktop app → /Applications
# or
./installers/cli/install-macos.sh      # CLI → /usr/local/bin/rum
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
Yes — once the one-time tools are installed, a single script builds Rum,
installs it, and puts it in your menu/Start Menu with its icon.

**Will it use my icon and app name?**
Yes. The app is named **Rum** and ships with the Rum icon. To use a different
icon, replace `build/appicon.png` (a square PNG, e.g. 512×512) and run the
installer again.

**Downloads fail with "403 Forbidden" or time out (restricted networks).**
Your network may be blocking Go's default download servers (common in Iran).
Every from-source installer can use a **mirror** — answer **y** when it asks
"Use a Go module mirror?", or pass the flag up front:

```bash
# Linux / macOS
./installers/gui/install-linux.sh --mirror          # default mirror
./installers/cli/install-linux.sh --mirror=https://your-mirror.example/   # custom
```
```powershell
# Windows
.\installers\gui\install-windows.ps1 -Mirror https://go.devneeds.ir/
```
The default mirror is `https://go.devneeds.ir/`; any Go proxy URL works.

**Something else went wrong.** Re-read the message the script printed — it names
exactly what's missing and the command to fix it. The most common cause is
skipping the one-time tools.

---

For the technical reference (flags like `--prefix`, `--yes`, building installers,
the release pipeline), see [`installers/README.md`](installers/README.md).
