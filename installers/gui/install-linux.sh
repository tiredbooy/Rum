#!/usr/bin/env bash
#
# Rum GUI (desktop) installer — Linux
#
# Builds the Wails desktop app, installs the binary onto PATH, and creates
# an application-menu entry (.desktop + icon). Prerequisites (Go, Node, Wails,
# GTK/WebKit build deps) are auto-detected and installed when possible.
#
# Usage:
#   ./installers/gui/install-linux.sh [options]
#
#   --prefix DIR   Install binary into DIR/bin (default: ~/.local).
#   --yes, -y      Non-interactive (implied when stdin is not a TTY).
#   --mirror[=URL] Use a Go module proxy (default URL if none given).
#   --uninstall    Remove the installed app, icon, and menu entry.
#   --verbose, -v  Stream full command output to the console.
#   --dry-run      Detect, print the plan, and exit without changing the system.
#   -h, --help     Show this help.
#
# Go: a Go that is already installed (PATH, /usr/local/go, ~/sdk, snap, ...) on
# go.mod's minor line is used as-is with GOTOOLCHAIN=local; nothing is
# downloaded. Only if none fits is Go fetched (go.dev, dl.google.com, then
# non-Google mirrors, sha256-verified). If only a newer Go exists and the
# download fails, that Go is used with a warning.
#   RUM_GO_ROOT=DIR          use the Go in DIR (a GOROOT)
#   RUM_GO_MIRROR=URL        try URL/go<ver>.<os>-<arch>.tar.gz first
#   RUM_GO_NO_DOWNLOAD=1     never download Go (fail with instructions)
#   GOPROXY / GOSUMDB        respected; a fallback chain is used only if the
#                            default proxy is unreachable and you set neither
#   RUM_NPM_REGISTRY=URL     npm fallback registry if registry.npmjs.org is down
#

# shellcheck disable=SC2034  # shared constants are read by installers/lib/rum-go.sh
set -Eeuo pipefail

APP="Rum"
PREFIX="${PREFIX:-$HOME/.local}"
UNINSTALL=0
ASSUME_YES=0
VERBOSE=0
DRY_RUN=0
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"
IRANIAN_GOPROXY="https://mirror-go.runflare.com|https://package-mirror.liara.ir/repository/go|https://mirror.abrha.net/repository/go|${DEFAULT_MIRROR%/}"
RETRY_MAX="${RUM_RETRY_MAX:-5}"
GO_DL_BASE="${RUM_GO_DL_BASE:-https://go.dev/dl}"
GO_GOOGLE_DL="${RUM_GO_GOOGLE_DL:-https://dl.google.com/go}"
GO_FALLBACK_DL="${RUM_GO_FALLBACK_DL:-$DEFAULT_MIRROR}"
NODE_MIN="18.0.0"
WAILS_FALLBACK="v2.17.0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a directory}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --mirror) MIRROR="$DEFAULT_MIRROR"; shift ;;
    --mirror=*) MIRROR="${1#*=}"; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    --verbose|-v) VERBOSE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help)
      {
        read -r _
        while IFS= read -r line; do
          case "$line" in
            \#*) printf '%s\n' "$line" ;;
            *) break ;;
          esac
        done
      } < "$0"
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

# Non-interactive when stdin is not a TTY (CI, pipes, `yes |` still counts as TTY).
if [[ ! -t 0 ]]; then
  ASSUME_YES=1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

BIN_DIR="$PREFIX/bin"
ICON_DIR="$PREFIX/share/icons/hicolor/512x512/apps"
DESKTOP_DIR="$PREFIX/share/applications"
TARGET="$BIN_DIR/$APP"
ICON_TARGET="$ICON_DIR/$APP.png"
DESKTOP_TARGET="$DESKTOP_DIR/$APP.desktop"

WORK_DIR=""
LOG_FILE=""
APT_UPDATED=0

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
init_log() {
  local state_dir
  state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/rum"
  if mkdir -p "$state_dir" 2>/dev/null && [[ -w "$state_dir" ]]; then
    :
  else
    state_dir="/tmp"
  fi
  LOG_FILE="${state_dir}/gui-install-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="/tmp/rum-gui-install-$$.log"; : >"$LOG_FILE"; }
}

redact() {
  printf '%s' "$*" | sed -E 's/(TOKEN|SECRET|PASSWORD|API[_-]?KEY|AUTH|BEARER)[=:][^[:space:]]*/\1=***/Ig'
}

log() {
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date)"
  printf '%s %s\n' "$ts" "$(redact "$*")" >>"$LOG_FILE" 2>/dev/null || true
}

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  C_CY=$'\033[0;36m'; C_GR=$'\033[0;32m'; C_YE=$'\033[0;33m'; C_RS=$'\033[0m'
else
  C_CY=""; C_GR=""; C_YE=""; C_RS=""
