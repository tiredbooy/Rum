# Rum installers

Self-contained installer scripts for both Rum apps on both platforms. Run them
from a checkout of the repository (they resolve the repo root from their own
location, so the working directory doesn't matter).

| App | Linux / macOS | Windows |
|-----|---------------|---------|
| **CLI** (`rum` terminal download manager) | `installers/cli/install-linux.sh` | `installers/cli/install-windows.ps1` |
| **GUI** (Wails desktop app) | `installers/gui/install-linux.sh` | `installers/gui/install-windows.ps1` |

## CLI

Builds `rum` from `backend/cmd/rum` and puts it on your PATH.

```bash
# Linux / macOS
chmod +x installers/cli/install-linux.sh
./installers/cli/install-linux.sh            # installs to /usr/local/bin or ~/.local/bin
./installers/cli/install-linux.sh --prefix ~/.local --yes
./installers/cli/install-linux.sh --uninstall
```

```powershell
# Windows (PowerShell)
.\installers\cli\install-windows.ps1         # installs to %LOCALAPPDATA%\Programs\Rum, adds to PATH
.\installers\cli\install-windows.ps1 -Uninstall
```

**Requires:** Go 1.25+.

## GUI (desktop app)

Builds the Wails app and installs it with a menu/Start-Menu entry.

```bash
# Linux
chmod +x installers/gui/install-linux.sh
./installers/gui/install-linux.sh            # installs to ~/.local + creates a .desktop entry
./installers/gui/install-linux.sh --uninstall
```

```powershell
# Windows (PowerShell)
.\installers\gui\install-windows.ps1         # builds installer if Inno Setup present, else installs + shortcuts
.\installers\gui\install-windows.ps1 -Installer   # force the Inno Setup Rum-Setup.exe
.\installers\gui\install-windows.ps1 -Uninstall
```

**Requires:** Go 1.25+, Node.js + npm, and the Wails toolchain + system
dependencies (GTK/WebKit on Linux; WebView2 on Windows). The GUI scripts will
auto-install the Wails CLI via `go install` if it's missing. See
<https://wails.io/docs/gettingstarted/installation>.

## Notes

- These supersede the older top-level `build-linux.sh` / `build-windows.sh`
  (GUI build helpers) and `backend/install.sh` / `backend/install.ps1` (CLI
  build helpers), unifying them into one place with install/uninstall and
  non-interactive flags.
- The Windows GUI installer reuses the repo's `installer.iss` (Inno Setup) when
  `iscc.exe` is on PATH; otherwise it does a per-user install with shortcuts.
