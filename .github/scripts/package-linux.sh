#!/usr/bin/env bash
#
# Package the Wails Linux binary (build/bin/Rum) as an AppImage and a .deb.
# Used by .github/workflows/release.yml.
#
# Usage: .github/scripts/package-linux.sh VERSION OUTDIR
#   VERSION  e.g. 1.2.3 (no leading "v")
#   OUTDIR   receives Rum-VERSION-x86_64.AppImage and Rum_VERSION_amd64.deb
#
# The AppImage and the .deb use the system GTK3 + WebKit2GTK 4.1 (they are not
# bundled), the same dependency the source installer needs.
set -Eeuo pipefail

VERSION="${1:?usage: package-linux.sh VERSION OUTDIR}"
OUTDIR="${2:?usage: package-linux.sh VERSION OUTDIR}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/build/bin/Rum"
ICON="$ROOT/build/appicon.png"

[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]] || { echo "bad version: $VERSION" >&2; exit 1; }
[[ -x "$BIN" ]] || { echo "missing $BIN (run wails build first)" >&2; exit 1; }
mkdir -p "$OUTDIR"
OUTDIR="$(cd "$OUTDIR" && pwd)"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

desktop_file() {
  cat <<DESKTOP
[Desktop Entry]
Type=Application
Name=Rum
Comment=Powerful, fast and modern download manager
Exec=Rum %U
Icon=rum
Terminal=false
Categories=Network;FileTransfer;
StartupWMClass=Rum
DESKTOP
}

# ---- .deb ---------------------------------------------------------------
DEB="$WORK/deb"
mkdir -p "$DEB/DEBIAN" "$DEB/usr/bin" "$DEB/usr/share/applications" \
  "$DEB/usr/share/icons/hicolor/256x256/apps"
install -m 0755 "$BIN" "$DEB/usr/bin/Rum"
install -m 0644 "$ICON" "$DEB/usr/share/icons/hicolor/256x256/apps/rum.png"
desktop_file >"$DEB/usr/share/applications/rum.desktop"
cat >"$DEB/DEBIAN/control" <<CONTROL
Package: rum
Version: $VERSION
Section: net
Priority: optional
Architecture: amd64
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Maintainer: tiredbooy <mahdykazemyo1i2@gmail.com>
Description: Rum - fast and modern download manager
CONTROL
dpkg-deb --build --root-owner-group "$DEB" "$OUTDIR/Rum_${VERSION}_amd64.deb"

# ---- AppImage -----------------------------------------------------------
APPDIR="$WORK/Rum.AppDir"
mkdir -p "$APPDIR/usr/bin"
install -m 0755 "$BIN" "$APPDIR/usr/bin/Rum"
install -m 0644 "$ICON" "$APPDIR/rum.png"
desktop_file >"$APPDIR/rum.desktop"
cat >"$APPDIR/AppRun" <<'APPRUN'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/Rum" "$@"
APPRUN
chmod 0755 "$APPDIR/AppRun"

# appimagetool is pinned by version and verified by SHA-256.
TOOL_VERSION="1.9.0"
TOOL_SHA256="46fdd785094c7f6e545b61afcfb0f3d98d8eab243f644b4b17698c01d06083d1"
TOOL="$WORK/appimagetool"
curl -fsSL --retry 3 -o "$TOOL" \
  "https://github.com/AppImage/appimagetool/releases/download/${TOOL_VERSION}/appimagetool-x86_64.AppImage"
echo "$TOOL_SHA256  $TOOL" | sha256sum -c -
chmod +x "$TOOL"
ARCH=x86_64 "$TOOL" --appimage-extract-and-run "$APPDIR" "$OUTDIR/Rum-${VERSION}-x86_64.AppImage"
