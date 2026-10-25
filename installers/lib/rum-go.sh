# shellcheck shell=bash
#
# Shared Go toolchain helpers for the Rum bash installers (cli/gui x linux/macos).
# Sourced by those scripts from the checkout; never executed on its own.
#
#   1. Use the Go that is already on this machine (PATH, then well-known
#      install locations) when it satisfies go.mod. No download.
#   2. Pin GOTOOLCHAIN=local so Go never tries to fetch a toolchain from Google.
#   3. Only when nothing suitable exists: download from go.dev, dl.google.com,
#      then non-Google mirrors, sha256-verified, resumable, atomic.
#   4. Pick a reachable module proxy / checksum DB / npm registry, never
#      overriding a value the user set.
#
# Contract: the sourcing script defines, before calling anything here:
#   info ok warn die log retry have_curl have_wget   (output + retry helpers)
#   WORK_DIR LOG_FILE DRY_RUN REPO_ROOT MIRROR DEFAULT_MIRROR IRANIAN_GOPROXY
#   RUM_GO_OS=linux|darwin   (set before sourcing)
# Optional: can_root as_root ensure_path_in_rc OS_ARCH is_musl
#   pkg_install_go      hook: try the distro package; return 0 if it ran
#   pre_go_download     hook: last checks right before a download (may die)
#
# Written for bash 3.2 (stock macOS): no mapfile, no associative arrays.

RUM_GO_OS="${RUM_GO_OS:-linux}"
RUM_GO_SAME_MINOR="${RUM_GO_SAME_MINOR:-0}"   # GUI: Wails needs go.mod's minor line
RUM_GO_BIN=""
RUM_GO_VER=""
RUM_GO_SOURCE=""
RUM_GO_OLDER_BIN=""
RUM_GO_OLDER_VER=""
RUM_GO_SPARE_BIN=""   # acceptable by minimum but not by RUM_GO_SAME_MINOR
RUM_GO_SPARE_VER=""
RUM_GO_DOWNLOADED=""
RUM_PROXY_NOTE=""

GO_DL_BASE="${GO_DL_BASE:-${RUM_GO_DL_BASE:-https://go.dev/dl}}"
GO_GOOGLE_DL="${GO_GOOGLE_DL:-${RUM_GO_GOOGLE_DL:-https://dl.google.com/go}}"
GO_FALLBACK_DL="${GO_FALLBACK_DL:-${RUM_GO_FALLBACK_DL:-https://go.devneeds.ir/}}"
GO_SUMDB_DEFAULT="${RUM_GO_SUMDB:-sum.golang.org}"
# GOPROXY lists use '|' so any error (timeout, 403, 429) advances to the next
# proxy; ',' only advances on 404/410.
BUILTIN_GOPROXY="${IRANIAN_GOPROXY:-https://mirror-go.runflare.com}|https://goproxy.cn|https://goproxy.io|https://proxy.golang.org,direct"
RUM_GOPROXY_CANDIDATES="https://goproxy.cn
https://goproxy.io
https://mirror-go.runflare.com
https://package-mirror.liara.ir/repository/go
https://mirror.abrha.net/repository/go
${DEFAULT_MIRROR:-https://go.devneeds.ir/}"

# ---------------------------------------------------------------------------
# Versions
# ---------------------------------------------------------------------------

# rum_ver_norm go1.25.7|1.25|1.26rc1 -> "MAJOR MINOR PATCH FINAL" (FINAL 0 = pre-release)
rum_ver_norm() {
  local v="${1#go version }" re='^([0-9]+)\.([0-9]+)(\.([0-9]+))?(.*)$' pre=1
  v="${v%% *}"; v="${v#go}"; v="${v#v}"
  [[ "$v" =~ $re ]] || return 1
  case "${BASH_REMATCH[5]}" in rc*|beta*|alpha*|-pre*) pre=0 ;; esac
  printf '%d %d %d %d' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[4]:-0}" "$pre"
}

# rum_ver_ge A B — A >= B (go1.25 == 1.25.0; release candidates sort below finals)
rum_ver_ge() {
  local a b a1 a2 a3 a4 b1 b2 b3 b4
  a="$(rum_ver_norm "$1")" || return 1
  b="$(rum_ver_norm "$2")" || return 1
  read -r a1 a2 a3 a4 <<<"$a"
  read -r b1 b2 b3 b4 <<<"$b"
  if (( a1 != b1 )); then (( a1 > b1 )); return; fi
  if (( a2 != b2 )); then (( a2 > b2 )); return; fi
  if (( a3 != b3 )); then (( a3 > b3 )); return; fi
  (( a4 >= b4 ))
}

# version_ge A B — the scripts also use this for the Node.js minimum.
version_ge() { rum_ver_ge "$@"; }

