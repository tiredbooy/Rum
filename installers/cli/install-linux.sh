#!/usr/bin/env bash
#
# Rum CLI installer — Linux
#
# Builds the `rum` command-line download manager from source and installs it
# onto PATH. Go is auto-detected and installed when missing or too old.
#
# Usage:
#   ./installers/cli/install-linux.sh [options]
#
#   --prefix DIR   Install into DIR/bin (default: /usr/local if writable or via
#                  passwordless sudo, otherwise ~/.local). Honors $PREFIX too.
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

if [[ ! -t 0 ]]; then
  ASSUME_YES=1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"

WORK_DIR=""
LOG_FILE=""
APT_UPDATED=0

init_log() {
  local state_dir
  state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/rum"
  if mkdir -p "$state_dir" 2>/dev/null && [[ -w "$state_dir" ]]; then
    :
  else
    state_dir="/tmp"
  fi
  LOG_FILE="${state_dir}/cli-install-$(date -u +%Y%m%dT%H%M%SZ)-$$.log"
  : >"$LOG_FILE" || { LOG_FILE="/tmp/rum-cli-install-$$.log"; : >"$LOG_FILE"; }
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
  while [ "$attempt" -le "$RETRY_MAX" ]; do
    if "$@"; then
      return 0
    else
      st=$?
    fi
    # curl -f uses exit 22 for HTTP 4xx/5xx. 404/403 never recover; 429/503 might.
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
    die "Need curl or wget to download $url"$'\n'"Install one as root, then re-run:"$'\n'"$(print_curl_install_cmd)"
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
  if command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$file")"; got="${got%% *}"
  elif command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$file")"; got="${got%% *}"
  elif command -v openssl >/dev/null 2>&1; then
    got="$(openssl dgst -sha256 "$file")"; got="${got##* }"
  else
    warn "No sha256 tool found; skipping checksum verification for $(basename "$file")"
    return 0
  fi
  [[ "$(printf '%s' "$got" | tr 'A-F' 'a-f')" == "$(printf '%s' "$expect" | tr 'A-F' 'a-f')" ]]
}

OS_KERNEL="$(uname -s 2>/dev/null || echo unknown)"
OS_ARCH_RAW="$(uname -m 2>/dev/null || echo unknown)"
is_musl() {
  [[ -e /lib/ld-musl-x86_64.so.1 || -e /lib/ld-musl-aarch64.so.1 ]] && return 0
  ldd --version 2>&1 | grep -qi musl
}
case "$OS_ARCH_RAW" in
  x86_64|amd64) OS_ARCH="amd64" ;;
  aarch64|arm64) OS_ARCH="arm64" ;;
  *) OS_ARCH="$OS_ARCH_RAW" ;;
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
    *" debian "*|*" ubuntu "*) command -v apt-get >/dev/null 2>&1 && { echo apt; return; } ;;
    *" fedora "*|*" rhel "*|*" centos "*|*" rocky "*|*" alma "*|*" nobara "*)
      command -v dnf >/dev/null 2>&1 && { echo dnf; return; }
      command -v yum >/dev/null 2>&1 && { echo yum; return; } ;;
    *" arch "*|*" manjaro "*|*" endeavour "*) command -v pacman >/dev/null 2>&1 && { echo pacman; return; } ;;
    *" suse "*|*" opensuse "*) command -v zypper >/dev/null 2>&1 && { echo zypper; return; } ;;
    *" alpine "*) command -v apk >/dev/null 2>&1 && { echo apk; return; } ;;
  esac
  for c in apt-get dnf yum pacman zypper apk brew; do
    if command -v "$c" >/dev/null 2>&1; then
      case "$c" in apt-get) echo apt; return ;; *) echo "$c"; return ;; esac
    fi
  done
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
  elif can_root; then
    sudo "$@"
  else
    return 1
  fi
}

IS_ROOT=0
(( EUID == 0 )) && IS_ROOT=1
HAS_SUDO_N=0
can_root && [[ "$IS_ROOT" -eq 0 ]] && HAS_SUDO_N=1

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

