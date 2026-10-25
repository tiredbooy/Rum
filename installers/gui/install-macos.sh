#!/usr/bin/env bash
#
# Rum GUI (desktop) installer — macOS
#
# Builds the Wails desktop app and installs Rum.app into /Applications
# (falls back to ~/Applications when /Applications is not writable).
#
# NOTE: this build is UNSIGNED. After installing, Gatekeeper may refuse to open
# it the first time; the script clears the quarantine flag when it can.
#
# Usage:
#   ./installers/gui/install-macos.sh [options]
#
#   --prefix DIR   Install Rum.app into DIR (default: /Applications, else ~/Applications).
#   --yes, -y      Non-interactive (implied when stdin is not a TTY).
#   --mirror[=URL] Use a Go module proxy (default URL if none given).
#   --uninstall    Remove the installed Rum.app.
#   --verbose, -v  Stream full command output to the console.
#   --dry-run      Detect, print the plan, and exit without changing the system.
#   -h, --help     Show this help.
#
# Go: a Go that is already installed (PATH, /opt/homebrew, /usr/local/go, ~/sdk, ...)
# on go.mod's minor line is used as-is with GOTOOLCHAIN=local; nothing is
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
APP_BUNDLE="$APP.app"
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
WAILS_FALLBACK="v2.12.0"
PREFIX="${PREFIX:-}"

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
    -h|--help) awk 'NR==1{next} /^#/{print; next} {exit}' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -t 0 ]]; then
  ASSUME_YES=1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

WORK_DIR=""
LOG_FILE=""

init_log() {
  local state_dir
  state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/rum"
  if mkdir -p "$state_dir" 2>/dev/null && [[ -w "$state_dir" ]]; then
    :
  else
    state_dir="${TMPDIR:-/tmp}"
  fi
  LOG_FILE="${state_dir}/gui-install-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="${TMPDIR:-/tmp}/rum-gui-install-$$.log"; : >"$LOG_FILE"; }
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
ok()   { printf '%s=>%s %s\n' "$C_GR" "$C_RS" "$*"; log "OK    $*"; }
warn() { printf '%s! %s%s\n' "$C_YE" "$*" "$C_RS"; log "WARN  $*"; }
err()  { printf '%sx %s%s\n' "$C_RD" "$*" "$C_RS" >&2; log "ERROR $*"; }
die()  { err "$*"; [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"; exit 1; }

cleanup() {
  local st=$?
  if [[ -n "${WORK_DIR:-}" && -d "${WORK_DIR:-}" ]]; then
    rm -rf "$WORK_DIR" || true
  fi
  trap - EXIT
  exit "$st"
}

on_err() { err "Failed at line ${1:-?}."; [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"; }
on_int()  { err "Interrupted"; exit 130; }
on_term() { err "Terminated"; exit 143; }

trap cleanup EXIT
trap 'on_err $LINENO' ERR
trap on_int INT
trap on_term TERM

init_log
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/rum-gui-XXXXXX")"

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
    if (( st == 0 )); then
      return 0
    fi
    if (( attempt == RETRY_MAX )); then
      err "${desc} failed after ${RETRY_MAX} attempts (last exit ${st})"
      err "Re-run this script; completed steps are skipped. Log: $LOG_FILE"
      return "$st"
    fi
    warn "${desc} failed (attempt ${attempt}/${RETRY_MAX}, exit ${st}); retrying…"
    backoff_sleep "$attempt"
    ((++attempt))
  done
  return "$st"
}

run_logged() {
  log "\$ $(redact "$*")"
  local st=0
  if [[ "$VERBOSE" -eq 1 ]]; then
    set +e
    set +o pipefail
    "$@" 2>&1 | tee -a "$LOG_FILE"
    st=${PIPESTATUS[0]:-1}
    set -e
    set -o pipefail
  else
    set +e
    "$@" >>"$LOG_FILE" 2>&1
    st=$?
    set -e
  fi
  return "$st"
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
    die "Need curl or wget to download $url"
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
  if command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$file" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v openssl >/dev/null 2>&1; then
    got="$(openssl dgst -sha256 "$file" | awk '{print $NF}')"
  else
    warn "No sha256 tool found; skipping checksum verification for $(basename "$file")"
    return 0
  fi
  local el="${expect}" gl="${got}"
  # macOS bash 3.2 has no ${var,,}; compare case-insensitively.
  [[ "$(printf '%s' "$gl" | tr 'A-F' 'a-f')" == "$(printf '%s' "$el" | tr 'A-F' 'a-f')" ]]
}

OS_ARCH_RAW="$(uname -m 2>/dev/null || echo unknown)"
case "$OS_ARCH_RAW" in
  x86_64|amd64) OS_ARCH="amd64"; NODE_ARCH="x64" ;;
  arm64|aarch64) OS_ARCH="arm64"; NODE_ARCH="arm64" ;;
  *) OS_ARCH="$OS_ARCH_RAW"; NODE_ARCH="$OS_ARCH_RAW" ;;
esac

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

choose_install_dir() {
  if [[ -n "$PREFIX" ]]; then
    printf '%s' "$PREFIX"
    return
  fi
  if [[ -d /Applications && -w /Applications ]]; then
    printf '%s' "/Applications"
    return
  fi
  if can_root; then
    printf '%s' "/Applications"
    return
  fi
  mkdir -p "$HOME/Applications" 2>/dev/null || true
  printf '%s' "$HOME/Applications"
}

INSTALL_DIR="$(choose_install_dir)"
TARGET="$INSTALL_DIR/$APP_BUNDLE"

as_install() {
  if [[ -w "$INSTALL_DIR" ]]; then
    "$@"
  else
    as_root "$@" || die "Cannot write $INSTALL_DIR (need write permission or passwordless sudo). Use --prefix \"\$HOME/Applications\"."
  fi
}

detect_shell_rc() {
  local shell_name rc
  shell_name="$(basename "${SHELL:-zsh}")"
  case "$shell_name" in
    zsh)  rc="$HOME/.zshrc" ;;
    bash)
      if [[ -f "$HOME/.bash_profile" ]]; then rc="$HOME/.bash_profile"
      elif [[ -f "$HOME/.bashrc" ]]; then rc="$HOME/.bashrc"
      else rc="$HOME/.profile"
      fi
      ;;
    fish) rc="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
    *)    rc="$HOME/.zprofile" ;;
  esac
  printf '%s' "$rc"
}
SHELL_RC="$(detect_shell_rc)"