# Same major.minor line (1.25.9 and 1.25.7 match; 1.26.0 does not).
rum_ver_same_minor() {
  local a b
  a="$(rum_ver_norm "$1")" || return 1
  b="$(rum_ver_norm "$2")" || return 1
  local a1 a2 a3 a4 b1 b2 b3 b4
  read -r a1 a2 a3 a4 <<<"$a"
  read -r b1 b2 b3 b4 <<<"$b"
  [[ "$a1" == "$b1" && "$a2" == "$b2" ]]
}

# rum_read_go_min FILE... — highest `go` directive among the go.mod files that
# exist. Reads the file; the `toolchain` line is deliberately ignored (we pin
# GOTOOLCHAIN=local, so only the language minimum matters).
rum_read_go_min() {
  local f v best=""
  for f in "$@"; do
    [[ -f "$f" ]] || continue
    v="$(awk '/^go[[:space:]]+[0-9]/ {print $2; exit}' "$f" 2>/dev/null || true)"
    [[ -n "$v" ]] || continue
    if [[ -z "$best" ]] || ! rum_ver_ge "$best" "$v"; then best="$v"; fi
  done
  printf '%s' "${best:-1.25.7}"
}

# rum_go_version_of BIN — prints "1.25.7"; runs from / with GOTOOLCHAIN=local so a
# go.mod in the current directory can never trigger a toolchain download.
rum_go_version_of() {
  local out v
  out="$(cd / && GOTOOLCHAIN=local GOFLAGS="" "$1" version 2>/dev/null)" || return 1
  v="${out#go version }"; v="${v%% *}"; v="${v#go}"
  v="$(rum_ver_norm "$v")" || return 1
  set -- $v
  if [[ "$4" == "1" ]]; then printf '%s.%s.%s' "$1" "$2" "$3"; else printf '%s.%s.%s-pre' "$1" "$2" "$3"; fi
}

# ---------------------------------------------------------------------------
# Finding a local Go
# ---------------------------------------------------------------------------

rum_realpath() { readlink -f "$1" 2>/dev/null || printf '%s' "$1"; }

# Candidate go executables, one per line, "P:" = priority (explicit/PATH), "L:" = scan.
rum_go_candidates() {
  local d p
  if [[ -n "${RUM_GO_ROOT:-}" ]]; then printf 'P:%s/bin/go\n' "${RUM_GO_ROOT%/}"; fi
  p="$(command -v go 2>/dev/null || true)"
  if [[ -n "$p" ]]; then printf 'P:%s\n' "$p"; fi
  if [[ -n "${GOROOT:-}" ]]; then printf 'L:%s/bin/go\n' "$GOROOT"; fi
  if [[ "${RUM_GO_NO_SCAN:-0}" == "1" ]]; then return 0; fi   # PATH / RUM_GO_ROOT only
  for d in /usr/local/go /usr/lib/go /usr/lib/golang /opt/go /opt/golang /snap/go/current \
           /usr/local/opt/go/libexec /opt/homebrew/opt/go/libexec \
           "$HOME/.local/go" "$HOME/go" "$HOME/.go" "$HOME/golang"; do
    printf 'L:%s/bin/go\n' "$d"
  done
  for d in /usr/local/bin /usr/bin /snap/bin /opt/homebrew/bin /home/linuxbrew/.linuxbrew/bin \
           "$HOME/.local/bin" "$HOME/bin" "$HOME/go/bin" "$HOME/.local/go/bin" /usr/local/go/bin; do
    printf 'L:%s/go\n' "$d"
  done
  # Versioned layouts: Debian golang-1.x, `go install golang.org/dl`, version managers.
  for d in /usr/lib/go-*/bin /usr/local/go*/bin "$HOME"/sdk/go*/bin "$HOME"/.local/go*/bin \
           "$HOME"/.goenv/versions/*/bin "$HOME"/.asdf/installs/golang/*/go/bin \
           "$HOME"/.local/share/mise/installs/go/*/bin "$HOME"/.gvm/gos/go*/bin \
           /opt/homebrew/Cellar/go/*/libexec/bin /usr/local/Cellar/go/*/libexec/bin \
           "$HOME"/scoop/apps/go/current/bin; do
    [[ -d "$d" ]] && printf 'L:%s/go\n' "$d"
  done
  return 0
}

# rum_go_acceptable VER — meets go.mod's minimum (and, for the GUI, its minor line).
rum_go_acceptable() {
  rum_ver_ge "$1" "$GO_MIN" || return 1
  if [[ "$RUM_GO_SAME_MINOR" == "1" ]]; then rum_ver_same_minor "$1" "$GO_MIN" || return 1; fi
  return 0
}

