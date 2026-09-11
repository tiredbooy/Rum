#!/usr/bin/env bash
#
# Rum CLI installer (backend/ convenience wrapper).
#
# Detects the OS and forwards every flag to the hardened CLI installer:
#   Linux  -> installers/cli/install-linux.sh
#   macOS  -> installers/cli/install-macos.sh
#
# Flags: --prefix DIR, --yes, --mirror[=URL], --uninstall, --verbose, --dry-run
#
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

os="$(uname -s 2>/dev/null || echo unknown)"
case "$os" in
  Linux)  INSTALLER="$REPO_ROOT/installers/cli/install-linux.sh" ;;
  Darwin) INSTALLER="$REPO_ROOT/installers/cli/install-macos.sh" ;;
  *)
    printf '✗ Unsupported OS: %s (use installers/cli/install-windows.ps1 on Windows)\n' "$os" >&2
    exit 1
    ;;
esac

if [[ ! -f "$INSTALLER" ]]; then
  printf '✗ Cannot find %s — run this from a clean checkout.\n' "$INSTALLER" >&2
  exit 1
fi
chmod +x "$INSTALLER" 2>/dev/null || true
exec "$INSTALLER" "$@"
