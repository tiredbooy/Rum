#!/usr/bin/env bash
#
# Rum GUI (desktop) installer — Linux
#
# Builds the Wails desktop app, installs the binary onto your PATH, and creates
# an application-menu entry (.desktop + icon).
#
# Usage:
#   ./installers/gui/install-linux.sh [--prefix DIR] [--uninstall]
#
#   --prefix DIR   Install binary into DIR/bin (default: ~/.local).
#   --uninstall    Remove the installed app, icon, and menu entry.
#
# Prerequisites: Go 1.25+, Node.js + npm, and the Wails system dependencies
# (gcc, pkg-config, libgtk-3 / webkit2gtk). See https://wails.io/docs/gettingstarted/installation
#
set -euo pipefail

APP="Rum"
PREFIX="${PREFIX:-$HOME/.local}"
UNINSTALL=0
ASSUME_YES=0
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --mirror) MIRROR="$DEFAULT_MIRROR"; shift ;;
    --mirror=*) MIRROR="${1#*=}"; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    -h|--help) sed -n '2,16p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BIN_DIR="$PREFIX/bin"
ICON_DIR="$PREFIX/share/icons/hicolor/512x512/apps"
DESKTOP_DIR="$PREFIX/share/applications"
TARGET="$BIN_DIR/$APP"
ICON_TARGET="$ICON_DIR/$APP.png"
DESKTOP_TARGET="$DESKTOP_DIR/$APP.desktop"

info() { printf '\033[0;36m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[0;32m✓\033[0m %s\n' "$*"; }
err()  { printf '\033[0;31m✗ %s\033[0m\n' "$*" >&2; }

# On restricted networks Go's default servers can return 403/timeouts. Offer a
# Go module proxy mirror (GOPROXY). GOSUMDB is disabled because the checksum
# database (sum.golang.org) is usually blocked on the same networks.
configure_goproxy() {
  if [[ -z "$MIRROR" && "$ASSUME_YES" -eq 0 && -t 0 ]]; then
    read -rp "Use a Go module mirror? Helps if downloads fail with 403/timeouts (e.g. in Iran) [y/N]: " ans
    if [[ "$ans" =~ ^[Yy] ]]; then
      read -rp "Mirror URL [$DEFAULT_MIRROR]: " url
      MIRROR="${url:-$DEFAULT_MIRROR}"
    fi
  fi
  if [[ -n "$MIRROR" ]]; then
    export GOPROXY="$MIRROR"
    export GOSUMDB=off
    ok "Using Go module mirror: $GOPROXY"
  fi
}

if [[ "$UNINSTALL" -eq 1 ]]; then
  rm -f "$TARGET" "$ICON_TARGET" "$DESKTOP_TARGET"
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
  ok "Uninstalled $APP (binary, icon, menu entry)."
  exit 0
fi

# --- Prerequisites ---
command -v go   >/dev/null 2>&1 || { err "Go 1.25+ required: https://go.dev/doc/install"; exit 1; }
command -v npm  >/dev/null 2>&1 || { err "Node.js + npm required: https://nodejs.org"; exit 1; }
ok "Found $(go version)"

# Set the mirror (if requested) BEFORE any 'go install' / 'wails build' downloads.
configure_goproxy

if ! command -v wails >/dev/null 2>&1; then
  info "Wails CLI not found — installing it with 'go install'…"
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  export PATH="$PATH:$(go env GOPATH)/bin"
  command -v wails >/dev/null 2>&1 || { err "wails still not on PATH; add \$(go env GOPATH)/bin to PATH"; exit 1; }
fi
ok "Found wails: $(wails version 2>/dev/null | head -1 || echo present)"