# rum_select_local_go — choose a usable Go already on this machine. Sets
# RUM_GO_BIN/RUM_GO_VER/RUM_GO_SOURCE, or leaves them empty. Also records the
# best too-old Go (for the explanation) and, in GUI mode, a newer-minor spare.
# Does not change PATH; rum_activate_go does.
rum_select_local_go() {
  local line tag c real v seen=" " lbest="" lbest_ver=""
  RUM_GO_BIN=""; RUM_GO_VER=""; RUM_GO_SOURCE=""
  RUM_GO_OLDER_BIN=""; RUM_GO_OLDER_VER=""; RUM_GO_SPARE_BIN=""; RUM_GO_SPARE_VER=""
  while IFS= read -r line; do
    tag="${line%%:*}"; c="${line#*:}"
    [[ -f "$c" && -x "$c" ]] || continue
    real="$(rum_realpath "$c")"
    case "$seen" in *" $real "*) continue ;; esac
    seen="${seen}${real} "
    v="$(rum_go_version_of "$c")" || continue
    if rum_go_acceptable "$v"; then
      if [[ "$tag" == "P" ]]; then
        RUM_GO_BIN="$c"; RUM_GO_VER="$v"; RUM_GO_SOURCE="local install"
        break
      fi
      if [[ -z "$lbest" ]] || rum_ver_ge "$v" "$lbest_ver"; then lbest="$c"; lbest_ver="$v"; fi
    elif rum_ver_ge "$v" "$GO_MIN"; then
      if [[ -z "$RUM_GO_SPARE_BIN" ]] || rum_ver_ge "$v" "$RUM_GO_SPARE_VER"; then
        RUM_GO_SPARE_BIN="$c"; RUM_GO_SPARE_VER="$v"
      fi
    else
      if [[ -z "$RUM_GO_OLDER_BIN" ]] || rum_ver_ge "$v" "$RUM_GO_OLDER_VER"; then
        RUM_GO_OLDER_BIN="$c"; RUM_GO_OLDER_VER="$v"
      fi
    fi
  done <<EOF
$(rum_go_candidates)
EOF
  if [[ -z "$RUM_GO_BIN" && -n "$lbest" ]]; then
    RUM_GO_BIN="$lbest"; RUM_GO_VER="$lbest_ver"; RUM_GO_SOURCE="local install"
  fi
  [[ -n "$RUM_GO_BIN" ]]
}

# go_meets_min — a suitable Go has been selected (kept for the scripts' plan lines).
go_meets_min() { [[ -n "$RUM_GO_BIN" ]]; }

# rum_activate_go BIN VER — put BIN first on PATH and forbid toolchain downloads.
rum_activate_go() {
  local bin="$1" ver="$2" dir
  dir="$(dirname "$bin")"
  case "$PATH" in
    "$dir":*) ;;
    *) PATH="$dir:$PATH" ;;
  esac
  # With the default GOTOOLCHAIN=auto, a newer go.mod 'go'/'toolchain' line makes
  # Go silently download a toolchain from Google, which fails where Google is
  # blocked. Local only: a Go that meets the minimum never needs one.
  export PATH GOTOOLCHAIN=local
  hash -r 2>/dev/null || true
  RUM_GO_BIN="$bin"; RUM_GO_VER="$ver"
  log "using go $ver at $bin (GOTOOLCHAIN=local)"
}

# rum_go_report — one-line verdict for the preflight output.
rum_go_report() {
  if [[ -n "$RUM_GO_BIN" ]]; then
    ok "Go ${RUM_GO_VER} found at ${RUM_GO_BIN} (go.mod needs >= ${GO_MIN}); no download needed"
  elif [[ -n "$RUM_GO_SPARE_BIN" ]]; then
    warn "Local Go ${RUM_GO_SPARE_VER} is newer than go.mod's ${GO_MIN} line; the desktop build needs ${GO_MIN%.*}.x"
  elif [[ -n "$RUM_GO_OLDER_BIN" ]]; then
    warn "Local Go ${RUM_GO_OLDER_VER} at ${RUM_GO_OLDER_BIN} is older than the ${GO_MIN} that go.mod needs"
  else
    info "No local Go found (looked on PATH and in common install locations)"
  fi
}

# ---------------------------------------------------------------------------
# Downloads
# ---------------------------------------------------------------------------

rum_sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then openssl dgst -sha256 "$1" | awk '{print $NF}'
  else return 1
  fi
}

