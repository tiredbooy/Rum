#!/usr/bin/env bash
#
# Quick rebuild of the `rum` CLI from backend/cmd/rum.
#
# Builds into a temp dir and atomically installs to PREFIX/bin (default:
# ~/.local/bin). Safe to re-run. Use the full installer for first-time
# machines that don't have Go yet:
#   ../../install.sh   or   ../../../installers/cli/install-linux.sh
#
# Usage:
#   ./backend/cmd/rum/build.sh [--prefix DIR] [--yes] [--mirror[=URL]] [--verbose] [--dry-run]
#
set -Eeuo pipefail

APP="rum"
PREFIX="${PREFIX:-$HOME/.local}"
ASSUME_YES=0
VERBOSE=0
DRY_RUN=0
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"
IRANIAN_GOPROXY="https://mirror-go.runflare.com|https://package-mirror.liara.ir/repository/go|https://mirror.abrha.net/repository/go|${DEFAULT_MIRROR%/}"
BUILTIN_GOPROXY="${IRANIAN_GOPROXY}|https://proxy.golang.org,direct"
GO_SUMDB_DEFAULT="${RUM_GO_SUMDB:-sum.golang.org}"
RETRY_MAX="${RUM_RETRY_MAX:-5}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a directory}"; shift 2 ;;
    --prefix=*) PREFIX="${1#*=}"; shift ;;
    --yes|-y) ASSUME_YES=1; shift ;;
    --mirror) MIRROR="$DEFAULT_MIRROR"; shift ;;
    --mirror=*) MIRROR="${1#*=}"; shift ;;
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
BACKEND_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN_DIR="$PREFIX/bin"
TARGET="$BIN_DIR/$APP"

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
  LOG_FILE="${state_dir}/cli-build-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="${TMPDIR:-/tmp}/rum-cli-build-$$.log"; : >"$LOG_FILE"; }
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

cleanup() {
  local st=$?
  if [[ -n "${WORK_DIR:-}" && -d "${WORK_DIR:-}" ]]; then
    rm -rf "$WORK_DIR"
  fi
  return "$st"
}
on_err() { err "Failed at line ${1:-?}."; [[ -n "$LOG_FILE" ]] && err "Full log: $LOG_FILE"; }

trap cleanup EXIT
trap 'on_err $LINENO' ERR
trap 'err "Interrupted"; exit 130' INT
trap 'err "Terminated"; exit 143' TERM

init_log
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/rum-build-XXXXXX")"

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

[[ -d "$BACKEND_DIR/cmd/rum" ]] || die "Cannot find $BACKEND_DIR/cmd/rum"

if [[ -n "$MIRROR" ]]; then
  if [[ "$MIRROR" == "$DEFAULT_MIRROR" ]]; then
    export GOPROXY="$BUILTIN_GOPROXY"
  else
    export GOPROXY="${MIRROR}|${BUILTIN_GOPROXY}"
  fi
  export GOSUMDB="$GO_SUMDB_DEFAULT"
  ok "Using Go module proxy chain: $GOPROXY"
fi

if ! command -v go >/dev/null 2>&1; then
  die "Go is not installed. Run $BACKEND_DIR/install.sh (it auto-installs Go) or see INSTALL.md."
fi
ok "Found $(go version)"
info "Install target: $TARGET  assume-yes=${ASSUME_YES}"
info "Log: $LOG_FILE"

if [[ "$DRY_RUN" -eq 1 ]]; then
  info "dry-run: would build ./cmd/rum and atomically install to $TARGET"
  exit 0
fi

BUILD_OUT="$WORK_DIR/$APP"
info "Building $APP…"
set +e
RETRY_DESC="go mod download" retry run_logged bash -c "cd \"$BACKEND_DIR\" && go mod download"
mod_st=$?
set -e
if (( mod_st != 0 )) && [[ "${GOPROXY:-}" != *"$IRANIAN_GOPROXY"* ]]; then
  warn "go mod download failed; retrying with bundled Iranian module mirrors"
  export GOPROXY="$BUILTIN_GOPROXY" GOSUMDB="$GO_SUMDB_DEFAULT"
  RETRY_DESC="go mod download (mirror)" retry run_logged bash -c "cd \"$BACKEND_DIR\" && go mod download"
fi

set +e
run_logged bash -c "cd \"$BACKEND_DIR\" && go build -buildvcs=false -trimpath -ldflags \"-s -w\" -o \"$BUILD_OUT\" ./cmd/rum"
st=$?
set -e
if (( st != 0 )); then
  if [[ "${GOPROXY:-}" != *"$IRANIAN_GOPROXY"* ]]; then
    warn "go build failed; retrying with bundled Iranian module mirrors"
    export GOPROXY="$BUILTIN_GOPROXY" GOSUMDB="$GO_SUMDB_DEFAULT"
    run_logged bash -c "cd \"$BACKEND_DIR\" && go build -buildvcs=false -trimpath -ldflags \"-s -w\" -o \"$BUILD_OUT\" ./cmd/rum"
  else
    die "go build failed. Log: $LOG_FILE"
  fi
fi
[[ -f "$BUILD_OUT" ]] || die "Build produced no executable"

mkdir -p "$BIN_DIR"
tmp="$BIN_DIR/.${APP}.$$.tmp"
cp -f "$BUILD_OUT" "$tmp"
chmod 0755 "$tmp"
mv -f "$tmp" "$TARGET"
ok "Installed $TARGET"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    info "$BIN_DIR is not on PATH. Add it with:"
    echo "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.bashrc   # or ~/.zshrc"
    ;;
esac

ok "Done. Try:  $APP --help"
info "Log: $LOG_FILE"
