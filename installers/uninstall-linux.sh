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
#   --prefix DIR   Where the GUI was installed (default: ~/.local). /usr/local,
#                  ~/bin and ~/.local are always checked too.
#   --purge        ALSO delete ~/.config/rum (settings.json + download history /
#                  jobs list). Off by default; this is irreversible.
#   --yes, -y      Non-interactive; don't prompt before --purge. Implied when
#                  stdin is not a TTY (except --purge, which still needs --yes).
#   --verbose, -v  Stream package-manager output to the console.
#   --dry-run      Print what would be removed and exit.
#   -h, --help     Show this help.
#
set -Eeuo pipefail

APP_GUI="Rum"
APP_CLI="rum"
PREFIX="${PREFIX:-$HOME/.local}"
PURGE=0
ASSUME_YES=0
VERBOSE=0
DRY_RUN=0
RETRY_MAX="${RUM_RETRY_MAX:-5}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a directory}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --purge) PURGE=1; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --verbose|-v) VERBOSE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) awk 'NR==1{next} /^#/{print; next} {exit}' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -t 0 ]]; then
  ASSUME_YES=1
fi

LOG_FILE=""
init_log() {
  local state_dir
  state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/rum"
  if mkdir -p "$state_dir" 2>/dev/null && [[ -w "$state_dir" ]]; then
    :
  else
    state_dir="/tmp"
  fi
  LOG_FILE="${state_dir}/uninstall-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="/tmp/rum-uninstall-$$.log"; : >"$LOG_FILE"; }
}

redact() {
  printf '%s' "$*" | sed -E 's/(TOKEN|SECRET|PASSWORD|API[_-]?KEY|AUTH|BEARER)[=:][^[:space:]]*/\1=***/Ig'
}

log() {
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date)"
  printf '%s %s\n' "$ts" "$(redact "$*")" >>"$LOG_FILE" 2>/dev/null || true
}