# rum_fetch URL DEST [tries] [max-seconds] — resumable download to DEST.part, then
# an atomic rename. HTTP errors (403 from a blocked mirror, 404) fail at once so
# the caller can move to the next mirror; timeouts and resets resume and retry.
rum_fetch() {
  local url="$1" dest="$2" tries="${3:-${RUM_FETCH_TRIES:-2}}" maxt="${4:-1200}"
  local part="${dest}.part" cto="${RUM_CURL_CONNECT_TIMEOUT:-10}" n=1 st=0 quiet="-sS"
  mkdir -p "$(dirname "$dest")"
  [[ -t 2 ]] && quiet="--progress-bar"
  while :; do
    if have_curl; then
      set +e
      curl -fL $quiet --connect-timeout "$cto" --max-time "$maxt" --speed-limit 2048 --speed-time 30 \
        -C - -o "$part" "$url" 2>>"$LOG_FILE"
      st=$?
      set -e
      if (( st == 33 )); then rm -f "$part"; fi   # server cannot resume: start over
    elif have_wget; then
      set +e
      wget -c -q -O "$part" --timeout="$cto" --tries=1 "$url" 2>>"$LOG_FILE"
      st=$?
      set -e
    else
      warn "Need curl or wget to download files"
      return 2
    fi
    if (( st == 0 )); then mv -f "$part" "$dest"; return 0; fi
    # curl 22 / wget 8: HTTP error status. Retrying a 403/404 never helps.
    if (( st == 22 )) && have_curl; then rm -f "$part"; return "$st"; fi
    if (( n >= tries )); then rm -f "$part"; return "$st"; fi
    log "fetch retry $n/$tries for $url (exit $st)"
    n=$((n + 1))
    sleep "$n" 2>/dev/null || true
  done
}

# rum_http_ok URL [seconds] — a plain GET succeeds quickly (proxies often reject HEAD).
rum_http_ok() {
  local url="$1" t="${2:-8}"
  if have_curl; then
    curl -fsSL -o /dev/null --connect-timeout 5 --max-time "$t" "$url" 2>/dev/null
  elif have_wget; then
    wget -q -O /dev/null --timeout="$t" --tries=1 "$url" 2>/dev/null
  else
    return 1
  fi
}

# rum_probe_many URL... — print the URLs that answer, in the order given, probing in parallel.
rum_probe_many() {
  local dir i=0 u
  dir="$(mktemp -d "${WORK_DIR:-${TMPDIR:-/tmp}}/probe.XXXXXX")"
  for u in "$@"; do
    ( rum_http_ok "$u" 7 && : >"$dir/$i" ) &
    i=$((i + 1))
  done
  wait
  i=0
  for u in "$@"; do
    if [[ -e "$dir/$i" ]]; then printf '%s\n' "$u"; fi
    i=$((i + 1))
  done
  rm -rf "$dir"
}

# Mirrors for a release file, best first. RUM_GO_MIRROR (your own) leads.
rum_go_urls() {
  local file="$1"
  if [[ -n "${RUM_GO_MIRROR:-}" ]]; then printf '%s/%s\n' "${RUM_GO_MIRROR%/}" "$file"; fi
  printf '%s/%s\n' "${GO_DL_BASE%/}" "$file"
  printf '%s/%s\n' "${GO_GOOGLE_DL%/}" "$file"
  if [[ "${RUM_GO_SKIP_EXTRA_MIRRORS:-0}" != "1" ]]; then
    printf '%s\n' "https://mirrors.aliyun.com/golang/$file" \
      "https://mirrors.ustc.edu.cn/golang/$file" \
      "https://golang.google.cn/dl/$file" \
      "${GO_FALLBACK_DL%/}/$file"
  fi
}

# sha256 for FILE from a go.dev-style release index (?mode=json). The index is
# the only authority we have, so try every index we can reach.
rum_go_sha_lookup() {
  local file="$1" base json n=0 sha=""
  for base in ${RUM_GO_MIRROR:+"${RUM_GO_MIRROR%/}"} "${GO_DL_BASE%/}" "https://golang.google.cn/dl"; do
    n=$((n + 1))
    json="$WORK_DIR/go-index-$n.json"
    if [[ ! -s "$json" ]]; then
      rum_fetch "$base/?mode=json&include=all" "$json" 1 90 >/dev/null 2>&1 || continue
    fi
    # Pretty-printed or compact JSON: one field per line, then pair filename -> sha256.
    sha="$(tr ',{}' '\n\n\n' <"$json" | awk -v f="$file" '
      /"filename"/ { gsub(/.*"filename"[ \t]*:[ \t]*"|".*/, ""); cur = $0; next }
      /"sha256"/   { if (cur == f) { gsub(/.*"sha256"[ \t]*:[ \t]*"|".*/, ""); print; exit } }
    ' 2>/dev/null || true)"
    if [[ "${#sha}" -eq 64 && "$sha" != *[!0-9a-f]* ]]; then printf '%s' "$sha"; return 0; fi
    sha=""
  done
  # Last resort: a plain FILE.sha256 sidecar (not every host serves one).
  local u body
  for u in $(rum_go_urls "$file"); do
    body="$WORK_DIR/sidecar.sha256"
    rum_fetch "$u.sha256" "$body" 1 20 >/dev/null 2>&1 || continue
    sha="$(awk '{print $1; exit}' "$body" 2>/dev/null || true)"
    rm -f "$body"
    if [[ "${#sha}" -eq 64 && "$sha" != *[!0-9a-f]* ]]; then printf '%s' "$sha"; return 0; fi
  done
  return 1
}