fi
if [[ -t 2 && -z "${NO_COLOR:-}" ]]; then C_RD=$'\033[0;31m'; else C_RD=""; fi
info() { printf '%s==>%s %s\n' "$C_CY" "$C_RS" "$*"; log "INFO  $*"; }
ok()   { printf '%s✓%s %s\n' "$C_GR" "$C_RS" "$*"; log "OK    $*"; }
warn() { printf '%s! %s%s\n' "$C_YE" "$*" "$C_RS"; log "WARN  $*"; }
err()  { printf '%s✗ %s%s\n' "$C_RD" "$*" "$C_RS" >&2; log "ERROR $*"; }
die()  { err "$*"; [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"; exit 1; }

# ---------------------------------------------------------------------------
# Cleanup / traps
# ---------------------------------------------------------------------------
cleanup() {
  local st=$?
  if [[ -n "${WORK_DIR:-}" && -d "${WORK_DIR:-}" ]]; then
    rm -rf "$WORK_DIR" || true
  fi
  trap - EXIT
  exit "$st"
}

on_err() {
  local line="${1:-?}"
  err "Failed at line ${line}."
  [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"
}
on_int()  { err "Interrupted"; exit 130; }
on_term() { err "Terminated"; exit 143; }

trap cleanup EXIT
trap 'on_err $LINENO' ERR
trap on_int INT
trap on_term TERM

init_log
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/rum-gui-XXXXXX")"
log "repo=$REPO_ROOT prefix=$PREFIX log=$LOG_FILE work=$WORK_DIR"

# ---------------------------------------------------------------------------
# Retry, run, download
# ---------------------------------------------------------------------------
backoff_sleep() {
  local attempt="$1"
  local exp=$(( 2 ** (attempt - 1) ))
  (( exp > 30 )) && exp=30
  local frac="0.$(( RANDOM % 1000 ))"
  sleep "$exp" 2>/dev/null || true
  sleep "$frac" 2>/dev/null || true
}

retry() {
  local attempt=1 st=0
  local desc="${RETRY_DESC:-$*}"
  while [ "$attempt" -le "$RETRY_MAX" ]; do
    if "$@"; then
      return 0
    else
      st=$?
    fi
    if [ "$st" -eq 22 ] && [ "$attempt" -ge 2 ]; then
      err "${desc} failed with HTTP error (curl exit 22) after ${attempt} attempts"
      return "$st"
    fi
    if [ "$attempt" -eq "$RETRY_MAX" ]; then
      err "${desc} failed after ${RETRY_MAX} attempts (last exit ${st})"
      err "Re-run this script; completed steps are skipped. Log: $LOG_FILE"
      return "$st"
    fi
    warn "${desc} failed (attempt ${attempt}/${RETRY_MAX}, exit ${st}); retrying…"
    backoff_sleep "$attempt"
    attempt=$((attempt + 1))
  done
  return "$st"
}

run_logged() {
  log "\$ $(redact "$*")"
  if [[ "$VERBOSE" -eq 1 ]]; then
    if "$@" 2>&1 | tee -a "$LOG_FILE"; then
      return 0
    else
      return 1
    fi
  fi
  if "$@" >>"$LOG_FILE" 2>&1; then
    return 0
  else
    return 1
  fi
}

have_curl() { command -v curl >/dev/null 2>&1; }
have_wget() { command -v wget >/dev/null 2>&1; }

download_file() {
  local url="$1" dest="$2" sha="${3:-}"
  local tmp="${dest}.part" st=0
  local connect_to="${RUM_CURL_CONNECT_TIMEOUT:-20}"
  mkdir -p "$(dirname "$dest")"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would download $url -> $dest"
    return 0
  fi
  if have_curl; then
    set +e
    RETRY_DESC="curl $url" retry curl -fL --connect-timeout "$connect_to" --max-time 600 -o "$tmp" "$url"
    st=$?
    set -e
  elif have_wget; then
    set +e
    RETRY_DESC="wget $url" retry wget -O "$tmp" --timeout="$connect_to" --tries=1 "$url"
    st=$?
    set -e
  else
    die "Need curl or wget to download $url"$'\n'"Install one as root, then re-run:"$'\n'"$(print_gui_dep_cmd)"
  fi
  if (( st != 0 )); then
    rm -f "$tmp"
    return "$st"
  fi
  if [[ -n "$sha" ]]; then
    verify_sha256 "$tmp" "$sha" || { rm -f "$tmp"; die "Checksum mismatch for $url — refusing to install."; }
  fi
  mv -f "$tmp" "$dest"
}

download_to_stdout() {
  local url="$1"
  if have_curl; then
    curl -fsSL --connect-timeout 20 --max-time 120 "$url"
  elif have_wget; then
    wget -q -O - --timeout=20 --tries=1 "$url"
  else
    return 1
  fi
}

reachable() {
  local url="$1"
  if have_curl; then
    curl -fsS -o /dev/null --connect-timeout 5 --max-time 8 -I "$url" 2>/dev/null \
      || curl -fsS -o /dev/null --connect-timeout 5 --max-time 8 "$url" 2>/dev/null
  elif have_wget; then
    wget -q --spider --timeout=8 --tries=1 "$url" 2>/dev/null
  else
    return 1
  fi
}

verify_sha256() {
  local file="$1" expect="$2" got=""
  [[ -n "$expect" ]] || return 0
  if command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$file" | awk '{print $1}')"
  elif command -v openssl >/dev/null 2>&1; then
    got="$(openssl dgst -sha256 "$file" | awk '{print $NF}')"
  else
    warn "No sha256 tool found; skipping checksum verification for $(basename "$file")"
    return 0
  fi
  [[ "$(printf '%s' "$got" | tr 'A-F' 'a-f')" == "$(printf '%s' "$expect" | tr 'A-F' 'a-f')" ]]
}

# ---------------------------------------------------------------------------
# Detection
# ---------------------------------------------------------------------------
OS_KERNEL="$(uname -s 2>/dev/null || echo unknown)"
OS_ARCH_RAW="$(uname -m 2>/dev/null || echo unknown)"
is_musl() {
  [[ -e /lib/ld-musl-x86_64.so.1 || -e /lib/ld-musl-aarch64.so.1 ]] && return 0
  ldd --version 2>&1 | grep -qi musl
}
case "$OS_ARCH_RAW" in
  x86_64|amd64) OS_ARCH="amd64"; NODE_ARCH="x64" ;;
  aarch64|arm64) OS_ARCH="arm64"; NODE_ARCH="arm64" ;;
  armv7l|armhf) OS_ARCH="armv7l"; NODE_ARCH="armv7l" ;;
  *) OS_ARCH="$OS_ARCH_RAW"; NODE_ARCH="$OS_ARCH_RAW" ;;
