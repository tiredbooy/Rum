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
BUILTIN_GOPROXY="${IRANIAN_GOPROXY}|https://proxy.golang.org,direct"
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

info() { printf '\033[0;36m==>\033[0m %s\n' "$*"; log "INFO  $*"; }
ok()   { printf '\033[0;32m=>\033[0m %s\n' "$*"; log "OK    $*"; }
warn() { printf '\033[0;33m! %s\033[0m\n' "$*"; log "WARN  $*"; }
err()  { printf '\033[0;31mx %s\033[0m\n' "$*" >&2; log "ERROR $*"; }
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

read_go_min() {
  local f="$BACKEND_DIR/go.mod" v=""
  [[ -f "$f" ]] || f="$REPO_ROOT/go.mod"
  [[ -f "$f" ]] && v="$(awk '/^go[[:space:]]/ {print $2; exit}' "$f")"
  printf '%s' "${v:-1.25.7}"
}
GO_MIN="$(read_go_min)"

version_ge() {
  local a="${1#go}" b="${2#go}"
  a="${a#v}"; b="${b#v}"
  a="${a%%-*}"; b="${b%%-*}"
  [[ "$(printf '%s\n%s\n' "$a" "$b" | sort -V | tail -n1)" == "$a" ]]
}

go_meets_min() {
  command -v go >/dev/null 2>&1 || return 1
  local cur
  cur="$(go env GOVERSION 2>/dev/null || true)"
  [[ -n "$cur" ]] || cur="$(go version 2>/dev/null | awk '{print $3}')"
  cur="${cur#go}"
  [[ -n "$cur" ]] || return 1
  version_ge "$cur" "$GO_MIN"
}

try_brew_install() {
  command -v brew >/dev/null 2>&1 || return 1
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would brew install $*"
    return 0
  fi
  RETRY_DESC="brew install $*" retry env HOMEBREW_NO_AUTO_UPDATE=1 brew install "$@"
}

fetch_go_sha256() {
  local filename="$1" json="$WORK_DIR/go-dl.json" sha=""
  download_file "${GO_DL_BASE%/}/?mode=json&include=all" "$json" || return 1
  if command -v python3 >/dev/null 2>&1; then
    sha="$(python3 - "$filename" "$json" <<'PY'
import json, sys
fn, path = sys.argv[1], sys.argv[2]
data = json.load(open(path, encoding="utf-8"))
for rel in data:
    for f in rel.get("files", []):
        if f.get("filename") == fn and f.get("sha256"):
            print(f["sha256"])
            sys.exit(0)
sys.exit(1)
PY
)" || true
  fi
  if [[ -z "$sha" ]]; then
    sha="$(grep -o "\"filename\":\"${filename}\"[^}]*\"sha256\":\"[a-f0-9]*\"" "$json" 2>/dev/null \
      | grep -o '"sha256":"[a-f0-9]*"' | head -1 | cut -d'"' -f4 || true)"
  fi
  [[ -n "$sha" ]] && printf '%s' "$sha"
}

# GOSUMDB remains enabled for both the mirror chain and toolchain fetches.
# The checksum database is the integrity check for modules fetched from any
# third-party proxy.
GO_SUMDB_DEFAULT="${RUM_GO_SUMDB:-sum.golang.org}"

# Holds the GOROOT of a toolchain fetched by go_toolchain_root (set by it).
GO_TOOLCHAIN_ROOT=""

# go_toolchain_root VER — resolve the requested Go toolchain through the module
# proxy, downloading it if necessary, and set GO_TOOLCHAIN_ROOT to its GOROOT.
#
# Toolchains are published as ordinary modules
# (golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>), so this reaches the Go
# module proxy rather than the tarball hosts — a different network path, which
# is the point of having it as a last resort.
#
# We let the `go` command do the download rather than fetching the zip
# ourselves: it verifies the module against the checksum database, whereas a
# raw download would be unverifiable (the sha256 published on go.dev is for the
# .tar.gz, a different artefact, and cannot validate the module zip).
go_toolchain_root() {
  local ver="$1" root=""
  GO_TOOLCHAIN_ROOT=""
  command -v go >/dev/null 2>&1 || return 1

  # GOFLAGS/GOPRIVATE are cleared so a caller's settings cannot redirect this,
  # and GOSUMDB is forced on so the module is verified even when a mirror path
  # has disabled it.
  root="$(GOTOOLCHAIN="go${ver}" GOSUMDB="$GO_SUMDB_DEFAULT" GOPRIVATE= GONOSUMDB= GOFLAGS= \
    go env GOROOT 2>>"$LOG_FILE")" || return 1
  [[ -n "$root" && -x "$root/bin/go" ]] || return 1

  # A go older than 1.21 ignores GOTOOLCHAIN and would hand back its own GOROOT.
  # Confirm we really got the version we asked for before trusting it.
  "$root/bin/go" version 2>/dev/null | grep -q "go${ver} " || return 1

  GO_TOOLCHAIN_ROOT="$root"
}