# ---------------------------------------------------------------------------
# Module-proxy toolchain route (last resort; needs an existing, older go)
# ---------------------------------------------------------------------------

# Toolchains are also published as modules (golang.org/toolchain), a different
# network path than the tarball hosts. The go command downloads and verifies it
# against the checksum database, which a raw fetch of the zip could not.
rum_go_from_module_proxy() {
  local ver="$1" extract="$2" root="" gobin
  gobin="${RUM_GO_OLDER_BIN:-${RUM_GO_BIN:-$(command -v go 2>/dev/null || true)}}"
  [[ -n "$gobin" && -x "$gobin" ]] || { warn "No existing Go to bootstrap the module-proxy route."; return 1; }
  info "Trying the Go module proxy for go${ver} (golang.org/toolchain)…"
  root="$(cd / && GOTOOLCHAIN="go${ver}" GOSUMDB="$GO_SUMDB_DEFAULT" GOPRIVATE="" GONOSUMDB="" GOFLAGS="" \
    "$gobin" env GOROOT 2>>"$LOG_FILE")" || { warn "Module proxy did not provide go${ver}."; return 1; }
  [[ -n "$root" && -x "$root/bin/go" ]] || return 1
  "$root/bin/go" version 2>/dev/null | grep -q "go${ver} " || return 1
  mkdir -p "$extract/go"
  cp -a "$root/." "$extract/go/" || return 1
  chmod -R u+w "$extract/go" 2>/dev/null || true
  [[ -x "$extract/go/bin/go" ]]
}

# ---------------------------------------------------------------------------
# Downloading + installing Go
# ---------------------------------------------------------------------------

# Release names to try for the go.mod minimum: "1.25" means go1.25.0 since 1.21.
rum_go_release_names() {
  local n minor patch re='^([0-9]+)\.([0-9]+)(\.([0-9]+))?$'
  if [[ "$GO_MIN" =~ $re ]]; then
    minor="${BASH_REMATCH[2]}"; patch="${BASH_REMATCH[4]}"
    if [[ -z "$patch" ]]; then
      if (( minor >= 21 )); then printf '%s.0\n' "$GO_MIN"; fi
      printf '%s\n' "$GO_MIN"
      return 0
    fi
  fi
  n="$GO_MIN"; printf '%s\n' "$n"
}

rum_go_install_root() {
  if [[ -n "${RUM_GO_INSTALL_DIR:-}" ]]; then printf '%s' "$RUM_GO_INSTALL_DIR"; return; fi
  if [[ "$RUM_GO_OS" == "linux" ]] && declare -F can_root >/dev/null && can_root; then
    printf '%s' /usr/local/go
  else
    printf '%s' "$HOME/.local/go"
  fi
}

# rum_swap_into_place SRC DEST — replace DEST with SRC without ever leaving a
# half-written toolchain (rename, then drop the old one).
rum_swap_into_place() {
  local src="$1" dest="$2" parent sudo=""
  parent="$(dirname "$dest")"
  mkdir -p "$parent" 2>/dev/null || true
  if [[ ! -w "$parent" ]]; then
    declare -F as_root >/dev/null || return 1
    sudo="as_root"
  fi
  $sudo mkdir -p "$parent" || return 1
  $sudo rm -rf "$dest.new" "$dest.old"
  $sudo mv "$src" "$dest.new" || return 1
  if [[ -e "$dest" ]]; then $sudo mv "$dest" "$dest.old" || return 1; fi
  if ! $sudo mv "$dest.new" "$dest"; then
    [[ -e "$dest.old" ]] && $sudo mv "$dest.old" "$dest"
    return 1
  fi
  $sudo rm -rf "$dest.old"
}

rum_go_failure_help() {
  local ver="$1" arch="${OS_ARCH:-amd64}"
  err "Could not obtain Go ${ver}. Nothing was changed."
  err "Any one of these fixes it, then re-run this script:"
  err "  - install Go ${GO_MIN}+ yourself and put it on PATH (or: RUM_GO_ROOT=/path/to/go)"
  err "  - download go${ver}.${RUM_GO_OS}-${arch}.tar.gz from a mirror you can reach, extract it, set RUM_GO_ROOT"
  err "    e.g. https://mirrors.aliyun.com/golang/  or  https://mirrors.ustc.edu.cn/golang/"
  err "  - point at your own mirror: RUM_GO_MIRROR=https://host/path  (serves host/path/go${ver}.${RUM_GO_OS}-${arch}.tar.gz)"
}

