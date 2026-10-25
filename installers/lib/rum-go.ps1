# Shared Go toolchain helpers for the Rum PowerShell installers (cli + gui).
# Dot-sourced from the checkout; never run on its own. Mirrors installers/lib/rum-go.sh:
#
#   1. Use the Go already on this machine (PATH, then common install folders)
#      when it satisfies go.mod. No download.
#   2. Pin GOTOOLCHAIN=local so Go never tries to fetch a toolchain from Google.
#   3. Only when nothing fits: download from go.dev, dl.google.com, then
#      non-Google mirrors, sha256-verified.
#   4. Pick a reachable module proxy / checksum DB / npm registry without
#      overriding anything the user set.
#
# Contract: the caller defines Info, Ok, Warn, Sync-SessionPath, $script:WorkDir,
# $script:IsTTY, $DryRun, $Mirror, $DefaultMirror, $script:IranianGoProxies.
# Optional: Install-Pkg (winget/choco/scoop), $script:RumSameMinor = $true (GUI).
# Windows PowerShell 5.1 compatible (no ternary, no ??).

$script:RumGoBin = ""
$script:RumGoVer = ""
$script:RumGoSource = ""
# Windows PowerShell 5.1 may default to TLS 1.0/1.1, which every mirror rejects.
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]3072 } catch { Write-Verbose $_.Exception.Message }
$script:RumGoOlderBin = ""
$script:RumGoOlderVer = ""
$script:RumGoSpareBin = ""
$script:RumGoSpareVer = ""
$script:RumProxyNote = ""
if (-not (Test-Path variable:script:RumSameMinor)) { $script:RumSameMinor = $false }
$script:GoSumDbDefault = if ($env:RUM_GO_SUMDB) { $env:RUM_GO_SUMDB } else { "sum.golang.org" }
# '|' between proxies: any error (timeout, 403, 429) advances; ',' only on 404/410.
$script:BuiltInGoProxy = (($script:IranianGoProxies + @("https://goproxy.cn", "https://goproxy.io")) -join "|") + "|https://proxy.golang.org,direct"
$script:RumProxyCandidates = @("https://goproxy.cn", "https://goproxy.io") + $script:IranianGoProxies

# ---------------------------------------------------------------------------
# Versions
# ---------------------------------------------------------------------------

# "go1.25.7" / "1.25" / "1.26rc1" -> @(major, minor, patch, final) ; final 0 = pre-release
function ConvertTo-RumVer([string]$v) {
    $v = (($v -replace '^go version ', '') -split '\s+')[0] -replace '^go', '' -replace '^v', ''
    if ($v -match '^(\d+)\.(\d+)(?:\.(\d+))?(.*)$') {
        $patch = 0
        if ($Matches[3]) { $patch = [int]$Matches[3] }
        $final = 1
        if ($Matches[4] -match '^(rc|beta|alpha|-pre)') { $final = 0 }
        return @([int]$Matches[1], [int]$Matches[2], $patch, $final)
    }
    return $null
}

function Test-RumVersionGe([string]$a, [string]$b) {
    $x = ConvertTo-RumVer $a
    $y = ConvertTo-RumVer $b
    if (-not $x -or -not $y) { return $false }
    for ($i = 0; $i -lt 4; $i++) {
        if ($x[$i] -ne $y[$i]) { return ($x[$i] -gt $y[$i]) }
    }
    return $true
}

function Test-RumSameMinor([string]$a, [string]$b) {
    $x = ConvertTo-RumVer $a
    $y = ConvertTo-RumVer $b
    if (-not $x -or -not $y) { return $false }
    return ($x[0] -eq $y[0] -and $x[1] -eq $y[1])
}

# Highest `go` directive among the go.mod files that exist. The `toolchain`
# line is ignored on purpose: we pin GOTOOLCHAIN=local.
function Get-RumGoMin([string[]]$Files) {
    $best = ""
    foreach ($f in $Files) {
        if (-not (Test-Path $f)) { continue }
        $line = Select-String -Path $f -Pattern '^go\s+(\d\S*)' | Select-Object -First 1
        if (-not $line) { continue }
        $v = $line.Matches[0].Groups[1].Value
        if (-not $best -or -not (Test-RumVersionGe $best $v)) { $best = $v }
    }
    if (-not $best) { return "1.25.7" }
    return $best
}

