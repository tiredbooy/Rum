<#
.SYNOPSIS
    Rum CLI installer for Windows.
.DESCRIPTION
    Builds rum.exe from source and installs it into %LOCALAPPDATA%\Programs\Rum,
    adding that directory to the user PATH. Go is auto-detected and installed
    via winget/choco/scoop or an official zip (checksum-verified).

    Non-interactive by default when stdin is not a TTY. -Yes is implied then.
.PARAMETER Prefix
    Install directory (default: $env:LOCALAPPDATA\Programs\Rum).
.PARAMETER Mirror
    Go module proxy URL. Empty = try the default proxy, then the built-in mirror.
.PARAMETER Uninstall
    Remove an installed rum.exe instead of installing.
.PARAMETER Yes
    Non-interactive; never prompt.
.PARAMETER DryRun
    Detect, print the plan, and exit without changing the system.
.EXAMPLE
    .\installers\cli\install-windows.ps1
.EXAMPLE
    .\installers\cli\install-windows.ps1 -Uninstall
#>
[CmdletBinding()]
param(
    [string]$Prefix = (Join-Path $env:LOCALAPPDATA "Programs\Rum"),
    [string]$Mirror = "",
    [switch]$Uninstall,
    [switch]$Yes,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$DefaultMirror = "https://go.devneeds.ir/"
$script:IranianGoProxies = @(
    "https://mirror-go.runflare.com",
    "https://package-mirror.liara.ir/repository/go",
    "https://mirror.abrha.net/repository/go",
    $DefaultMirror.TrimEnd('/')
)
$script:BuiltInGoProxy = ($script:IranianGoProxies + "https://proxy.golang.org,direct") -join "|"
$RetryMax = if ($env:RUM_RETRY_MAX) { [int]$env:RUM_RETRY_MAX } else { 5 }

$script:IsTTY = $true
try { if ([Console]::IsInputRedirected) { $script:IsTTY = $false } } catch { $script:IsTTY = $false }
if (-not $script:IsTTY) { $Yes = $true }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
$BackendDir = Join-Path $RepoRoot "backend"
$Target = Join-Path $Prefix "rum.exe"
$script:TempDirs = New-Object System.Collections.Generic.List[string]
$script:LogFile = $null
$script:WorkDir = $null

function Get-LogDir {
    if ($env:XDG_STATE_HOME) { return (Join-Path $env:XDG_STATE_HOME "rum") }
    if ($env:LOCALAPPDATA) { return (Join-Path $env:LOCALAPPDATA "Rum\logs") }
    return $env:TEMP
}

function Initialize-Log {
    $dir = Get-LogDir
    try {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $script:LogFile = Join-Path $dir ("cli-install-{0:yyyyMMddTHHmmss}-pid{1}.log" -f (Get-Date).ToUniversalTime(), $PID)
        New-Item -ItemType File -Force -Path $script:LogFile | Out-Null
    } catch {
        $script:LogFile = Join-Path $env:TEMP "rum-cli-install-$PID.log"
    }
}

function Redact([string]$s) {
    return [regex]::Replace($s, '(TOKEN|SECRET|PASSWORD|API[_-]?KEY|AUTH|BEARER)[=:]\S+', '$1=***', 'IgnoreCase')
}

function Write-RumLog([string]$level, [string]$m) {
    $ts = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    if ($script:LogFile) {
        try { Add-Content -Path $script:LogFile -Value ("{0} {1} {2}" -f $ts, $level, (Redact $m)) -ErrorAction SilentlyContinue } catch { Write-Verbose $_.Exception.Message }
    }
}

function Info($m) { Write-Host "==> $m" -ForegroundColor Cyan;  Write-RumLog "INFO " $m }
function Ok($m)   { Write-Host "OK  $m" -ForegroundColor Green; Write-RumLog "OK   " $m }
function Warn($m) { Write-Host "!   $m" -ForegroundColor Yellow; Write-RumLog "WARN " $m }
function Fail($m) {
    Write-Host "ERR $m" -ForegroundColor Red
    Write-RumLog "ERROR" $m
    if ($script:LogFile) { Write-Host "ERR Full log: $($script:LogFile)" -ForegroundColor Red }
}

function Invoke-WithRetry {
    param(
        [Parameter(Mandatory)][scriptblock]$Script,
        [string]$Name = "command",
        [int]$Max = $RetryMax
    )
    $delay = 1
    for ($i = 1; $i -le $Max; $i++) {
        try {
            & $Script
            return
        } catch {
            if ($i -eq $Max) {
                Fail "$Name failed after $Max attempts: $($_.Exception.Message)"
                throw
            }
            Warn "$Name failed (attempt $i/$Max): $($_.Exception.Message); retrying in ${delay}s…"
            $jitter = Get-Random -Minimum 0 -Maximum 500
            Start-Sleep -Milliseconds ([int]($delay * 1000 + $jitter))
            $delay = [Math]::Min($delay * 2, 30)
        }
    }
}

function Get-CommandSafe($name) { return Get-Command $name -ErrorAction SilentlyContinue }

function Sync-SessionPath {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = @($machine, $user, $env:Path) -join ";"
}

function Test-Reachable([string]$Url) {
    try {
        $req = [System.Net.HttpWebRequest]::Create($Url)
        $req.Method = "HEAD"
        $req.Timeout = 8000
        $req.AllowAutoRedirect = $true
        $resp = $req.GetResponse()
        $resp.Close()
        return $true
    } catch {
        try {
            if (Get-CommandSafe "curl.exe") {
                & curl.exe -fsS -o NUL --connect-timeout 5 --max-time 8 -I $Url 2>$null
                if ($LASTEXITCODE -eq 0) { return $true }
            }
        } catch { Write-Verbose $_.Exception.Message }
        return $false
    }
}

function Get-RemoteFile {
    param([Parameter(Mandatory)][string]$Url, [Parameter(Mandatory)][string]$Dest, [string]$Sha256 = "")
    if ($DryRun) { Info "dry-run: would download $Url -> $Dest"; return }
    $dir = Split-Path -Parent $Dest
    if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $tmp = "$Dest.part"
    Invoke-WithRetry -Name "download $Url" -Script {
        if (Get-CommandSafe "curl.exe") {
            & curl.exe -fL --connect-timeout 20 --max-time 600 -o $tmp $Url
            if ($LASTEXITCODE -ne 0) { throw "curl.exe exit $LASTEXITCODE" }
        } else {
            Invoke-WebRequest -Uri $Url -OutFile $tmp -UseBasicParsing
        }
    }
    if ($Sha256) {
        $got = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash
        if ($got.ToLowerInvariant() -ne $Sha256.ToLowerInvariant()) {
            Remove-Item -Force $tmp -ErrorAction SilentlyContinue
            throw "Checksum mismatch for $Url"
        }
    }
    Move-Item -Force $tmp $Dest
}

function Get-GoMin {
    foreach ($p in @((Join-Path $BackendDir "go.mod"), (Join-Path $RepoRoot "go.mod"))) {
        if (Test-Path $p) {
            $line = Select-String -Path $p -Pattern '^go\s+(\S+)' | Select-Object -First 1
            if ($line) { return $line.Matches[0].Groups[1].Value }
        }
    }
    return "1.25.7"
}

function Convert-GoVersion([string]$v) {
    $v = $v -replace '^go', '' -replace '-.*$', ''
    try { return [version]$v } catch { return [version]"0.0.0" }
}

function Test-GoMeetsMin([string]$min) {
    if (-not (Get-CommandSafe "go")) { return $false }
    $cur = (& go env GOVERSION 2>$null)
    if (-not $cur) { return $false }
    return (Convert-GoVersion $cur) -ge (Convert-GoVersion $min)
}

function Get-CpuArch {
    if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { return "arm64" }
    return "amd64"
}

function Get-PkgMgr {
    if (Get-CommandSafe "winget") { return "winget" }
    if (Get-CommandSafe "choco") { return "choco" }
    if (Get-CommandSafe "scoop") { return "scoop" }
    return "none"
}

function Install-Pkg {
    param([string]$WingetId, [string]$ChocoId, [string]$ScoopId)
    if ($DryRun) { Info "dry-run: would install $WingetId via package manager"; return $true }
    $mgr = Get-PkgMgr
    try {
        switch ($mgr) {
            "winget" {
                Invoke-WithRetry -Name "winget install $WingetId" -Script {
                    & winget install --id $WingetId -e --accept-package-agreements --accept-source-agreements --disable-interactivity --silent
                    if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne -1978335189) { throw "winget exit $LASTEXITCODE" }
                }
                Sync-SessionPath
                return $true
            }
            "choco" {
                Invoke-WithRetry -Name "choco install $ChocoId" -Script {
                    & choco install $ChocoId -y --no-progress
                    if ($LASTEXITCODE -ne 0) { throw "choco exit $LASTEXITCODE" }
                }
                Sync-SessionPath
                return $true
            }
            "scoop" {
                Invoke-WithRetry -Name "scoop install $ScoopId" -Script {
                    & scoop install $ScoopId
                    if ($LASTEXITCODE -ne 0) { throw "scoop exit $LASTEXITCODE" }
                }
                Sync-SessionPath
                return $true
            }
            default { return $false }
        }
    } catch {
        Warn "Package manager install failed: $($_.Exception.Message)"
        return $false
    }
}

