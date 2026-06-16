# Installing Rum — Simple Guide

Rum comes in two flavours. Pick the one you want:

- **Rum (Desktop app)** — a normal window with buttons, like any other app. Most people want this.
- **Rum CLI** — the version you run by typing in a black terminal window. For people who like the keyboard.

You can install on **Linux** or **Windows**. Just follow the section for your computer.

> **One-time setup first.** Rum is built right on your computer from its source
> code, so a few free tools need to be present first. Don't worry — it's a
> copy‑and‑paste command, and you only ever do it **once**. Each section tells
> you exactly what to paste.

---

## 🐧 Linux

### A) The Desktop app (the one with a window)

**Step 1 — Install the one-time tools.** Open your **Terminal** app and paste the
line that matches your system, then press Enter (it may ask for your password):

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

**Step 2 — Install Rum.** Still in the Terminal, go into the Rum folder and run the
installer:
```bash
cd path/to/Rum          # the folder you downloaded
./installers/gui/install-linux.sh
```

**That's it.** The script does everything: it builds the app, installs it, and
adds **Rum** (with its icon) to your application menu. When it finishes, just
search for **Rum** in your apps menu and click it. 🎉

To remove it later: `./installers/gui/install-linux.sh --uninstall`

### B) The CLI (terminal) version

**Step 1 — One-time tool:** install Go (only Go is needed for the CLI):
- Ubuntu/Debian: `sudo apt install -y golang`
- Fedora: `sudo dnf install -y golang`
- Arch: `sudo pacman -S --needed go`

**Step 2 — Install:**
```bash
cd path/to/Rum
./installers/cli/install-linux.sh
```
When it's done, open a **new** terminal and type `rum --help` to see how to use it.

To remove it later: `./installers/cli/install-linux.sh --uninstall`

---

## 🪟 Windows

> Open **PowerShell** to run these: click Start, type `PowerShell`, and click it.
> If a script is blocked the first time, paste this once and press Enter:
> `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`

### A) The Desktop app (the one with a window)

**Step 1 — Install the one-time tools** (download and click through each installer):
- **Go** — https://go.dev/dl/  (download the Windows installer, click Next → Next → Finish)
- **Node.js** — https://nodejs.org  (download the "LTS" version, install it)

**Step 2 — Install Rum.** In PowerShell, go to the Rum folder and run:
```powershell
cd C:\path\to\Rum
.\installers\gui\install-windows.ps1
```
The script builds the app and installs it, then adds **Rum** (with its icon) to
your **Start Menu** and **Desktop**. Click the Rum icon to open it. 🎉

> If you have **Inno Setup** installed, the same script instead makes a classic
> `Rum-Setup.exe` you can share with friends — they just double-click it.

To remove it later: `.\installers\gui\install-windows.ps1 -Uninstall`

### B) The CLI (terminal) version

**Step 1 — One-time tool:** install **Go** from https://go.dev/dl/.

**Step 2 — Install:**
```powershell
cd C:\path\to\Rum
.\installers\cli\install-windows.ps1
```
Open a **new** PowerShell window and type `rum --version` to check it worked.

To remove it later: `.\installers\cli\install-windows.ps1 -Uninstall`

---

## ❓ Common questions

**Do the scripts really do everything?**
Yes — once the one-time tools (above) are installed, a single script builds Rum,
installs it, and puts it in your menu/Start Menu with its icon. You don't type
anything else.

**Will it use my icon and app name?**
Yes. The app is named **Rum** and ships with the Rum icon. If you ever want a
different icon, replace the file `build/appicon.png` (a square PNG, e.g.
512×512) and run the installer again — it will rebuild with your new icon.

**I'm not technical — is there an even simpler way?**
The simplest experience is to get a **ready-made file** instead of building it.
The project maintainer can run the script once and produce that file:
- **Windows:** a `Rum-Setup.exe` you double-click (built when Inno Setup is present).
- **Linux:** the built app appears at `build/bin/Rum` and can be shared.

Then anyone can install it **without** any of the one-time tools above.

**Something went wrong.** Re-read the message the script printed — it usually
names exactly what's missing and the command to fix it. The most common cause is
skipping the one-time tools in Step 1.

---

For the technical reference (flags like `--prefix`, `--yes`, building installers),
see [`installers/README.md`](installers/README.md).