# "1.25.7" for a go.exe, or "" if it does not run.
function Get-RumGoVersionOf([string]$Bin) {
    $saved = $env:GOTOOLCHAIN
    try {
        $env:GOTOOLCHAIN = "local"
        $out = (& $Bin version 2>$null) -join " "
        if ($out -match 'go version (go\S+)') {
            $n = ConvertTo-RumVer $Matches[1]
            if ($n) {
                if ($n[3] -eq 1) { return ("{0}.{1}.{2}" -f $n[0], $n[1], $n[2]) }
                return ("{0}.{1}.{2}-pre" -f $n[0], $n[1], $n[2])
            }
        }
    } catch {
        Write-Verbose $_.Exception.Message
    } finally {
        $env:GOTOOLCHAIN = $saved
    }
    return ""
}

# ---------------------------------------------------------------------------
# Finding a local Go
# ---------------------------------------------------------------------------

function Get-RumGoCandidates {
    $out = @()
    if ($env:RUM_GO_ROOT) { $out += [pscustomobject]@{ Tag = "P"; Path = (Join-Path $env:RUM_GO_ROOT "bin\go.exe") } }
    foreach ($c in @(Get-Command go -All -ErrorAction SilentlyContinue)) {
        if ($c.CommandType -eq "Application") { $out += [pscustomobject]@{ Tag = "P"; Path = $c.Source } }
    }
    if ($env:RUM_GO_NO_SCAN -eq "1") { return $out }
    $paths = @()
    if ($env:GOROOT) { $paths += (Join-Path $env:GOROOT "bin\go.exe") }
    $paths += @("C:\Program Files\Go\bin\go.exe", "C:\Program Files (x86)\Go\bin\go.exe", "C:\Go\bin\go.exe", "C:\tools\go\bin\go.exe",
                "C:\msys64\mingw64\bin\go.exe", "C:\msys64\ucrt64\bin\go.exe")
    if ($env:LOCALAPPDATA) {
        $paths += (Join-Path $env:LOCALAPPDATA "Go\bin\go.exe")
        $paths += (Join-Path $env:LOCALAPPDATA "Programs\Go\bin\go.exe")
    }
    if ($env:USERPROFILE) {
        $paths += (Join-Path $env:USERPROFILE "go\bin\go.exe")
        $paths += (Join-Path $env:USERPROFILE ".go\bin\go.exe")
        $paths += (Join-Path $env:USERPROFILE "scoop\apps\go\current\bin\go.exe")
        $paths += (Join-Path $env:USERPROFILE "scoop\shims\go.exe")
        $sdk = Join-Path $env:USERPROFILE "sdk"
        if (Test-Path $sdk) {
            foreach ($d in @(Get-ChildItem -Path $sdk -Directory -Filter "go*" -ErrorAction SilentlyContinue)) {
                $paths += (Join-Path $d.FullName "bin\go.exe")
            }
        }
    }
    if ($env:SCOOP) { $paths += (Join-Path $env:SCOOP "apps\go\current\bin\go.exe") }
    if ($env:ChocolateyInstall) {
        $paths += (Join-Path $env:ChocolateyInstall "lib\golang\tools\go\bin\go.exe")
        $paths += (Join-Path $env:ChocolateyInstall "bin\go.exe")
    }
    foreach ($p in $paths) { $out += [pscustomobject]@{ Tag = "L"; Path = $p } }
    return $out
}

function Test-RumGoAcceptable([string]$Ver, [string]$GoMin) {
    if (-not (Test-RumVersionGe $Ver $GoMin)) { return $false }
    if ($script:RumSameMinor -and -not (Test-RumSameMinor $Ver $GoMin)) { return $false }
    return $true
}