esac

DISTRO_ID="unknown"
DISTRO_LIKE=""
if [[ -r /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  DISTRO_ID="${ID:-unknown}"
  DISTRO_LIKE="${ID_LIKE:-}"
fi

detect_pkg_mgr() {
  case " $DISTRO_ID $DISTRO_LIKE " in
    *" debian "*|*" ubuntu "*)
      command -v apt-get >/dev/null 2>&1 && { echo apt; return; } ;;
    *" fedora "*|*" rhel "*|*" centos "*|*" rocky "*|*" alma "*|*" nobara "*)
      command -v dnf >/dev/null 2>&1 && { echo dnf; return; }
      command -v yum >/dev/null 2>&1 && { echo yum; return; } ;;
    *" arch "*|*" manjaro "*|*" endeavour "*)
      command -v pacman >/dev/null 2>&1 && { echo pacman; return; } ;;
    *" suse "*|*" opensuse "*)
      command -v zypper >/dev/null 2>&1 && { echo zypper; return; } ;;
    *" alpine "*)
      command -v apk >/dev/null 2>&1 && { echo apk; return; } ;;
  esac
  if command -v apt-get >/dev/null 2>&1; then echo apt; return; fi
  if command -v dnf >/dev/null 2>&1; then echo dnf; return; fi
  if command -v yum >/dev/null 2>&1; then echo yum; return; fi
  if command -v pacman >/dev/null 2>&1; then echo pacman; return; fi
  if command -v zypper >/dev/null 2>&1; then echo zypper; return; fi
  if command -v apk >/dev/null 2>&1; then echo apk; return; fi
  if command -v brew >/dev/null 2>&1; then echo brew; return; fi
  echo none
}
PKG_MGR="$(detect_pkg_mgr)"

can_root() {
  (( EUID == 0 )) && return 0
  command -v sudo >/dev/null 2>&1 || return 1
  sudo -n true >/dev/null 2>&1
}

as_root() {
  if (( EUID == 0 )); then
    "$@"
  elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
    sudo "$@"
  else
    return 1
  fi
}

IS_ROOT=0
(( EUID == 0 )) && IS_ROOT=1
HAS_SUDO_N=0
can_root && [[ "$IS_ROOT" -eq 0 ]] && HAS_SUDO_N=1

detect_shell_rc() {
  local shell_name rc
  shell_name="$(basename "${SHELL:-bash}")"
  case "$shell_name" in
    zsh)  rc="$HOME/.zshrc" ;;
    bash)
      if [[ -f "$HOME/.bashrc" ]]; then rc="$HOME/.bashrc"
      elif [[ -f "$HOME/.bash_profile" ]]; then rc="$HOME/.bash_profile"
      else rc="$HOME/.profile"
      fi
      ;;
    fish) rc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
    *)    rc="$HOME/.profile" ;;
  esac
  printf '%s' "$rc"
}
SHELL_RC="$(detect_shell_rc)"

for _d in "$HOME/.local/bin" "$BIN_DIR"; do
  [[ -n "$_d" && -d "$_d" ]] || continue
  case ":$PATH:" in *":$_d:"*) continue ;; esac
  PATH="$_d:$PATH"
done
unset _d
export PATH

RUM_GO_OS=linux
RUM_GO_SAME_MINOR=0   # Wails >= 2.17 pins a new-enough x/tools; any Go >= go.mod works
if [[ ! -r "$REPO_ROOT/installers/lib/rum-go.sh" ]]; then
  printf 'Missing %s\nRun this script from a full checkout of the repository.\n' "$REPO_ROOT/installers/lib/rum-go.sh" >&2
  exit 1