# A previous user-local Go / rum must be visible so re-runs skip completed work.
for _d in /usr/local/go/bin "$HOME/.local/go/bin" "$HOME/go/bin" "$HOME/.local/bin" "$BIN_DIR"; do
  [[ -n "$_d" && -d "$_d" ]] || continue
  case ":$PATH:" in *":$_d:"*) continue ;; esac
  PATH="$_d:$PATH"
done
unset _d
export PATH

read_go_min() {
  local f="$BACKEND_DIR/go.mod" kw ver
  [[ -f "$f" ]] || f="$REPO_ROOT/go.mod"
  if [[ -f "$f" ]]; then
    while read -r kw ver _; do
      if [[ "$kw" == "go" && -n "${ver:-}" ]]; then
        printf '%s' "$ver"
        return 0
      fi
    done < "$f"
  fi
  printf '%s' "1.25.7"
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
  [[ -n "$cur" ]] || cur="$(go version 2>/dev/null)"
  cur="${cur#go version }"
  cur="${cur%% *}"
  cur="${cur#go}"
  [[ -n "$cur" ]] || return 1
  version_ge "$cur" "$GO_MIN"
}

try_pkg_install() {
  (( $# == 0 )) && return 0
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install packages via $PKG_MGR: $*"
    return 0
  fi
  can_root || return 1
  case "$PKG_MGR" in
    apt)
      if (( APT_UPDATED == 0 )); then
        RETRY_DESC="apt-get update" retry as_root env DEBIAN_FRONTEND=noninteractive apt-get update -qq
        APT_UPDATED=1
      fi
      RETRY_DESC="apt-get install $*" retry as_root env DEBIAN_FRONTEND=noninteractive \
        apt-get install -y -qq --no-install-recommends "$@"
      ;;
    dnf) RETRY_DESC="dnf install $*" retry as_root dnf install -y "$@" ;;
    yum) RETRY_DESC="yum install $*" retry as_root yum install -y "$@" ;;
    pacman) RETRY_DESC="pacman -S $*" retry as_root pacman -Sy --needed --noconfirm "$@" ;;
    zypper) RETRY_DESC="zypper install $*" retry as_root zypper --non-interactive --gpg-auto-import-keys install -y "$@" ;;
    apk) RETRY_DESC="apk add $*" retry as_root apk add --no-cache "$@" ;;
    brew) RETRY_DESC="brew install $*" retry env HOMEBREW_NO_AUTO_UPDATE=1 brew install "$@" ;;
    *) return 1 ;;
  esac
}

print_go_install_cmd() {
  case "$PKG_MGR" in
    apt) echo "  sudo apt-get update && sudo apt-get install -y golang" ;;
    dnf) echo "  sudo dnf install -y golang" ;;
    yum) echo "  sudo yum install -y golang" ;;
    pacman) echo "  sudo pacman -S --needed go" ;;
    zypper) echo "  sudo zypper install -y go" ;;
    apk) echo "  sudo apk add go" ;;
    brew) echo "  brew install go" ;;
    *) echo "  Install Go ${GO_MIN}+ from https://go.dev/dl/" ;;
  esac
}

print_curl_install_cmd() {
  case "$PKG_MGR" in
    apt) echo "  sudo apt-get update && sudo apt-get install -y curl ca-certificates" ;;
    dnf) echo "  sudo dnf install -y curl ca-certificates" ;;
    yum) echo "  sudo yum install -y curl ca-certificates" ;;
    pacman) echo "  sudo pacman -S --needed curl ca-certificates" ;;
    zypper) echo "  sudo zypper install -y curl ca-certificates" ;;
    apk) echo "  sudo apk add curl ca-certificates" ;;
    brew) echo "  brew install curl" ;;
    *) echo "  Install curl (or wget) from your package manager" ;;
  esac
}

fetch_go_index() {
  local json="$WORK_DIR/go-dl.json"
  if [[ -s "$json" ]]; then
    printf '%s' "$json"
    return 0
  fi
  if download_file "${GO_DL_BASE%/}/?mode=json" "$json" \
     || download_file "${GO_DL_BASE%/}/?mode=json&include=all" "$json"; then
    printf '%s' "$json"
    return 0
  fi
  return 1
}

