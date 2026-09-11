# Rum installer hardening — session report

Written 2026-09-07 for a cold reader. This is **untracked working notes**, not a
release document. Do not `git add` it.

Work lived in: `installers/**`, `build-linux.sh`, `build-windows.sh`,
`backend/install.sh`, `backend/install.ps1`, `backend/cmd/rum/build.sh`,
`INSTALL.md`, `installers/README.md`. Frontend and `backend/internal/**` were
owned by another agent and were not touched.

User-facing docs (`INSTALL.md`, `installers/README.md`) were rewritten
against the current scripts (flags, auto vs not, GUI Go pin,
`GOTOOLCHAIN=local`, mirror UNKNOWN). Do not treat older copies as truth.

Host constraint (late in the session): this is the user's live machine
(54 running containers, 464 volumes, `/` at ~93% / ~12–13 GB free, 137 GB
Docker build cache). No `docker system/volume/image/container/builder prune`,
no stop/rm/kill of containers we did not create, no `docker build`. Tests used
`docker run --rm` against **already-local** images. A later batch was supposed
to be sequential with `--memory=2g` and `--name rum-test-*`; earlier batches
ran in parallel before that rule existed.

---

## 1. Matrix

Legend:

- **PASS** — we saw the container finish the claimed path and print the
  success marker (or the expected refusal).
- **FAILED** — the *installer* failed, with the real reason.
- **NOT TESTED** — we did not see that cell succeed. Includes “prior batch
  passed, then the script changed, and we did not re-run”.

### 1.1 CLI installer (`installers/cli/install-linux.sh`)

| Distro | Image | Root | Non-root |
|---|---|---|---|
| Debian 12 | `debian:12` (local 185 MB) | **PASS** (first batch, see caveat A) | **PASS** (second batch logs) |
| Fedora | `fedora:latest` (local 276 MB) | **PASS** (first batch, caveat A) | **PASS** (second batch logs) |
| Arch | `archlinux:latest` (local 572 MB) | **PASS** (first batch, caveat A) | **PASS** (second batch logs) |
| Alpine | `alpine:latest` (local 13 MB) | **PASS** (first batch). **FAILED** then **PASS** after trap rewrite: see bugs 23–24. Final re-run `rum-test-cli-root-alpine --memory=2g`: apk `go` 1.26.8, `/usr/local/bin/rum`, second run `Found go version go1.26.8`, `CLI_ROOT_OK`, EXIT=0, host still 12 GB free. | **PASS\*** without Go: expected musl refusal. **PASS** with apk Go preinstalled (skip tarball, rum → `~/.local/bin`). |
| openSUSE Tumbleweed | `opensuse/tumbleweed:latest` (local 166 MB) | **PASS** (first batch, caveat A) | **PASS** (second attempt; first attempt was harness-only, see 1.4) |

**Caveat A (CLI root):** debian/fedora/arch/opensuse root **PASS** cells are
from the **first batch**, before the EXIT-trap rewrite, PATH persist,
ERR-trap `if !` wrap, and `-buildvcs=false`. Alpine root **was re-run**
after all of those and **PASS**ed. Do not treat debian/fedora/arch/opensuse
root as re-verified on the current tree.

**Caveat B (debian:12-slim):** **NOT TESTED**. The later disk rule asked to
prefer `debian:12-slim`; that tag was not local and we did not pull it.
Tests used already-local `debian:12`.

### 1.2 What each CLI PASS actually showed

**Root, first batch (pre trap-rewrite):**

- **debian:12** — `pkg=apt`. Distro `golang` is 1.19.8, below `go.mod` 1.25.7.
  Installer then fetched published toolchain **go1.27.1** (not go1.25.7 — that
  language version is not a published `go.dev/dl` archive). go.dev +
  dl.google.com returned HTTP 404 (curl 22, capped at 2 retries). Aliyun
  served the 67 MB tarball. Installed Go to `/usr/local/go`, rum to
  `/usr/local/bin/rum`. Second run rebuilt rum, exit 0 (`CLI_ROOT_OK`).
- **fedora** — `pkg=dnf`. `golang` 1.26.7 from dnf, **skipped tarball**.
  Second run: `All build prerequisites already present` / `Found go version`.
- **arch** — `pkg=pacman`. Distro `go` 1.27, skipped tarball.
- **alpine** — `pkg=apk` after `apk add bash`. Distro `go` 1.26.8, skipped
  tarball. Official glibc tarball is not used when apk Go meets min.
- **opensuse** — `pkg=zypper`. Distro `go` 1.27.0. First attempt died because
  the image has no `awk`; installer was made awk-free (`read` for go.mod,
  sha256/df without awk) and then passed.

**Non-root, second batch (post PATH persist; logs under `/tmp/rum-matrix2/`):**

All five installed rum to `/home/rumuser/.local/bin/rum` and printed:

```
User-local install: /home/rumuser/.local/bin/rum
PATH for this shell:  export PATH="/home/rumuser/.local/bin:$PATH"
```

Go (when the tarball path ran) went to `/home/rumuser/.local/go`. `.bashrc`
and `.profile` got `export PATH=...` lines. Second run: `Found go version
go1.27.1` (or `go1.26.8` on Alpine with apk Go) — **no 67 MB re-download**.

- **debian/fedora/arch/opensuse non-root:** no system Go (or too old / no
  sudo to install it) → tarball to `~/.local/go`.
- **alpine non-root without Go (first batch):** **PASS as designed refusal** —
  musl, no root, glibc tarball would not work. Message:
  `sudo apk add go curl ca-certificates bash`.
- **alpine non-root with apk Go (second batch):** `Found go version go1.26.8`,
  built rum, second run `All build prerequisites already present`. Marker
  `ALPINE_USER_GO_OK`.

