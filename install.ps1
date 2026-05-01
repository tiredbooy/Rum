$ErrorActionPreference = "Stop"

# Find project root (directory containing go.mod)
$PROJECT_ROOT = Get-Location
while (-not (Test-Path (Join-Path $PROJECT_ROOT "go.mod"))) {
    $PROJECT_ROOT = Split-Path $PROJECT_ROOT -Parent
    if ($PROJECT_ROOT -eq "") {
        Write-Host "ERROR: Cannot find Rum project root (no go.mod)." -ForegroundColor Red
        Write-Host "Make sure you run this script from inside the cloned repository."
        exit 1
    }
}
Set-Location $PROJECT_ROOT

Write-Host "=== Rum Installer ===" -ForegroundColor Cyan
Write-Host ""

# Check Go
Write-Host "Checking prerequisites..."
$goPath = Get-Command go -ErrorAction SilentlyContinue
if (-not $goPath) {
    Write-Host "ERROR: Go is not installed." -ForegroundColor Red
    Write-Host "Please install Go from https://go.dev/doc/install"
    exit 1
}
Write-Host "  Found: $(go version)" -ForegroundColor Green

if (-not (Test-Path "cmd\rum")) {
    Write-Host "ERROR: Could not find cmd\rum in the project root." -ForegroundColor Red
    exit 1
}
Write-Host "  Repository structure OK" -ForegroundColor Green
Write-Host ""

# Build confirmation
$buildChoice = Read-Host "Ready to build Rum? (Y/n)"
if ($buildChoice -eq "") { $buildChoice = "y" }
if ($buildChoice -ne "y" -and $buildChoice -ne "Y") {
    Write-Host "Build cancelled."
    exit 0
}

# Optional mirror
$mirrorChoice = Read-Host "Use Iranian mirror for Go modules? (Y/n)"
if ($mirrorChoice -eq "") { $mirrorChoice = "y" }
if ($mirrorChoice -eq "y" -or $mirrorChoice -eq "Y") {
    $env:GOPROXY = "https://mirror-go.runflare.com"
    Write-Host "  Using mirror: $env:GOPROXY" -ForegroundColor Green
} else {
    Write-Host "  Using default Go proxy" -ForegroundColor Green
}

# Build
Write-Host "Building Rum binary..."
& go build -o rum.exe ./cmd/rum
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed." -ForegroundColor Red
    exit 1
}
Write-Host "  Build complete" -ForegroundColor Green

$size = [math]::Round((Get-Item "rum.exe").Length / 1KB, 2)
Write-Host "  Binary size: ${size} KB" -ForegroundColor Green

# Install directory
$INSTALL_DIR = Join-Path $env:USERPROFILE "bin"
if (-not (Test-Path $INSTALL_DIR)) {
    New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
}
Write-Host "Installing to $INSTALL_DIR..."

$targetExe = Join-Path $INSTALL_DIR "rum.exe"
if (Test-Path $targetExe) {
    $overwrite = Read-Host "Existing rum.exe found. Overwrite? (Y/n)"
    if ($overwrite -eq "") { $overwrite = "y" }
    if ($overwrite -ne "y" -and $overwrite -ne "Y") {
        Write-Host "Installation aborted."
        exit 0
    }
}
Copy-Item -Force "rum.exe" $INSTALL_DIR
Write-Host "  Installed to $targetExe" -ForegroundColor Green

# PATH configuration
$pathChoice = Read-Host "Add $INSTALL_DIR to your PATH? (Y/n)"
if ($pathChoice -eq "") { $pathChoice = "y" }
if ($pathChoice -eq "y" -or $pathChoice -eq "Y") {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -like "*$INSTALL_DIR*") {
        Write-Host "  Already in PATH" -ForegroundColor Green
    } else {
        $newPath = if ($userPath) { "$userPath;$INSTALL_DIR" } else { $INSTALL_DIR }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "  Added to user PATH" -ForegroundColor Green
        Write-Host "  NOTE: This will take effect in new PowerShell windows." -ForegroundColor Yellow
    }
} else {
    Write-Host "Skipping PATH setup. You can manually add $INSTALL_DIR to your PATH." -ForegroundColor Yellow
}

# Cleanup
Remove-Item -Force "rum.exe" -ErrorAction SilentlyContinue

# Done
Write-Host ""
Write-Host "=== Installation Complete! ===" -ForegroundColor Green
Write-Host ""
Write-Host "To use Rum:"
Write-Host "  1. Open a NEW PowerShell window"
Write-Host "  2. Or refresh PATH in this window by running:"
Write-Host ""
Write-Host '     $env:Path = [System.Environment]::GetEnvironmentVariable("Path","User") + ";" + [System.Environment]::GetEnvironmentVariable("Path","Machine")'
Write-Host ""
Write-Host "Then type: rum --help"
Write-Host ""