# rum_install_go_download — fetch, verify and install Go for GO_MIN. Returns 1
# (after explaining) on failure instead of dying, so the GUI can fall back.
rum_install_go_download() {
  local arch="${OS_ARCH:-amd64}" ver file sha="" dest="" extract route="" verified=0 u names got=""
  if [[ "${RUM_GO_NO_DOWNLOAD:-0}" == "1" ]]; then
    err "RUM_GO_NO_DOWNLOAD=1 is set and no suitable local Go exists."
    rum_go_failure_help "$GO_MIN"
    return 1
  fi
  if declare -F pre_go_download >/dev/null; then pre_go_download || return 1; fi
  if ! have_curl && ! have_wget; then
    err "Need curl or wget to download Go."
    rum_go_failure_help "$GO_MIN"
    return 1
  fi
  command -v tar >/dev/null 2>&1 || { err "Need tar to unpack Go."; return 1; }

  extract="$WORK_DIR/go-extract"
  names="$(rum_go_release_names)"
  for ver in $names; do
    file="go${ver}.${RUM_GO_OS}-${arch}.tar.gz"
    dest="$WORK_DIR/$file"
    if [[ "$DRY_RUN" -eq 1 ]]; then
      info "dry-run: would download ${file} from: $(rum_go_urls "$file" | tr '\n' ' ')"
      return 0
    fi
    info "Downloading Go ${ver} (${file})"
    sha="$(rum_go_sha_lookup "$file" || true)"
    if [[ -n "$sha" ]]; then log "go sha256 $file $sha"; else warn "No official sha256 reachable for ${file}."; fi
    for u in $(rum_go_urls "$file"); do
      if rum_fetch "$u" "$dest"; then
        if [[ -z "$sha" ]]; then route="tarball"; break; fi
        got="$(rum_sha256_file "$dest" || true)"
        if [[ -z "$got" ]]; then warn "No sha256 tool found; cannot verify ${file}."; route="tarball"; break; fi
        if [[ "$got" == "$sha" ]]; then route="tarball"; verified=1; break; fi
        warn "Checksum mismatch from ${u%/*} (expected ${sha:0:12}…, got ${got:0:12}…); trying the next mirror"
        rm -f "$dest"
      else
        warn "Unavailable: ${u%/*}"
      fi
    done
    if [[ -n "$route" ]]; then break; fi
  done

  if [[ -z "$route" ]]; then
    warn "Every Go download mirror failed."
    if [[ -n "${RUM_GO_OLDER_BIN}${RUM_GO_BIN}" ]]; then
      rm -rf "$extract"; mkdir -p "$extract"
      ver="${names%%$'\n'*}"
      if rum_go_from_module_proxy "$ver" "$extract"; then route="module proxy"; verified=1; fi
    fi
  fi
  if [[ -z "$route" ]]; then rum_go_failure_help "$ver"; return 1; fi

  if [[ "$route" == "tarball" ]]; then
    rm -rf "$extract"; mkdir -p "$extract"
    tar -C "$extract" -xzf "$dest" || { err "The downloaded archive is corrupt."; return 1; }
  fi
  [[ -x "$extract/go/bin/go" ]] || { err "The downloaded Go did not contain bin/go."; return 1; }

  if (( verified == 1 )); then
    ok "Go ${ver} obtained via the ${route} and verified."
  else
    warn "Go ${ver} obtained via the ${route} but could NOT be verified: no official sha256 was reachable."
    if [[ "${RUM_REQUIRE_VERIFIED_GO:-0}" == "1" ]]; then
      err "RUM_REQUIRE_VERIFIED_GO=1 is set — refusing to install an unverified Go."
      return 1
    fi
    warn "Continuing. Set RUM_REQUIRE_VERIFIED_GO=1 to make this fatal."
  fi

  local goroot
  goroot="$(rum_go_install_root)"
  info "Installing Go to $goroot"
  rum_swap_into_place "$extract/go" "$goroot" || { err "Cannot write $goroot (set RUM_GO_INSTALL_DIR to a writable directory)."; return 1; }
  # shellcheck disable=SC2034  # read by the sourcing scripts
  RUM_GO_DOWNLOADED="$goroot"
  rum_activate_go "$goroot/bin/go" "$ver"
  if declare -F ensure_path_in_rc >/dev/null; then ensure_path_in_rc "$goroot/bin"; fi
  RUM_GO_SOURCE="downloaded to $goroot"
  ok "Installed $("$goroot/bin/go" version) at $goroot"
}

# _rum_try_pkg_go — distro package via the script's pkg_install_go hook. Returns 0
# when that yielded a Go we can use (a newer-minor spare counts in GUI mode).
_rum_try_pkg_go() {
  declare -F pkg_install_go >/dev/null || return 1
  pkg_install_go || return 1
  hash -r 2>/dev/null || true
  rum_select_local_go || true
  if [[ -n "$RUM_GO_BIN" ]]; then
    rum_activate_go "$RUM_GO_BIN" "$RUM_GO_VER"
    ok "Installed Go ${RUM_GO_VER} via the system package manager"
    return 0
  fi
  return 1
}

