# Run this script from the repository root (where cmd/rum is)
# PowerShell might require execution policy: Set-ExecutionPolicy -Scope CurrentUser RemoteSigned

$ErrorActionPreference = "Stop"

$green = "Green"
$cyan  = "Cyan"
$yellow = "Yellow"

Write-Host "[INFO] Checking Go ..." -ForegroundColor $cyan
if (Get-Command go -ErrorAction SilentlyContinue) {
    go version
} else {
    Write-Host "Go is not installed. Please install Go first: https://go.dev/doc/install" -ForegroundColor Red
    exit 1
}

Write-Host "[INFO] Building Rum ..." -ForegroundColor $cyan
go build -o rum.exe ./cmd/rum
Write-Host "[OK] Build complete" -ForegroundColor $green

# Install directory
$installDir = "$env:USERPROFILE\bin"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

Move-Item -Force rum.exe $installDir
Write-Host "[OK] Binary installed to $installDir\rum.exe" -ForegroundColor $green

# Add to User PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "[OK] Added $installDir to user PATH" -ForegroundColor $green
    # Note: The PATH change will only take effect in new PowerShell sessions.
} else {
    Write-Host "[INFO] $installDir is already in PATH" -ForegroundColor $cyan
}

Write-Host ""
Write-Host "✓ Rum is ready! Open a new PowerShell window and type: rum --help" -ForegroundColor $green