function Get-GoSha256([string]$filename) {
    $jsonPath = Join-Path $script:WorkDir "go-dl.json"
    Get-RemoteFile -Url "https://go.dev/dl/?mode=json&include=all" -Dest $jsonPath
    $data = Get-Content -Raw $jsonPath | ConvertFrom-Json
    foreach ($rel in $data) {
        foreach ($f in $rel.files) {
            if ($f.filename -eq $filename -and $f.sha256) { return $f.sha256 }
        }
    }
    return ""
}

# GOSUMDB remains enabled for both the mirror chain and toolchain fetches.
# The checksum database is the integrity check for modules fetched from any
# third-party proxy.
$script:GoSumDbDefault = if ($env:RUM_GO_SUMDB) { $env:RUM_GO_SUMDB } else { "sum.golang.org" }

# Copy-Item-based staging of a Go toolchain obtained through the Go module proxy.
#
# Toolchains are published as ordinary modules
# (golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>), so this reaches the module
# proxy rather than the tarball hosts - a different network path, which is the
# point of having it as a last resort.
#
# The `go` command does the download rather than us fetching the zip directly:
# it verifies the module against the checksum database, whereas a raw download
# would be unverifiable (the sha256 published on go.dev is for the .zip release
# artefact, not the module, and cannot validate it).
function Install-GoFromModuleProxy([string]$ver, [string]$destRoot) {
    if (-not (Get-CommandSafe "go")) {
        Warn "No existing Go, so the module-proxy fallback cannot be used (it needs a go command to bootstrap)."
        return $false
    }
    Info "Trying the Go module proxy for go$ver (golang.org/toolchain)..."

    $saved = @{
        GOTOOLCHAIN = $env:GOTOOLCHAIN; GOSUMDB = $env:GOSUMDB
        GOPRIVATE   = $env:GOPRIVATE;   GOFLAGS = $env:GOFLAGS
    }
    try {
        # Clear GOFLAGS/GOPRIVATE so a caller's settings cannot redirect this,
        # and force GOSUMDB on so the module is verified even when a mirror
        # path has disabled it.
        $env:GOTOOLCHAIN = "go$ver"
        $env:GOSUMDB     = $script:GoSumDbDefault
        $env:GOPRIVATE   = ""
        $env:GOFLAGS     = ""

        $root = ""
        Invoke-WithRetry -Name "go toolchain fetch" -Script {
            $script:__tcRoot = (& go env GOROOT 2>$null)
            if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($script:__tcRoot)) { throw "go env GOROOT failed" }
        }
        $root = $script:__tcRoot
        if ([string]::IsNullOrWhiteSpace($root) -or -not (Test-Path (Join-Path $root "bin\go.exe"))) {
            Warn "Module proxy did not provide go$ver."
            return $false
        }
        # A go older than 1.21 ignores GOTOOLCHAIN and would hand back its own
        # GOROOT. Confirm we really got the version we asked for.
        $got = & (Join-Path $root "bin\go.exe") version 2>$null
        if ($got -notmatch "go$([regex]::Escape($ver))\s") {
            Warn "Module proxy returned $got, not go$ver."
            return $false
        }

        # The module layout is NOT the release-zip layout: entries live under
        # golang.org/toolchain@v0.0.1-go<ver>.<os>-<arch>/ rather than go/, and
        # the module cache is stored read-only. Normalise both.
        if (Test-Path $destRoot) { Remove-Item -Recurse -Force $destRoot }
        New-Item -ItemType Directory -Force -Path $destRoot | Out-Null
        Copy-Item -Recurse -Force -Path (Join-Path $root "*") -Destination $destRoot
        Get-ChildItem -Recurse -Force $destRoot | ForEach-Object { $_.IsReadOnly = $false }
        if (-not (Test-Path (Join-Path $destRoot "bin\go.exe"))) {
            Warn "Module toolchain did not contain bin\go.exe"
            return $false
        }
        # The caller prints the single authoritative verification verdict.
        return $true
    } catch {
        Warn "Module-proxy fallback failed: $($_.Exception.Message)"
        return $false
    } finally {
        $env:GOTOOLCHAIN = $saved.GOTOOLCHAIN; $env:GOSUMDB = $saved.GOSUMDB
        $env:GOPRIVATE   = $saved.GOPRIVATE;   $env:GOFLAGS = $saved.GOFLAGS
    }
}