# rum_ensure_go — the whole policy in one place. Local Go first; then the distro
# package (hook); then mirrors. Dies only when nothing worked.
rum_ensure_go() {
  rum_select_local_go || true
  if [[ -n "$RUM_GO_BIN" ]]; then
    rum_activate_go "$RUM_GO_BIN" "$RUM_GO_VER"
    ok "Using local Go ${RUM_GO_VER} (${RUM_GO_BIN}); GOTOOLCHAIN=local, nothing to download"
    return 0
  fi
  if [[ -n "$RUM_GO_OLDER_BIN" ]]; then
    warn "Local Go ${RUM_GO_OLDER_VER} (${RUM_GO_OLDER_BIN}) is too old: go.mod needs >= ${GO_MIN}. Fetching a newer one; the old one is left untouched."
  elif [[ -n "$RUM_GO_SPARE_BIN" ]]; then
    info "Local Go ${RUM_GO_SPARE_VER} is newer than go.mod's line; trying to fetch the exact ${GO_MIN} for Wails"
  else
    info "No local Go found; ${GO_MIN}+ is needed"
  fi
  if [[ "$DRY_RUN" -eq 1 ]]; then
    info "dry-run: would install Go ${GO_MIN} (mirrors: go.dev, dl.google.com, aliyun, ustc, golang.google.cn)"
    return 0
  fi
  if [[ "$RUM_GO_SAME_MINOR" != "1" ]] && _rum_try_pkg_go; then return 0; fi
  if rum_install_go_download; then return 0; fi
  # GUI: a newer local or packaged Go beats no Go. Wails may reject it, so say so.
  if [[ -z "$RUM_GO_SPARE_BIN" && "$RUM_GO_SAME_MINOR" == "1" ]]; then
    declare -F pkg_install_go >/dev/null && pkg_install_go && { hash -r 2>/dev/null || true; rum_select_local_go || true; }
  fi
  if [[ -n "$RUM_GO_SPARE_BIN" ]]; then
    warn "Using Go ${RUM_GO_SPARE_VER} (${RUM_GO_SPARE_BIN}); if 'wails build' fails typechecking, install Go ${GO_MIN} (see above)."
    rum_activate_go "$RUM_GO_SPARE_BIN" "$RUM_GO_SPARE_VER"
    RUM_GO_SOURCE="$RUM_GO_SPARE_BIN"
    return 0
  fi
  if [[ -n "$RUM_GO_BIN" ]]; then rum_activate_go "$RUM_GO_BIN" "$RUM_GO_VER"; return 0; fi
  die "No usable Go ${GO_MIN}+ (see the options above)."
}

# ---------------------------------------------------------------------------
# Module proxy, checksum DB, npm registry
# ---------------------------------------------------------------------------

rum_go_env() { (cd / && GOTOOLCHAIN=local "${RUM_GO_BIN:-go}" env "$1" 2>/dev/null) || true; }

# configure_goproxy — respect anything the user set; otherwise probe and, only
# if the default proxy is unreachable, use a fallback chain.
configure_goproxy() {
  local cur="" sumdb="" ok_list="" chain="" u
  if [[ -n "$MIRROR" ]]; then
    if [[ "$MIRROR" == "$DEFAULT_MIRROR" ]]; then export GOPROXY="$BUILTIN_GOPROXY"; else export GOPROXY="${MIRROR}|${BUILTIN_GOPROXY}"; fi
    export GOSUMDB="$GO_SUMDB_DEFAULT"
    RUM_PROXY_NOTE="--mirror chain"
    ok "Using Go module proxy chain: $GOPROXY"
    return 0
  fi
  cur="${GOPROXY:-}"
  [[ -n "$cur" ]] || cur="$(rum_go_env GOPROXY)"
  if [[ -n "$cur" && "$cur" != "https://proxy.golang.org,direct" ]]; then
    RUM_PROXY_NOTE="GOPROXY from your environment"
    ok "Using your GOPROXY: $cur"
    return 0
  fi
  if ! have_curl && ! have_wget; then return 0; fi
  if rum_http_ok "https://proxy.golang.org/github.com/google/uuid/@v/list" 6; then
    RUM_PROXY_NOTE="proxy.golang.org"
    ok "Go module proxy reachable (proxy.golang.org); default GOPROXY kept"
  else
    warn "proxy.golang.org is unreachable (Google is blocked here?); probing fallback module proxies…"
    # shellcheck disable=SC2046
    ok_list="$(rum_probe_many $(printf '%s\n' "$RUM_GOPROXY_CANDIDATES" | awk 'NF{printf "%s/github.com/google/uuid/@v/list ", $1}'))"
    for u in $ok_list; do chain="${chain:+$chain|}${u%/github.com/google/uuid/@v/list}"; done
    if [[ -z "$chain" ]]; then
      warn "None of the fallback proxies answered a quick probe; trying them all anyway"
      export GOPROXY="$BUILTIN_GOPROXY"
    else
      export GOPROXY="${chain}|direct"
    fi
    RUM_PROXY_NOTE="fallback chain"
    ok "Using fallback GOPROXY: $GOPROXY"
  fi
  # Checksum DB: keep verification on. Only swap to Google's China-hosted mirror
  # of the same database, and only if the user has not chosen one.
  sumdb="${GOSUMDB:-}"
  [[ -n "$sumdb" ]] || sumdb="$(rum_go_env GOSUMDB)"
  if [[ -n "${RUM_GO_SUMDB:-}" ]]; then
    export GOSUMDB="$RUM_GO_SUMDB"
  elif [[ -z "$sumdb" || "$sumdb" == "sum.golang.org" ]]; then
    if ! rum_http_ok "https://sum.golang.org/latest" 6 && rum_http_ok "https://sum.golang.google.cn/latest" 6; then
      export GOSUMDB="sum.golang.org https://sum.golang.google.cn"
      ok "sum.golang.org is unreachable; verifying via sum.golang.google.cn (same database)"
    fi
  fi
}