# install_go_from_module_proxy VER EXTRACTDIR — populate EXTRACTDIR/go with the
# requested toolchain, so the caller's install/move logic is identical to the
# tarball route. Returns non-zero (without dying) when the route is unavailable.
install_go_from_module_proxy() {
  local ver="$1" extract="$2"

  if ! command -v go >/dev/null 2>&1; then
    warn "No existing Go, so the module-proxy fallback cannot be used (it needs a go command to bootstrap)."
    return 1
  fi

  info "Trying the Go module proxy for go${ver} (golang.org/toolchain)…"
  if ! RETRY_DESC="go toolchain fetch" retry go_toolchain_root "$ver"; then
    warn "Module proxy did not provide go${ver}."
    return 1
  fi

  # The module layout is NOT the tarball layout: entries live under
  # golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>/ rather than go/, and the
  # module cache is stored read-only. Normalise both so the shared code below
  # sees exactly what `tar -xzf` would have produced.
  mkdir -p "$extract/go"
  cp -a "$GO_TOOLCHAIN_ROOT/." "$extract/go/" || { warn "Could not stage the toolchain from $GO_TOOLCHAIN_ROOT"; return 1; }
  chmod -R u+w "$extract/go" 2>/dev/null || true
  [[ -x "$extract/go/bin/go" ]] || { warn "Module toolchain did not contain bin/go"; return 1; }

  # The caller prints the single authoritative verification verdict.
  log "module proxy supplied go${ver} from $GO_TOOLCHAIN_ROOT"
}

# go_can_bootstrap_module_proxy — whether the module-proxy route is usable at all.
#
# It drives the download with the `go` command already on this machine, so a box
# with no Go cannot use it: there the tarball is the ONLY way to get a toolchain,
# and preferring the module route would deadlock the installer. This is the
# explicit guard for that; do not collapse it into an inline test.
go_can_bootstrap_module_proxy() {
  command -v go >/dev/null 2>&1
}

install_go_tarball() {
  local ver="$1"
  local file="go${ver}.darwin-${OS_ARCH}.tar.gz"
  local dest="$WORK_DIR/$file"
  local sha=""
  local extract="$WORK_DIR/go-extract"
  info "Obtaining Go ${ver}"
  sha="$(fetch_go_sha256 "$file" || true)"
  local urls=(
    "${GO_DL_BASE%/}/${file}"
    "${GO_GOOGLE_DL%/}/${file}"
    "${GO_FALLBACK_DL%/}/${file}"
  )
  # Route precedence, strongest verification first: never take an unverified
  # route while a verified one is available.
  #   1. tarball + known sha256        -> verified
  #   2. module proxy (sumdb enforced) -> verified
  #   3. tarball with no sha256        -> UNVERIFIED, last resort, loud warning
  # The module route is verified or it fails (go_toolchain_root pins GOSUMDB),
  # so it has no "succeeded but unverified" outcome.
  local route="" verified=0 module_tried=0
  if [[ -z "$sha" ]] && go_can_bootstrap_module_proxy; then
    info "No official checksum for ${file}; trying the verified module-proxy route first."
    module_tried=1
    rm -rf "$extract"; mkdir -p "$extract"
    if install_go_from_module_proxy "$ver" "$extract"; then
      route="module proxy"; verified=1
    else
      warn "Verified module-proxy route unavailable; falling back to an unverified tarball."
    fi
  fi

  if [[ -z "$route" ]]; then
    local ok_dl=0 u
    for u in "${urls[@]}"; do
      if download_file "$u" "$dest" "$sha"; then ok_dl=1; break; fi
      warn "Download failed: $u"
    done
    if (( ok_dl == 1 )); then
      rm -rf "$extract"; mkdir -p "$extract"
      tar -C "$extract" -xzf "$dest"
      route="tarball"; [[ -n "$sha" ]] && verified=1
    elif (( module_tried == 0 )) && go_can_bootstrap_module_proxy; then
      warn "All Go tarball mirrors failed; falling back to the Go module proxy."
      rm -rf "$extract"; mkdir -p "$extract"
      install_go_from_module_proxy "$ver" "$extract" || die "Could not download Go ${ver} from any tarball mirror or the module proxy. brew install go  or https://go.dev/dl/"
      route="module proxy"; verified=1
    else
      die "Could not download Go ${ver} from any tarball mirror or the module proxy. brew install go  or https://go.dev/dl/"
    fi
  fi

  if (( verified == 1 )); then
    if [[ "$route" == "tarball" ]]; then
      ok "Go ${ver} obtained via the ${route} and verified (sha256)."
    else
      ok "Go ${ver} obtained via the ${route} and verified (checksum database ${GO_SUMDB_DEFAULT})."
    fi
  else
    warn "Go ${ver} obtained via the ${route} but could NOT be verified: no official sha256 was reachable$(go_can_bootstrap_module_proxy || printf '%s' ', and no existing Go was present to use the verified module-proxy route')."
    if [[ "${RUM_REQUIRE_VERIFIED_GO:-0}" == "1" ]]; then
      die "RUM_REQUIRE_VERIFIED_GO=1 is set and Go ${ver} could not be verified — refusing to install."
    fi
    warn "Continuing anyway. Set RUM_REQUIRE_VERIFIED_GO=1 to make this fatal."
  fi

  [[ -x "$extract/go/bin/go" ]] || die "The downloaded Go ${ver} did not contain bin/go"
  local goroot="$HOME/.local/go"
  info "Installing Go to $goroot (user-local)"
  mkdir -p "$HOME/.local"
  rm -rf "$goroot"
  mv "$extract/go" "$goroot"
  export GOROOT="$goroot"
  export PATH="$goroot/bin:$PATH"
  hash -r 2>/dev/null || true
  ok "Installed $(go version) at $goroot"
}

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