# Choose a usable Go already on this machine. Sets $script:RumGoBin/RumGoVer,
# or leaves them empty. Does not touch PATH; Enable-RumGo does.
function Select-RumLocalGo([string]$GoMin) {
    $script:RumGoBin = ""; $script:RumGoVer = ""; $script:RumGoSource = ""
    $script:RumGoOlderBin = ""; $script:RumGoOlderVer = ""; $script:RumGoSpareBin = ""; $script:RumGoSpareVer = ""
    $seen = @{}
    $lbest = ""; $lbestVer = ""
    foreach ($c in @(Get-RumGoCandidates)) {
        if (-not (Test-Path -LiteralPath $c.Path -PathType Leaf)) { continue }
        $real = $c.Path.ToLowerInvariant()
        if ($seen.ContainsKey($real)) { continue }
        $seen[$real] = $true
        $v = Get-RumGoVersionOf $c.Path
        if (-not $v) { continue }
        if (Test-RumGoAcceptable $v $GoMin) {
            if ($c.Tag -eq "P") {
                $script:RumGoBin = $c.Path; $script:RumGoVer = $v; $script:RumGoSource = "local install"
                break
            }
            if (-not $lbest -or (Test-RumVersionGe $v $lbestVer)) { $lbest = $c.Path; $lbestVer = $v }
        } elseif (Test-RumVersionGe $v $GoMin) {
            if (-not $script:RumGoSpareBin -or (Test-RumVersionGe $v $script:RumGoSpareVer)) { $script:RumGoSpareBin = $c.Path; $script:RumGoSpareVer = $v }
        } else {
            if (-not $script:RumGoOlderBin -or (Test-RumVersionGe $v $script:RumGoOlderVer)) { $script:RumGoOlderBin = $c.Path; $script:RumGoOlderVer = $v }
        }
    }
    if (-not $script:RumGoBin -and $lbest) {
        $script:RumGoBin = $lbest; $script:RumGoVer = $lbestVer; $script:RumGoSource = "local install"
    }
    return [bool]$script:RumGoBin
}

# Put this Go first on PATH and forbid toolchain downloads. With the default
# GOTOOLCHAIN=auto a newer go.mod makes Go fetch a toolchain from Google, which
# fails where Google is blocked; a Go that meets the minimum never needs one.
function Enable-RumGo([string]$Bin, [string]$Ver) {
    $dir = Split-Path -Parent $Bin
    $first = ($env:Path -split ';')[0]
    if ($first -ne $dir) { $env:Path = "$dir;$env:Path" }
    $env:GOTOOLCHAIN = "local"
    $script:RumGoBin = $Bin
    $script:RumGoVer = $Ver
    Write-RumLog "INFO " "using go $Ver at $Bin (GOTOOLCHAIN=local)"
}

function Write-RumGoReport([string]$GoMin) {
    if ($script:RumGoBin) {
        Ok "Go $($script:RumGoVer) found at $($script:RumGoBin) (go.mod needs >= $GoMin); no download needed"
    } elseif ($script:RumGoSpareBin) {
        Warn "Local Go $($script:RumGoSpareVer) is newer than go.mod's $GoMin line; the desktop build needs the same minor version"
    } elseif ($script:RumGoOlderBin) {
        Warn "Local Go $($script:RumGoOlderVer) at $($script:RumGoOlderBin) is older than the $GoMin that go.mod needs"
    } else {
        Info "No local Go found (looked on PATH and in common install folders)"
    }
}

# ---------------------------------------------------------------------------
# Downloads
# ---------------------------------------------------------------------------

function Test-RumCurl { return [bool](Get-Command curl.exe -ErrorAction SilentlyContinue) }