go_sha_for() {
  local json="$1" filename="$2" sha=""
  sha="$(grep -o "\"filename\":\"${filename}\"[^}]*\"sha256\":\"[a-f0-9]*\"" "$json" 2>/dev/null \
    | grep -o '"sha256":"[a-f0-9]*"' | head -1 | cut -d'"' -f4 || true)"
  [[ -n "$sha" ]] && printf '%s' "$sha"
}

pick_go_linux_archive() {
  local json="$1" arch="$2" want="$3"
  local exact="go${want}.linux-${arch}.tar.gz"
  if grep -q "\"filename\":\"${exact}\"" "$json" 2>/dev/null; then
    printf '%s' "$exact"
    return 0
  fi
  grep -oE "go[0-9]+\.[0-9]+(\.[0-9]+)?\.linux-${arch}\.tar\.gz" "$json" 2>/dev/null | head -1
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
  local json="" file="" sha="" dest
  json="$(fetch_go_index || true)"
  if [[ -n "$json" && -s "$json" ]]; then
    file="$(pick_go_linux_archive "$json" "$OS_ARCH" "$ver" || true)"
    [[ -n "$file" ]] && sha="$(go_sha_for "$json" "$file" || true)"
  fi
  if [[ -z "$file" ]]; then
    file="go${ver}.linux-${OS_ARCH}.tar.gz"
  fi
  if [[ "$file" != "go${ver}.linux-${OS_ARCH}.tar.gz" ]]; then
    info "go.mod wants Go ${ver}; using published toolchain $file (same or newer)"
  else
    info "Obtaining Go ${ver}"
  fi
  dest="$WORK_DIR/$file"
  local urls=(
    "${GO_DL_BASE%/}/${file}"
    "${GO_GOOGLE_DL%/}/${file}"
  )
  if [[ "${RUM_GO_SKIP_EXTRA_MIRRORS:-0}" != "1" ]]; then
    urls+=("https://mirrors.aliyun.com/golang/${file}")
    urls+=("${GO_FALLBACK_DL%/}/${file}")
  fi
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
      install_go_from_module_proxy "$ver" "$extract" || die "Could not download Go ${ver} from any tarball mirror or the module proxy."$'\n'"$(print_go_install_cmd)"
      route="module proxy"; verified=1
    else
      die "Could not download Go ${ver} from any tarball mirror or the module proxy."$'\n'"$(print_go_install_cmd)"
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

  local goroot=""
  if can_root; then
    goroot="/usr/local/go"
    info "Installing Go to $goroot"
    as_root rm -rf "$goroot"
    as_root mkdir -p /usr/local
    as_root mv "$extract/go" "$goroot"
  else
    goroot="$HOME/.local/go"
    info "Installing Go to $goroot (user-local; no root)"
    mkdir -p "$HOME/.local"
    rm -rf "$goroot"
    mv "$extract/go" "$goroot"
  fi
  export GOROOT="$goroot"
  export PATH="$goroot/bin:$PATH"
  hash -r 2>/dev/null || true
  ensure_path_in_rc "$goroot/bin"
  ok "Installed $(go version) at $goroot"
}

