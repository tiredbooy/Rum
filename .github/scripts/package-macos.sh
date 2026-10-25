#!/usr/bin/env bash
#
# Wrap build/bin/Rum.app in an unsigned .dmg. Used by release.yml.
# Usage: .github/scripts/package-macos.sh VERSION OUTDIR
set -Eeuo pipefail

VERSION="${1:?usage: package-macos.sh VERSION OUTDIR}"
OUTDIR="${2:?usage: package-macos.sh VERSION OUTDIR}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP="$ROOT/build/bin/Rum.app"

[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]] || { echo "bad version: $VERSION" >&2; exit 1; }
[[ -d "$APP" ]] || { echo "missing $APP (run wails build first)" >&2; exit 1; }
mkdir -p "$OUTDIR"
OUTDIR="$(cd "$OUTDIR" && pwd)"

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
cp -R "$APP" "$STAGE/Rum.app"
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname "Rum" -srcfolder "$STAGE" -ov -format UDZO \
  "$OUTDIR/Rum-${VERSION}-macOS-unsigned.dmg"
