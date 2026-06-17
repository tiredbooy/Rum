<#
.SYNOPSIS
    Rum GUI (desktop) installer for Windows.
.DESCRIPTION
    Builds the Wails desktop app for Windows and installs it. If Inno Setup
    (iscc.exe) is available it produces a proper Rum-Setup.exe installer;
    otherwise it installs Rum.exe into %LOCALAPPDATA%\Programs\Rum and creates
    Start Menu and Desktop shortcuts.

    Prerequisites: Go 1.25+, Node.js + npm, and the Wails toolchain
    (https://wails.io/docs/gettingstarted/installation). WebView2 runtime is
    required at run time (present on Windows 10/11 by default).
.PARAMETER Prefix
    Install directory for the no-installer path (default: %LOCALAPPDATA%\Programs\Rum).
.PARAMETER Installer
    Force building the Inno Setup installer (fails if iscc.exe is missing).
.PARAMETER Uninstall
    Remove the app installed by the no-installer path.
.EXAMPLE
    .\installers\gui\install-windows.ps1
#>
param(
    [string]$Prefix = (Join-Path $env:LOCALAPPDATA "Programs\Rum"),
    [string]$Mirror = "",
    [switch]$Installer,
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

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = (Resolve-Path (Join-Path $ScriptDir "..\..")).Path
$AppName   = "Rum"
$Target    = Join-Path $Prefix "$AppName.exe"

function New-Shortcut($lnkPath, $targetPath) {
    $shell = New-Object -ComObject WScript.Shell
    $sc = $shell.CreateShortcut($lnkPath)
    $sc.TargetPath = $targetPath
    $sc.WorkingDirectory = (Split-Path -Parent $targetPath)
    $sc.Save()
}

if ($Uninstall) {
    if (Test-Path $Prefix) { Remove-Item -Recurse -Force $Prefix; Ok "Removed $Prefix" }
    $sm = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$AppName.lnk"
    $dt = Join-Path ([Environment]::GetFolderPath("Desktop")) "$AppName.lnk"
    foreach ($l in @($sm, $dt)) { if (Test-Path $l) { Remove-Item -Force $l; Ok "Removed shortcut $l" } }
    exit 0
}

# --- Prerequisites ---
# Collect ALL missing tools first, then print one actionable message.
$missing = @()
if (-not (Get-Command go   -ErrorAction SilentlyContinue)) { $missing += "Go (https://go.dev/dl/)" }
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { $missing += "Node.js (https://nodejs.org, the LTS installer)" }
if (-not (Get-Command npm  -ErrorAction SilentlyContinue)) { $missing += "npm (bundled with Node.js)" }
if ($missing.Count -gt 0) {
    Fail "Missing required tool(s) to build the GUI:"
    foreach ($m in $missing) { Write-Host "    - $m" -ForegroundColor Red }
    Write-Host "  Install them (click through each installer), reopen PowerShell, then re-run. See INSTALL.md." -ForegroundColor Red
    Write-Host "  Note: the app needs the WebView2 runtime at run time (preinstalled on Windows 10/11)." -ForegroundColor Yellow
    exit 1
}
Ok "Found $(go version)"
Ok "Found node $(node --version) / npm $(npm --version)"

# Set the mirror (if requested) BEFORE any 'go install' / 'wails build' downloads.
Set-GoProxy

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Info "Wails CLI not found — installing with 'go install'..."
    & go install github.com/wailsapp/wails/v2/cmd/wails@latest
    $env:Path = "$env:Path;$(& go env GOPATH)\bin"
    if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
        Fail "wails still not on PATH; add %GOPATH%\bin to PATH and re-run."; exit 1
    }
}
Ok "Found wails"

# --- Ensure YOUR app icon is wired into the build (not the Wails default) ---
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

# --- Build ---
Info "Building the Rum desktop app for Windows (wails build)..."
Push-Location $RepoRoot
try {
    & wails build -platform windows/amd64 -clean
    if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
} finally {
    Pop-Location
}
$BuiltExe = Join-Path $RepoRoot "build\bin\$AppName.exe"
if (-not (Test-Path $BuiltExe)) { Fail "Expected build output $BuiltExe not found"; exit 1 }
Ok "Built $BuiltExe"

# --- Path A: Inno Setup installer (if available or requested) ---
$iscc = Get-Command iscc -ErrorAction SilentlyContinue
if ($Installer -or $iscc) {
    if (-not $iscc) { Fail "Inno Setup (iscc.exe) not found — install it or drop -Installer."; exit 1 }
    Info "Building installer with Inno Setup..."
    Push-Location $RepoRoot
    try {
        & $iscc.Source "installer.iss"
        if ($LASTEXITCODE -ne 0) { throw "iscc failed" }
    } finally { Pop-Location }
    Ok "Installer created: $(Join-Path $RepoRoot 'build\bin\Rum-Setup.exe')"
    exit 0
}

# --- Path B: direct install + shortcuts ---
Info "Inno Setup not found — installing directly to $Prefix"
New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
Copy-Item -Force $BuiltExe $Target
Ok "Installed to $Target"

$startMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$AppName.lnk"
$desktop   = Join-Path ([Environment]::GetFolderPath("Desktop")) "$AppName.lnk"
New-Shortcut $startMenu $Target; Ok "Created Start Menu shortcut"
New-Shortcut $desktop   $Target; Ok "Created Desktop shortcut"

Write-Host ""
Ok "Done. Launch Rum from the Start Menu or run: `"$Target`""
