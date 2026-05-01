#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Build and install Rum on Windows.
.DESCRIPTION
    Detects the project root, builds the Go binary, and installs it to %USERPROFILE%\bin.
    Optionally adds that directory to the user's PATH.
#>

$ErrorActionPreference = "Stop"

# ------------------------------------------------------------------------------
# Colors & Styles (ANSI escape sequences for better visuals)
# If your terminal doesn’t support ANSI, it falls back to default colours.
# ------------------------------------------------------------------------------
$ESC = [char]27
$BOLD = "${ESC}[1m"
$RESET = "${ESC}[0m"
$GREEN = "${ESC}[0;32m"
$CYAN = "${ESC}[0;36m"
$YELLOW = "${ESC}[1;33m"
$MAGENTA = "${ESC}[0;35m"
$WHITE = "${ESC}[1;37m"
$BG_GREEN = "${ESC}[42m"
$BG_MAGENTA = "${ESC}[45m"
$BG_RED = "${ESC}[41m"

function Write-Box {
    param([string]$Color, [string]$Text)
    $width = 60
    $len = $Text.Length
    $pad = [math]::Floor(($width - $len - 2) / 2)
    $extra = ($width - $len - 2) % 2
    Write-Host "$Color╔$('═' * $width)╗$RESET"
    Write-Host "$Color║$(' ' * $pad)$Text$(' ' * ($pad + $extra))║$RESET"
    Write-Host "$Color╚$(' ' * ($width - 1))╝$RESET" -NoNewline
    Write-Host ""
}

# ------------------------------------------------------------------------------
# Spinner (uses Write-Progress for a smooth animation)
# ------------------------------------------------------------------------------
function Start-Spinner {
    param([string]$Activity, [scriptblock]$ScriptBlock)
    $job = Start-Job -ScriptBlock $ScriptBlock
    $spinChars = @('◐', '◓', '◑', '◒')
    $i = 0
    while ($job.State -eq 'Running') {
        Write-Progress -Activity $Activity -Status "$($spinChars[$i % 4]) Working..." -PercentComplete -1
        Start-Sleep -Milliseconds 100
        $i++
    }
    Write-Progress -Activity $Activity -Completed
    $result = Receive-Job -Job $job -Wait
    Remove-Job -Job $job
    return $result
}

# ------------------------------------------------------------------------------
# 1. Find project root (directory containing go.mod)
# ------------------------------------------------------------------------------
$PROJECT_ROOT = Get-Location
while (-not (Test-Path (Join-Path $PROJECT_ROOT "go.mod"))) {
    $PROJECT_ROOT = Split-Path $PROJECT_ROOT -Parent
    if ($PROJECT_ROOT -eq "") {
        Write-Host "${BG_RED} ERROR $RESET Cannot find Rum project root (no go.mod)." -ForegroundColor Red
        Write-Host "Make sure you run this script from inside the cloned repository."
        exit 1
    }
}
Set-Location $PROJECT_ROOT

# ------------------------------------------------------------------------------
# 2. Welcome & box
# ------------------------------------------------------------------------------
Clear-Host
Write-Host ""
Write-Host "$MAGENTA$BOLD          R U M   -   I N S T A L L E R$RESET"
Write-Host ""
Write-Box -Color $BG_MAGENTA -Text " Smart CLI Download Manager "
Write-Host ""
Write-Host "$CYAN This script will build and install Rum on your system.$RESET"
Write-Host ""

# ------------------------------------------------------------------------------
# 3. Check prerequisites
# ------------------------------------------------------------------------------
Write-Host "$BOLD▶ Checking prerequisites …$RESET"

# Check Go
$goPath = Get-Command go -ErrorAction SilentlyContinue
if (-not $goPath) {
    Write-Host "${BG_RED} ERROR $RESET Go is not installed." -ForegroundColor Red
    Write-Host "Please install Go from https://go.dev/doc/install and then re-run this script."
    exit 1
}
$goVersion = & go version
Write-Host "  ${GREEN}✓ $goVersion$RESET"

# Verify folder structure
if (-not (Test-Path "cmd\rum")) {
    Write-Host "${BG_RED} ERROR $RESET Could not find cmd\rum in the project root." -ForegroundColor Red
    Write-Host "Ensure you have the correct repository structure."
    exit 1
}
Write-Host "  ${GREEN}✓ Repository structure OK$RESET"
Write-Host ""

# ------------------------------------------------------------------------------
# 4. Build confirmation
# ------------------------------------------------------------------------------
$buildChoice = Read-Host "$YELLOW Ready to build Rum?$RESET ($BOLD Y$RESET es / $BOLD n$RESET o) [Y/n]"
if ([string]::IsNullOrWhiteSpace($buildChoice)) { $buildChoice = "y" }
if ($buildChoice -notmatch '^[Yy]$') {
    Write-Host "$YELLOW Build cancelled. Exiting.$RESET"
    exit 0
}

