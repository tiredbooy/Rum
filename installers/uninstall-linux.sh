#!/usr/bin/env bash
#
# Rum uninstaller — Linux
#
# Removes everything the Rum installers and the app itself put on disk:
#   • the GUI desktop binary           (Rum)
#   • the CLI binary                   (rum)
#   • the application-menu entry + icon (Rum.desktop, Rum.png)
#   • the login-autostart entry        (~/.config/autostart/rum.desktop)
#   • a .deb system package, if present (handed off to apt/dpkg, not removed by hand)
#   • (optional, --purge) your config + download history (~/.config/rum)
#
# It is safe to run repeatedly — anything already gone is simply skipped. It uses
# sudo ONLY for files in directories you cannot write (e.g. a CLI installed into
# /usr/local/bin), and never touches your downloaded files.
#
# Usage:
#   ./installers/uninstall-linux.sh [--prefix DIR] [--purge] [--yes]
#
#   --prefix DIR   Where the GUI was installed (default: ~/.local). /usr/local is
#                  always checked too.
#   --purge        ALSO delete ~/.config/rum (settings.json + download history /
#                  jobs list). Off by default; this is irreversible.
#   --yes, -y      Non-interactive; don't prompt before --purge.
#   -h, --help     Show this help.
#
set -euo pipefail

APP_GUI="Rum"
APP_CLI="rum"
PREFIX="${PREFIX:-$HOME/.local}"
PURGE=0
ASSUME_YES=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a directory}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --purge) PURGE=1; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    -h|--help) sed -n '2,28p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

info() { printf '\033[0;36m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[0;32m✓\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m! %s\033[0m\n' "$*"; }
err()  { printf '\033[0;31m✗ %s\033[0m\n' "$*" >&2; }

# XDG config root, matching Go's os.UserConfigDir() used by the app/backend.
CONFIG_ROOT="${XDG_CONFIG_HOME:-$HOME/.config}"
# Defend against a hostile/empty/root XDG_CONFIG_HOME so --purge can never expand
# to something like /rum: fall back to the standard per-user location.
if [[ -z "$CONFIG_ROOT" || "$CONFIG_ROOT" == "/" ]]; then
  CONFIG_ROOT="$HOME/.config"
fi
RUM_CONFIG_DIR="$CONFIG_ROOT/rum"
AUTOSTART_ENTRY="$CONFIG_ROOT/autostart/rum.desktop"

# Track whether any application-menu entry was touched so we only refresh the
# desktop database directories that actually changed.
TOUCHED_DESKTOP_DIRS=()
REMOVED_ANY=0

# remove_file PATH — delete a single file/symlink if present, using sudo only
# when its parent directory is not writable. No-op if the path doesn't exist.
remove_file() {
  local p="$1"
  [[ -e "$p" || -L "$p" ]] || return 0
  local dir; dir="$(dirname "$p")"
  if [[ -w "$dir" ]]; then
    rm -f "$p" && { ok "Removed $p"; REMOVED_ANY=1; }
  else
    info "Removing $p (needs sudo — $dir is not writable)"
    sudo rm -f "$p" && { ok "Removed $p"; REMOVED_ANY=1; }
  fi
}

# remove_config_dir DIR — guarded recursive delete, used only for --purge. It
# refuses to remove anything whose path does not end in /rum, so a mis-set
# XDG_CONFIG_HOME can never turn this into a dangerous rm -rf.
remove_config_dir() {
  local d="$1"
  [[ -n "$d" ]] || return 0
  [[ -d "$d" ]] || { info "No config dir at $d (nothing to purge)"; return 0; }
  # Exact-match guard: only ever delete the one known per-user config dir, so a
  # mis-set XDG_CONFIG_HOME can never turn this into a dangerous rm -rf.
  if [[ "$d" != "$CONFIG_ROOT/rum" ]]; then
    err "Refusing to delete unexpected config path: $d"; return 1
  fi
  if [[ -w "$(dirname "$d")" ]]; then rm -rf "$d"; else sudo rm -rf "$d"; fi
  ok "Removed config + history: $d"
  REMOVED_ANY=1
}

# remove_deb_package — if Rum was installed from the .deb (the release pipeline
# stages files under /usr, owned by dpkg), remove it through the package manager.
# Deleting dpkg-owned files by hand would corrupt its database, so we never touch
# /usr directly — we hand off to apt/dpkg. The package name is "Rum" (nfpm uses
# APP_NAME, capitalized), but we probe both capitalizations defensively.
remove_deb_package() {
  command -v dpkg >/dev/null 2>&1 || return 0
  local pkg="" cand ans=""
  for cand in Rum rum; do
    if dpkg -s "$cand" >/dev/null 2>&1; then pkg="$cand"; break; fi
  done
  [[ -n "$pkg" ]] || return 0

  warn "Rum is also installed as the system package '$pkg' (.deb; files under /usr, owned by dpkg)."
  local do_remove="$ASSUME_YES"
  if [[ "$ASSUME_YES" -eq 0 && -t 0 ]]; then
    read -rp "Remove the '$pkg' package now via the package manager? [y/N]: " ans
    [[ "$ans" =~ ^[Yy] ]] && do_remove=1
  fi

  if [[ "$do_remove" -eq 1 ]]; then
    if command -v apt-get >/dev/null 2>&1; then
      sudo apt-get remove -y "$pkg"
    else
      sudo dpkg -r "$pkg"
    fi
    ok "Removed system package '$pkg'."
    REMOVED_ANY=1
  else
    info "Left the '$pkg' package installed. Remove it with: sudo apt remove $pkg"
  fi
}