### 1.3 GUI installer (`installers/gui/install-linux.sh`)

| Distro | `--dry-run` detection + plan | Full Wails build | gtk/webkit packages actually installed |
|---|---|---|---|
| debian:12 | **PASS** (`GUI_DRY_OK`) | **NOT TESTED** (too heavy; not claimed) | **NOT TESTED** (names queried via `apt-cache policy` only) |
| fedora | **PASS** | **NOT TESTED** | **NOT TESTED** (`dnf info` only) |
| arch | **PASS** | **NOT TESTED** | **NOT TESTED** (`pacman -Ss` only) |
| alpine | **PASS** | **NOT TESTED** | **NOT TESTED** (`apk search` only) |
| opensuse | **PASS** | **NOT TESTED** | **NOT TESTED** (`zypper search` only) |

Dry-run on each printed `pkg=apt|dnf|pacman|apk|zypper`, plan
`Go 1.25.7+ Node 18 npm git gcc pkg-config GTK3+WebKit2GTK wails v2.12.0`.
That is **not** a GUI install.

After the Alpine/openSUSE WebKit **name** fix (section 2), dry-run was not
re-run. Dry-run does not call `try_pkg_install`, so the plan text is the same;
the name fix only matters on a real GUI install, which was never done.

### 1.4 Harness noise — do not confuse with installer failures

**Exit 2 from `/tmp/rum-verify2.sh` (debian-user, fedora-user, arch-user,
alpine-user-go, arch-gui):** the host harness script was **edited while those
`bash /tmp/rum-verify2.sh …` processes were still running**. Bash re-read the
file after `docker run` returned and hit a syntax error:

```
DOCKER_OK fedora-cli-user
/tmp/rum-verify2.sh: line 224: syntax error near unexpected token `;;'
```

The **containers had already succeeded**. Recovered from logs:

- `/tmp/rum-matrix2/debian-cli-user.log` → `CLI_USER_OK` + `DOCKER_OK`
- `/tmp/rum-matrix2/fedora-cli-user.log` → `CLI_USER_OK` + `DOCKER_OK`
- `/tmp/rum-matrix2/arch-cli-user.log` → `CLI_USER_OK` + `DOCKER_OK`
- `/tmp/rum-matrix2/alpine-cli-user-go.log` → `ALPINE_USER_GO_OK` + `DOCKER_OK`
- GUI dry logs → `GUI_DRY_OK` + `DOCKER_OK`

**openSUSE non-root first attempt, docker/harness exit 104:** **not an
installer bug**. `zypper install sudo` failed (`No provider of 'sudo' found`,
and `download.opensuse.org` DNS failed on that pull of repo metadata). The
installer never started. Re-run used `runuser -u rumuser` (no sudo package)
and the installer **PASS**ed (`CLI_USER_OK`).

**First-batch fedora `cli-user` FAIL in `/tmp/rum-matrix/SUMMARY.txt`:**
harness passed `as_rumuser 'cmd --yes'` into GNU `su`, which ate `--yes`
(`su: unrecognized option '--yes'`), then `su` password prompt /
`Authentication token manipulation error`. **Installer not at fault.**
Working drop: `sudo -n -u rumuser -- /bin/bash -lc "$cmd"` (or `runuser`).
A dedicated fedora user run then **PASS**ed (`FEDORA_USER_OK` /
later `CLI_USER_OK`).

**First-batch fedora `retry` FAIL in SUMMARY.txt:** retries **did** fire
against `127.0.0.1:1`, but Aliyun still had the tarball so the install
**succeeded** (EXIT=0). That is not fail-closed. Fail-closed was proven later
with `RUM_GO_SKIP_EXTRA_MIRRORS=1` (section 2).

**First-batch SIGINT EXIT=0:** SIGINT was sent too late (install already
done) and/or the EXIT trap ate 130. After the trap rewrite, fedora non-root
SIGINT during retry sleep: **`Interrupted`, EXIT=130, TEMP_CLEAN, NO_PARTIAL**.

### 1.5 Extra proofs (not a distro cell)

| Proof | Result |
|---|---|
| PowerShell `Parser::ParseFile` on all 3 `.ps1` in `mcr.microsoft.com/powershell:latest` | **PASS** (`PARSE OK` ×3) |
| PSScriptAnalyzer 1.21.0 (latest analyzer refused pwsh 7.4.2; 1.21.0 installed in the container) | **PASS** (`ANALYZE OK` ×3, `PS_ALL_OK`) |
| Idempotent second CLI run (non-root, 5 distros) | **PASS** — Go skipped, exit 0 |
| SIGINT mid-run (fedora non-root, dead Go URLs) | **PASS** — 130, temp dir gone, no `~/.local/bin/rum` |
| Retry fail-closed (fedora non-root, `RUM_RETRY_MAX=3`, all Go URLs `http://127.0.0.1:1`, `RUM_GO_SKIP_EXTRA_MIRRORS=1`) | **PASS** — attempts 1/3 and 2/3 `retrying…`, then `failed after 3 attempts (last exit 7)`, then `Could not download Go 1.25.7. Get it from https://go.dev/dl/ and re-run.` + `sudo dnf install -y golang`. EXIT=1. |
| macOS CLI/GUI (`install-macos.sh`) | **NOT TESTED** (Linux host, no Darwin VM) |
| Windows CLI/GUI (`.ps1` execution, winget/choco/WebView2) | **NOT TESTED** (parse + analyzer only; no Windows) |
| `backend/install.sh` / `build-linux.sh` wrappers | **NOT TESTED** as entry points (they `exec` the linux installers; those installers were tested directly) |
| `backend/cmd/rum/build.sh` | **NOT TESTED** in containers |
| `installers/uninstall-linux.sh` | **PASS** — `rum-test-uninstall-alpine --memory=2g` (local `alpine:latest`, no pull). CLI install → `/usr/local/bin/rum`; seeded `.desktop`, icon, autostart, `~/.config/rum/settings.json`, `~/Downloads/rum-keep-me.txt`. First uninstall removed binary + desktop + icon + autostart; **left** config and Downloads file; printed `Left your settings + history`. Second uninstall on the clean tree: `Nothing to remove`, EXIT=0, `UNINSTALL_OK`. `--purge` itself was **not** run. |
| Full GUI Wails build | **NOT TESTED** on any distro |