# Resumable download to DEST.part then an atomic rename. An HTTP error (403 from
# a blocked mirror, 404) fails at once so the caller moves to the next mirror.
function Invoke-RumFetch([string]$Url, [string]$Dest, [int]$Tries = 2, [int]$MaxSec = 1200) {
    $dir = Split-Path -Parent $Dest
    if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $part = "$Dest.part"
    for ($n = 1; $n -le $Tries; $n++) {
        $good = $false
        $code = 0
        try {
            if (Test-RumCurl) {
                $mode = "-sS"
                if ($script:IsTTY) { $mode = "--progress-bar" }
                & curl.exe -fL $mode --connect-timeout 10 --max-time $MaxSec --speed-limit 2048 --speed-time 30 -C - -o $part $Url 2>$null
                $code = $LASTEXITCODE
                if ($code -eq 33) { Remove-Item -Force $part -ErrorAction SilentlyContinue }
                $good = ($code -eq 0)
            } else {
                Invoke-WebRequest -Uri $Url -OutFile $part -UseBasicParsing -TimeoutSec $MaxSec
                $good = $true
            }
        } catch {
            $code = 1
            Write-RumLog "WARN " "fetch error for ${Url}: $($_.Exception.Message)"
        }
        if ($good) { Move-Item -Force $part $Dest; return $true }
        if ($code -eq 22) { break }
        Start-Sleep -Seconds $n
    }
    Remove-Item -Force $part -ErrorAction SilentlyContinue
    return $false
}

# A plain GET succeeds quickly (proxies often reject HEAD).
function Test-RumHttp([string]$Url, [int]$Sec = 7) {
    try {
        if (Test-RumCurl) {
            & curl.exe -fsSL -o NUL --connect-timeout 5 --max-time $Sec $Url 2>$null
            return ($LASTEXITCODE -eq 0)
        }
        Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec $Sec | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Get-RumGoUrls([string]$File) {
    $urls = @()
    if ($env:RUM_GO_MIRROR) { $urls += ($env:RUM_GO_MIRROR.TrimEnd('/') + "/$File") }
    $dl = "https://go.dev/dl"; if ($env:RUM_GO_DL_BASE) { $dl = $env:RUM_GO_DL_BASE.TrimEnd('/') }
    $gg = "https://dl.google.com/go"; if ($env:RUM_GO_GOOGLE_DL) { $gg = $env:RUM_GO_GOOGLE_DL.TrimEnd('/') }
    $urls += "$dl/$File"
    $urls += "$gg/$File"
    if ($env:RUM_GO_SKIP_EXTRA_MIRRORS -ne "1") {
        $fb = $DefaultMirror.TrimEnd('/'); if ($env:RUM_GO_FALLBACK_DL) { $fb = $env:RUM_GO_FALLBACK_DL.TrimEnd('/') }
        $urls += "https://mirrors.aliyun.com/golang/$File"
        $urls += "https://mirrors.ustc.edu.cn/golang/$File"
        $urls += "https://golang.google.cn/dl/$File"
        $urls += "$fb/$File"
    }
    return $urls
}

# sha256 for FILE from a go.dev-style release index, trying every index we can reach.
function Get-RumGoSha([string]$File) {
    $bases = @()
    if ($env:RUM_GO_MIRROR) { $bases += $env:RUM_GO_MIRROR.TrimEnd('/') }
    $dl = "https://go.dev/dl"; if ($env:RUM_GO_DL_BASE) { $dl = $env:RUM_GO_DL_BASE.TrimEnd('/') }
    $bases += $dl
    $bases += "https://golang.google.cn/dl"
    $n = 0
    $pattern = '"filename"\s*:\s*"' + [regex]::Escape($File) + '"[^}]*?"sha256"\s*:\s*"([0-9a-fA-F]{64})"'
    foreach ($b in $bases) {
        $n++
        $json = Join-Path $script:WorkDir "go-index-$n.json"
        if (-not (Test-Path $json)) {
            if (-not (Invoke-RumFetch "$b/?mode=json&include=all" $json 1 90)) { continue }
        }
        $raw = Get-Content -Raw $json -ErrorAction SilentlyContinue
        if ($raw -and $raw -match $pattern) { return $Matches[1].ToLowerInvariant() }
    }
    foreach ($u in (Get-RumGoUrls $File)) {
        $side = Join-Path $script:WorkDir "sidecar.sha256"
        if (Invoke-RumFetch "$u.sha256" $side 1 20) {
            $s = ((Get-Content -Raw $side) -split '\s+')[0]
            Remove-Item -Force $side -ErrorAction SilentlyContinue
            if ($s -match '^[0-9a-fA-F]{64}$') { return $s.ToLowerInvariant() }
        }
    }
    return ""
}

# Toolchains are also modules (golang.org/toolchain): a different network path.
# The go command verifies them against the checksum database.
function Install-RumGoFromModuleProxy([string]$Ver, [string]$DestRoot) {
    $gobin = $script:RumGoOlderBin
    if (-not $gobin) { $gobin = $script:RumGoBin }
    if (-not $gobin) { $c = Get-Command go -ErrorAction SilentlyContinue; if ($c) { $gobin = $c.Source } }
    if (-not $gobin) { Warn "No existing Go to bootstrap the module-proxy route."; return $false }
    Info "Trying the Go module proxy for go$Ver (golang.org/toolchain)..."
    $saved = @{ T = $env:GOTOOLCHAIN; S = $env:GOSUMDB; P = $env:GOPRIVATE; F = $env:GOFLAGS }
    try {
        $env:GOTOOLCHAIN = "go$Ver"; $env:GOSUMDB = $script:GoSumDbDefault; $env:GOPRIVATE = ""; $env:GOFLAGS = ""
        $root = (& $gobin env GOROOT 2>$null)
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($root) -or -not (Test-Path (Join-Path $root "bin\go.exe"))) {
            Warn "Module proxy did not provide go$Ver."
            return $false
        }
        $got = & (Join-Path $root "bin\go.exe") version 2>$null
        if ($got -notmatch "go$([regex]::Escape($Ver))\s") { Warn "Module proxy returned $got, not go$Ver."; return $false }
        if (Test-Path $DestRoot) { Remove-Item -Recurse -Force $DestRoot }
        New-Item -ItemType Directory -Force -Path $DestRoot | Out-Null
        Copy-Item -Recurse -Force -Path (Join-Path $root "*") -Destination $DestRoot
        Get-ChildItem -Recurse -Force $DestRoot | ForEach-Object { $_.IsReadOnly = $false }
        return (Test-Path (Join-Path $DestRoot "bin\go.exe"))
    } catch {
        Warn "Module-proxy route failed: $($_.Exception.Message)"
        return $false
    } finally {
        $env:GOTOOLCHAIN = $saved.T; $env:GOSUMDB = $saved.S; $env:GOPRIVATE = $saved.P; $env:GOFLAGS = $saved.F
    }
}

function Add-RumUserPath([string]$Dir) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ([string]::IsNullOrEmpty($userPath) -or ($userPath -split ';') -notcontains $Dir) {
        $new = $Dir
        if (-not [string]::IsNullOrEmpty($userPath)) { $new = "$Dir;$userPath" }
        [Environment]::SetEnvironmentVariable("Path", $new, "User")
    }
}