# Test-GoCanBootstrapModuleProxy - whether the module-proxy route is usable.
#
# It drives the download with the `go` command already on this machine, so a box
# with no Go cannot use it: there the archive is the ONLY way to get a toolchain
# and preferring the module route would deadlock the installer. Explicit guard.
function Test-GoCanBootstrapModuleProxy {
    return [bool](Get-CommandSafe "go")
}

function Install-GoZip([string]$ver) {
    $arch = Get-CpuArch
    $file = "go$ver.windows-$arch.zip"
    $dest = Join-Path $script:WorkDir $file
    Info "Obtaining Go $ver ($file)"
    $sha = ""
    try { $sha = Get-GoSha256 $file } catch { Warn "Could not fetch Go checksums: $($_.Exception.Message)" }
    $urls = @("https://go.dev/dl/$file", "https://dl.google.com/go/$file", "$($DefaultMirror.TrimEnd('/'))/$file")
    # Route precedence, strongest verification first: never take an unverified
    # route while a verified one is available.
    #   1. archive + known sha256       -> verified
    #   2. module proxy (sumdb enforced)-> verified
    #   3. archive with no sha256       -> UNVERIFIED, last resort, loud warning
    # The module route is verified or it fails (Install-GoFromModuleProxy pins
    # GOSUMDB), so it has no "succeeded but unverified" outcome.
    $staged = Join-Path $script:WorkDir "go-module-toolchain"
    $route = ""; $verified = $false; $moduleTried = $false
    if ([string]::IsNullOrEmpty($sha) -and (Test-GoCanBootstrapModuleProxy)) {
        Info "No official checksum for $file; trying the verified module-proxy route first."
        $moduleTried = $true
        if (Install-GoFromModuleProxy $ver $staged) { $route = "module proxy"; $verified = $true }
        else { Warn "Verified module-proxy route unavailable; falling back to an unverified archive." }
    }

    if ([string]::IsNullOrEmpty($route)) {
        $ok = $false
        foreach ($u in $urls) {
            try { Get-RemoteFile -Url $u -Dest $dest -Sha256 $sha; $ok = $true; break } catch { Warn "Download failed: $u" }
        }
        if ($ok) {
            $route = "archive"; if (-not [string]::IsNullOrEmpty($sha)) { $verified = $true }
        } elseif ((-not $moduleTried) -and (Test-GoCanBootstrapModuleProxy)) {
            Warn "All Go download mirrors failed; falling back to the Go module proxy."
            if (Install-GoFromModuleProxy $ver $staged) { $route = "module proxy"; $verified = $true }
            else { throw "Could not download Go $ver from any mirror or the module proxy. Install from https://go.dev/dl/ and re-run." }
        } else {
            throw "Could not download Go $ver from any mirror or the module proxy. Install from https://go.dev/dl/ and re-run."
        }
    }
    $fromProxy = ($route -eq "module proxy")

    if ($verified) {
        if ($route -eq "archive") { Ok "Go $ver obtained via the $route and verified (sha256)." }
        else { Ok "Go $ver obtained via the $route and verified (checksum database $script:GoSumDbDefault)." }
    } else {
        $why = "no official sha256 was reachable"
        if (-not (Test-GoCanBootstrapModuleProxy)) { $why += ", and no existing Go was present to use the verified module-proxy route" }
        Warn "Go $ver obtained via the $route but could NOT be verified: $why."
        if ($env:RUM_REQUIRE_VERIFIED_GO -eq "1") {
            throw "RUM_REQUIRE_VERIFIED_GO=1 is set and Go $ver could not be verified - refusing to install."
        }
        Warn "Continuing anyway. Set RUM_REQUIRE_VERIFIED_GO=1 to make this fatal."
    }
    $goroot = Join-Path $env:LOCALAPPDATA "Go"
    Info "Installing Go to $goroot (user-local)"
    if (Test-Path $goroot) { Remove-Item -Recurse -Force $goroot }
    if ($fromProxy) {
        Move-Item -Force $staged $goroot
    } else {
        Expand-Archive -Path $dest -DestinationPath $script:WorkDir -Force
        Move-Item -Force (Join-Path $script:WorkDir "go") $goroot
    }
    $bin = Join-Path $goroot "bin"
    $env:Path = "$bin;$env:Path"
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$bin*") {
        $new = if ([string]::IsNullOrEmpty($userPath)) { $bin } else { "$bin;$userPath" }
        [Environment]::SetEnvironmentVariable("Path", $new, "User")
    }
    Sync-SessionPath
    Ok "Installed $(go version) at $goroot"
}

