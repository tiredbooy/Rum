<#
.SYNOPSIS
    Rum CLI installer (backend/ convenience wrapper).
.DESCRIPTION
    Forwards every argument to the hardened CLI installer at
    installers/cli/install-windows.ps1. Flags: -Prefix, -Mirror, -Uninstall,
    -Yes, -DryRun, -Verbose.
#>
$ErrorActionPreference = "Stop"
$Installer = Join-Path $PSScriptRoot "..\installers\cli\install-windows.ps1"
if (-not (Test-Path $Installer)) {
    Write-Host "ERR Cannot find $Installer — run this from a clean checkout." -ForegroundColor Red
    exit 1
}
& $Installer @args
exit $LASTEXITCODE
