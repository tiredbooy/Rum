#!/usr/bin/env bash
#
# Rum GUI build/install entry point for Linux.
#
# Delegates to the hardened GUI installer so there is a single implementation
# of detection, retries, atomic install, and uninstall. All installer flags
# (--prefix, --yes, --mirror, --uninstall, --verbose, --dry-run) are forwarded.
#
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER="$SCRIPT_DIR/installers/gui/install-linux.sh"

if [[ ! -f "$INSTALLER" ]]; then
  printf '✗ Cannot find %s — run this from a clean checkout.\n' "$INSTALLER" >&2
  exit 1
fi
chmod +x "$INSTALLER" 2>/dev/null || true
exec "$INSTALLER" "$@"
