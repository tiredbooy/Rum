# Rum – A Smart CLI Download Manager

**Rum** is a fast, beautiful, and fully-featured download manager for the terminal.  
It supports parallel downloads, resume, automatic file organisation, and a gorgeous TUI with live progress bars.

> **Rum is under active development.**  
> Features, flags, and internal behaviour may change.  
> If you’d like to contribute, ideas and pull requests are genuinely welcome – this is an open project and I appreciate any help.

---

## ✨ Features

- **Parallel downloads** – download multiple files at the same time.
- **Resume support** – pick up where you left off, even after a crash.
- **Smart file organisation** – automatically puts files into folders by type (Videos, Music, Archives, Documents, etc.).
- **Beautiful TUI** – coloured status badges, smooth gradient progress bars, live speed, and ETA.
- **Bulk download** – feed a text file with dozens of links.
- **Single binary** – no runtime dependencies after build.

---

## 📦 Installation

### Prerequisites
- [Go](https://go.dev/doc/install) version **1.25 or later**

### 1. Build the binary
Clone the repository and compile the project:

```bash
git clone https://github.com/YourUsername/Rum.git
cd Rum
go build -o rum ./cmd/rum
```
This produces an executable named rum in the current folder.

### 2. Make it runnable from anywhere (Linux / macOS)
To use rum from any terminal, move it to a directory that is in your PATH.

Create a personal bin folder (if it doesn't exist)
```bash
mkdir -p ~/bin
```
Move the binary
```bash
mv rum ~/bin/
Add ~/bin to your shell’s PATH
```
If you use bash:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```
If you use zsh:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```
Now you can type rum in a new terminal window and it will run.

### 3. Automatic Installation script
If you've already cloned the repository, the easiest way to install is with the provided installer script.

#### Windows
Use the PowerShell installer. Make sure Go is installed first.

```bash
cd Rum
Set-ExecutionPolicy -Scope CurrentUser RemoteSigned   # only needed once, if scripts are blocked
.\install.ps1
```

#### Linux / macOS
Run the interactive bash installer:

```bash
cd Rum
chmod +x install.sh
./install.sh
```

#### The installer will:

- Detect where the project root is (it works even if you're inside a subfolder).
- Check that Go is installed.
- Build the rum binary.
- Install it to ~/bin (creating the folder if needed).
- Offer to automatically add ~/bin to your shell's PATH (bash or zsh).
- Provide a beautiful, guided experience with coloured output and a spinner.

#### After the script finishes, open a new terminal or run:
```bash
source ~/.bashrc   # for bash
source ~/.zshrc    # for zsh
```

### 🚀 Usage
Rum is controlled with command‑line flags and an interactive text interface.
Once launched, use your keyboard to control the downloads.

Basic download
```bash
rum --url "https://example.com/file"
Multiple URLs at once
```
```bash
rum --url "https://example.com/file1" --url "https://example.com/file2"
Download from a text file
Create a .txt file with one URL per line:
```

Then run:

```bash
rum --input mylinks.txt
```
You will be prompted to optionally place all files inside a single sub‑folder.

### Control the TUI
Once downloads begin, the following keys are active:

Key	Action
```bash
Ctrl+C	Pause all running downloads
r	Resume all paused downloads
q	Quit (pauses everything)
← / →	Manually scroll through the job list
```
While you are manually scrolling, automatic page advancement is temporarily disabled. It will resume after a few seconds of inactivity.

⚙️ Complete Flag Reference
| Flag         | Type   | Default         | Description |
|--------------|--------|-----------------|-------------|
| `-url`      | string | –                | URL to download. Use multiple times for several files. |
| `-input`    | string | –                | Path to a `.txt` file containing URLs (one per line). |
| `-out`      | string | `~/Downloads/Rum` | Output directory for downloaded files. |
| `-p`        | int | –        1          | Number of parallel downloads. |
| `-limit`    | float | –      0          | Bandwidth limit in MB/s (0 = unlimited). |
| `-uA`      | string | `random` | Custom User‑Agent header. |
| `-rE`      | string | `scheme://host` | Custom Referer header. If empty, derived from the download URL. |

#### Note: All flags can be combined. For example:
rum --input links.txt --p 4 -uA userAgent -rE Referer

### 🧠 How It Works (Briefly)
Job creation – Each URL becomes a Job with a unique ID. Jobs are saved to disk so they survive crashes and restarts.

Size detection – A HEAD request is attempted to read the file size. If the server refuses HEAD, the app falls back and downloads anyway.

Session warming – Before downloading, a request to the referrer page may be sent to capture any required session cookies (helps with sites that protect direct links).

Resume via Range – If a partial local file exists, a Range: bytes=… header is sent. If the server responds with 200 OK (ignoring the range), the file is restarted automatically.

Smart folders – The Content-Type header determines where the file is saved (e.g. videos/, audios/, compressed/, documents/, …). Unknown types go into others/.

### 🌍 Example Session
```bash
$ rum --input season1.txt --p 4 --limit 2.5
```
```bash
Do you want a Group Folder? (Y/N): Y
Enter folder name: season-1
```
```bash
⬇ Rum – Download Manager
──────────────────────────────────────────────────
STATUS     NAME                     SPEED       ETA        PROGRESS               PCT    SIZE
completed  001-Pilot.mp4            2.1 MB/s    --:--      [████████████████████] 100.0%  512 MB / 512 MB
running    002-Characters.mp4       1.8 MB/s    3m12s      [███████████░░░░░░░░░]  61.2%  398 MB / 650 MB
running    003-Soundtrack.flac      850 KB/s    2m45s      [███████░░░░░░░░░░░░░]  42.0%   78 MB / 185 MB
error      004-Review.mp4              0 B/s    --:--      [                    ]   0.0%     ? / ?
waiting    005-Bloopers.mp4              –       –         [                    ]   0.0%     ? / ?
──────────────────────────────────────────────────
Showing 1–5 of 12 downloads
Ctrl+C: pause • r: resume • q: quit
```
### 🤝 Contributing
Rum is an open source project and contributions are sincerely appreciated.
Whether it’s a bug report, a feature suggestion, or a pull request – feel free to open an issue or submit a PR.

Please:

Keep changes focused and clearly described.

Follow the existing code style.

Add comments where the logic isn’t obvious.

If you’re unsure about anything, just open an issue and we can discuss it first. All help is welcome, and contributors will be credited in the project.

### 📝 License
This project is licensed under the MIT License – see the LICENSE file for details.

#### 📬 Contact / Support
Issues & feature requests: GitHub Issues

Discussions: GitHub Discussions (if enabled)

Happy downloading! 🚀