---

## 2. Bugs the containers actually exposed (and the fix)

### 2.1 First hardening pass (before the verification matrix)

These were found while writing/linting, then confirmed or refined in Docker.

1. **Help text leaked `set -Eeuo pipefail`.** `sed` range over the header
   was wrong. Help is now a comment-only reader (no `sed` range).
2. **Uninstall could delete the repo `Rum.desktop`.** Cwd wipe removed.
3. **`local` outside a function** in uninstall purge — fixed.
4. **Leftover `run_go_net env`** in GUI linux — removed.
5. **shellcheck** SC2034 / SC2155 / SC2015 cleaned; `bash -n` clean.

### 2.2 PowerShell (official `pwsh` image)

6. **Parse** was already OK after the hardening rewrite.
7. **PSScriptAnalyzer latest** refused pwsh **7.4.2** in
   `mcr.microsoft.com/powershell:latest`. Used **PSScriptAnalyzer 1.21.0**.
8. Analyzer findings that were real (not Write-Host / BOM):
   - `Write-Log` → `Write-RumLog` (approved verb)
   - `Refresh-SessionPath` → `Sync-SessionPath`
   - `Ensure-*` → `Install-RequiredGo` / `Install-RequiredNode` /
     `Install-WebView2Runtime` / `Install-WailsCli`
   - `New-Shortcut` → `Add-AppShortcut`
   - empty `catch` → `Write-Verbose`
   - `SupportsShouldProcess` on `Set-GoProxy` and `Add-AppShortcut`
   Write-Host kept as installer UX. Re-run: PARSE OK, no remaining real findings.

### 2.3 Runtime, from clean containers

9. **`download_file` under `set -e` aborted the whole script** on a failed
   URL, so fallback mirrors never ran. Now returns non-zero; the caller loops
   URLs.
10. **openSUSE has no `awk`.** `read_go_min` and disk-free used awk and
    died with 127. Now: bash `read` of `go.mod`; sha256/`df` without awk.
11. **`go1.25.7.linux-amd64.tar.gz` 404.** `go 1.25.7` in go.mod is the
    **language** version, not a published toolchain archive. Installer now
    reads `go.dev/dl/?mode=json` and picks a published
    `go1.X.Y.linux-<arch>.tar.gz` (observed **go1.27.1**).
12. **go.dev/dl and dl.google.com 404 from this network** (often curl 22).
    **Aliyun `https://mirrors.aliyun.com/golang/` returned 200** and served
    the 67 MB tarball. That URL is in the extra-mirror list, skippable with
    `RUM_GO_SKIP_EXTRA_MIRRORS=1`. HTTP 4xx (curl 22) retries capped at 2.
13. **Alpine non-root cannot use the official glibc tarball.** Die with
    `sudo apk add go …` instead of installing a binary that will not run.
    Root Alpine uses apk `go` (musl). Optional `gcompat` if a tarball is
    attempted as root.
14. **debian:12 has no curl.** Tarball path now installs
    `curl` + `ca-certificates` as root when missing. Non-root without curl
    dies with an actionable `sudo apt-get install -y curl ca-certificates`.
15. **`su --yes` / `su` password in user tests.** Not installer: harness.
    `sudo -n -u rumuser -- /bin/bash -lc` or `runuser`. util-linux `su`
    wants options **before** the username.
16. **`run_logged` re-enabled `set -e` then `return "$st"`**, which fired
    the ERR trap and aborted GOPROXY fallback. Now:
    `if "$@"; then return 0; else return 1; fi`.
17. **`retry()` same ERR-trap footgun** (`(( st == 0 ))` / `return "$st"`
    under `set -e`/`set -E`). Now `if "$@"; then return 0; else st=$?; fi`
    and `[ ]` tests. GUI linux retry was rewritten the same way.
18. **debian without curl made `reachable()` fail**, so GOPROXY was forced
    to `go.devneeds.ir` *before* curl existed, then `go mod download` failed.
    `reachable` now no-ops unless curl or wget exists; do not force the
    module mirror blindly.
19. **EXIT trap swallowed SIGINT 130.** `cleanup` used `return "$st"` and
    the shell exited 0. Now `trap - EXIT; exit "$st"`. INT/TERM handlers
    `on_int` / `on_term` call `exit 130` / `exit 143`. Proven: Interrupted,
    EXIT=130, temp dir removed.
20. **Second non-root run re-downloaded 67 MB of Go.** `~/.local/go/bin`
    was exported in-process after install, then `ensure_path_in_rc` saw it
    already on PATH and **did not write rc**. Next login/`bash -lc` had no
    Go. Fix: prepend `/usr/local/go/bin`, `~/.local/go/bin`, `~/.local/bin`
    at startup; persist PATH to `$SHELL_RC` **and** `~/.profile` even if
    this process already has the dir; always print PATH guidance for
    non-root. Second run then printed `Found go version go1.27.1`.