# --- System libraries (GTK + WebKit) needed by the Wails GUI ---
# Wails defaults to webkit2gtk-4.0; modern distros ship 4.1, which Wails builds
# against only with the 'webkit2_41' build tag. Detect and set it automatically.
WAILS_TAGS=""
if command -v pkg-config >/dev/null 2>&1; then
  have_40=0; have_41=0
  pkg-config --exists webkit2gtk-4.0 2>/dev/null && have_40=1
  pkg-config --exists webkit2gtk-4.1 2>/dev/null && have_41=1
  if ! pkg-config --exists gtk+-3.0 2>/dev/null || { [[ "$have_40" -eq 0 && "$have_41" -eq 0 ]]; }; then
    err "Missing GTK3 / WebKit2GTK system libraries (required to build the GUI). Install them, then re-run:"
    echo "  Debian/Ubuntu : sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev"
    echo "  Fedora        : sudo dnf install gcc pkg-config gtk3-devel webkit2gtk4.1-devel"
    echo "  Arch          : sudo pacman -S --needed base-devel gtk3 webkit2gtk-4.1"
    exit 1
  fi
  if [[ "$have_40" -eq 0 && "$have_41" -eq 1 ]]; then
    WAILS_TAGS="webkit2_41"
    ok "Found GTK3 + WebKit2GTK 4.1 (building with -tags webkit2_41)"
  else
    ok "Found GTK3 + WebKit2GTK"
  fi
else
  info "pkg-config not found; skipping GTK/WebKit check (Wails may still fail without them)."
fi

# --- Ensure YOUR app icon is wired into the build (not the Wails default) ---
ensure_icons() {
  local appicon="$REPO_ROOT/build/appicon.png"
  [[ -f "$appicon" ]] && return 0
  local src=""
  for s in "$REPO_ROOT/build/icon.png" "$REPO_ROOT/icon.png"; do [[ -f "$s" ]] && { src="$s"; break; }; done
  [[ -z "$src" ]] && { info "No source icon found; the build will use the default Wails icon."; return 0; }
  mkdir -p "$REPO_ROOT/build/windows"
  if command -v magick >/dev/null 2>&1; then
    magick "$src" -resize 512x512 -background none -gravity center -extent 512x512 "$appicon"
    cp "$appicon" "$REPO_ROOT/build/icon.png"
    magick "$appicon" -background none -define icon:auto-resize=256,128,64,48,32,16 "$REPO_ROOT/build/windows/icon.ico" 2>/dev/null || true
    ok "Generated app icon from $src"
  else
    cp "$src" "$appicon"; cp "$src" "$REPO_ROOT/build/icon.png"
    info "Wired $src as the app icon (install ImageMagick for a properly squared icon)."
  fi
}
ensure_icons

# --- Build ---
info "Building the Rum desktop app (wails build)…"
if [[ -n "$WAILS_TAGS" ]]; then
  ( cd "$REPO_ROOT" && wails build -clean -tags "$WAILS_TAGS" )
else
  ( cd "$REPO_ROOT" && wails build -clean )
fi

BUILT_BIN="$REPO_ROOT/build/bin/$APP"
[[ -f "$BUILT_BIN" ]] || { err "Expected build output $BUILT_BIN not found"; exit 1; }
ok "Built $BUILT_BIN"

# --- Install binary + icon + desktop entry ---
mkdir -p "$BIN_DIR" "$ICON_DIR" "$DESKTOP_DIR"
install -m 0755 "$BUILT_BIN" "$TARGET"
ok "Installed binary to $TARGET"

# Menu icon: prefer the squared appicon, then build/icon.png, then repo icon.png
for ico in "$REPO_ROOT/build/appicon.png" "$REPO_ROOT/build/icon.png" "$REPO_ROOT/icon.png"; do
  if [[ -f "$ico" ]]; then install -m 0644 "$ico" "$ICON_TARGET"; ok "Installed icon"; break; fi
done

cat > "$DESKTOP_TARGET" <<EOF
[Desktop Entry]
Type=Application
Name=$APP
Comment=Powerful, fast and modern download manager
Exec=$TARGET
Icon=$APP
Terminal=false
Categories=Network;FileTransfer;Utility;
StartupWMClass=$APP
EOF
chmod 0644 "$DESKTOP_TARGET"
ok "Installed menu entry to $DESKTOP_TARGET"

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true

case ":$PATH:" in
  *":$BIN_DIR:"*) : ;;
  *) info "Add $BIN_DIR to PATH to launch '$APP' from a terminal." ;;
esac

echo
ok "Done. Launch '$APP' from your application menu, or run: $TARGET"
