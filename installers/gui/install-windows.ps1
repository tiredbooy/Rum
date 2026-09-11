<#
.SYNOPSIS
    Rum GUI (desktop) installer for Windows.
.DESCRIPTION
    Builds the Wails desktop app for Windows and installs it. Prerequisites
    (Go, Node.js + npm, Wails CLI, WebView2) are auto-detected and installed
    when a package manager (winget / choco / scoop) is available, otherwise
    official archives are downloaded with checksum verification.

    If Inno Setup (iscc.exe) is available it produces Rum-Setup.exe;
    otherwise it installs Rum.exe into %LOCALAPPDATA%\Programs\Rum and
    creates Start Menu and Desktop shortcuts.

    Non-interactive by default when stdin is not a TTY. -Yes is implied then.
.PARAMETER Prefix
    Install directory for the no-installer path (default: %LOCALAPPDATA%\Programs\Rum).
.PARAMETER Mirror
    Go module proxy URL. Empty = try the default proxy, then the built-in mirror.
.PARAMETER Installer
    Force building the Inno Setup installer (fails if iscc.exe is missing).
.PARAMETER Uninstall
    Remove the app installed by the no-installer path.
.PARAMETER Yes
    Non-interactive; never prompt.
.PARAMETER DryRun
    Detect, print the plan, and exit without changing the system.
.EXAMPLE
    .\installers\gui\install-windows.ps1
#>
[CmdletBinding()]
param(
    [string]$Prefix = (Join-Path $env:LOCALAPPDATA "Programs\Rum"),
    [string]$Mirror = "",
    [switch]$Installer,
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
$NodeMin = [version]"18.0.0"
$WailsFallback = "v2.12.0"

$script:IsTTY = $true
try {
    if ([Console]::IsInputRedirected) { $script:IsTTY = $false }
} catch { $script:IsTTY = $false }
if (-not $script:IsTTY) { $Yes = $true }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
$AppName   = "Rum"
$Target    = Join-Path $Prefix "$AppName.exe"
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
        $script:LogFile = Join-Path $dir ("gui-install-{0:yyyyMMddTHHmmss}-pid{1}.log" -f (Get-Date).ToUniversalTime(), $PID)
        New-Item -ItemType File -Force -Path $script:LogFile | Out-Null
    } catch {
        $script:LogFile = Join-Path $env:TEMP "rum-gui-install-$PID.log"
    }
}

function Redact([string]$s) {
    return [regex]::Replace($s, '(TOKEN|SECRET|PASSWORD|API[_-]?KEY|AUTH|BEARER)[=:]\S+', '$1=***', 'IgnoreCase')
}