21. **Alpine GUI package `webkit2gtk-dev` does not exist** on current
    `alpine:latest`. Live `apk search`: `webkit2gtk-4.1-dev` (and 6.0, which
    is GTK4 — we must not use 6.0 for Wails GTK3). Installer now tries
    `webkit2gtk-4.1-dev` then `webkit2gtk-dev`.
22. **openSUSE GUI `webkit2gtk3-devel` is not in Tumbleweed repos.** Live
    `zypper search`: **`webkitgtk3-devel`**, runtime `libwebkit2gtk-4_1-0`.
    Installer now tries `webkit2gtk3-devel` then `webkitgtk3-devel` then
    `webkit2gtk4-devel` / `libwebkit2gtk-4_1-devel`.
23. **`set +e` does not silence the ERR trap** (`set -E` / errtrace).
    Alpine root re-run: `go build` failed, `on_err` printed
    `Failed at line 679`, then the GOPROXY retry ran **after `set -e`**,
    so the second `run_logged` killed the script (EXIT=1) instead of
    `die`. Fix: wrap `go mod download` / `go build` in `if ! …; then`
    with `|| die`. Proven: the next Alpine run printed
    `go build failed. Log: …` instead of a line-number abort.
24. **Alpine `go build` VCS stamp (`exit 128`).** Verbose log:
    `error obtaining VCS status: exit status 128` /
    `Use -buildvcs=false to disable VCS stamping.` The tree was `cp -a`
    from a bind mount; `git status` fails (often “dubious ownership”).
    GOPROXY retry cannot fix that. Installer now passes
    `-buildvcs=false` on `go build` (CLI linux, CLI macOS,
    `backend/cmd/rum/build.sh`). **Alpine root re-run after this flag:
    PASS** (`CLI_ROOT_OK`, `Rum v0.1.1`, second run skipped Go).
25. **Wails v2.12.0 cannot `wails build` on Go 1.26/1.27.** Relayed from a
    read-only audit. `go_meets_min` accepts any Go ≥ go.mod `1.25.7`.
    Wails v2.12.0 pins `golang.org/x/tools` v0.30.0, which cannot read
    Go 1.27 export data:
    `internal error: package "context" without types was imported from
    "github.com/tiredbooy/Rum"`. Predates this hardening work (`git archive
    HEAD`). Retry/mirror does not help.

26. **`GOTOOLCHAIN=go${GO_MIN}` + `GOSUMDB=off` is a hard fail** (relayed,
    measured in clean containers, fresh cache per row, default proxy,
    only GOSUMDB changing). Go prints:
    `download go1.25.7: golang.org/toolchain@v0.0.1-go1.25.7.linux-amd64:
    verifying module: checksum database disabled by GOSUMDB=off`.
    Also fails when the toolchain is **already in the module cache**.
    Variants that still FAIL: `GOFLAGS=-mod=mod`,
    `GOPRIVATE='golang.org/*'`, `GONOSUMDB='golang.org/*'`,
    `GONOSUMCHECK=1`, `GOPROXY=direct GOSUMDB=off`. Default (sumdb on)
    is OK. Our GUI scripts set `GOSUMDB=off` whenever a mirror is in play
    (`configure_goproxy` runs **before** the old GOTOOLCHAIN export), and
    auto-select the mirror when proxy.golang.org / go.dev / github.com
    look unreachable — the Iranian case. So the pin made mirror users
    fail **earlier** than the x/tools error.

    **Fix chosen:** do not switch toolchains through GOPROXY. If system
    Go is **newer** than `GO_MIN`, `pin_wails_go` / `Install-GoZip`
    installs **exactly** `go$GO_MIN` via the existing tarball/zip URL
    list, then `export GOTOOLCHAIN=local` (PS: `$env:GOTOOLCHAIN =
    "local"`) immediately before `wails build`. Files:
    `installers/gui/install-linux.sh`, `install-macos.sh`,
    `install-windows.ps1`, `build-windows.sh`. Avoids the sumdb question.
    Dropping `GOSUMDB=off` while keeping `GOPROXY=$MIRROR` was **not**
    chosen: that depends on the mirror proxying `sum.golang.org`, which
    could not be tested.

    **Linux/macOS/`build-windows.sh` pin: RESOLVED** (other agent, container:
    0 toolchain downloads, sumdb never consulted). **Windows was still
    broken** after that pin — see 28.

28. **Windows `Sync-SessionPath` put Machine PATH first**, wiping the
    session prepend from `Install-GoZip`. Sequence: pin Go into
    `%LOCALAPPDATA%\Go\bin` (prepended), then `Install-RequiredGo` called
    `Sync-SessionPath` again, which did
    `$env:Path = @($machine, $user, $env:Path)`. A machine-wide Go
    (`C:\Program Files\Go\bin` from winget/MSI) then won.
    `GOTOOLCHAIN=local` handed Wails the newer Go → original
    `internal error: package "context" without types`. Negative control
    by the other agent. One-line fix in
    `installers/gui/install-windows.ps1`:
    `$env:Path = @($env:Path, $machine, $user) -join ";"`.
    Parses clean. **Not functionally tested here (no Windows host).**