ensure_go() {
  if go_meets_min; then
    ok "Found $(go version)"
    return 0
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install Go ${GO_MIN}"
    return 0
  fi
  if try_brew_install go && go_meets_min; then
    hash -r 2>/dev/null || true
    ok "Installed $(go version) via Homebrew"
    return 0
  fi
  install_go_tarball "$GO_MIN"
  go_meets_min || die "Go ${GO_MIN}+ is required. brew install go  or https://go.dev/dl/"
}

configure_goproxy() {
  if [[ -n "$MIRROR" ]]; then
    if [[ "$MIRROR" == "$DEFAULT_MIRROR" ]]; then
      export GOPROXY="$BUILTIN_GOPROXY"
    else
      export GOPROXY="${MIRROR}|${BUILTIN_GOPROXY}"
    fi
    export GOSUMDB="$GO_SUMDB_DEFAULT"
    ok "Using Go module proxy chain: $GOPROXY"
  fi
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

if ! reachable "https://proxy.golang.org" && ! reachable "https://go.dev" && ! reachable "https://github.com"; then
  warn "Could not reach go.dev / proxy.golang.org / github.com — will try bundled Iranian module mirrors"
  [[ -z "$MIRROR" ]] && MIRROR="$DEFAULT_MIRROR"
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  info "dry-run complete (no changes). Re-run without --dry-run to install."
  exit 0
fi

ensure_xcode_clt
configure_goproxy
ensure_go

info "Building $APP (this may take a moment)…"
BUILD_OUT="$WORK_DIR/$APP"
if ! RETRY_DESC="go mod download" retry run_logged bash -c "cd \"$BACKEND_DIR\" && go mod download"; then
  if [[ "${GOPROXY:-}" != *"$IRANIAN_GOPROXY"* ]]; then
    warn "go mod download failed; retrying with bundled Iranian module mirrors"
    export GOPROXY="$BUILTIN_GOPROXY" GOSUMDB="$GO_SUMDB_DEFAULT"
    RETRY_DESC="go mod download (mirror)" retry run_logged bash -c "cd \"$BACKEND_DIR\" && go mod download" \
      || die "go mod download failed. Log: $LOG_FILE"
  else
    die "go mod download failed. Log: $LOG_FILE"
  fi
fi

if ! run_logged bash -c "cd \"$BACKEND_DIR\" && go build -buildvcs=false -trimpath -ldflags \"-s -w\" -o \"$BUILD_OUT\" ./cmd/rum"; then
  if [[ "${GOPROXY:-}" != *"$IRANIAN_GOPROXY"* ]]; then
    warn "go build failed; retrying with bundled Iranian module mirrors"
    export GOPROXY="$BUILTIN_GOPROXY" GOSUMDB="$GO_SUMDB_DEFAULT"
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
ok "Done. Try:  $APP --version  &&  $APP --help"
info "Log: $LOG_FILE"
