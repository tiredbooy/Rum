#!/usr/bin/env bash
#
# Rum GUI (desktop) installer — macOS
#
# Builds the Wails desktop app and installs Rum.app into /Applications.
#
# NOTE: this build is UNSIGNED. After installing, Gatekeeper may refuse to open
# it the first time; clear the quarantine flag once with:
#   xattr -dr com.apple.quarantine /Applications/Rum.app
# (the script does this for you when it can write to /Applications).
#
# Usage:
#   ./installers/gui/install-macos.sh [--uninstall] [--mirror[=URL]]
#
#   --uninstall    Remove /Applications/Rum.app.
#   --mirror[=URL] Use a Go module proxy (helps on restricted networks).
#
set -euo pipefail

APP="Rum"
APP_BUNDLE="$APP.app"
INSTALL_DIR="/Applications"
TARGET="$INSTALL_DIR/$APP_BUNDLE"
UNINSTALL=0
ASSUME_YES=0
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes|-y) ASSUME_YES=1; shift ;;
    --mirror) MIRROR="$DEFAULT_MIRROR"; shift ;;
    --mirror=*) MIRROR="${1#*=}"; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

info() { printf '\033[0;36m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[0;32m=>\033[0m %s\n' "$*"; }
err()  { printf '\033[0;31mx %s\033[0m\n' "$*" >&2; }

# Write into /Applications with sudo only when the directory isn't writable.
as_install() {
  if [[ -w "$INSTALL_DIR" ]]; then "$@"; else sudo "$@"; fi
}

# On restricted networks Go's default servers can return 403/timeouts. Offer a
# Go module proxy mirror; GOSUMDB is disabled since sum.golang.org is usually
# blocked on the same networks.
configure_goproxy() {
  if [[ -z "$MIRROR" && "$ASSUME_YES" -eq 0 && -t 0 ]]; then
    read -rp "Use a Go module mirror? Helps if downloads fail with 403/timeouts [y/N]: " ans
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
  if [[ -d "$TARGET" ]]; then
    info "Removing $TARGET"
    as_install rm -rf "$TARGET"
    ok "Uninstalled $APP."
  else
    info "Nothing to remove at $TARGET"
  fi
  exit 0
fi

# --- Prerequisites ---
# Xcode Command Line Tools provide the C toolchain Wails/CGo needs.
if ! xcode-select -p >/dev/null 2>&1; then
  err "Xcode Command Line Tools are required. Install them with:"
  echo "  xcode-select --install" >&2
  exit 1
fi
ok "Found Xcode Command Line Tools"

MISSING=()
command -v go   >/dev/null 2>&1 || MISSING+=("go")
command -v node >/dev/null 2>&1 || MISSING+=("node")
command -v npm  >/dev/null 2>&1 || MISSING+=("npm")
if [[ ${#MISSING[@]} -gt 0 ]]; then
  err "Missing required tool(s) to build the GUI: ${MISSING[*]}"
  if command -v brew >/dev/null 2>&1; then
    echo "  brew install go node" >&2
  else
    echo "  Install Homebrew (https://brew.sh), then: brew install go node" >&2
  fi
  exit 1
fi
ok "Found $(go version)"
ok "Found node $(node --version 2>/dev/null) / npm $(npm --version 2>/dev/null)"

configure_goproxy

# Install the Wails CLI if it's missing.
if ! command -v wails >/dev/null 2>&1; then
  info "Wails CLI not found - installing it with 'go install'..."
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  GOPATH_BIN="$(go env GOPATH)/bin"
  export PATH="$PATH:$GOPATH_BIN"
  command -v wails >/dev/null 2>&1 || { err "wails still not on PATH; add \$(go env GOPATH)/bin to PATH"; exit 1; }
fi
ok "Found wails: $(wails version 2>/dev/null | head -1 || echo present)"

# --- Build (universal binary so it runs on Intel + Apple Silicon) ---
info "Building the Rum desktop app (wails build -platform darwin/universal)..."
( cd "$REPO_ROOT" && wails build -clean -platform darwin/universal )

BUILT_APP="$REPO_ROOT/build/bin/$APP_BUNDLE"
[[ -d "$BUILT_APP" ]] || { err "Expected build output $BUILT_APP not found"; exit 1; }
ok "Built $BUILT_APP"

# --- Install into /Applications ---
info "Installing $APP_BUNDLE into $INSTALL_DIR"
as_install rm -rf "$TARGET"
as_install cp -R "$BUILT_APP" "$TARGET"
ok "Installed $TARGET"

# This build is unsigned; proactively clear the quarantine flag so the first
# launch doesn't get blocked by Gatekeeper.
as_install xattr -dr com.apple.quarantine "$TARGET" 2>/dev/null || \
  info "Could not clear quarantine flag; if Rum won't open, run: xattr -dr com.apple.quarantine \"$TARGET\""

echo
ok "Done. Launch Rum from Launchpad / Applications, or run: open \"$TARGET\""
info "This app is UNSIGNED. If macOS refuses to open it, run once:"
echo "  xattr -dr com.apple.quarantine \"$TARGET\""