function Write-RumLog([string]$level, [string]$m) {
    $ts = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    $line = "{0} {1} {2}" -f $ts, $level, (Redact $m)
    if ($script:LogFile) {
        try { Add-Content -Path $script:LogFile -Value $line -ErrorAction SilentlyContinue } catch { Write-Verbose $_.Exception.Message }
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

function Invoke-Logged {
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [string[]]$ArgumentList = @(),
        [string]$Name = $FilePath
    )
    Write-RumLog "CMD  " ("$FilePath " + ($ArgumentList -join " "))
    $out = Join-Path $script:WorkDir ("cmd-{0}.out" -f [guid]::NewGuid().ToString("N"))
    try {
        $p = Start-Process -FilePath $FilePath -ArgumentList $ArgumentList -Wait -PassThru -NoNewWindow `
            -RedirectStandardOutput $out -RedirectStandardError $out
        if (Test-Path $out) {
            $text = Get-Content -Raw -ErrorAction SilentlyContinue $out
            if ($text) {
                Add-Content -Path $script:LogFile -Value $text -ErrorAction SilentlyContinue
                if ($PSBoundParameters.ContainsKey("Verbose") -or $VerbosePreference -eq "Continue") {
                    Write-Host $text
                }
            }
        }
        if ($p.ExitCode -ne 0) { throw "$Name exited $($p.ExitCode)" }
    } finally {
        Remove-Item -Force $out -ErrorAction SilentlyContinue
    }
}

function Get-CommandSafe($name) {
    return Get-Command $name -ErrorAction SilentlyContinue
}

function Sync-SessionPath {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = @($env:Path, $machine, $user) -join ";"
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
    $gomod = Join-Path $RepoRoot "go.mod"
    if (Test-Path $gomod) {
        $line = Select-String -Path $gomod -Pattern '^go\s+(\S+)' | Select-Object -First 1
        if ($line) { return $line.Matches[0].Groups[1].Value }
    }
    return "1.25.7"
}

function Get-WailsVer {
    $gomod = Join-Path $RepoRoot "go.mod"
    if (Test-Path $gomod) {
        $line = Select-String -Path $gomod -Pattern 'github.com/wailsapp/wails/v2\s+(\S+)' | Select-Object -First 1
        if ($line) { return $line.Matches[0].Groups[1].Value }
    }
    return $WailsFallback
}

function Convert-GoVersion([string]$v) {
    $v = $v -replace '^go', '' -replace '-.*$', ''
    try { return [version]$v } catch { return [version]"0.0.0" }
}

function Test-GoMeetsMin([string]$min) {
    $cmd = Get-CommandSafe "go"
    if (-not $cmd) { return $false }
    $cur = (& go env GOVERSION 2>$null)
    if (-not $cur) { return $false }
    return (Convert-GoVersion $cur) -ge (Convert-GoVersion $min)
}

function Test-GoNewerThanMin([string]$min) {
    $cmd = Get-CommandSafe "go"
    if (-not $cmd) { return $false }
    $cur = (& go env GOVERSION 2>$null)
    if (-not $cur) { return $false }
    return (Convert-GoVersion $cur) -gt (Convert-GoVersion $min)
}

function Test-NodeMeetsMin {
    $n = Get-CommandSafe "node"
    $p = Get-CommandSafe "npm"
    if (-not $n -or -not $p) { return $false }
    $cur = (& node -v 2>$null)
    if (-not $cur) { return $false }
    $cur = $cur.TrimStart("v")
    try { return ([version]$cur) -ge $NodeMin } catch { return $false }
}

function Get-CpuArch {
    $a = $env:PROCESSOR_ARCHITECTURE
    if ($a -match 'ARM64') { return @{ Go = "arm64"; Node = "arm64"; Winget = "arm64" } }
    return @{ Go = "amd64"; Node = "x64"; Winget = "x64" }
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
                    if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne -1978335189) {
                        # -1978335189 = already installed
                        throw "winget exit $LASTEXITCODE"
                    }
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
    $file = "go$ver.windows-$($arch.Go).zip"
    $dest = Join-Path $script:WorkDir $file
    Info "Obtaining Go $ver ($file)"
    $sha = ""
    try { $sha = Get-GoSha256 $file } catch { Warn "Could not fetch Go checksums: $($_.Exception.Message)" }
    $urls = @(
        "https://go.dev/dl/$file",
        "https://dl.google.com/go/$file",
        "$($DefaultMirror.TrimEnd('/'))/$file"
    )
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
            else { throw "Could not download Go $ver from any mirror or the module proxy" }
        } else {
            throw "Could not download Go $ver from any mirror or the module proxy"
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
        $extracted = Join-Path $script:WorkDir "go"
        Move-Item -Force $extracted $goroot
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

function Install-NodeZip {
    $arch = Get-CpuArch
    $shasums = Join-Path $script:WorkDir "node-SHASUMS256.txt"
    $base = "https://nodejs.org/dist/latest-v22.x"
    Info "Downloading Node.js LTS (v22) zip for win-$($arch.Node)"
    Get-RemoteFile -Url "$base/SHASUMS256.txt" -Dest $shasums
    $line = Get-Content $shasums | Where-Object { $_ -match "node-v[0-9.]+-win-$($arch.Node)\.zip$" } | Select-Object -First 1
    if (-not $line) { throw "No win-$($arch.Node) Node.js zip listed in SHASUMS256.txt" }
    $parts = $line.Trim() -split '\s+'
    $sha = $parts[0]
    $filename = $parts[1]
    $dest = Join-Path $script:WorkDir $filename
    $ok = $false
    foreach ($u in @("$base/$filename", "$($DefaultMirror.TrimEnd('/'))/$filename")) {
        try { Get-RemoteFile -Url $u -Dest $dest -Sha256 $sha; $ok = $true; break } catch { Write-Verbose $_.Exception.Message }
    }
    if (-not $ok) { throw "Could not download Node.js from nodejs.org" }
    $prefix = Join-Path $env:LOCALAPPDATA "Node"
    Info "Installing Node.js to $prefix (user-local)"
    if (Test-Path $prefix) { Remove-Item -Recurse -Force $prefix }
    Expand-Archive -Path $dest -DestinationPath $script:WorkDir -Force
    $unpacked = Get-ChildItem -Directory $script:WorkDir | Where-Object { $_.Name -like "node-v*" } | Select-Object -First 1
    Move-Item -Force $unpacked.FullName $prefix
    $env:Path = "$prefix;$env:Path"
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$prefix*") {
        $new = if ([string]::IsNullOrEmpty($userPath)) { $prefix } else { "$prefix;$userPath" }
        [Environment]::SetEnvironmentVariable("Path", $new, "User")
    }
    Sync-SessionPath
    Ok "Installed node $(node --version) / npm $(npm --version)"
}

function Install-RequiredGo([string]$min) {
    Sync-SessionPath
    if (Test-GoMeetsMin $min) {
        Ok "Found $(go version)"
    } else {
        if (Get-CommandSafe "go") { Warn "Go is too old ($(go version)); need >= $min" } else { Info "Go $min+ is not installed" }
        if ($DryRun) { Info "dry-run: would install Go $min"; return }
        $ok = Install-Pkg -WingetId "GoLang.Go" -ChocoId "golang" -ScoopId "go"
        Sync-SessionPath
        if (-not ($ok -and (Test-GoMeetsMin $min))) {
            Install-GoZip $min
            Sync-SessionPath
        }
        if (-not (Test-GoMeetsMin $min)) { throw "Go $min+ is required. Install from https://go.dev/dl/ and re-run." }
        Ok "Installed $(go version)"
    }
    if (Test-GoNewerThanMin $min) {
        Info "Go $(go version) is newer than $min; installing exact go$min for Wails (GOTOOLCHAIN=local)."
        if ($DryRun) { Info "dry-run: would install Go $min zip"; return }
        Install-GoZip $min
        Sync-SessionPath
    }
}

function Install-RequiredNode {
    Sync-SessionPath
    if (Test-NodeMeetsMin) { Ok "Found node $(node --version) / npm $(npm --version)"; return }
    if ($DryRun) { Info "dry-run: would install Node.js LTS"; return }
    $ok = Install-Pkg -WingetId "OpenJS.NodeJS.LTS" -ChocoId "nodejs-lts" -ScoopId "nodejs-lts"
    Sync-SessionPath
    if ($ok -and (Test-NodeMeetsMin)) { Ok "Installed node $(node --version) / npm $(npm --version) via package manager"; return }
    Install-NodeZip
    Sync-SessionPath
    if (-not (Test-NodeMeetsMin)) { throw "Node.js $NodeMin+ and npm are required. Install from https://nodejs.org and re-run." }
}

function Install-WebView2Runtime {
    $key = "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
    $key2 = "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
    if ((Test-Path $key) -or (Test-Path $key2)) { Ok "Found WebView2 runtime"; return }
    Info "WebView2 runtime not found — installing"
    if ($DryRun) { Info "dry-run: would install Microsoft.EdgeWebView2Runtime"; return }
    $ok = Install-Pkg -WingetId "Microsoft.EdgeWebView2Runtime" -ChocoId "webview2-runtime" -ScoopId "webview2-runtime"
    if (-not $ok) {
        Warn "Could not auto-install WebView2. The app needs it at run time (preinstalled on most Windows 10/11)."
    }
}

function Set-GoProxy {
    [CmdletBinding(SupportsShouldProcess)]
    param()
    if (-not $PSCmdlet.ShouldProcess("GOPROXY", "configure")) { return }
    if ($Mirror) {
        $env:GOPROXY = if ($Mirror -eq $DefaultMirror) { $script:BuiltInGoProxy } else { "$Mirror|$script:BuiltInGoProxy" }
        $env:GOSUMDB = $script:GoSumDbDefault
        Ok "Using Go module proxy chain: $env:GOPROXY"
        return
    }
    if ($env:GOPROXY -and $env:GOPROXY -ne "https://proxy.golang.org,direct") {
        Ok "Using GOPROXY from environment"
    }
}

function Invoke-GoNet {
    param([Parameter(ValueFromRemainingArguments=$true)][string[]]$GoArgs)
    try {
        Invoke-WithRetry -Name "go $($GoArgs -join ' ')" -Script {
            & go @GoArgs
            if ($LASTEXITCODE -ne 0) { throw "go exit $LASTEXITCODE" }
        }
    } catch {
        if ($env:GOPROXY -and $env:GOPROXY.Contains($script:IranianGoProxies[0])) { throw }
        Warn "Go command failed; retrying with bundled Iranian module mirrors"
        $env:GOPROXY = $script:BuiltInGoProxy
        $env:GOSUMDB = $script:GoSumDbDefault
        Invoke-WithRetry -Name "go $($GoArgs -join ' ') (mirror)" -Script {
            & go @GoArgs
            if ($LASTEXITCODE -ne 0) { throw "go exit $LASTEXITCODE" }
        }
    }
}

function Install-WailsCli([string]$ver) {
    Sync-SessionPath
    $gp = ""
    try { $gp = & go env GOPATH } catch { Write-Verbose $_.Exception.Message }
    if ($gp) { $env:Path = "$env:Path;$(Join-Path $gp 'bin')" }
    if (Get-CommandSafe "wails") { Ok "Found wails"; return }
    if ($DryRun) { Info "dry-run: would go install wails@$ver"; return }
    Info "Wails CLI not found — installing github.com/wailsapp/wails/v2/cmd/wails@$ver"
    Invoke-GoNet install "github.com/wailsapp/wails/v2/cmd/wails@$ver"
    if ($gp) { $env:Path = "$env:Path;$(Join-Path $gp 'bin')" }
    Sync-SessionPath
    if (-not (Get-CommandSafe "wails")) { throw "wails still not on PATH; add %GOPATH%\bin to PATH and re-run." }
    Ok "Found wails"
}

function Add-AppShortcut {
    [CmdletBinding(SupportsShouldProcess)]
    param([string]$LnkPath, [string]$TargetPath)
    if (-not $PSCmdlet.ShouldProcess($LnkPath, "create shortcut")) { return }
    $shell = New-Object -ComObject WScript.Shell
    $sc = $shell.CreateShortcut($LnkPath)
    $sc.TargetPath = $TargetPath
    $sc.WorkingDirectory = (Split-Path -Parent $TargetPath)
    $sc.Save()
}

Initialize-Log
$script:WorkDir = Join-Path $env:TEMP ("rum-gui-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $script:WorkDir | Out-Null
$script:TempDirs.Add($script:WorkDir)

try {
    if ($Uninstall) {
        if ($DryRun) { Info "dry-run: would remove $Prefix and shortcuts"; return }
        if (Test-Path $Prefix) { Remove-Item -Recurse -Force $Prefix; Ok "Removed $Prefix" }
        $sm = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$AppName.lnk"
        $dt = Join-Path ([Environment]::GetFolderPath("Desktop")) "$AppName.lnk"
        foreach ($l in @($sm, $dt)) {
            if (Test-Path $l) { Remove-Item -Force $l; Ok "Removed shortcut $l" }
        }
        Info "Log: $($script:LogFile)"
        return
    }

    $GoMin = Get-GoMin
    $WailsVer = Get-WailsVer
    $arch = Get-CpuArch
    $mgr = Get-PkgMgr
    Info "Detected: windows/$($arch.Go) pkg=$mgr tty=$($script:IsTTY) yes=$Yes prefix=$Prefix"
    Info "Go minimum (from go.mod): $GoMin  |  Wails: $WailsVer  |  Node minimum: $NodeMin"

    $plan = @()
    if (-not (Test-GoMeetsMin $GoMin)) { $plan += "Go $GoMin+" }
    if (-not (Test-NodeMeetsMin)) { $plan += "Node.js $NodeMin+ / npm" }
    if (-not (Get-CommandSafe "wails")) { $plan += "wails CLI $WailsVer" }
    if ($plan.Count -gt 0) { Info "Will install: $($plan -join ', ')" } else { Info "All build prerequisites already present" }
    if (Test-Path $Target) { Info "Existing install will be replaced: $Target" } else { Info "Install target: $Target" }
    Info "Log file: $($script:LogFile)"

    $free = (Get-PSDrive -Name ($env:LOCALAPPDATA.Substring(0,1))).Free
    if ($free -and $free -lt 2GB) { Warn "Low disk space on $($env:LOCALAPPDATA.Substring(0,1)): (GUI build may need ~2 GiB)" }

    $net = (Test-Reachable "https://proxy.golang.org") -or (Test-Reachable "https://go.dev") -or (Test-Reachable "https://github.com")
    if ($net) { Ok "Network is reachable" } else {
        Warn "Could not reach go.dev / proxy.golang.org / github.com — will try bundled Iranian module mirrors"
        if (-not $Mirror) { $Mirror = $DefaultMirror }
    }

    if ($DryRun) { Info "dry-run complete (no changes). Re-run without -DryRun to install."; return }

    Set-GoProxy
    Install-RequiredGo $GoMin
    Install-RequiredNode
    Install-WebView2Runtime
    Install-WailsCli $WailsVer

    $appicon = Join-Path $RepoRoot "build\appicon.png"
    if (-not (Test-Path $appicon)) {
        $src = @((Join-Path $RepoRoot "build\icon.png"), (Join-Path $RepoRoot "icon.png")) | Where-Object { Test-Path $_ } | Select-Object -First 1
        if ($src) {
            New-Item -ItemType Directory -Force -Path (Split-Path $appicon) | Out-Null
            Copy-Item -Force $src $appicon
            Info "Wired $src as the app icon."
        } else {
            Info "No source icon found; the build will use the default Wails icon."
        }
    }

    $frontend = Join-Path $RepoRoot "frontend"
    if (Test-Path $frontend) {
        Info "Installing frontend npm dependencies"
        Push-Location $frontend
        try {
            try {
                if (Test-Path "package-lock.json") {
                    Invoke-WithRetry -Name "npm ci" -Script {
                        & npm ci --no-audit --no-fund
                        if ($LASTEXITCODE -ne 0) { throw "npm ci exit $LASTEXITCODE" }
                    }
                } else {
                    throw "no lockfile"
                }
            } catch {
                Warn "npm ci failed; falling back to npm install"
                Invoke-WithRetry -Name "npm install" -Script {
                    & npm install --no-audit --no-fund
                    if ($LASTEXITCODE -ne 0) { throw "npm install exit $LASTEXITCODE" }
                }
            }
        } catch {
            Warn "npm install had errors; wails build will try again"
        } finally { Pop-Location }
    }

    Info "Building the Rum desktop app for Windows (wails build)…"
    $env:GOTOOLCHAIN = "local"
    Push-Location $RepoRoot
    try {
        try {
            & wails build -platform windows/amd64 -clean
            if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
        } catch {
            if (-not $env:GOPROXY -or -not $env:GOPROXY.Contains($script:IranianGoProxies[0])) {
                Warn "wails build failed; retrying with bundled Iranian module mirrors"
                $env:GOPROXY = $script:BuiltInGoProxy
                $env:GOSUMDB = $script:GoSumDbDefault
                & wails build -platform windows/amd64 -clean
                if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
            } else { throw }
        }
    } finally { Pop-Location }

    $BuiltExe = Join-Path $RepoRoot "build\bin\$AppName.exe"
    if (-not (Test-Path $BuiltExe)) { throw "Expected build output $BuiltExe not found" }
    Ok "Built $BuiltExe"

    $iscc = Get-CommandSafe "iscc"
    if ($Installer -or $iscc) {
        if (-not $iscc) { throw "Inno Setup (iscc.exe) not found — install it or drop -Installer." }
        Info "Building installer with Inno Setup…"
        Push-Location $RepoRoot
        try {
            Invoke-WithRetry -Name "iscc installer.iss" -Script {
                & $iscc.Source "installer.iss"
                if ($LASTEXITCODE -ne 0) { throw "iscc failed" }
            }
        } finally { Pop-Location }
        Ok "Installer created: $(Join-Path $RepoRoot 'build\bin\Rum-Setup.exe')"
        Info "Log: $($script:LogFile)"
        return
    }

    Info "Inno Setup not found — installing directly to $Prefix"
    New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
    $partial = "$Target.partial"
    Copy-Item -Force $BuiltExe $partial
    Move-Item -Force $partial $Target
    Ok "Installed to $Target"

    $startMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$AppName.lnk"
    $desktop   = Join-Path ([Environment]::GetFolderPath("Desktop")) "$AppName.lnk"
    Add-AppShortcut $startMenu $Target; Ok "Created Start Menu shortcut"
    Add-AppShortcut $desktop   $Target; Ok "Created Desktop shortcut"

    Write-Host ""
    Ok "Done. Launch Rum from the Start Menu or run: `"$Target`""
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
