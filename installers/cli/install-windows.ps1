<#
.SYNOPSIS
    Rum CLI installer for Windows.
.DESCRIPTION
    Builds rum.exe from source and installs it into %LOCALAPPDATA%\Programs\Rum,
    adding that directory to the user PATH. A Go that is already installed and
    meets go.mod is used as-is (GOTOOLCHAIN=local, nothing downloaded); only if
    none fits is Go fetched from go.dev, dl.google.com or non-Google mirrors
    (checksum-verified), then winget/choco/scoop as a last resort.

    Environment: RUM_GO_ROOT (use this Go), RUM_GO_MIRROR (try this mirror
    first), RUM_GO_NO_DOWNLOAD=1, GOPROXY / GOSUMDB (respected; a fallback
    chain is used only if the default proxy is unreachable and you set neither).

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

function Add-UserPath($dir) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$dir*") {
        $newPath = if ([string]::IsNullOrEmpty($userPath)) { $dir } else { "$userPath;$dir" }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$dir;$env:Path"
        Info "Added $dir to your user PATH (restart your terminal to pick it up)."
    }
}

$RumGoLib = Join-Path $RepoRoot "installers\lib\rum-go.ps1"
if (-not (Test-Path $RumGoLib)) {
    Write-Host "ERR Missing $RumGoLib - run this script from a full checkout of the repository." -ForegroundColor Red
    exit 1
}
. $RumGoLib

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

    $GoMin = Get-RumGoMin @((Join-Path $BackendDir "go.mod"), (Join-Path $RepoRoot "go.mod"))
    $arch = Get-CpuArch
    $mgr = Get-PkgMgr
    Info "Detected: windows/$arch pkg=$mgr tty=$($script:IsTTY) yes=$Yes prefix=$Prefix"
    Info "Go minimum (from go.mod): $GoMin  |  install target: $Target"
    $null = Select-RumLocalGo $GoMin
    if (-not $script:RumGoBin) { Info "Will install: Go $GoMin+" } else { Info "All build prerequisites already present" }
    if (Test-Path $Target) { Info "Existing install will be replaced: $Target" }
    Info "Log file: $($script:LogFile)"

    Test-RumFreeSpace $Prefix 512MB
    Write-RumGoReport $GoMin
    Set-RumGoProxy

    if ($DryRun) { Install-RumGo $GoMin; Info "dry-run complete (no changes). Re-run without -DryRun to install."; return }

    Install-RumGo $GoMin

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
            if (Use-RumProxyFallback) {
                Warn "go mod download failed; retrying with the fallback module proxies"
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
            if (Use-RumProxyFallback) {
                Warn "go build failed; retrying with the fallback module proxies"
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
    Ok "Done. Installed rum.exe to $Target"
    Write-RumSummary
    Info "Next: open a new terminal, then run:  rum --version"
    Info "Remove later:  $($MyInvocation.MyCommand.Path) -Uninstall"
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