RUM_GO_OS=darwin
RUM_GO_SAME_MINOR=1
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
  local f="$REPO_ROOT/go.mod" v=""
  [[ -f "$f" ]] && v="$(awk '/github.com\/wailsapp\/wails\/v2[[:space:]]/ {print $2; exit}' "$f")"
  printf '%s' "${v:-$WAILS_FALLBACK}"
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

try_brew_install() {
  command -v brew >/dev/null 2>&1 || return 1
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would brew install $*"
    return 0
  fi
  RETRY_DESC="brew install $*" retry env HOMEBREW_NO_AUTO_UPDATE=1 brew install "$@"
}

install_node_tarball() {
  local shasums="$WORK_DIR/node-SHASUMS256.txt"
  local base="https://nodejs.org/dist/latest-v22.x"
  info "Downloading Node.js LTS (v22) tarball for darwin-${NODE_ARCH}"
  download_file "${base}/SHASUMS256.txt" "$shasums" || die "Could not fetch Node.js checksums"
  local line filename sha
  line="$(grep -E "node-v[0-9.]+-darwin-${NODE_ARCH}\\.tar\\.gz$" "$shasums" | head -1 || true)"
  [[ -n "$line" ]] || die "No darwin-${NODE_ARCH} Node.js tarball listed in SHASUMS256.txt"
  sha="$(awk '{print $1}' <<<"$line")"
  filename="$(awk '{print $2}' <<<"$line")"
  local dest="$WORK_DIR/$filename"
  local urls=(
    "${base}/${filename}"
    "${DEFAULT_MIRROR%/}/${filename}"
  )
  local ok_dl=0 u
  for u in "${urls[@]}"; do
    if download_file "$u" "$dest" "$sha"; then ok_dl=1; break; fi
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
  mkdir -p "$HOME/.local"
  rm -rf "$prefix"
  mv "$unpacked" "$prefix"
  export PATH="$prefix/bin:$PATH"
  hash -r 2>/dev/null || true
  ok "Installed node $(node --version) / npm $(npm --version)"
}

pkg_install_go() { try_brew_install go; }

ensure_go() { rum_ensure_go; }

ensure_xcode_clt() {
  if xcode-select -p >/dev/null 2>&1; then
    ok "Found Xcode Command Line Tools"
    return 0
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would require Xcode Command Line Tools"
    return 0
  fi
  warn "Xcode Command Line Tools are missing; attempting xcode-select --install"
  xcode-select --install >/dev/null 2>&1 || true
  die "Xcode Command Line Tools are required. Finish the installer dialog, then re-run this script."$'\n'"  xcode-select --install"
}

ensure_node() {
  if node_meets_min; then
    ok "Found node $(node --version 2>/dev/null) / npm $(npm --version 2>/dev/null)"
    return 0
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install Node.js ${NODE_MIN}+"
    return 0
  fi
  if try_brew_install node && node_meets_min; then
    hash -r 2>/dev/null || true
    ok "Installed node $(node --version) / npm $(npm --version) via Homebrew"
    return 0
  fi
  install_node_tarball
  node_meets_min || die "Node.js ${NODE_MIN}+ and npm are required. brew install node   or download https://nodejs.org"
}