fi
# shellcheck source=../lib/rum-go.sh
. "$REPO_ROOT/installers/lib/rum-go.sh"
GO_MIN="$(rum_read_go_min "$REPO_ROOT/go.mod" "$REPO_ROOT/backend/go.mod")"
# Prefer a Go that is already installed; nothing is downloaded unless none fits go.mod.
rum_select_local_go || true

read_wails_ver() {
  local f="$REPO_ROOT/go.mod" line ver
  if [[ -f "$f" ]]; then
    while IFS= read -r line; do
      case "$line" in
        *github.com/wailsapp/wails/v2*)
          ver="${line##* }"
          printf '%s' "$ver"
          return 0
          ;;
      esac
    done < "$f"
  fi
  printf '%s' "${WAILS_FALLBACK}"
}
WAILS_VER="$(read_wails_ver)"

version_ge() {
  local a="${1#go}" b="${2#go}"
  a="${a#v}"; b="${b#v}"
  a="${a%%-*}"; b="${b%%-*}"
  [[ "$(printf '%s\n%s\n' "$a" "$b" | sort -V | tail -n1)" == "$a" ]]
}

node_meets_min() {
  command -v node >/dev/null 2>&1 || return 1
  command -v npm >/dev/null 2>&1 || return 1
  local cur
  cur="$(node -v 2>/dev/null || true)"
  cur="${cur#v}"
  [[ -n "$cur" ]] || return 1
  version_ge "$cur" "$NODE_MIN"
}

disk_kb_free() {
  local path="$1" avail=""
  read -r _ _ _ avail _ <<<"$(df -Pk "$path" 2>/dev/null | tail -n 1)" || true
  printf '%s' "${avail:-0}"
}

# ---------------------------------------------------------------------------
# Package install
# ---------------------------------------------------------------------------
pkg_update_once() {
  case "$PKG_MGR" in
    apt)
      (( APT_UPDATED == 1 )) && return 0
      RETRY_DESC="apt-get update" retry as_root env DEBIAN_FRONTEND=noninteractive apt-get update -qq
      APT_UPDATED=1
      ;;
  esac
}

try_pkg_install() {
  (( $# == 0 )) && return 0
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install packages via $PKG_MGR: $*"
    return 0
  fi
  can_root || return 1
  pkg_update_once || true
  case "$PKG_MGR" in
    apt)
      RETRY_DESC="apt-get install $*" retry as_root env DEBIAN_FRONTEND=noninteractive \
        apt-get install -y -qq --no-install-recommends "$@"
      ;;
    dnf)
      RETRY_DESC="dnf install $*" retry as_root dnf install -y "$@"
      ;;
    yum)
      RETRY_DESC="yum install $*" retry as_root yum install -y "$@"
      ;;
    pacman)
      RETRY_DESC="pacman -S $*" retry as_root pacman -Sy --needed --noconfirm "$@"
      ;;
    zypper)
      RETRY_DESC="zypper install $*" retry as_root zypper --non-interactive --gpg-auto-import-keys install -y "$@"
      ;;
    apk)
      RETRY_DESC="apk add $*" retry as_root apk add --no-cache "$@"
      ;;
    brew)
      RETRY_DESC="brew install $*" retry env HOMEBREW_NO_AUTO_UPDATE=1 brew install "$@"
      ;;
    *)
      return 1
      ;;
  esac
}

pkg_install_go() {
  local go_pkgs=()
  case "$PKG_MGR" in
    apt|dnf|yum) go_pkgs=(golang) ;;
    pacman|zypper|apk|brew) go_pkgs=(go) ;;
  esac
  if ((${#go_pkgs[@]})); then try_pkg_install "${go_pkgs[@]}"; else return 1; fi
}

pre_go_download() {
  if ! have_curl && ! have_wget; then
    try_pkg_install curl ca-certificates || try_pkg_install wget ca-certificates || true
    hash -r 2>/dev/null || true
  fi
  if is_musl; then
    if can_root; then
      try_pkg_install gcompat || try_pkg_install libc6-compat || true
    else
      err "This is a musl/Alpine system. Official Go tarballs are glibc-linked, and there is no root to run apk. As root:"
      err "  sudo apk add go nodejs npm build-base pkgconf gtk+3.0-dev webkit2gtk-4.1-dev"
      return 1
    fi
  fi
}

ensure_go() { rum_ensure_go; }

print_gui_dep_cmd() {
  case "$PKG_MGR" in
    apt) echo "  sudo apt-get update && sudo DEBIAN_FRONTEND=noninteractive apt-get install -y golang nodejs npm build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev git" ;;
    dnf) echo "  sudo dnf install -y golang nodejs npm gcc pkg-config gtk3-devel webkit2gtk4.1-devel git" ;;
    yum) echo "  sudo yum install -y golang nodejs npm gcc pkgconfig gtk3-devel webkit2gtk3-devel git" ;;
    pacman) echo "  sudo pacman -S --needed go nodejs npm base-devel gtk3 webkit2gtk-4.1 git" ;;
    zypper) echo "  sudo zypper install -y go nodejs npm gcc pkg-config gtk3-devel webkitgtk3-devel git" ;;
    apk) echo "  sudo apk add go nodejs npm build-base pkgconf gtk+3.0-dev webkit2gtk-4.1-dev git" ;;
    brew) echo "  brew install go node gtk+3" ;;
    *) echo "  Install Go ${GO_MIN}+, Node.js ${NODE_MIN}+, gcc, pkg-config, GTK3 and WebKit2GTK (see INSTALL.md)." ;;
  esac
}