function Install-RequiredGo([string]$min) {
    Sync-SessionPath
    if (Test-GoMeetsMin $min) { Ok "Found $(go version)"; return }
    if (Get-CommandSafe "go") { Warn "Go is too old ($(go version)); need >= $min" } else { Info "Go $min+ is not installed" }
    if ($DryRun) { Info "dry-run: would install Go $min"; return }
    $ok = Install-Pkg -WingetId "GoLang.Go" -ChocoId "golang" -ScoopId "go"
    Sync-SessionPath
    if ($ok -and (Test-GoMeetsMin $min)) { Ok "Installed $(go version) via package manager"; return }
    Install-GoZip $min
    Sync-SessionPath
    if (-not (Test-GoMeetsMin $min)) { throw "Go $min+ is required. Install from https://go.dev/dl/ and re-run." }
}

function Set-GoProxy {
    [CmdletBinding(SupportsShouldProcess)]
    param()
    if (-not $PSCmdlet.ShouldProcess("GOPROXY", "configure")) { return }
    if ($Mirror) {
        $env:GOPROXY = if ($Mirror -eq $DefaultMirror) { $script:BuiltInGoProxy } else { "$Mirror|$script:BuiltInGoProxy" }
        $env:GOSUMDB = $script:GoSumDbDefault
        Ok "Using Go module proxy chain: $env:GOPROXY"
    }
}