function Write-RumGoHelp([string]$Ver, [string]$Arch) {
    Fail "Could not obtain Go $Ver. Nothing was changed."
    Warn "Any one of these fixes it, then re-run this script:"
    Warn "  - install Go and put it on PATH (or set RUM_GO_ROOT to its folder)"
    Warn "  - download go$Ver.windows-$Arch.zip from a mirror you can reach (https://mirrors.aliyun.com/golang/ or https://mirrors.ustc.edu.cn/golang/), extract it, set RUM_GO_ROOT"
    Warn "  - point at your own mirror: `$env:RUM_GO_MIRROR='https://host/path'  (serves host/path/go$Ver.windows-$Arch.zip)"
}

# Fetch, verify and install Go for $GoMin. Returns $false (after explaining)
# instead of throwing, so the GUI can fall back to a newer local Go.
function Install-RumGoDownload([string]$GoMin) {
    $arch = "amd64"
    if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { $arch = "arm64" }
    if ($env:RUM_GO_NO_DOWNLOAD -eq "1") {
        Fail "RUM_GO_NO_DOWNLOAD=1 is set and no suitable local Go exists."
        Write-RumGoHelp $GoMin $arch
        return $false
    }
    $names = @($GoMin)
    if ($GoMin -match '^(\d+)\.(\d+)$' -and [int]$Matches[2] -ge 21) { $names = @("$GoMin.0", $GoMin) }
    $route = ""; $verified = $false; $ver = $names[0]
    $dest = ""; $staged = Join-Path $script:WorkDir "go-module-toolchain"
    foreach ($nm in $names) {
        $ver = $nm
        $file = "go$nm.windows-$arch.zip"
        $dest = Join-Path $script:WorkDir $file
        if ($DryRun) {
            Info "dry-run: would download $file from: $((Get-RumGoUrls $file) -join ' ')"
            return $true
        }
        Info "Downloading Go $nm ($file)"
        $sha = Get-RumGoSha $file
        if ($sha) { Write-RumLog "INFO " "go sha256 $file $sha" } else { Warn "No official sha256 reachable for $file." }
        foreach ($u in (Get-RumGoUrls $file)) {
            if (Invoke-RumFetch $u $dest) {
                if (-not $sha) { $route = "archive"; break }
                $got = (Get-FileHash -Algorithm SHA256 -Path $dest).Hash.ToLowerInvariant()
                if ($got -eq $sha) { $route = "archive"; $verified = $true; break }
                Warn "Checksum mismatch from $(Split-Path -Parent $u) (expected $($sha.Substring(0,12))..., got $($got.Substring(0,12))...); trying the next mirror"
                Remove-Item -Force $dest -ErrorAction SilentlyContinue
            } else {
                Warn "Unavailable: $(($u -replace '/[^/]*$', ''))"
            }
        }
        if ($route) { break }
    }
    if (-not $route) {
        Warn "Every Go download mirror failed."
        $ver = $names[0]
        if (Install-RumGoFromModuleProxy $ver $staged) { $route = "module proxy"; $verified = $true }
    }
    if (-not $route) { Write-RumGoHelp $ver $arch; return $false }

    if ($verified) {
        Ok "Go $ver obtained via the $route and verified."
    } else {
        Warn "Go $ver obtained via the $route but could NOT be verified: no official sha256 was reachable."
        if ($env:RUM_REQUIRE_VERIFIED_GO -eq "1") {
            Fail "RUM_REQUIRE_VERIFIED_GO=1 is set - refusing to install an unverified Go."
            return $false
        }
        Warn "Continuing. Set RUM_REQUIRE_VERIFIED_GO=1 to make this fatal."
    }

    $goroot = Join-Path $env:LOCALAPPDATA "Go"
    if ($env:RUM_GO_INSTALL_DIR) { $goroot = $env:RUM_GO_INSTALL_DIR }
    $src = $staged
    if ($route -eq "archive") {
        $extract = Join-Path $script:WorkDir "go-extract"
        if (Test-Path $extract) { Remove-Item -Recurse -Force $extract }
        try { Expand-Archive -Path $dest -DestinationPath $extract -Force } catch { Fail "The downloaded archive is corrupt."; return $false }
        $src = Join-Path $extract "go"
    }
    if (-not (Test-Path (Join-Path $src "bin\go.exe"))) { Fail "The downloaded Go did not contain bin\go.exe."; return $false }
    Info "Installing Go to $goroot"
    # Rename into place so a failure never leaves a half-written toolchain.
    $parent = Split-Path -Parent $goroot
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    foreach ($t in @("$goroot.new", "$goroot.old")) { if (Test-Path $t) { Remove-Item -Recurse -Force $t } }
    Move-Item -Force $src "$goroot.new"
    if (Test-Path $goroot) { Move-Item -Force $goroot "$goroot.old" }
    Move-Item -Force "$goroot.new" $goroot
    if (Test-Path "$goroot.old") { Remove-Item -Recurse -Force "$goroot.old" -ErrorAction SilentlyContinue }
    $bin = Join-Path $goroot "bin"
    Add-RumUserPath $bin
    Enable-RumGo (Join-Path $bin "go.exe") $ver
    $script:RumGoSource = "downloaded to $goroot"
    Ok "Installed $(& (Join-Path $bin 'go.exe') version) at $goroot"
    return $true
}