install_node_tarball() {
  local shasums="$WORK_DIR/node-SHASUMS256.txt"
  local base="https://nodejs.org/dist/latest-v22.x"
  info "Downloading Node.js LTS (v22) tarball for linux-${NODE_ARCH}"
  if ! download_file "${base}/SHASUMS256.txt" "$shasums"; then
    download_file "${DEFAULT_MIRROR%/}/node/SHASUMS256.txt" "$shasums" || die "Could not fetch Node.js checksums"
  fi
  local line filename sha
  line="$(grep -E "node-v[0-9.]+-linux-${NODE_ARCH}\\.tar\\.gz$" "$shasums" | head -1 || true)"
  [[ -n "$line" ]] || die "No linux-${NODE_ARCH} Node.js tarball listed in SHASUMS256.txt"
  sha="$(awk '{print $1}' <<<"$line")"
  filename="$(awk '{print $2}' <<<"$line")"
  local dest="$WORK_DIR/$filename"
  local urls=(
    "${base}/${filename}"
    "https://nodejs.org/dist/latest-v22.x/${filename}"
    "${DEFAULT_MIRROR%/}/${filename}"
  )
  local ok_dl=0 u
  for u in "${urls[@]}"; do
    if download_file "$u" "$dest" "$sha"; then
      ok_dl=1
      break
    fi
  done
  (( ok_dl == 1 )) || die "Could not download Node.js. Get LTS from https://nodejs.org and re-run."

  local extract="$WORK_DIR/node-extract"
  rm -rf "$extract"
  mkdir -p "$extract"
  tar -C "$extract" -xzf "$dest"
  local unpacked
  unpacked="$(find "$extract" -maxdepth 1 -mindepth 1 -type d | head -1)"
  [[ -x "$unpacked/bin/node" ]] || die "Node tarball did not contain bin/node"

  local prefix="$HOME/.local/node"
  info "Installing Node.js to $prefix (user-local)"
  mkdir -p "$HOME/.local"
  rm -rf "$prefix"
  mv "$unpacked" "$prefix"
  export PATH="$prefix/bin:$PATH"
  hash -r 2>/dev/null || true
  if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    die "Node installed to $prefix but node/npm not on PATH"
  fi
  ok "Installed node $(node --version) / npm $(npm --version)"
}

ensure_node() {
  if node_meets_min; then
    ok "Found node $(node --version 2>/dev/null) / npm $(npm --version 2>/dev/null)"
    return 0
  fi
  if command -v node >/dev/null 2>&1; then
    warn "Node.js is too old ($(node --version)); need >= ${NODE_MIN} (with npm)"
  else
    info "Node.js ${NODE_MIN}+ / npm is not installed"
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install Node.js ${NODE_MIN}+"
    return 0
  fi
  local node_pkgs=()
  case "$PKG_MGR" in
    apt) node_pkgs=(nodejs npm) ;;
    dnf|yum) node_pkgs=(nodejs npm) ;;
    pacman) node_pkgs=(nodejs npm) ;;
    zypper) node_pkgs=(nodejs npm) ;;
    apk) node_pkgs=(nodejs npm) ;;
    brew) node_pkgs=(node) ;;
  esac
  if ((${#node_pkgs[@]})) && try_pkg_install "${node_pkgs[@]}"; then
    hash -r 2>/dev/null || true
    if node_meets_min; then
      ok "Installed node $(node --version) / npm $(npm --version) via $PKG_MGR"
      return 0
    fi
    warn "Package-manager Node.js is too old or incomplete; using the official tarball"
  fi
  install_node_tarball
  node_meets_min || die "Node.js ${NODE_MIN}+ and npm are required. Install from https://nodejs.org and re-run."
}

ensure_build_deps() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would ensure gcc, pkg-config, git, GTK3, WebKit2GTK"
    return 0
  fi
  command -v git >/dev/null 2>&1 || try_pkg_install git || warn "git not installed (Go modules may still work)"
  case "$PKG_MGR" in
    apt) try_pkg_install build-essential pkg-config ca-certificates libgtk-3-dev || true
         try_pkg_install libwebkit2gtk-4.1-dev || try_pkg_install libwebkit2gtk-4.0-dev || true
         ;;
    dnf) try_pkg_install gcc pkg-config gtk3-devel || true
         try_pkg_install webkit2gtk4.1-devel || try_pkg_install webkit2gtk4.0-devel || try_pkg_install webkit2gtk3-devel || true
         ;;
    yum) try_pkg_install gcc pkgconfig gtk3-devel || true
         try_pkg_install webkit2gtk3-devel || try_pkg_install webkit2gtk4.1-devel || true
         ;;
    pacman) try_pkg_install base-devel gtk3 || true
            try_pkg_install webkit2gtk-4.1 || try_pkg_install webkit2gtk || true
            ;;
    zypper) try_pkg_install gcc pkg-config gtk3-devel || true
            try_pkg_install webkit2gtk3-devel || try_pkg_install webkitgtk3-devel \
              || try_pkg_install webkit2gtk4-devel || try_pkg_install libwebkit2gtk-4_1-devel || true
            ;;
    apk) try_pkg_install build-base pkgconf gtk+3.0-dev || true
         try_pkg_install webkit2gtk-4.1-dev || try_pkg_install webkit2gtk-dev || true
         ;;
    brew) try_pkg_install pkg-config gtk+3 || true ;;
  esac

  if ! command -v gcc >/dev/null 2>&1 && ! command -v cc >/dev/null 2>&1; then
    die "A C compiler (gcc) is required to build the GUI. Install it, then re-run:"$'\n'"$(print_gui_dep_cmd)"
  fi
  if ! command -v pkg-config >/dev/null 2>&1; then
    die "pkg-config is required to build the GUI. Install it, then re-run:"$'\n'"$(print_gui_dep_cmd)"
  fi
  local have_gtk=0 have_wk=0
  pkg-config --exists gtk+-3.0 2>/dev/null && have_gtk=1
  pkg-config --exists webkit2gtk-4.1 2>/dev/null && have_wk=1
  pkg-config --exists webkit2gtk-4.0 2>/dev/null && have_wk=1
  if [[ "$have_gtk" -eq 0 || "$have_wk" -eq 0 ]]; then
    die "GTK3 and WebKit2GTK (4.0 or 4.1) are required to build the GUI. Install them, then re-run:"$'\n'"$(print_gui_dep_cmd)"
  fi
  ok "Found C toolchain + GTK3 + WebKit2GTK"
}