# ------------------------------------------------------------------------------
# 5. Optional Iranian mirror for Go modules
# ------------------------------------------------------------------------------
Write-Host ""
$mirrorChoice = Read-Host "$YELLOW Would you like to use an Iranian mirror for downloading Go modules?$RESET`n  (This can greatly speed up downloads and bypass restrictions for users in Iran) [Y/n]"
if ([string]::IsNullOrWhiteSpace($mirrorChoice)) { $mirrorChoice = "y" }

$buildCmd = "go build -o rum.exe ./cmd/rum"
if ($mirrorChoice -match '^[Yy]$') {
    $mirrorUrl = "https://mirror-go.runflare.com"
    Write-Host "  ${CYAN}✓ Using mirror: $mirrorUrl$RESET"
    $env:GOPROXY = $mirrorUrl
}
else {
    Write-Host "  ${CYAN}Using default Go proxy (or direct connection)$RESET"
}
Write-Host ""

# ------------------------------------------------------------------------------
# 6. Build with spinner
# ------------------------------------------------------------------------------
Write-Host "Building Rum binary …" -NoNewline
$buildResult = Start-Spinner -Activity "Building" -ScriptBlock {
    & go build -o rum.exe ./cmd/rum 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Build failed" }
}
if ($LASTEXITCODE -ne 0) {
    Write-Host "`r${BG_RED} ERROR $RESET Build failed." -ForegroundColor Red
    exit 1
}
Write-Host "`r  ${GREEN}✓ Build complete$RESET"

# Check binary
if (-not (Test-Path "rum.exe")) {
    Write-Host "${BG_RED} ERROR $RESET Build produced no executable." -ForegroundColor Red
    exit 1
}
$size = [math]::Round((Get-Item "rum.exe").Length / 1KB, 2)
Write-Host "  ${GREEN}✓ Binary created (${size} KB)$RESET"
Write-Host ""

# ------------------------------------------------------------------------------
# 7. Install directory
# ------------------------------------------------------------------------------
$INSTALL_DIR = Join-Path $env:USERPROFILE "bin"
Write-Host "$BOLD▶ Preparing installation directory …$RESET"
if (-not (Test-Path $INSTALL_DIR)) {
    New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
}
Write-Host "  ${GREEN}✓ $INSTALL_DIR$RESET"

# Overwrite check
$targetExe = Join-Path $INSTALL_DIR "rum.exe"
if (Test-Path $targetExe) {
    $overwrite = Read-Host "$YELLOW An existing Rum binary was found. Overwrite?$RESET [Y/n]"
    if ([string]::IsNullOrWhiteSpace($overwrite)) { $overwrite = "y" }
    if ($overwrite -notmatch '^[Yy]$') {
        Write-Host "$YELLOW Installation aborted.$RESET"
        exit 0
    }
}

Copy-Item -Force "rum.exe" $INSTALL_DIR
Write-Host "  ${GREEN}✓ Binary installed to $targetExe$RESET"
Write-Host ""

# ------------------------------------------------------------------------------
# 8. PATH configuration
# ------------------------------------------------------------------------------
$pathChoice = Read-Host "$YELLOW Would you like to add Rum to your PATH?$RESET ($BOLD Y$RESET es / $BOLD n$RESET o) [Y/n]"
if ([string]::IsNullOrWhiteSpace($pathChoice)) { $pathChoice = "y" }

if ($pathChoice -match '^[Yy]$') {
    Write-Host "$BOLD▶ Configuring PATH …$RESET"
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -like "*$INSTALL_DIR*") {
        Write-Host "  ${CYAN}✓ Already present in user PATH$RESET"
    }
    else {
        $newPath = if ($userPath) { "$userPath;$INSTALL_DIR" } else { $INSTALL_DIR }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "  ${GREEN}✓ Added $INSTALL_DIR to user PATH$RESET"
        Write-Host "  $YELLOW Note: This change will apply to new PowerShell sessions.$RESET"
    }
}
else {
    Write-Host "$YELLOW Skipping PATH setup. Add $INSTALL_DIR to your PATH manually if needed.$RESET"
}
Write-Host ""

# ------------------------------------------------------------------------------
# 9. Cleanup & finish
# ------------------------------------------------------------------------------
Remove-Item -Force "rum.exe" -ErrorAction SilentlyContinue

Clear-Host
Write-Host ""
Write-Box -Color $BG_GREEN -Text " Installation Complete! "
Write-Host ""
Write-Host "$WHITE$BOLD Rum is now installed on your system!$RESET"
Write-Host ""
Write-Host "To start using it immediately, either:"
Write-Host "  1. Open a new PowerShell window"
Write-Host "  2. Run: `$env:Path = [System.Environment]::GetEnvironmentVariable('Path','User') + ';' + [System.Environment]::GetEnvironmentVariable('Path','Machine')"
Write-Host ""
Write-Host "Then try: rum --help"
Write-Host ""
Write-Host "$YELLOW Happy downloading! 🚀$RESET"
Write-Host ""