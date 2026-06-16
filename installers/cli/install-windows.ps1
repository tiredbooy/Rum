<#
.SYNOPSIS
    Rum CLI installer for Windows.
.DESCRIPTION
    Builds the `rum.exe` command-line download manager from source and installs
    it into %LOCALAPPDATA%\Programs\Rum, adding that directory to the user PATH.
.PARAMETER Prefix
    Install directory (default: $env:LOCALAPPDATA\Programs\Rum).
.PARAMETER Uninstall
    Remove an installed rum.exe instead of installing.
.EXAMPLE
    .\installers\cli\install-windows.ps1
.EXAMPLE
    .\installers\cli\install-windows.ps1 -Uninstall
#>
param(
    [string]$Prefix = (Join-Path $env:LOCALAPPDATA "Programs\Rum"),
    [string]$Mirror = "",
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$DefaultMirror = "https://go.devneeds.ir/"

function Info($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Ok($m)   { Write-Host "OK  $m" -ForegroundColor Green }
function Fail($m) { Write-Host "ERR $m" -ForegroundColor Red }

# On restricted networks Go's default servers can return 403/timeouts. Offer a
# Go module proxy mirror (GOPROXY); disable GOSUMDB since sum.golang.org is
# usually blocked too.
function Set-GoProxy {
    if (-not $Mirror) {
        $ans = Read-Host "Use a Go module mirror? Helps if downloads fail with 403/timeouts (e.g. in Iran) [y/N]"
        if ($ans -match '^[Yy]') {
            $url = Read-Host "Mirror URL [$DefaultMirror]"
            $script:Mirror = if ([string]::IsNullOrWhiteSpace($url)) { $DefaultMirror } else { $url }
        }
    }
    if ($Mirror) {
        $env:GOPROXY = $Mirror
        $env:GOSUMDB = "off"
        Ok "Using Go module mirror: $env:GOPROXY"
    }
}

# Resolve repo root from this script: installers\cli\ -> repo
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
$BackendDir = Join-Path $RepoRoot "backend"
$Target = Join-Path $Prefix "rum.exe"

function Add-UserPath($dir) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$dir*") {
        $newPath = if ([string]::IsNullOrEmpty($userPath)) { $dir } else { "$userPath;$dir" }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Info "Added $dir to your user PATH (restart your terminal to pick it up)."
    }
}

if ($Uninstall) {
    if (Test-Path $Target) {
        Info "Removing $Target"
        Remove-Item -Force $Target
        Ok "Removed $Target"
    } else {
        Info "Nothing to remove at $Target"
    }
    exit 0
}

# --- Prerequisites ---
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    Fail "Go is not installed. Install Go 1.25+ from https://go.dev/doc/install"
    exit 1
}
Ok "Found $(go version)"

if (-not (Test-Path (Join-Path $BackendDir "cmd\rum"))) {
    Fail "Cannot find $BackendDir\cmd\rum — run this from a clean checkout."
    exit 1
}

Set-GoProxy

# --- Build ---
Info "Building rum.exe (this may take a moment)..."
New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
Push-Location $BackendDir
try {
    & go build -trimpath -ldflags "-s -w" -o $Target ".\cmd\rum"
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally {
    Pop-Location
}
Ok "Installed to $Target"

Add-UserPath $Prefix

Write-Host ""
Ok "Done. Open a new terminal and try:  rum --version"