run_go_net() {
  # Network-ish Go commands: retry, then fall back to the bundled mirror chain.
  local st=0
  set +e
  RETRY_DESC="go $*" retry run_logged go "$@"
  st=$?
  set -e
  if (( st == 0 )); then
    return 0
  fi
  rum_proxy_fallback || return "$st"
  warn "Go command failed; retrying with the fallback module proxies"
  RETRY_DESC="go $* (mirror)" retry run_logged go "$@"
}

ensure_wails() {
  PATH="${PATH}:$(go env GOPATH 2>/dev/null)/bin:${HOME}/go/bin:${HOME}/.local/bin"
  export PATH
  hash -r 2>/dev/null || true
  if command -v wails >/dev/null 2>&1; then
    ok "Found wails: $(wails version 2>/dev/null | head -1 || echo present)"
    return 0
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would go install wails@${WAILS_VER}"
    return 0
  fi
  info "Wails CLI not found — installing github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VER}"
  run_go_net install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VER}"
  PATH="${PATH}:$(go env GOPATH)/bin"
  export PATH
  hash -r 2>/dev/null || true
  command -v wails >/dev/null 2>&1 || die "wails still not on PATH; add \$(go env GOPATH)/bin to PATH and re-run."
  ok "Found wails: $(wails version 2>/dev/null | head -1 || echo present)"
}

wails_tags() {
  if command -v pkg-config >/dev/null 2>&1 \
     && pkg-config --exists webkit2gtk-4.1 2>/dev/null \
     && ! pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
    printf '%s' "webkit2_41"
  fi
}

_append_path_rc() {
  local dir="$1" rc="$2" line
  [[ -n "$rc" ]] || return 0
  if [[ "$(basename "${SHELL:-}")" == "fish" ]]; then
    line="fish_add_path $dir"
  else
    line="export PATH=\"$dir:\$PATH\""
  fi
  mkdir -p "$(dirname "$rc")"
  touch "$rc"
  grep -Fqs "$dir" "$rc" 2>/dev/null && return 0
  printf '\n# Added by Rum installer\n%s\n' "$line" >>"$rc"
  ok "Added $dir to PATH in $rc (open a new terminal to pick it up)"
}

ensure_path_in_rc() {
  local dir="$1"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would add $dir to PATH in ${SHELL_RC:-$HOME/.profile}"
    return 0
  fi
  _append_path_rc "$dir" "$SHELL_RC"
  if [[ "${SHELL_RC:-}" != "$HOME/.profile" ]]; then
    _append_path_rc "$dir" "$HOME/.profile"
  fi
}