# The whole policy in one place: local Go, then mirrors, then the system package
# manager. Throws only when nothing worked.
function Install-RumGo([string]$GoMin) {
    Sync-SessionPath
    if (Select-RumLocalGo $GoMin) {
        Enable-RumGo $script:RumGoBin $script:RumGoVer
        Ok "Using local Go $($script:RumGoVer) ($($script:RumGoBin)); GOTOOLCHAIN=local, nothing to download"
        return
    }
    if ($script:RumGoOlderBin) {
        Warn "Local Go $($script:RumGoOlderVer) ($($script:RumGoOlderBin)) is too old: go.mod needs >= $GoMin. Fetching a newer one; the old one is left untouched."
    } elseif ($script:RumGoSpareBin) {
        Info "Local Go $($script:RumGoSpareVer) is newer than go.mod's line; trying to fetch the exact $GoMin for Wails"
    } else {
        Info "No local Go found; $GoMin+ is needed"
    }
    if ($DryRun) { Info "dry-run: would install Go $GoMin (mirrors: go.dev, dl.google.com, aliyun, ustc, golang.google.cn)"; return }
    if (Install-RumGoDownload $GoMin) { return }
    # Package manager: system-wide installer, so only after the mirrors failed.
    if (Get-Command Install-Pkg -ErrorAction SilentlyContinue) {
        if (Install-Pkg -WingetId "GoLang.Go" -ChocoId "golang" -ScoopId "go") {
            Sync-SessionPath
            if (Select-RumLocalGo $GoMin) {
                Enable-RumGo $script:RumGoBin $script:RumGoVer
                Ok "Installed Go $($script:RumGoVer) via the package manager"
                return
            }
        }
    }
    if ($script:RumGoSpareBin) {
        Warn "Using Go $($script:RumGoSpareVer) ($($script:RumGoSpareBin)); if 'wails build' fails typechecking, install Go $GoMin (see above)."
        Enable-RumGo $script:RumGoSpareBin $script:RumGoSpareVer
        $script:RumGoSource = $script:RumGoSpareBin
        return
    }
    throw "No usable Go $GoMin+ (see the options above)."
}

