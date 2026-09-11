#!/usr/bin/env bash
#
# Cross-compile Rum for Windows from Linux/macOS and (optionally) wrap it
# in an Inno Setup installer via Docker.
#
# Prerequisites (Go, Node, Wails, Docker for the installer) are auto-detected
# and installed when possible. Network operations retry with backoff.
#
# Usage:
#   ./build-windows.sh [--yes] [--mirror[=URL]] [--verbose] [--dry-run] [--skip-installer]
#
set -Eeuo pipefail

ASSUME_YES=0
VERBOSE=0
DRY_RUN=0
SKIP_INSTALLER=0
MIRROR=""
DEFAULT_MIRROR="https://go.devneeds.ir/"
IRANIAN_GOPROXY="https://mirror-go.runflare.com|https://package-mirror.liara.ir/repository/go|https://mirror.abrha.net/repository/go|${DEFAULT_MIRROR%/}"
BUILTIN_GOPROXY="${IRANIAN_GOPROXY}|https://proxy.golang.org,direct"
RETRY_MAX="${RUM_RETRY_MAX:-5}"
GO_DL_BASE="${RUM_GO_DL_BASE:-https://go.dev/dl}"
GO_GOOGLE_DL="${RUM_GO_GOOGLE_DL:-https://dl.google.com/go}"
GO_FALLBACK_DL="${RUM_GO_FALLBACK_DL:-$DEFAULT_MIRROR}"
NODE_MIN="18.0.0"
WAILS_FALLBACK="v2.12.0"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes|-y) ASSUME_YES=1; shift ;;
    --mirror) MIRROR="$DEFAULT_MIRROR"; shift ;;
    --mirror=*) MIRROR="${1#*=}"; shift ;;
    --verbose|-v) VERBOSE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-installer) SKIP_INSTALLER=1; shift ;;
    -h|--help) awk 'NR==1{next} /^#/{print; next} {exit}' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -t 0 ]]; then
  ASSUME_YES=1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"
APP="Rum"

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
  LOG_FILE="${state_dir}/build-windows-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="${TMPDIR:-/tmp}/rum-build-windows-$$.log"; : >"$LOG_FILE"; }
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
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/rum-winbuild-XXXXXX")"

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

have_curl() { command -v curl >/dev/null 2>&1; }
have_wget() { command -v wget >/dev/null 2>&1; }

download_file() {
  local url="$1" dest="$2" sha="${3:-}"
  local tmp="${dest}.part" st=0
  local connect_to="${RUM_CURL_CONNECT_TIMEOUT:-20}"
  mkdir -p "$(dirname "$dest")"
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
    local got=""
    if command -v sha256sum >/dev/null 2>&1; then
      got="$(sha256sum "$tmp" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      got="$(shasum -a 256 "$tmp" | awk '{print $1}')"
    fi
    if [[ -n "$got" && "$(printf '%s' "$got" | tr 'A-F' 'a-f')" != "$(printf '%s' "$sha" | tr 'A-F' 'a-f')" ]]; then
      rm -f "$tmp"
      die "Checksum mismatch for $url"
    fi
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

OS_KERNEL="$(uname -s 2>/dev/null || echo unknown)"
OS_ARCH_RAW="$(uname -m 2>/dev/null || echo unknown)"
case "$OS_ARCH_RAW" in
  x86_64|amd64) OS_ARCH="amd64" ;;
  aarch64|arm64) OS_ARCH="arm64" ;;
  *) OS_ARCH="$OS_ARCH_RAW" ;;
esac

GO_OS="linux"
[[ "$OS_KERNEL" == "Darwin" ]] && GO_OS="darwin"

read_go_min() {
  local f="$REPO_ROOT/go.mod" v=""
  [[ -f "$f" ]] && v="$(awk '/^go[[:space:]]/ {print $2; exit}' "$f")"
  printf '%s' "${v:-1.25.7}"
}
GO_MIN="$(read_go_min)"

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

go_ver_norm() {
  local v="${1#go version }"
  v="${v%% *}"
  v="${v#go}"
  v="${v#v}"
  printf '%s' "${v%%-*}"
}