function Add-UserPath($dir) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$dir*") {
        $newPath = if ([string]::IsNullOrEmpty($userPath)) { $dir } else { "$userPath;$dir" }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$dir;$env:Path"
        Info "Added $dir to your user PATH (restart your terminal to pick it up)."
    }
}

Initialize-Log
$script:WorkDir = Join-Path $env:TEMP ("rum-cli-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $script:WorkDir | Out-Null
$script:TempDirs.Add($script:WorkDir)

try {
    if ($Uninstall) {
        if ($DryRun) { Info "dry-run: would remove $Target"; return }
        if (Test-Path $Target) {
            Info "Removing $Target"
            Remove-Item -Force $Target
            Ok "Removed $Target"
        } else {
            Info "Nothing to remove at $Target"
        }
        $legacy = Join-Path $env:USERPROFILE "bin\rum.exe"
        if (Test-Path $legacy) {
            Info "Removing legacy $legacy"
            Remove-Item -Force $legacy
            Ok "Removed $legacy"
        }
        Info "Log: $($script:LogFile)"
        return
    }

    if (-not (Test-Path (Join-Path $BackendDir "cmd\rum"))) {
        throw "Cannot find $BackendDir\cmd\rum — run this from a clean checkout."
    }

    $GoMin = Get-GoMin
    $arch = Get-CpuArch
    $mgr = Get-PkgMgr
    Info "Detected: windows/$arch pkg=$mgr tty=$($script:IsTTY) yes=$Yes prefix=$Prefix"
    Info "Go minimum (from go.mod): $GoMin  |  install target: $Target"
    if (-not (Test-GoMeetsMin $GoMin)) { Info "Will install: Go $GoMin+" } else { Info "All build prerequisites already present" }
    if (Test-Path $Target) { Info "Existing install will be replaced: $Target" }
    Info "Log file: $($script:LogFile)"

    $net = (Test-Reachable "https://proxy.golang.org") -or (Test-Reachable "https://go.dev") -or (Test-Reachable "https://github.com")
    if ($net) { Ok "Network is reachable" } else {
        Warn "Could not reach go.dev / proxy.golang.org / github.com — will try bundled Iranian module mirrors"
        if (-not $Mirror) { $Mirror = $DefaultMirror }
    }

    if ($DryRun) { Info "dry-run complete (no changes). Re-run without -DryRun to install."; return }

    Set-GoProxy
    Install-RequiredGo $GoMin

    Info "Building rum.exe (this may take a moment)..."
    $built = Join-Path $script:WorkDir "rum.exe"
    Push-Location $BackendDir
    try {
        try {
            Invoke-WithRetry -Name "go mod download" -Script {
                & go mod download
                if ($LASTEXITCODE -ne 0) { throw "go mod download exit $LASTEXITCODE" }
            }
        } catch {
            if (-not ($env:GOPROXY -and $env:GOPROXY.Contains($script:IranianGoProxies[0]))) {
                Warn "go mod download failed; retrying with bundled Iranian module mirrors"
                $env:GOPROXY = $script:BuiltInGoProxy
                $env:GOSUMDB = $script:GoSumDbDefault
                Invoke-WithRetry -Name "go mod download (mirror)" -Script {
                    & go mod download
                    if ($LASTEXITCODE -ne 0) { throw "go mod download exit $LASTEXITCODE" }
                }
            } else { throw }
        }
        try {
            & go build -trimpath -ldflags "-s -w" -o $built ".\cmd\rum"
            if ($LASTEXITCODE -ne 0) { throw "go build failed" }
        } catch {
            if (-not ($env:GOPROXY -and $env:GOPROXY.Contains($script:IranianGoProxies[0]))) {
                Warn "go build failed; retrying with bundled Iranian module mirrors"
                $env:GOPROXY = $script:BuiltInGoProxy
                $env:GOSUMDB = $script:GoSumDbDefault
                & go build -trimpath -ldflags "-s -w" -o $built ".\cmd\rum"
                if ($LASTEXITCODE -ne 0) { throw "go build failed" }
            } else { throw }
        }
    } finally { Pop-Location }
    if (-not (Test-Path $built)) { throw "Build produced no executable" }
    Ok "Built rum.exe"

    New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
    $partial = "$Target.partial"
    Copy-Item -Force $built $partial
    Move-Item -Force $partial $Target
    Ok "Installed to $Target"

    Add-UserPath $Prefix

    Write-Host ""
    Ok "Done. Open a new terminal and try:  rum --version"
    Info "Log: $($script:LogFile)"
} catch {
    Fail $_.Exception.Message
    throw
} finally {
    foreach ($d in $script:TempDirs) {
        if ($d -and (Test-Path $d)) {
            Remove-Item -Recurse -Force $d -ErrorAction SilentlyContinue
        }
    }
}
