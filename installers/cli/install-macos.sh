#!/usr/bin/env bash
#
# Rum CLI installer — macOS
#
# Builds the `rum` command-line download manager from source and installs it
# into /usr/local/bin (or ~/.local/bin if that isn't writable).
#
# Usage:
#   ./installers/cli/install-macos.sh [--prefix DIR] [--yes] [--uninstall] [--mirror[=URL]]
#
#   --prefix DIR   Install into DIR/bin (default: /usr/local, falling back to
#                  ~/.local). Honors $PREFIX too.
#   --yes          Non-interactive; assume "yes" to prompts.
#   --uninstall    Remove an installed `rum` binary instead of installing.
#   --mirror[=URL] Use a Go module proxy (helps on restricted networks).
#
set -euo pipefail

APP="rum"
ASSUME_YES=0
UNINSTALL=0
PREFIX="${PREFIX:-}"
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a directory}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --mirror) MIRROR="$DEFAULT_MIRROR"; shift ;;
    --mirror=*) MIRROR="${1#*=}"; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    -h|--help) sed -n '2,19p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"

info() { printf '\033[0;36m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[0;32m=>\033[0m %s\n' "$*"; }
err()  { printf '\033[0;31mx %s\033[0m\n' "$*" >&2; }

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

choose_bindir() {
  if [[ -n "$PREFIX" ]]; then echo "$PREFIX/bin"; return; fi
  if [[ -w /usr/local/bin ]]; then echo "/usr/local/bin"; return; fi
  if command -v sudo >/dev/null 2>&1 && [[ "$ASSUME_YES" -eq 0 ]]; then
    echo "/usr/local/bin"; return   # will sudo on install
  fi
  echo "$HOME/.local/bin"
}

BIN_DIR="$(choose_bindir)"
TARGET="$BIN_DIR/$APP"

if [[ "$UNINSTALL" -eq 1 ]]; then
  for cand in "$TARGET" "/usr/local/bin/$APP" "$HOME/.local/bin/$APP"; do
    if [[ -e "$cand" ]]; then
      info "Removing $cand"
      if [[ -w "$(dirname "$cand")" ]]; then rm -f "$cand"; else sudo rm -f "$cand"; fi
      ok "Removed $cand"
    fi
  done
  exit 0
fi

# --- Prerequisites ---
# Xcode Command Line Tools provide the C toolchain Go's CGo paths may need.
if ! xcode-select -p >/dev/null 2>&1; then
  err "Xcode Command Line Tools are required. Install them with:"
  echo "  xcode-select --install" >&2
  exit 1
fi
ok "Found Xcode Command Line Tools"

if ! command -v go >/dev/null 2>&1; then
  err "Go is not installed (required to build the CLI)."
  if command -v brew >/dev/null 2>&1; then
    echo "  brew install go" >&2
  else
    echo "  Install Homebrew (https://brew.sh) then: brew install go" >&2
    echo "  (or install Go 1.25+ from https://go.dev/doc/install)" >&2
  fi
  exit 1
fi
ok "Found $(go version)"

if [[ ! -d "$BACKEND_DIR/cmd/rum" ]]; then
  err "Cannot find $BACKEND_DIR/cmd/rum - run this from a clean checkout."
  exit 1
fi

configure_goproxy

# --- Build ---
info "Building $APP (this may take a moment)..."
BUILD_OUT="$(mktemp -d)/$APP"
( cd "$BACKEND_DIR" && go build -trimpath -ldflags "-s -w" -o "$BUILD_OUT" ./cmd/rum )
ok "Built $APP"

# --- Install ---
mkdir -p "$BIN_DIR"
if [[ -w "$BIN_DIR" ]]; then
  install -m 0755 "$BUILD_OUT" "$TARGET"
else
  info "Installing to $BIN_DIR requires sudo"
  sudo install -m 0755 "$BUILD_OUT" "$TARGET"
fi
ok "Installed to $TARGET"

# --- PATH hint ---
case ":$PATH:" in
  *":$BIN_DIR:"*) : ;;
  *)
    echo
    info "$BIN_DIR is not on your PATH. Add it:"
    echo "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.zshrc"
    echo "  source ~/.zshrc"
    ;;
esac

echo
ok "Done. Try:  $APP --version  &&  $APP --help"