# Candidate install prefixes for the binaries/desktop files: the requested
# --prefix plus the common system prefix, de-duplicated.
PREFIXES=("$PREFIX" "/usr/local")
declare -A SEEN_PREFIX=()

info "Removing Rum (GUI + CLI), menu entry, icon, and autostart…"

for p in "${PREFIXES[@]}"; do
  [[ -n "$p" ]] || continue
  [[ -n "${SEEN_PREFIX[$p]:-}" ]] && continue
  SEEN_PREFIX[$p]=1

  # GUI desktop binary (installers/gui/install-linux.sh installs $PREFIX/bin/Rum).
  remove_file "$p/bin/$APP_GUI"
  # CLI binary (installers/cli/install-linux.sh installs $PREFIX/bin/rum).
  remove_file "$p/bin/$APP_CLI"
  # Menu icon + application-menu entry created by the GUI installer.
  remove_file "$p/share/icons/hicolor/512x512/apps/$APP_GUI.png"
  local_desktop_dir="$p/share/applications"
  if [[ -e "$local_desktop_dir/$APP_GUI.desktop" ]]; then
    remove_file "$local_desktop_dir/$APP_GUI.desktop"
    TOUCHED_DESKTOP_DIRS+=("$local_desktop_dir")
  fi
done

# Login-autostart entry written by the app (autostart_linux.go) when
# "Launch on startup" is enabled.
remove_file "$AUTOSTART_ENTRY"

# Refresh the desktop database for any menu dir we changed (best effort).
if command -v update-desktop-database >/dev/null 2>&1 && ((${#TOUCHED_DESKTOP_DIRS[@]})); then
  for d in "${TOUCHED_DESKTOP_DIRS[@]}"; do
    update-desktop-database "$d" 2>/dev/null || true
  done
fi
# Refresh the icon cache too (best effort, harmless if the tool is absent).
command -v gtk-update-icon-cache >/dev/null 2>&1 && \
  gtk-update-icon-cache -f "$PREFIX/share/icons/hicolor" 2>/dev/null || true
# KDE Plasma keeps its own menu cache (sycoca); rebuild it so the removed entry
# disappears without a re-login. No-op on GNOME/other desktops.
if [[ ${#TOUCHED_DESKTOP_DIRS[@]} -gt 0 ]]; then
  for kb in kbuildsycoca6 kbuildsycoca5; do
    if command -v "$kb" >/dev/null 2>&1; then "$kb" >/dev/null 2>&1 || true; break; fi
  done
fi

# Rum installed from the .deb package (system prefix /usr, owned by dpkg): remove
# it via the package manager rather than leaving it behind or rm-ing /usr by hand.
remove_deb_package

# Optional purge of settings + download history.
if [[ "$PURGE" -eq 1 ]]; then
  if [[ -d "$RUM_CONFIG_DIR" ]]; then
    if [[ "$ASSUME_YES" -eq 0 ]]; then
      # Irreversible: never delete without explicit consent. With no TTY to prompt
      # on and no --yes, fail closed instead of silently wiping the data.
      if [[ ! -t 0 ]]; then
        err "--purge needs confirmation but there is no terminal to prompt on."
        err "Re-run with --yes to purge non-interactively. Nothing was deleted."
        exit 3
      fi
      warn "About to delete $RUM_CONFIG_DIR — this removes your settings AND download history."
      warn "(Close Rum first so it can't re-create the file on exit.)"
      read -rp "Type 'yes' to confirm: " ans
      [[ "$ans" == "yes" ]] || { info "Skipped config purge."; PURGE=0; }
    fi
  fi
  [[ "$PURGE" -eq 1 ]] && remove_config_dir "$RUM_CONFIG_DIR"
else
  if [[ -d "$RUM_CONFIG_DIR" ]]; then
    info "Left your settings + history at $RUM_CONFIG_DIR (re-run with --purge to delete them)."
  fi
fi

echo
if [[ "$REMOVED_ANY" -eq 1 ]]; then
  ok "Done. Rum has been uninstalled."
else
  ok "Nothing to remove — Rum did not appear to be installed in the checked locations."
fi