29. **Module-proxy fallback for the Go toolchain — IMPLEMENTED and verified.**

    Google's download hosts are blocked on this network:

    | URL | Result |
    |---|---|
    | `go.dev/dl/go1.25.7.linux-amd64.tar.gz` | HTTP 404 |
    | `dl.google.com/go/…` | HTTP 404 (every Go version, not just 1.25.7) |
    | `storage.googleapis.com/golang/…` | HTTP 403 |
    | `mirrors.aliyun.com/golang/…` | HTTP 206 — **works** |
    | `proxy.golang.org` toolchain module | HTTP 206 — **works** |

    **Correction to the earlier claim.** This was previously written up as
    "`install_go_tarball` cannot fetch go1.25.7 at all". That was measured
    before the Aliyun mirror was added to the URL list. Aliyun does work
    here, and serves the byte-correct release — its sha256 matches the
    official `12e6d6a191091ae27dc31f6efc630e3a3b8ba409baf3573d955b196fdf086005`
    exactly. So the tarball route is **not** dead on this network, and the
    module-proxy route is a genuine last resort rather than the unblocker.

    Implemented anyway, because Aliyun is itself blocked in some places and
    because it is the only *always-verified* route. `install_go_tarball` now
    ends with `install_go_from_module_proxy`, which resolves
    `golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>` through `GOPROXY`.

    Files: `installers/gui/install-linux.sh`, `installers/gui/install-macos.sh`,
    `installers/cli/install-linux.sh`, `installers/cli/install-macos.sh`,
    `build-windows.sh` (`go_toolchain_root` + `install_go_from_module_proxy`),
    and `installers/gui/install-windows.ps1`, `installers/cli/install-windows.ps1`
    (`Install-GoFromModuleProxy`).

    **Integrity — deliberately not a raw download.** The zip is fetched *by
    the `go` command*, not by curl, so it is verified against the checksum
    database. A raw fetch would be unverifiable: the sha256 published on
    go.dev is for the `.tar.gz`, a different artefact
    (`12e6d6a1…` vs the module zip's `43a6a446…`), and cannot validate it.
    The helper therefore forces `GOSUMDB=$GO_SUMDB_DEFAULT` (default
    `sum.golang.org`) for the fetch, so a mirror path's `GOSUMDB=off` cannot
    leak in and disable the only check this route has. `GOFLAGS`/`GOPRIVATE`
    are cleared so a caller cannot redirect it.

    **Layout is normalised, not assumed.** Verified: the module zip's entries
    are prefixed `golang.org/toolchain@v0.0.1-go1.25.7.linux-amd64/`, *not*
    `go/`, and the module cache is stored read-only (mode 555). The helper
    copies the tree to `$extract/go` and restores write permission, so the
    existing install/move/PATH code is identical for both routes.

    **Bootstrap requirement, stated at runtime.** The route needs an existing
    `go` to drive the download, so it cannot help a machine with no Go at all —
    it warns and returns rather than dying silently. It also re-checks that the
    fetched toolchain really reports the requested version, because a `go`
    older than 1.21 ignores `GOTOOLCHAIN` and would hand back its own GOROOT.

    **Verified end-to-end** in a throwaway `archlinux:latest` container on this
    network, driving the real `install_go_tarball` from
    `installers/gui/install-linux.sh`:

    ```
    RUN 1  tarball route allowed
      go.dev 404 -> dl.google.com 404 -> aliyun 57 MB OK
      -> Installed go version go1.25.7 linux/amd64

    RUN 2  all tarball hosts forced unreachable
      ! All Go tarball mirrors failed; falling back to the Go module proxy.
      ==> Trying the Go module proxy for go1.25.7 (golang.org/toolchain)…
      ✓ Fetched go1.25.7 via the module proxy, verified against the
        checksum database (sum.golang.org).
      ==> Installing Go to /usr/local/go
      ✓ Installed go version go1.25.7 linux/amd64
    ```

30. **Unverified tarball downloads were silent — now surfaced at runtime.**
    When `fetch_go_sha256` could not obtain the official checksum, the archive
    was installed with no verification and no message. `install_go_tarball` now
    says so on both branches:

    - `Go <ver> tarball verified (sha256).`
    - `Go <ver> tarball installed WITHOUT checksum verification (no sha256 was available).`
    - `Could not fetch the official sha256 for <file>; a tarball download will NOT be checksum-verified.`

    This is not hypothetical: RUN 1 above hit it, because that container had no
    `python3` for the index parser. Previously that download would have been
    installed silently unverified.

31. **Verified route now preferred over an unverified one — IMPLEMENTED.**

    Route precedence in every Go download path, strongest verification first:

    | # | Route | Verified by |
    |---|---|---|
    | 1 | archive + known sha256 | official checksum from `go.dev/dl/?mode=json` |
    | 2 | module proxy | checksum database (`sum.golang.org`) |
    | 3 | archive with no sha256 | **nothing** — last resort, loud warning |

    The rule is: never take an unverified route while a verified one is
    available. Concretely, when the checksum index is unreachable (so an archive
    download would be unverifiable) the installer now tries the module proxy
    *first* instead of silently installing an unchecked archive.

    **Why the module route outranks a checksum-less archive.** It is verified or
    it fails — `go_toolchain_root` pins `GOSUMDB=$GO_SUMDB_DEFAULT` for the
    fetch, so an ambient `GOSUMDB=off` from a mirror path cannot turn it into an
    unverified download. There is no "succeeded but unverified" outcome, which
    is what makes the ordering sound. (An earlier framing of this task assumed
    the module route could itself be unverified; it cannot, by construction.)

    **The bootstrap case is explicit, not implied.** The module route drives the
    download with the `go` already on the machine, so a box with no Go at all
    cannot use it — preferring it there would deadlock the installer. Guarded by
    a named predicate (`go_can_bootstrap_module_proxy` /
    `Test-GoCanBootstrapModuleProxy`) rather than an inline test, and the
    unverified warning names that as the reason when it applies.

    **Escape hatch.** `RUM_REQUIRE_VERIFIED_GO=1` turns the unverified
    last resort into a hard failure. It is not the default because on the
    restricted networks these scripts target the verification infrastructure is
    itself often unreachable, and failing outright would make the installer
    unusable there.

    Applied to all seven paths: `installers/gui/install-{linux,macos}.sh`,
    `installers/cli/install-{linux,macos}.sh`, `build-windows.sh`,
    `installers/gui/install-windows.ps1`, `installers/cli/install-windows.ps1`.

    **Every branch proven in a container on this network** (real
    `install_go_tarball` from `installers/gui/install-linux.sh`):

    ```
    (a) sha256 available + go present
        ✓ Go 1.25.7 obtained via the tarball and verified (sha256).

    (b) checksum index unreachable + go present          <- the new behaviour
        ! Could not fetch the official sha256 for go1.25.7.linux-amd64.tar.gz.
        ==> No official checksum ...; trying the verified module-proxy route first.
        ✓ Go 1.25.7 obtained via the module proxy and verified (checksum database sum.golang.org).

    (c) checksum index unreachable AND no go at all      <- bootstrap case
        ! Go 1.25.7 obtained via the tarball but could NOT be verified: no official
          sha256 was reachable, and no existing Go was present to use the verified
          module-proxy route.
        ! Continuing anyway. Set RUM_REQUIRE_VERIFIED_GO=1 to make this fatal.
        ✓ Installed go version go1.25.7 linux/amd64      (does NOT deadlock)

    (c') same + RUM_REQUIRE_VERIFIED_GO=1
        ✗ RUM_REQUIRE_VERIFIED_GO=1 is set and Go 1.25.7 could not be verified
          — refusing to install.                          (exit 1)

    (d) sha256 available but every archive host down + go present
        ! All Go tarball mirrors failed; falling back to the Go module proxy.
        ✓ Go 1.25.7 obtained via the module proxy and verified (checksum database sum.golang.org).
    ```

    `--dry-run` still short-circuits in `ensure_go`/`pin_wails_go` before any
    download, unchanged.

    **`https://go.devneeds.ir/` status: UNKNOWN.** From host and
    container it returned **HTTP 429** for everything (three attempts).
    Do not claim that mirror works.

27. **`GOPROXY=a,b` does not fall through on HTTP 429.** Go only advances
    past a comma-separated proxy on 404/410. `GOPROXY=a|b` advances on
    **any** error (measured: reached proxy.golang.org). We had not built
    a two-HTTP-proxy comma list (only compared against Go’s default
    `https://proxy.golang.org,direct`). Mirror assignments are now
    `${MIRROR}|https://proxy.golang.org,direct` so a 429 on the first
    hop can proceed. Whether that second hop is reachable in the Iranian
    case is **NOT TESTED**.

---

## 3. Go download mirrors from this network

Observed while installing Go tarballs inside the containers (this host’s
egress, 2026-09-07). This is **not** a global truth; it is what this
machine saw.

| URL | What we saw |
|---|---|
| `https://go.dev/dl/?mode=json` | Often **reachable enough to return JSON** (used to pick `go1.27.1`). Direct archive URLs were not. |
| `https://go.dev/dl/go1.27.1.linux-amd64.tar.gz` | **curl 22 / HTTP 404** (small 75-byte body). Retried twice then abandoned. |
| `https://dl.google.com/go/go1.27.1.linux-amd64.tar.gz` | **curl 22 / HTTP 404**. Same. |
| `https://mirrors.aliyun.com/golang/go1.27.1.linux-amd64.tar.gz` | **HTTP 200**, ~67.28 MB, this is what actually installed Go for non-root debian/fedora/arch/opensuse and debian-root. |
| `https://go.devneeds.ir/` (`DEFAULT_MIRROR` / `GO_FALLBACK_DL`) | Used as **Go module proxy** (`GOPROXY`) when proxy.golang.org / go.dev / github.com look unreachable. Also last extra tarball URL. Aliyun usually won before this was needed for the tarball. |
| `http://127.0.0.1:1/...` (forced) | curl **exit 7**, retries with backoff, then fail-closed. |

Env overrides (for tests and air-gapped machines):

- `RUM_GO_DL_BASE` (default `https://go.dev/dl`)
- `RUM_GO_GOOGLE_DL` (default `https://dl.google.com/go`)
- `RUM_GO_FALLBACK_DL` (default `https://go.devneeds.ir/`)
- `RUM_GO_SKIP_EXTRA_MIRRORS=1` — skip Aliyun and the fallback tarball URL
- `RUM_RETRY_MAX` (default 5; curl 22 stops at 2)
- `RUM_CURL_CONNECT_TIMEOUT` (default 20)

**Do not assume go.dev tarballs work here.** Plan on Aliyun or a local
mirror. `go 1.25.7` in go.mod will 404 if you request that exact archive
name even on a healthy go.dev.

---

## 4. gtk3 / WebKit2GTK package names (live repo query)

The GUI installer does **not** hard-require one SONAME. It:

1. Tries distro packages in order (4.1 first where that is the current name).
2. After install, `pkg-config --exists gtk+-3.0` and
   `webkit2gtk-4.1` **or** `webkit2gtk-4.0`.
3. If **only** 4.1 exists (no 4.0), `wails_tags` prints `webkit2_41` so the
   Wails build gets `-tags webkit2_41`.

| Distro | gtk3 devel | WebKit tried (order) | What the repo actually had (2026-09-07) |
|---|---|---|---|
| Debian 12 | `libgtk-3-dev` (candidate 3.24.38-2~deb12u3) | `libwebkit2gtk-4.1-dev` then `libwebkit2gtk-4.0-dev` | **Both** 4.1 and 4.0 candidates `2.50.6-1~deb12u2` |
| Fedora 44 | `gtk3-devel` 3.24.52 | `webkit2gtk4.1-devel` then `4.0` then `webkit2gtk3-devel` | `gtk3-devel` and **`webkit2gtk4.1-devel` 2.52.5** present |
| Arch | `gtk3` 1:3.24.52-1 | `webkit2gtk-4.1` then `webkit2gtk` | **`webkit2gtk-4.1` 2.52.6-1** present |
| Alpine | `gtk+3.0-dev` 3.24.52-r0 | `webkit2gtk-4.1-dev` then `webkit2gtk-dev` | **`webkit2gtk-4.1-dev`** (also `webkit2gtk-6.0-dev` — GTK4, ignore). **No `webkit2gtk-dev`.** |
| openSUSE TW | `gtk3-devel` | `webkit2gtk3-devel` then **`webkitgtk3-devel`** then `webkit2gtk4-devel` / `libwebkit2gtk-4_1-devel` | `gtk3-devel` and **`webkitgtk3-devel`**; runtime `libwebkit2gtk-4_1-0`. `webkit2gtk3-devel` **not** in the search results. |

These names were **queried**, not installed. A real GUI `try_pkg_install` of
WebKit is **NOT TESTED**.

CLI does not need gtk/webkit. CLI Go package names: apt/dnf/yum `golang`;
pacman/zypper/apk `go`.

---

## 5. What the scripts do now

Shared flags (linux/mac bash; PS has `-Prefix -Yes -Mirror -Uninstall -DryRun`):
`--prefix`, `--yes`/`-y` (implied when stdin is not a TTY), `--mirror[=URL]`,
`--uninstall`, `--verbose`, `--dry-run`, `-h`.

### 5.1 `installers/cli/install-linux.sh` (the one we executed)

- `set -Eeuo pipefail`.
- Detect: kernel/arch, `/etc/os-release`, package manager (ID/ID_LIKE then
  PATH), root vs passwordless `sudo -n`, TTY, shell rc, `go.mod` `go`
  line (min version), musl (`ld-musl` / `ldd`).
- Prepend existing `/usr/local/go/bin`, `~/.local/go/bin`, `~/.local/bin`.
- Choose prefix: `--prefix`, else `/usr/local/bin` if writable or
  `can_root`, else `~/.local/bin`.
- Preflight: print detection line, plan (Go, optional git), log path,
  low-disk warn on `$HOME`.
- Log: `$XDG_STATE_HOME/rum` or `~/.local/state/rum` or `/tmp`.
- Retry: exponential backoff 1,2,4,… capped 30s plus jitter;
  `RUM_RETRY_MAX`; curl 22 stops at 2 attempts.
- Download: curl `-fL` else wget; `.part` then `mv`; optional sha256 from
  the go.dev JSON.
- Go: pkg manager first; if too old/missing, official tarball (JSON pick
  ≥ min), URLs go.dev → dl.google.com → Aliyun → fallback unless
  `RUM_GO_SKIP_EXTRA_MIRRORS=1`. User-local vs `/usr/local/go`.
- GOPROXY: `--mirror` or auto `https://go.devneeds.ir/` if proxy.golang.org
  / go.dev / github.com look dead **and** curl/wget exist. `go mod download`
  / `go build` retry with that mirror on failure.
- Build: `go build -trimpath -ldflags "-s -w" -o $WORK_DIR/rum ./cmd/rum`.
- Atomic install: copy to `.$name.$$.tmp` in dest dir, chmod, `mv`.
- Idempotent: second run finds Go on PATH, skips tarball, rebuilds rum,
  `mv` over the existing binary, exit 0.
- Traps: EXIT cleans `$WORK_DIR` (`mktemp …/rum-cli-XXXXXX`) and
  **preserves** the exit status; INT=130; TERM=143; ERR prints line + log.
- Non-root: persist PATH in rc + `.profile`; print export line.

### 5.2 `installers/gui/install-linux.sh`

Same skeleton, plus Node ≥ 18 (pkg then Node 22 tarball), Wails CLI
(`go install` @ version from repo or `v2.12.0`), GTK3+WebKit packages
(section 4), `.desktop` + icon, `wails build` with `webkit2_41` tag when
needed. Default prefix `~/.local`. **Never executed past dry-run in
containers.**

### 5.3 macOS (`installers/cli|gui/install-macos.sh`)

Same retry/trap/log/Go-tarball ideas; Homebrew; Xcode CLT required.
**NOT TESTED** on Darwin.

### 5.4 Windows (`installers/cli|gui/install-windows.ps1`, `backend/install.ps1`)

`$ErrorActionPreference = Stop`, `Invoke-WithRetry`, winget/choco/scoop or
official zip, WebView2 on GUI, user PATH. `backend/install.ps1` only
forwards to the CLI ps1. **Parse + PSScriptAnalyzer only.**

### 5.5 Wrappers

- `backend/install.sh` — `uname` → CLI linux or macOS installer, `exec`.
- `build-linux.sh` — `exec` GUI linux installer.
- `build-windows.sh` — Windows GUI build wrapper (not run here).
- `backend/cmd/rum/build.sh` — rebuild CLI assuming Go exists; atomic
  install to `PREFIX` (default `~/.local`). No longer hardcoded
  `/home/tiredboy/bin`.

### 5.6 `installers/uninstall-linux.sh`

Removes GUI `Rum` and CLI `rum` under `--prefix`, `/usr/local`, `~/.local`,
`$HOME/bin`; icons; `.desktop`; autostart `~/.config/autostart/rum.desktop`;
optional dpkg `.deb` via apt. Does **not** delete downloaded files.
`--purge` deletes only `~/.config/rum` (needs `--yes` with no TTY, else
exit 3). Container proof: **PASS** (see matrix extra proofs). `--purge`
not exercised.

---

## 6. NOT VERIFIED / KNOWN GAPS

- **GUI full install** (Node, Wails, gtk/webkit actually installed, `wails
  build`, `.desktop`) on every distro. Dry-run + package-name probes only.
  WebKit 4.0/4.1 detection and `-tags webkit2_41` reviewed as **correct**
  (read-only audit; no change). `GOTOOLCHAIN=go1.25.7` GUI build was
  verified by the other agent on this host, not by this installer run.
- **`GOTOOLCHAIN=go${GO_MIN}` toolchain download** — **abandoned** (bug 26).
- **Exact go1.25.7 tarball on this network:** go.dev 404, dl.google.com 404,
  storage.googleapis.com 403 — but **`mirrors.aliyun.com` works** and serves
  the byte-correct release (sha256 matches the official index). The earlier
  "cannot work" claim predates that mirror being added; corrected in bug 29.
  **Module-proxy toolchain fallback: IMPLEMENTED and verified end-to-end**
  (bug 29), sumdb-verified, in all seven install paths.
- **Windows GUI pin + `GOTOOLCHAIN=local`:** PATH order bug **fixed in
  source** (bug 28). **Not functionally tested** (no Windows host).
- **`https://go.devneeds.ir/` as GOPROXY:** **UNKNOWN** (HTTP 429 from
  host and container). Do not claim it works.
- **`GOPROXY=mirror|proxy.golang.org,direct` 429 fall-through** in the
  Iranian case: **NOT TESTED**.
- **CLI root after EXIT-trap rewrite + `-buildvcs=false`** on debian,
  fedora, arch, opensuse: **NOT TESTED** (sequential disk cap; only Alpine
  was re-run). Alpine: **PASS**.
- Non-root CLI on debian/fedora/arch/opensuse used `go build` **before**
  `-buildvcs=false` and still **PASS**ed (their `cp -a` trees still had
  working git metadata, or Go did not error). Alpine root is the cell that
  actually hit VCS 128.
- **debian:12-slim** — not pulled (disk rule).
- **macOS** CLI and GUI — no Darwin.
- **Windows** real install (winget, zip, PATH, WebView2, shortcuts).
  `Sync-SessionPath` one-liner is untested on a Windows host.
- **`uninstall-linux.sh --purge`:** **NOT TESTED** (default uninstall
  **PASS**ed and left `~/.config/rum`).
- **Docs** (`INSTALL.md`, `installers/README.md`) rewritten against the
  current scripts (flags, auto vs not, GOTOOLCHAIN=local pin, mirror
  UNKNOWN). Wrapper scripts as argv0: **NOT TESTED**.
- **Checksum of Aliyun tarball vs go.dev JSON sha256** — go.dev JSON was
  fetched; Aliyun file was used. If JSON sha is for the go.dev object and
  the file matches, verify_sha256 would pass; we did not separately record
  a sha mismatch. Debian/fedora user installs completed, so either sha
  matched or JSON sha was empty for that filename.
- **`go.devneeds.ir` tarball** as the sole remaining URL — not isolated;
  Aliyun usually succeeded first.
- **Idempotent skip of the Go *build*** — second run still compiles rum.
  “Skip completed work” is true for Go toolchain + packages, not the
  compile step.
- **SIGINT during `go build` / `dnf install`** — proven during retry
  `sleep`, not during a compiler. Process-group delivery to children is
  unverified.
- **Password-prompt sudo** (not passwordless): we never typed a password;
  non-root had `can_root=0`.
- **Disk**: `/` ~12 GB free. Further pulls or parallel containers were
  forbidden. Do **not** prune Docker to close gaps.

---

## 7. If there is more budget

Do these **one image at a time**, `--rm --memory=2g --name rum-test-…`,
`df -h /` first, **stop if free < 5 GB**. Do not pull if the image is
local. Do not prune.

1. ~~Alpine root after trap rewrite + `-buildvcs=false`~~ **PASS**
   (`rum-test-cli-root-alpine --memory=2g`: apk go 1.26.8, Built rum,
   `/usr/local/bin/rum`, second run `Found go version go1.26.8`,
   `CLI_ROOT_OK`, EXIT=0). ERR-trap wrap first turned the failure into an
   actionable `go build failed`; the remaining VCS 128 was fixed with
   `-buildvcs=false`; that re-run is the PASS.
2. Sequential CLI **root** on local `debian:12`, then `fedora:latest`, then
   `opensuse/tumbleweed`, then `archlinux:latest` (largest) — still needed
   to close caveat A. Check `df -h /` first; stop under 5 GB free.
3. One GUI run **as far as `ensure_build_deps`** (not `wails build`) on
   debian:12 to prove `libwebkit2gtk-4.1-dev` actually installs. Then stop.
   Do not do this on five distros if disk is tight.
4. `bash installers/uninstall-linux.sh` inside a container that just
   installed CLI, to prove uninstall does not delete the bind-mounted repo.
5. On a Windows box or Windows container: run the three `.ps1` files with
   `-DryRun` at least.
6. On a Mac: CLI installer `--dry-run`, then real if CLT/brew exist.

When updating this file after a run, change only the cells you just saw,
and keep PASS/FAILED/NOT TESTED literal.

---

## 8. Log locations (on the build host, not in git)

- First batch: `/tmp/rum-matrix/*.log`, `/tmp/rum-matrix/SUMMARY.txt`
- Second batch (non-root + GUI dry + PS + SIGINT + retry):
  `/tmp/rum-matrix2/*.log`
- PowerShell: `/tmp/rum-matrix2/ps-check.log`
- Harness (not in repo): `/tmp/rum-verify2.sh`, `/tmp/ps-check.ps1`

These `/tmp` files will vanish on reboot. This markdown is the durable copy.