# ---------------------------------------------------------------------------
# Module proxy, checksum DB, npm registry
# ---------------------------------------------------------------------------

function Get-RumGoEnv([string]$Name) {
    $bin = $script:RumGoBin
    if (-not $bin) { $c = Get-Command go -ErrorAction SilentlyContinue; if ($c) { $bin = $c.Source } }
    if (-not $bin) { return "" }
    $saved = $env:GOTOOLCHAIN
    try { $env:GOTOOLCHAIN = "local"; return ((& $bin env $Name 2>$null) -join "").Trim() } catch { return "" } finally { $env:GOTOOLCHAIN = $saved }
}

# Respect anything the user set; otherwise probe and, only if the default proxy
# is unreachable, use a fallback chain.
function Set-RumGoProxy {
    if ($Mirror) {
        $env:GOPROXY = if ($Mirror -eq $DefaultMirror) { $script:BuiltInGoProxy } else { "$Mirror|$script:BuiltInGoProxy" }
        $env:GOSUMDB = $script:GoSumDbDefault
        $script:RumProxyNote = "-Mirror chain"
        Ok "Using Go module proxy chain: $env:GOPROXY"
        return
    }
    $cur = $env:GOPROXY
    if (-not $cur) { $cur = Get-RumGoEnv "GOPROXY" }
    if ($cur -and $cur -ne "https://proxy.golang.org,direct") {
        $script:RumProxyNote = "GOPROXY from your environment"
        Ok "Using your GOPROXY: $cur"
        return
    }
    if (Test-RumHttp "https://proxy.golang.org/github.com/google/uuid/@v/list" 6) {
        $script:RumProxyNote = "proxy.golang.org"
        Ok "Go module proxy reachable (proxy.golang.org); default GOPROXY kept"
    } else {
        Warn "proxy.golang.org is unreachable (Google is blocked here?); probing fallback module proxies..."
        $chain = @()
        foreach ($p in $script:RumProxyCandidates) {
            if (Test-RumHttp ($p.TrimEnd('/') + "/github.com/google/uuid/@v/list") 6) { $chain += $p }
        }
        if ($chain.Count -eq 0) {
            Warn "None of the fallback proxies answered a quick probe; trying them all anyway"
            $env:GOPROXY = $script:BuiltInGoProxy
        } else {
            $env:GOPROXY = ($chain -join "|") + "|direct"
        }
        $script:RumProxyNote = "fallback chain"
        Ok "Using fallback GOPROXY: $env:GOPROXY"
    }
    # Keep checksum verification on; only switch to Google's China-hosted mirror
    # of the same database, and only if the user has not chosen one.
    $sumdb = $env:GOSUMDB
    if (-not $sumdb) { $sumdb = Get-RumGoEnv "GOSUMDB" }
    if ($env:RUM_GO_SUMDB) {
        $env:GOSUMDB = $env:RUM_GO_SUMDB
    } elseif ((-not $sumdb) -or $sumdb -eq "sum.golang.org") {
        if (-not (Test-RumHttp "https://sum.golang.org/latest" 6) -and (Test-RumHttp "https://sum.golang.google.cn/latest" 6)) {
            $env:GOSUMDB = "sum.golang.org https://sum.golang.google.cn"
            Ok "sum.golang.org is unreachable; verifying via sum.golang.google.cn (same database)"
        }
    }
}