info() { printf '\033[0;36m==>\033[0m %s\n' "$*"; log "INFO  $*"; }
ok()   { printf '\033[0;32m✓\033[0m %s\n' "$*"; log "OK    $*"; }
warn() { printf '\033[0;33m! %s\033[0m\n' "$*"; log "WARN  $*"; }
err()  { printf '\033[0;31m✗ %s\033[0m\n' "$*" >&2; log "ERROR $*"; }
die()  { err "$*"; [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"; exit 1; }

on_err() { err "Failed at line ${1:-?}."; [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"; }
trap 'on_err $LINENO' ERR
trap 'err "Interrupted"; exit 130' INT
trap 'err "Terminated"; exit 143' TERM

init_log

backoff_sleep() {
  local attempt="$1"
  local exp=$(( 2 ** (attempt - 1) ))
  (( exp > 30 )) && exp=30
  sleep "$exp" 2>/dev/null || true
  sleep "0.$(( RANDOM % 1000 ))" 2>/dev/null || true
}

retry() {
  local attempt=1 st=0
  local desc="${RETRY_DESC:-$*}"
  while (( attempt <= RETRY_MAX )); do
    set +e
    "$@"
    st=$?
    set -e
    if (( st == 0 )); then return 0; fi
    if (( attempt == RETRY_MAX )); then
      err "${desc} failed after ${RETRY_MAX} attempts (last exit ${st})"
      return "$st"
    fi
    warn "${desc} failed (attempt ${attempt}/${RETRY_MAX}, exit ${st}); retrying…"
    backoff_sleep "$attempt"
    ((++attempt))
  done
  return "$st"
}

can_root() {
  (( EUID == 0 )) && return 0
  command -v sudo >/dev/null 2>&1 || return 1
  sudo -n true >/dev/null 2>&1
}

as_root() {
  if (( EUID == 0 )); then
    "$@"
  elif can_root; then
    sudo "$@"
  else
    return 1
  fi
}

# XDG config root, matching Go's os.UserConfigDir() used by the app/backend.
CONFIG_ROOT="${XDG_CONFIG_HOME:-$HOME/.config}"
# Defend against a hostile/empty/root XDG_CONFIG_HOME so --purge can never expand
# to something like /rum: fall back to the standard per-user location.
if [[ -z "$CONFIG_ROOT" || "$CONFIG_ROOT" == "/" ]]; then
  CONFIG_ROOT="$HOME/.config"
fi
RUM_CONFIG_DIR="$CONFIG_ROOT/rum"
AUTOSTART_ENTRY="$CONFIG_ROOT/autostart/rum.desktop"

TOUCHED_DESKTOP_DIRS=()
REMOVED_ANY=0

remove_file() {
  local p="$1"
  [[ -e "$p" || -L "$p" ]] || return 0
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove $p"
    REMOVED_ANY=1
    return 0
  fi
  local dir; dir="$(dirname "$p")"
  if [[ -w "$dir" ]]; then
    rm -f "$p" && { ok "Removed $p"; REMOVED_ANY=1; }
  else
    info "Removing $p (needs sudo — $dir is not writable)"
    if as_root rm -f "$p"; then
      ok "Removed $p"
      REMOVED_ANY=1
    else
      warn "Could not remove $p (no write permission and no passwordless sudo)"
    fi
  fi
}

remove_config_dir() {
  local d="$1"
  [[ -n "$d" ]] || return 0
  [[ -d "$d" ]] || { info "No config dir at $d (nothing to purge)"; return 0; }
  if [[ "$d" != "$CONFIG_ROOT/rum" ]]; then
    err "Refusing to delete unexpected config path: $d"; return 1
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove config + history: $d"
    REMOVED_ANY=1
    return 0
  fi
  if [[ -w "$(dirname "$d")" ]]; then rm -rf "$d"; else as_root rm -rf "$d"; fi
  ok "Removed config + history: $d"
  REMOVED_ANY=1
}

remove_deb_package() {
  command -v dpkg >/dev/null 2>&1 || return 0
  local pkg="" cand
  for cand in Rum rum; do
    if dpkg -s "$cand" >/dev/null 2>&1; then pkg="$cand"; break; fi
  done
  [[ -n "$pkg" ]] || return 0

  warn "Rum is also installed as the system package '$pkg' (.deb; files under /usr, owned by dpkg)."
  local do_remove="$ASSUME_YES"
  if [[ "$ASSUME_YES" -eq 0 && -t 0 ]]; then
    local ans=""
    read -rp "Remove the '$pkg' package now via the package manager? [y/N]: " ans
    [[ "$ans" =~ ^[Yy] ]] && do_remove=1
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove system package '$pkg'"
    REMOVED_ANY=1
    return 0
  fi

  if [[ "$do_remove" -eq 1 ]]; then
    if command -v apt-get >/dev/null 2>&1; then
      RETRY_DESC="apt-get remove $pkg" retry as_root apt-get remove -y "$pkg" \
        || die "Could not remove package '$pkg'. Try: sudo apt-get remove $pkg"
    else
      RETRY_DESC="dpkg -r $pkg" retry as_root dpkg -r "$pkg" \
        || die "Could not remove package '$pkg'. Try: sudo dpkg -r $pkg"
    fi
    ok "Removed system package '$pkg'."
    REMOVED_ANY=1
  else
    info "Left the '$pkg' package installed. Remove it with: sudo apt remove $pkg"
  fi
}

PREFIXES=("$PREFIX" "/usr/local" "$HOME/.local" "$HOME")
declare -A SEEN_PREFIX=()

info "Removing Rum (GUI + CLI), menu entry, icon, and autostart…"
info "assume-yes=${ASSUME_YES} verbose=${VERBOSE} dry-run=${DRY_RUN} prefix=${PREFIX}"
info "Log: $LOG_FILE"

for p in "${PREFIXES[@]}"; do
  [[ -n "$p" ]] || continue
  [[ -n "${SEEN_PREFIX[$p]:-}" ]] && continue
  SEEN_PREFIX[$p]=1

  # GUI desktop binary (installers/gui/install-linux.sh installs $PREFIX/bin/Rum).
  remove_file "$p/bin/$APP_GUI"
  # CLI binary (installers/cli/install-linux.sh installs $PREFIX/bin/rum;
  # older backend/install.sh used $HOME/bin/rum).
  remove_file "$p/bin/$APP_CLI"
  # Menu icon + application-menu entry created by the GUI installer (512 and 256).
  remove_file "$p/share/icons/hicolor/512x512/apps/$APP_GUI.png"
  remove_file "$p/share/icons/hicolor/256x256/apps/$APP_GUI.png"
  local_desktop_dir="$p/share/applications"
  if [[ -e "$local_desktop_dir/$APP_GUI.desktop" || -L "$local_desktop_dir/$APP_GUI.desktop" ]]; then
    remove_file "$local_desktop_dir/$APP_GUI.desktop"
    TOUCHED_DESKTOP_DIRS+=("$local_desktop_dir")
  fi
done

# Login-autostart entry written by the app (autostart_linux.go) when
# "Launch on startup" is enabled.
remove_file "$AUTOSTART_ENTRY"

if command -v update-desktop-database >/dev/null 2>&1 && ((${#TOUCHED_DESKTOP_DIRS[@]})); then
  for d in "${TOUCHED_DESKTOP_DIRS[@]}"; do
    [[ "$DRY_RUN" -eq 1 ]] && continue
    update-desktop-database "$d" 2>/dev/null || true
  done
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f "$PREFIX/share/icons/hicolor" 2>/dev/null || true
fi
if [[ ${#TOUCHED_DESKTOP_DIRS[@]} -gt 0 ]]; then
  for kb in kbuildsycoca6 kbuildsycoca5; do
    if command -v "$kb" >/dev/null 2>&1; then "$kb" >/dev/null 2>&1 || true; break; fi
  done
fi

remove_deb_package

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
      ans=""
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
info "Log: $LOG_FILE"