go_meets_min() {
  command -v go >/dev/null 2>&1 || return 1
  local cur
  cur="$(go env GOVERSION 2>/dev/null || true)"
  [[ -n "$cur" ]] || cur="$(go version 2>/dev/null | awk '{print $3}')"
  cur="$(go_ver_norm "$cur")"
  [[ -n "$cur" ]] || return 1
  version_ge "$cur" "$GO_MIN"
}

pin_wails_go() {
  command -v go >/dev/null 2>&1 || return 0
  local cur min
  cur="$(go_ver_norm "$(go env GOVERSION 2>/dev/null || go version 2>/dev/null || true)")"
  min="$(go_ver_norm "$GO_MIN")"
  [[ -n "$cur" && "$cur" != "$min" ]] || return 0
  version_ge "$cur" "$min" || return 0
  info "Go $(go version) is newer than ${GO_MIN}; installing exact go${GO_MIN} for Wails (GOTOOLCHAIN=local)."
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install Go ${GO_MIN} tarball"
    return 0
  fi
  install_go_tarball "$GO_MIN"
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

can_root() {
  (( EUID == 0 )) && return 0
  command -v sudo >/dev/null 2>&1 || return 1
  sudo -n true >/dev/null 2>&1
}

as_root() {
  if (( EUID == 0 )); then "$@"
  elif can_root; then sudo "$@"
  else return 1
  fi
}

try_pkg_install() {
  (( $# == 0 )) && return 0
  can_root || return 1
  if command -v apt-get >/dev/null 2>&1; then
    RETRY_DESC="apt-get install $*" retry as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends "$@"
  elif command -v dnf >/dev/null 2>&1; then
    RETRY_DESC="dnf install $*" retry as_root dnf install -y "$@"
  elif command -v brew >/dev/null 2>&1; then
    RETRY_DESC="brew install $*" retry env HOMEBREW_NO_AUTO_UPDATE=1 brew install "$@"
  else
    return 1
  fi
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
  local file="go${ver}.${GO_OS}-${OS_ARCH}.tar.gz"
  local dest="$WORK_DIR/$file"
  local sha=""
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
  local extract="$WORK_DIR/go-extract"
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
      install_go_from_module_proxy "$ver" "$extract" || die "Could not download Go ${ver} from any tarball mirror or the module proxy"
      route="module proxy"; verified=1
    else
      die "Could not download Go ${ver} from any tarball mirror or the module proxy"
    fi
  fi

  [[ -x "$extract/go/bin/go" ]] || die "The downloaded Go ${ver} did not contain bin/go"

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

  local goroot="$HOME/.local/go"
  mkdir -p "$HOME/.local"
  rm -rf "$goroot"
  mv "$extract/go" "$goroot"
  export GOROOT="$goroot" PATH="$goroot/bin:$PATH"
  hash -r 2>/dev/null || true
  ok "Installed $(go version) at $goroot"
}

ensure_go() {
  if go_meets_min; then
    ok "Found $(go version)"
    pin_wails_go
    return 0
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then info "dry-run: would install Go ${GO_MIN}"; return 0; fi
  try_pkg_install golang || try_pkg_install go || true
  hash -r 2>/dev/null || true
  if go_meets_min; then
    ok "Installed $(go version)"
    pin_wails_go
    return 0
  fi
  install_go_tarball "$GO_MIN"
  go_meets_min || die "Go ${GO_MIN}+ is required. See INSTALL.md"
  pin_wails_go
}

ensure_node() {
  node_meets_min && { ok "Found node $(node --version) / npm $(npm --version)"; return 0; }
  if [[ "$DRY_RUN" -eq 1 ]]; then info "dry-run: would install Node.js"; return 0; fi
  try_pkg_install nodejs npm || try_pkg_install node || true
  hash -r 2>/dev/null || true
  if node_meets_min; then
    ok "Installed node $(node --version)"
    return 0
  fi
  die "Node.js ${NODE_MIN}+ and npm are required for the Windows GUI build. Install from https://nodejs.org and re-run."
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

ensure_wails() {
  PATH="${PATH}:$(go env GOPATH 2>/dev/null)/bin:${HOME}/go/bin"
  export PATH
  hash -r 2>/dev/null || true
  command -v wails >/dev/null 2>&1 && { ok "Found wails"; return 0; }
  if [[ "$DRY_RUN" -eq 1 ]]; then info "dry-run: would go install wails@${WAILS_VER}"; return 0; fi
  info "Installing Wails CLI ${WAILS_VER}"
  set +e
  RETRY_DESC="go install wails" retry run_logged go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VER}"
  st=$?
  set -e
  if (( st != 0 )) && [[ "${GOPROXY:-}" != *"$IRANIAN_GOPROXY"* ]]; then
    export GOPROXY="$BUILTIN_GOPROXY" GOSUMDB="$GO_SUMDB_DEFAULT"
    RETRY_DESC="go install wails (mirror)" retry run_logged go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VER}"
  elif (( st != 0 )); then
    die "go install wails failed"
  fi
  PATH="${PATH}:$(go env GOPATH)/bin"
  export PATH
  hash -r 2>/dev/null || true
  command -v wails >/dev/null 2>&1 || die "wails still not on PATH"
  ok "Found wails"
}

info "=== Building Rum for Windows (cross-compile from ${OS_KERNEL}/${OS_ARCH}) ==="
info "Go minimum: ${GO_MIN}  |  Wails: ${WAILS_VER}  |  assume-yes=${ASSUME_YES}  |  log: ${LOG_FILE}"

PLAN=()
go_meets_min || PLAN+=("Go ${GO_MIN}+")
node_meets_min || PLAN+=("Node.js ${NODE_MIN}+ / npm")
command -v wails >/dev/null 2>&1 || PLAN+=("wails CLI")
command -v docker >/dev/null 2>&1 || PLAN+=("docker (optional, for Rum-Setup.exe)")
if ((${#PLAN[@]})); then info "Will install / need: ${PLAN[*]}"; else info "All build prerequisites already present"; fi

if ! reachable "https://proxy.golang.org" && ! reachable "https://go.dev"; then
  warn "Could not reach go.dev — will try bundled Iranian module mirrors"
  [[ -z "$MIRROR" ]] && MIRROR="$DEFAULT_MIRROR"
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  info "dry-run complete (no changes)."
  exit 0
fi

configure_goproxy
ensure_go
ensure_node
ensure_wails

if [[ ! -f "$REPO_ROOT/LICENSE" ]]; then
  info "No LICENSE file found. Creating a placeholder (MIT)."
  cat > "$REPO_ROOT/LICENSE" <<'EOF'
MIT License
Copyright (c) 2026 tiredbooy
Permission is hereby granted, free of charge, to any person obtaining a copy...
EOF
fi

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

info "Cross-compiling the Rum desktop app (wails build -platform windows/amd64)…"
export GOTOOLCHAIN=local
(
  cd "$REPO_ROOT"
  set +e
  run_logged wails build -platform windows/amd64 -clean
  st=$?
  set -e
  if (( st != 0 )) && [[ "${GOPROXY:-}" != *"$IRANIAN_GOPROXY"* ]]; then
    warn "wails build failed; retrying with bundled Iranian module mirrors"
    export GOPROXY="$BUILTIN_GOPROXY" GOSUMDB="$GO_SUMDB_DEFAULT"
    run_logged wails build -platform windows/amd64 -clean
  elif (( st != 0 )); then
    exit "$st"
  fi
)
BUILT_EXE="$REPO_ROOT/build/bin/${APP}.exe"
[[ -f "$BUILT_EXE" ]] || die "Expected build output $BUILT_EXE not found"
ok "Built $BUILT_EXE"

if [[ "$SKIP_INSTALLER" -eq 1 ]]; then
  ok "Skipping Inno Setup (--skip-installer). Windows exe: $BUILT_EXE"
  info "Log: $LOG_FILE"
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  warn "Docker is not installed — cannot wrap Rum-Setup.exe with Inno Setup."
  warn "The Windows binary is still at $BUILT_EXE"
  info "Install Docker and re-run, or pass --skip-installer."
  exit 0
fi

info "Building installer with Docker (amake/innosetup)…"
RETRY_DESC="docker pull amake/innosetup" retry run_logged docker pull amake/innosetup || warn "docker pull failed; trying run anyway"
RETRY_DESC="docker run innosetup" retry run_logged docker run --rm -v "$REPO_ROOT":/work -w /work amake/innosetup installer.iss \
  || die "Inno Setup via Docker failed. Binary is still at $BUILT_EXE"

ok "Windows installer created: $REPO_ROOT/build/bin/Rum-Setup.exe"
info "Log: $LOG_FILE"