# After a failed Go network step, switch once to the built-in chain. Never
# overrides a GOPROXY the user chose. Returns $false when there is nothing new to try.
function Use-RumProxyFallback {
    if ($script:RumProxyNote -eq "GOPROXY from your environment") { return $false }
    if ($env:GOPROXY -eq $script:BuiltInGoProxy) { return $false }
    if ($env:GOPROXY -and $env:GOPROXY.Contains($script:IranianGoProxies[0])) { return $false }
    $env:GOPROXY = $script:BuiltInGoProxy
    $script:RumProxyNote = "built-in fallback chain"
    return $true
}

# Keep the user's npm registry; use a mirror only if the default is unreachable.
function Set-RumNpmRegistry {
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) { return }
    if ($env:NPM_CONFIG_REGISTRY -or $env:npm_config_registry) { return }
    $reg = ""
    try { $reg = ((& npm config get registry 2>$null) -join "").Trim() } catch { Write-Verbose $_.Exception.Message }
    if ($reg -and $reg.TrimEnd('/') -ne "https://registry.npmjs.org") { Ok "Using your npm registry: $reg"; return }
    if (Test-RumHttp "https://registry.npmjs.org/" 8) { return }
    $alt = "https://registry.npmmirror.com"; if ($env:RUM_NPM_REGISTRY) { $alt = $env:RUM_NPM_REGISTRY }
    if (Test-RumHttp $alt 8) {
        $env:NPM_CONFIG_REGISTRY = $alt
        Warn "registry.npmjs.org is unreachable; using $alt for this run only (your npm config is untouched)"
    } else {
        Warn "No npm registry answered; npm steps may fail"
    }
}

function Write-RumSummary {
    Info "Go: $($script:RumGoVer) at $($script:RumGoBin) ($($script:RumGoSource)); GOTOOLCHAIN=$($env:GOTOOLCHAIN)"
    if ($script:RumProxyNote) { Info "Go modules via: $($script:RumProxyNote) (GOPROXY=$($env:GOPROXY))" }
}

# Warn (never fail) when free space on the drive holding $Path looks low.
function Test-RumFreeSpace([string]$Path, [long]$NeedBytes) {
    try {
        $drive = (Split-Path -Qualifier $Path).TrimEnd(':')
        $free = (Get-PSDrive -Name $drive -ErrorAction Stop).Free
        if ($free -and $free -lt $NeedBytes) {
            Warn ("Low disk space on {0}: ({1} MiB free; about {2} MiB recommended)" -f $drive, [int]($free / 1MB), [int]($NeedBytes / 1MB))
        }
    } catch { Write-Verbose $_.Exception.Message }
}