ensure_go() {
  if go_meets_min; then
    ok "Found $(go version)"
    return 0
  fi
  if command -v go >/dev/null 2>&1; then
    warn "Go is too old ($(go version)); need >= ${GO_MIN}"
  else
    info "Go ${GO_MIN}+ is not installed"
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install Go ${GO_MIN}"
    return 0
  fi
  local go_pkgs=()
  case "$PKG_MGR" in
    apt|dnf|yum) go_pkgs=(golang) ;;
    pacman|zypper|apk|brew) go_pkgs=(go) ;;
  esac
  if ((${#go_pkgs[@]})) && try_pkg_install "${go_pkgs[@]}"; then
    hash -r 2>/dev/null || true
    if go_meets_min; then
      ok "Installed $(go version) via $PKG_MGR"
      return 0
    fi
    warn "Package-manager Go is too old or missing; using the official tarball"
  fi
  if ! have_curl && ! have_wget; then
    try_pkg_install curl ca-certificates || try_pkg_install wget ca-certificates || true
    hash -r 2>/dev/null || true
  fi
  if is_musl; then
    if can_root; then
      try_pkg_install gcompat || try_pkg_install libc6-compat || true
    else
      die "This is a musl/Alpine system. Official Go tarballs are glibc-linked, and there is no root to run apk. Install Go as root and re-run:"$'\n'"  sudo apk add go curl ca-certificates bash"
    fi
  fi
  install_go_tarball "$GO_MIN"
  go_meets_min || die "Go ${GO_MIN}+ is required."$'\n'"$(print_go_install_cmd)"
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
  # Persist even if this process already has $dir on PATH (we prepend during the run).
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
      die "Cannot write $dest (need write permission or passwordless sudo). Re-run with --prefix \"\$HOME/.local\""
    }
  fi
  rm -f "$tmp" 2>/dev/null || true
}

if [[ "$UNINSTALL" -eq 1 ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would remove $TARGET /usr/local/bin/$APP $HOME/.local/bin/$APP $HOME/bin/$APP"
    exit 0
  fi
  local_any=0
  for cand in "$TARGET" "/usr/local/bin/$APP" "$HOME/.local/bin/$APP" "$HOME/bin/$APP"; do
    if [[ -e "$cand" ]]; then
      info "Removing $cand"
      if [[ -w "$(dirname "$cand")" ]]; then rm -f "$cand"; else as_root rm -f "$cand"; fi
      ok "Removed $cand"
      local_any=1
    fi
  done
  (( local_any == 1 )) || info "Nothing to remove"
  info "Log: $LOG_FILE"
  exit 0
fi

[[ -d "$BACKEND_DIR/cmd/rum" ]] || die "Cannot find $BACKEND_DIR/cmd/rum — run this from a clean checkout."

info "Detected: ${OS_KERNEL}/${OS_ARCH} distro=${DISTRO_ID} pkg=${PKG_MGR} root=${IS_ROOT} passwordless-sudo=${HAS_SUDO_N} tty=$([[ -t 0 ]] && echo yes || echo no) assume-yes=${ASSUME_YES} shell-rc=${SHELL_RC}"
info "Go minimum (from go.mod): ${GO_MIN}  |  install target: ${TARGET}"

PLAN=()
go_meets_min || PLAN+=("Go ${GO_MIN}+")
command -v git >/dev/null 2>&1 || PLAN+=("git (optional)")
if ((${#PLAN[@]})); then
  info "Will install: ${PLAN[*]}"
else
  info "All build prerequisites already present"
fi
if [[ -e "$TARGET" ]]; then
  info "Existing install will be replaced: $TARGET"
fi
info "Log file: $LOG_FILE"

HOME_FREE=""
read -r _ _ _ HOME_FREE _ <<<"$(df -Pk "$HOME" 2>/dev/null | tail -n 1)" || HOME_FREE=0
if [[ "${HOME_FREE:-0}" -gt 0 && "${HOME_FREE}" -lt 524288 ]]; then
  warn "Low disk space on \$HOME; the CLI build may need a few hundred MiB"
fi

if have_curl || have_wget; then
  if ! reachable "https://proxy.golang.org" && ! reachable "https://go.dev" && ! reachable "https://github.com"; then
  warn "Could not reach go.dev / proxy.golang.org / github.com — will try bundled Iranian module mirrors"
    [[ -z "$MIRROR" ]] && MIRROR="$DEFAULT_MIRROR"
  fi
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  info "dry-run complete (no changes). Re-run without --dry-run to install."
  exit 0
fi

configure_goproxy
ensure_go
command -v git >/dev/null 2>&1 || try_pkg_install git || true

info "Building $APP (this may take a moment)…"
BUILD_OUT="$WORK_DIR/$APP"
# `if` so the ERR trap does not fire on a handled failure (set -E + set +e is not enough).
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

# -buildvcs=false: a copied tree (docker, tarball, dubious git ownership) makes
# `git status` exit 128 and Go refuses to build.
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
[[ -d "$HOME/.local/go/bin" ]] && ensure_path_in_rc "$HOME/.local/go/bin"
export PATH="$BIN_DIR:$PATH"

echo
ok "Done. Try:  $APP --version  &&  $APP --help"
if [[ "$IS_ROOT" -eq 0 ]]; then
  info "User-local install: $TARGET"
  info "PATH for this shell:  export PATH=\"$BIN_DIR:\$PATH\""
fi
info "Log: $LOG_FILE"
