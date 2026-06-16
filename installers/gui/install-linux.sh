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

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
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

if ! command -v wails >/dev/null 2>&1; then
  info "Wails CLI not found — installing it with 'go install'…"
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  export PATH="$PATH:$(go env GOPATH)/bin"
  command -v wails >/dev/null 2>&1 || { err "wails still not on PATH; add \$(go env GOPATH)/bin to PATH"; exit 1; }
fi
ok "Found wails: $(wails version 2>/dev/null | head -1 || echo present)"

# --- Build ---
info "Building the Rum desktop app (wails build)…"
( cd "$REPO_ROOT" && wails build -clean )

BUILT_BIN="$REPO_ROOT/build/bin/$APP"
[[ -f "$BUILT_BIN" ]] || { err "Expected build output $BUILT_BIN not found"; exit 1; }
ok "Built $BUILT_BIN"

# --- Install binary + icon + desktop entry ---
mkdir -p "$BIN_DIR" "$ICON_DIR" "$DESKTOP_DIR"
install -m 0755 "$BUILT_BIN" "$TARGET"
ok "Installed binary to $TARGET"

# Icon: prefer build/icon.png, fall back to repo icon.png
for ico in "$REPO_ROOT/build/icon.png" "$REPO_ROOT/icon.png"; do
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