# configure_npm — keep the user's registry; fall back to a mirror only if the
# default one cannot be reached.
configure_npm() {
  local reg
  command -v npm >/dev/null 2>&1 || return 0
  if [[ -n "${NPM_CONFIG_REGISTRY:-}${npm_config_registry:-}" ]]; then return 0; fi
  reg="$(npm config get registry 2>/dev/null || true)"
  case "$reg" in ""|"https://registry.npmjs.org"|"https://registry.npmjs.org/") ;; *) ok "Using your npm registry: $reg"; return 0 ;; esac
  if rum_http_ok "https://registry.npmjs.org/" 8; then return 0; fi
  reg="${RUM_NPM_REGISTRY:-https://registry.npmmirror.com}"
  if rum_http_ok "$reg" 8; then
    export NPM_CONFIG_REGISTRY="$reg"
    warn "registry.npmjs.org is unreachable; using $reg for this run only (your npm config is untouched)"
  else
    warn "No npm registry answered; npm steps may fail"
  fi
}

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------

# rum_preflight_space DIR NEED_KB — warn (never fail) when free space looks low.
rum_preflight_space() {
  local dir="$1" need="$2" free=""
  while [[ ! -d "$dir" && "$dir" != "/" && "$dir" != "." ]]; do dir="$(dirname "$dir")"; done
  free="$(df -Pk "$dir" 2>/dev/null | awk 'NR==2 {print $4}' || true)"
  if [[ -n "$free" && "$free" -lt "$need" ]]; then
    warn "Low disk space on $dir ($((free / 1024)) MiB free; about $((need / 1024)) MiB recommended)"
  fi
  return 0
}

# rum_preflight_writable DIR HINT — DIR (or its nearest existing parent) must be
# writable, or passwordless sudo must be available; fail early rather than after
# a long build.
rum_preflight_writable() {
  local d="$1" hint="$2"
  while [[ ! -d "$d" && "$d" != "/" && "$d" != "." ]]; do d="$(dirname "$d")"; done
  if [[ -w "$d" ]]; then return 0; fi
  if declare -F can_root >/dev/null && can_root; then return 0; fi
  die "Cannot write to $1 and there is no passwordless sudo. ${hint}"
}

# rum_proxy_fallback — after a failed Go network step, switch once to the
# built-in mirror chain. Never overrides a GOPROXY the user chose. Returns 1
# when there is nothing new to try.
rum_proxy_fallback() {
  [[ "$RUM_PROXY_NOTE" == "GOPROXY from your environment" ]] && return 1
  [[ "${GOPROXY:-}" == "$BUILTIN_GOPROXY" ]] && return 1
  case "${GOPROXY:-}" in *"${IRANIAN_GOPROXY:-@none@}"*) return 1 ;; esac
  export GOPROXY="$BUILTIN_GOPROXY"
  RUM_PROXY_NOTE="built-in fallback chain"
  return 0
}

# rum_print_summary — what was used, for the end of the run.
rum_print_summary() {
  info "Go: ${RUM_GO_VER:-?} at ${RUM_GO_BIN:-?} (${RUM_GO_SOURCE:-local}); GOTOOLCHAIN=${GOTOOLCHAIN:-unset}"
  if [[ -n "$RUM_PROXY_NOTE" ]]; then info "Go modules via: ${RUM_PROXY_NOTE} (GOPROXY=${GOPROXY:-default})"; fi
}
