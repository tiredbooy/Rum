#!/usr/bin/env bash
#
# Rum CLI installer — macOS
#
# Builds the `rum` command-line download manager from source and installs it
# into /usr/local/bin (or ~/.local/bin if that isn't writable). Go is
# auto-detected and installed when missing or too old.
#
# Usage:
#   ./installers/cli/install-macos.sh [options]
#
#   --prefix DIR   Install into DIR/bin (default: /usr/local, falling back to
#                  ~/.local). Honors $PREFIX too.
#   --yes, -y      Non-interactive (implied when stdin is not a TTY).
#   --mirror[=URL] Use a Go module proxy (default URL if none given).
#   --uninstall    Remove an installed `rum` binary instead of installing.
#   --verbose, -v  Stream full command output to the console.
#   --dry-run      Detect, print the plan, and exit without changing the system.
#   -h, --help     Show this help.
#
# Go: a Go that is already installed (PATH, /opt/homebrew, /usr/local/go, ~/sdk, ...)
# and meets go.mod is used as-is with GOTOOLCHAIN=local; nothing is downloaded.
# Only if none fits is Go fetched (go.dev, dl.google.com, then non-Google
# mirrors, sha256-verified).
#   RUM_GO_ROOT=DIR          use the Go in DIR (a GOROOT)
#   RUM_GO_MIRROR=URL        try URL/go<ver>.<os>-<arch>.tar.gz first
#   RUM_GO_NO_DOWNLOAD=1     never download Go (fail with instructions)
#   GOPROXY / GOSUMDB        respected; a fallback chain is used only if the
#                            default proxy is unreachable and you set neither
#

# shellcheck disable=SC2034  # shared constants are read by installers/lib/rum-go.sh
set -Eeuo pipefail

APP="rum"
ASSUME_YES=0
UNINSTALL=0
VERBOSE=0
DRY_RUN=0
PREFIX="${PREFIX:-}"
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"
IRANIAN_GOPROXY="https://mirror-go.runflare.com|https://package-mirror.liara.ir/repository/go|https://mirror.abrha.net/repository/go|${DEFAULT_MIRROR%/}"
RETRY_MAX="${RUM_RETRY_MAX:-5}"
GO_DL_BASE="${RUM_GO_DL_BASE:-https://go.dev/dl}"
GO_GOOGLE_DL="${RUM_GO_GOOGLE_DL:-https://dl.google.com/go}"
GO_FALLBACK_DL="${RUM_GO_FALLBACK_DL:-$DEFAULT_MIRROR}"

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
BACKEND_DIR="$REPO_ROOT/backend"

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
  LOG_FILE="${state_dir}/cli-install-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="${TMPDIR:-/tmp}/rum-cli-install-$$.log"; : >"$LOG_FILE"; }
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
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/rum-cli-XXXXXX")"

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
  [[ "$(printf '%s' "$got" | tr 'A-F' 'a-f')" == "$(printf '%s' "$expect" | tr 'A-F' 'a-f')" ]]
}

OS_ARCH_RAW="$(uname -m 2>/dev/null || echo unknown)"
case "$OS_ARCH_RAW" in
  x86_64|amd64) OS_ARCH="amd64" ;;
  arm64|aarch64) OS_ARCH="arm64" ;;
  *) OS_ARCH="$OS_ARCH_RAW" ;;
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

choose_bindir() {
  if [[ -n "$PREFIX" ]]; then echo "$PREFIX/bin"; return; fi
  if [[ -w /usr/local/bin ]]; then echo "/usr/local/bin"; return; fi
  if can_root; then echo "/usr/local/bin"; return; fi
  echo "$HOME/.local/bin"
}
BIN_DIR="$(choose_bindir)"
TARGET="$BIN_DIR/$APP"

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
if [[ ! -r "$REPO_ROOT/installers/lib/rum-go.sh" ]]; then
  printf 'Missing %s\nRun this script from a full checkout of the repository.\n' "$REPO_ROOT/installers/lib/rum-go.sh" >&2
  exit 1
fi
# shellcheck source=../lib/rum-go.sh
. "$REPO_ROOT/installers/lib/rum-go.sh"
GO_MIN="$(rum_read_go_min "$BACKEND_DIR/go.mod" "$REPO_ROOT/go.mod")"
# Prefer a Go that is already installed; nothing is downloaded unless none fits go.mod.
rum_select_local_go || true

try_brew_install() {
  command -v brew >/dev/null 2>&1 || return 1
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would brew install $*"
    return 0
  fi
  RETRY_DESC="brew install $*" retry env HOMEBREW_NO_AUTO_UPDATE=1 brew install "$@"
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
  die "Xcode Command Line Tools are required. Finish the installer dialog, then re-run:"$'\n'"  xcode-select --install"
}