atomic_install_file() {
  local src="$1" dest="$2" mode="${3:-0755}"
  local dest_dir
  dest_dir="$(dirname "$dest")"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install $src -> $dest"
    return 0
  fi
  mkdir -p "$dest_dir" 2>/dev/null || as_root mkdir -p "$dest_dir"
  local tmp
  tmp="$dest_dir/.$(basename "$dest").$$.tmp"
  if [[ -w "$dest_dir" ]]; then
    cp -f "$src" "$tmp"
    chmod "$mode" "$tmp"
    mv -f "$tmp" "$dest"
  else
    as_root install -m "$mode" "$src" "$dest" || {
      rm -f "$tmp"
      die "Cannot write $dest (need write permission or passwordless sudo)"
    }
  fi
  rm -f "$tmp" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Uninstall
# ---------------------------------------------------------------------------
if [[ "$UNINSTALL" -eq 1 ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove $TARGET $ICON_TARGET $DESKTOP_TARGET"
    exit 0
  fi
  rm -f "$TARGET" "$ICON_TARGET" "$DESKTOP_TARGET"
  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
  fi
  for kb in kbuildsycoca6 kbuildsycoca5; do
    if command -v "$kb" >/dev/null 2>&1; then "$kb" >/dev/null 2>&1 || true; break; fi
  done
  ok "Uninstalled $APP (binary, icon, menu entry)."
  info "Log: $LOG_FILE"
  exit 0
fi

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------
NEED_KB=2097152
HOME_FREE="$(disk_kb_free "$HOME" || echo 0)"
TMP_FREE="$(disk_kb_free "${TMPDIR:-/tmp}" || echo 0)"

info "Detected: ${OS_KERNEL}/${OS_ARCH} distro=${DISTRO_ID} pkg=${PKG_MGR} root=${IS_ROOT} passwordless-sudo=${HAS_SUDO_N} tty=$([[ -t 0 ]] && echo yes || echo no) assume-yes=${ASSUME_YES} shell-rc=${SHELL_RC}"
info "Go minimum (from go.mod): ${GO_MIN}  |  Wails: ${WAILS_VER}  |  Node minimum: ${NODE_MIN}"

PLAN=()
go_meets_min || PLAN+=("Go ${GO_MIN}+")
node_meets_min || PLAN+=("Node.js ${NODE_MIN}+ / npm")
command -v git >/dev/null 2>&1 || PLAN+=("git")
command -v gcc >/dev/null 2>&1 || command -v cc >/dev/null 2>&1 || PLAN+=("gcc")
command -v pkg-config >/dev/null 2>&1 || PLAN+=("pkg-config")
if command -v pkg-config >/dev/null 2>&1; then
  pkg-config --exists gtk+-3.0 2>/dev/null || PLAN+=("libgtk-3")
  if ! pkg-config --exists webkit2gtk-4.1 2>/dev/null && ! pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
    PLAN+=("webkit2gtk (4.0 or 4.1)")
  fi
else
  PLAN+=("GTK3 + WebKit2GTK")
fi
command -v wails >/dev/null 2>&1 || PLAN+=("wails CLI ${WAILS_VER}")

if ((${#PLAN[@]})); then
  info "Will install: ${PLAN[*]}"
else
  info "All build prerequisites already present"
fi

if [[ -e "$TARGET" ]]; then
  info "Existing install will be replaced: $TARGET"
else
  info "Install target: $TARGET"
fi
info "Log file: $LOG_FILE"

if [[ "${HOME_FREE:-0}" -gt 0 && "${HOME_FREE}" -lt "$NEED_KB" ]]; then
  warn "Low disk space on \$HOME ($(awk -v k="$HOME_FREE" 'BEGIN{printf "%.1f GiB", k/1024/1024}') free); the GUI build may need ~2 GiB"
fi
if [[ "${TMP_FREE:-0}" -gt 0 && "${TMP_FREE}" -lt 524288 ]]; then
  warn "Low disk space on /tmp"
fi

# Write permission for the prefix (or passwordless sudo).
if [[ ! -d "$PREFIX" ]]; then
  if [[ "$DRY_RUN" -eq 0 ]]; then
    mkdir -p "$BIN_DIR" "$ICON_DIR" "$DESKTOP_DIR" 2>/dev/null \
      || as_root mkdir -p "$BIN_DIR" "$ICON_DIR" "$DESKTOP_DIR" \
      || die "Cannot create $PREFIX — pick a writable --prefix (e.g. --prefix \"\$HOME/.local\")"
  fi
elif [[ ! -w "$PREFIX" ]] && ! can_root; then
  die "Cannot write to $PREFIX and no passwordless sudo. Re-run with --prefix \"\$HOME/.local\""
fi

rum_go_report
configure_goproxy

if [[ "$DRY_RUN" -eq 1 ]]; then
  ensure_go   # dry-run: reports local Go or the planned download, changes nothing
  info "dry-run complete (no changes). Re-run without --dry-run to install."
  exit 0
fi

# ---------------------------------------------------------------------------
# Install prerequisites + build
# ---------------------------------------------------------------------------
ensure_go
ensure_node
configure_npm
ensure_build_deps
ensure_wails

WAILS_TAGS="$(wails_tags)"
if [[ -n "$WAILS_TAGS" ]]; then
  ok "WebKit2GTK 4.1 only — building with -tags ${WAILS_TAGS}"
fi

# Frontend deps: npm ci, fall back to npm install. Wails will also run npm.
if [[ -d "$REPO_ROOT/frontend" ]]; then
  info "Installing frontend npm dependencies"
  (
    cd "$REPO_ROOT/frontend"
    set +e
    if [[ -f package-lock.json ]]; then
      RETRY_DESC="npm ci" retry run_logged npm ci --no-audit --no-fund --prefer-offline=false
      st=$?
      if (( st != 0 )); then
        warn "npm ci failed; falling back to npm install"
        RETRY_DESC="npm install" retry run_logged npm install --no-audit --no-fund
        st=$?
      fi
    else
      RETRY_DESC="npm install" retry run_logged npm install --no-audit --no-fund
      st=$?
    fi
    set -e
    exit "$st"
  ) || warn "npm install had errors; wails build will try again"
fi

# App icon (don't ship the Wails default if a Rum icon exists)
ensure_icons() {
  local appicon="$REPO_ROOT/build/appicon.png"
  [[ -f "$appicon" ]] && return 0
  local src="" s
  for s in "$REPO_ROOT/build/icon.png" "$REPO_ROOT/icon.png"; do
    [[ -f "$s" ]] && { src="$s"; break; }
  done
  [[ -z "$src" ]] && { info "No source icon found; the build will use the default Wails icon."; return 0; }
  mkdir -p "$REPO_ROOT/build/windows"
  if command -v magick >/dev/null 2>&1; then
    magick "$src" -resize 512x512 -background none -gravity center -extent 512x512 "$appicon"
    cp "$appicon" "$REPO_ROOT/build/icon.png"
    magick "$appicon" -background none -define icon:auto-resize=256,128,64,48,32,16 "$REPO_ROOT/build/windows/icon.ico" 2>/dev/null || true
    ok "Generated app icon from $src"
  else
    cp "$src" "$appicon"
    cp "$src" "$REPO_ROOT/build/icon.png"
    info "Wired $src as the app icon (install ImageMagick for a properly squared icon)."
  fi
}
ensure_icons

info "Building the Rum desktop app (wails build)…"
export GOTOOLCHAIN=local
(
  cd "$REPO_ROOT"
  set +e
  if [[ -n "$WAILS_TAGS" ]]; then
    run_logged wails build -clean -tags "$WAILS_TAGS"
  else
    run_logged wails build -clean
  fi
  st=$?
  set -e
  if (( st != 0 )) && rum_proxy_fallback; then
    warn "wails build failed; retrying with the fallback module proxies"
    if [[ -n "$WAILS_TAGS" ]]; then
      run_logged wails build -clean -tags "$WAILS_TAGS"
    else
      run_logged wails build -clean
    fi
  elif (( st != 0 )); then
    exit "$st"
  fi
)

BUILT_BIN="$REPO_ROOT/build/bin/$APP"
[[ -f "$BUILT_BIN" ]] || die "Expected build output $BUILT_BIN not found"
ok "Built $BUILT_BIN"

# Stage into WORK_DIR then atomically move into place so a failure cannot
# leave a half-written binary at $TARGET.
STAGE="$WORK_DIR/stage"
mkdir -p "$STAGE/bin" "$STAGE/share/icons/hicolor/512x512/apps" "$STAGE/share/applications"
cp -f "$BUILT_BIN" "$STAGE/bin/$APP"
chmod 0755 "$STAGE/bin/$APP"
for ico in "$REPO_ROOT/build/appicon.png" "$REPO_ROOT/build/icon.png" "$REPO_ROOT/icon.png"; do
  if [[ -f "$ico" ]]; then
    cp -f "$ico" "$STAGE/share/icons/hicolor/512x512/apps/$APP.png"
    chmod 0644 "$STAGE/share/icons/hicolor/512x512/apps/$APP.png"
    break
  fi
done
cat > "$STAGE/share/applications/$APP.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=$APP
Comment=Powerful, fast and modern download manager
Exec=$TARGET
Icon=$APP
Terminal=false
Categories=Network;FileTransfer;
StartupWMClass=$APP
EOF
chmod 0644 "$STAGE/share/applications/$APP.desktop"

mkdir -p "$BIN_DIR" "$ICON_DIR" "$DESKTOP_DIR"
atomic_install_file "$STAGE/bin/$APP" "$TARGET" 0755
ok "Installed binary to $TARGET"
if [[ -f "$STAGE/share/icons/hicolor/512x512/apps/$APP.png" ]]; then
  atomic_install_file "$STAGE/share/icons/hicolor/512x512/apps/$APP.png" "$ICON_TARGET" 0644
  ok "Installed icon"
fi
atomic_install_file "$STAGE/share/applications/$APP.desktop" "$DESKTOP_TARGET" 0644
ok "Installed menu entry to $DESKTOP_TARGET"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f "$PREFIX/share/icons/hicolor" 2>/dev/null || true
fi
for kb in kbuildsycoca6 kbuildsycoca5; do
  if command -v "$kb" >/dev/null 2>&1; then "$kb" >/dev/null 2>&1 || true; break; fi
done

ensure_path_in_rc "$BIN_DIR"
export PATH="$BIN_DIR:$PATH"

echo
ok "Done. Installed $APP to $TARGET"
rum_print_summary
info "Next: launch '$APP' from your application menu, or run: $TARGET"
if [[ "$IS_ROOT" -eq 0 ]]; then
  info "This shell only:  export PATH=\"$BIN_DIR:\$PATH\""
fi
info "Remove later:  $0 --uninstall"
info "Log: $LOG_FILE"