run_go_net() {
  local st=0
  set +e
  RETRY_DESC="go $*" retry run_logged go "$@"
  st=$?
  set -e
  if (( st == 0 )); then return 0; fi
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

if [[ "$UNINSTALL" -eq 1 ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove $TARGET (and ~/Applications/$APP_BUNDLE if present)"
    exit 0
  fi
  for cand in "$TARGET" "/Applications/$APP_BUNDLE" "$HOME/Applications/$APP_BUNDLE"; do
    if [[ -d "$cand" ]]; then
      info "Removing $cand"
      if [[ -w "$(dirname "$cand")" ]]; then
        rm -rf "$cand"
      else
        as_root rm -rf "$cand"
      fi
      ok "Removed $cand"
    fi
  done
  info "Log: $LOG_FILE"
  exit 0
fi

info "Detected: macOS/${OS_ARCH} brew=$(command -v brew >/dev/null && echo yes || echo no) root=$( ((EUID==0)) && echo yes || echo no ) passwordless-sudo=$(can_root && ((EUID!=0)) && echo yes || echo no) tty=$([[ -t 0 ]] && echo yes || echo no) assume-yes=${ASSUME_YES} shell-rc=${SHELL_RC}"
info "Go minimum (from go.mod): ${GO_MIN}  |  Wails: ${WAILS_VER}  |  install dir: ${INSTALL_DIR}"

PLAN=()
go_meets_min || PLAN+=("Go ${GO_MIN}+")
node_meets_min || PLAN+=("Node.js ${NODE_MIN}+ / npm")
xcode-select -p >/dev/null 2>&1 || PLAN+=("Xcode Command Line Tools")
command -v wails >/dev/null 2>&1 || PLAN+=("wails CLI ${WAILS_VER}")
if ((${#PLAN[@]})); then
  info "Will install: ${PLAN[*]}"
else
  info "All build prerequisites already present"
fi
if [[ -d "$TARGET" ]]; then
  info "Existing install will be replaced: $TARGET"
else
  info "Install target: $TARGET"
fi
info "Log file: $LOG_FILE"

rum_preflight_space "$HOME" 2097152
rum_preflight_space "${TMPDIR:-/tmp}" 524288
rum_go_report
configure_goproxy

if [[ "$DRY_RUN" -eq 1 ]]; then
  ensure_go   # dry-run: reports local Go or the planned download, changes nothing
  info "dry-run complete (no changes). Re-run without --dry-run to install."
  exit 0
fi

ensure_xcode_clt
ensure_go
ensure_node
configure_npm
ensure_wails

if [[ -d "$REPO_ROOT/frontend" ]]; then
  info "Installing frontend npm dependencies"
  (
    cd "$REPO_ROOT/frontend"
    set +e
    if [[ -f package-lock.json ]]; then
      RETRY_DESC="npm ci" retry run_logged npm ci --no-audit --no-fund
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

info "Building the Rum desktop app (wails build -platform darwin/universal)…"
export GOTOOLCHAIN=local
(
  cd "$REPO_ROOT"
  set +e
  run_logged wails build -clean -platform darwin/universal
  st=$?
  set -e
  if (( st != 0 )); then
    warn "Universal build failed; retrying native darwin/${OS_ARCH}"
    rum_proxy_fallback || true
    set +e
    run_logged wails build -clean -platform "darwin/${OS_ARCH}"
    st=$?
    set -e
    (( st == 0 )) || exit "$st"
  fi
)

BUILT_APP="$REPO_ROOT/build/bin/$APP_BUNDLE"
[[ -d "$BUILT_APP" ]] || die "Expected build output $BUILT_APP not found"
ok "Built $BUILT_APP"

info "Installing $APP_BUNDLE into $INSTALL_DIR"
mkdir -p "$INSTALL_DIR" 2>/dev/null || as_install mkdir -p "$INSTALL_DIR"
STAGE="$WORK_DIR/$APP_BUNDLE"
rm -rf "$STAGE"
cp -R "$BUILT_APP" "$STAGE"
as_install rm -rf "$TARGET"
as_install mv "$STAGE" "$TARGET"
ok "Installed $TARGET"

as_install xattr -dr com.apple.quarantine "$TARGET" 2>/dev/null || \
  info "Could not clear quarantine flag; if Rum won't open, run: xattr -dr com.apple.quarantine \"$TARGET\""

echo
ok "Done. Installed $APP_BUNDLE to $TARGET"
rum_print_summary
info "Next: launch Rum from Launchpad / Applications, or run: open \"$TARGET\""
info "Remove later:  $0 --uninstall"
info "This app is UNSIGNED. If macOS refuses to open it, run once:"
echo "  xattr -dr com.apple.quarantine \"$TARGET\""
info "Log: $LOG_FILE"