ensure_path_in_rc() {
  local dir="$1"
  case ":$PATH:" in
    *":$dir:"*) return 0 ;;
  esac
  local rc="$SHELL_RC"
  [[ -n "$rc" ]] || return 0
  local line
  if [[ "$(basename "${SHELL:-}")" == "fish" ]]; then
    line="fish_add_path $dir"
  else
    line="export PATH=\"$dir:\$PATH\""
  fi
  if [[ -f "$rc" ]] && grep -Fqs "$dir" "$rc" 2>/dev/null; then
    return 0
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would add $dir to PATH in $rc"
    return 0
  fi
  mkdir -p "$(dirname "$rc")"
  touch "$rc"
  printf '\n# Added by Rum installer\n%s\n' "$line" >>"$rc"
  ok "Added $dir to PATH in $rc (open a new terminal to pick it up)"
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
      die "Cannot write $dest. Re-run with --prefix \"\$HOME/.local\""
    }
  fi
  rm -f "$tmp" 2>/dev/null || true
}

if [[ "$UNINSTALL" -eq 1 ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove $TARGET /usr/local/bin/$APP $HOME/.local/bin/$APP $HOME/bin/$APP"
    exit 0
  fi
  for cand in "$TARGET" "/usr/local/bin/$APP" "$HOME/.local/bin/$APP" "$HOME/bin/$APP"; do
    if [[ -e "$cand" ]]; then
      info "Removing $cand"
      if [[ -w "$(dirname "$cand")" ]]; then rm -f "$cand"; else as_root rm -f "$cand"; fi
      ok "Removed $cand"
    fi
  done
  info "Log: $LOG_FILE"
  exit 0
fi

[[ -d "$BACKEND_DIR/cmd/rum" ]] || die "Cannot find $BACKEND_DIR/cmd/rum — run this from a clean checkout."

info "Detected: macOS/${OS_ARCH} brew=$(command -v brew >/dev/null && echo yes || echo no) passwordless-sudo=$(can_root && ((EUID!=0)) && echo yes || echo no) tty=$([[ -t 0 ]] && echo yes || echo no) assume-yes=${ASSUME_YES} shell-rc=${SHELL_RC}"
info "Go minimum (from go.mod): ${GO_MIN}  |  install target: ${TARGET}"

PLAN=()
go_meets_min || PLAN+=("Go ${GO_MIN}+")
xcode-select -p >/dev/null 2>&1 || PLAN+=("Xcode Command Line Tools")
if ((${#PLAN[@]})); then info "Will install: ${PLAN[*]}"; else info "All build prerequisites already present"; fi
[[ -e "$TARGET" ]] && info "Existing install will be replaced: $TARGET"
info "Log file: $LOG_FILE"

rum_preflight_space "$HOME" 524288
rum_preflight_space "${TMPDIR:-/tmp}" 262144
rum_preflight_writable "$BIN_DIR" "Re-run with --prefix \"\$HOME/.local\""
rum_go_report
configure_goproxy

if [[ "$DRY_RUN" -eq 1 ]]; then
  ensure_go   # dry-run: reports local Go or the planned download, changes nothing
  info "dry-run complete (no changes). Re-run without --dry-run to install."
  exit 0
fi

ensure_xcode_clt
ensure_go

info "Building $APP (this may take a moment)…"
BUILD_OUT="$WORK_DIR/$APP"
if ! RETRY_DESC="go mod download" retry run_logged bash -c "cd \"$BACKEND_DIR\" && go mod download"; then
  if rum_proxy_fallback; then
    warn "go mod download failed; retrying with the fallback module proxies"
    RETRY_DESC="go mod download (mirror)" retry run_logged bash -c "cd \"$BACKEND_DIR\" && go mod download" \
      || die "go mod download failed. Log: $LOG_FILE"
  else
    die "go mod download failed. Log: $LOG_FILE"
  fi
fi

if ! run_logged bash -c "cd \"$BACKEND_DIR\" && go build -buildvcs=false -trimpath -ldflags \"-s -w\" -o \"$BUILD_OUT\" ./cmd/rum"; then
  if rum_proxy_fallback; then
    warn "go build failed; retrying with the fallback module proxies"
    run_logged bash -c "cd \"$BACKEND_DIR\" && go build -buildvcs=false -trimpath -ldflags \"-s -w\" -o \"$BUILD_OUT\" ./cmd/rum" \
      || die "go build failed. Log: $LOG_FILE"
  else
    die "go build failed. Log: $LOG_FILE"
  fi
fi
[[ -f "$BUILD_OUT" ]] || die "Build produced no executable"
ok "Built $APP"

atomic_install_file "$BUILD_OUT" "$TARGET" 0755
ok "Installed to $TARGET"

ensure_path_in_rc "$BIN_DIR"
export PATH="$BIN_DIR:$PATH"

echo
ok "Done. Installed $APP to $TARGET"
rum_print_summary
info "Next: open a new terminal, then run:  $APP --version  &&  $APP --help"
info "Remove later:  $0 --uninstall"
info "Log: $LOG_FILE"
