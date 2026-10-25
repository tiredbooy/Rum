<#
.SYNOPSIS
    Rum GUI (desktop) installer for Windows.
.DESCRIPTION
    Builds the Wails desktop app for Windows and installs it. Prerequisites
    (Go, Node.js + npm, Wails CLI, WebView2) are auto-detected. A Go that is
    already installed on go.mod's minor line is used as-is (GOTOOLCHAIN=local,
    nothing downloaded); otherwise Go is fetched from go.dev, dl.google.com or
    non-Google mirrors (checksum-verified). Node.js and WebView2 use
    winget / choco / scoop or official archives.

    Environment: RUM_GO_ROOT (use this Go), RUM_GO_MIRROR (try this mirror
    first), RUM_GO_NO_DOWNLOAD=1, RUM_NPM_REGISTRY, GOPROXY / GOSUMDB
    (respected; a fallback chain is used only if the default proxy is
    unreachable and you set neither).

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

function Get-WailsVer {
    $gomod = Join-Path $RepoRoot "go.mod"
    if (Test-Path $gomod) {
        $line = Select-String -Path $gomod -Pattern 'github.com/wailsapp/wails/v2\s+(\S+)' | Select-Object -First 1
        if ($line) { return $line.Matches[0].Groups[1].Value }
    }
    return $WailsFallback
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

function Invoke-GoNet {
    param([Parameter(ValueFromRemainingArguments=$true)][string[]]$GoArgs)
    try {
        Invoke-WithRetry -Name "go $($GoArgs -join ' ')" -Script {
            & go @GoArgs
            if ($LASTEXITCODE -ne 0) { throw "go exit $LASTEXITCODE" }
        }
    } catch {
        if (-not (Use-RumProxyFallback)) { throw }
        Warn "Go command failed; retrying with the fallback module proxies"
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

# The desktop build needs go.mod's minor line (Wails cannot typecheck newer Go).
$script:RumSameMinor = $true
$RumGoLib = Join-Path $RepoRoot "installers\lib\rum-go.ps1"
if (-not (Test-Path $RumGoLib)) {
    Write-Host "ERR Missing $RumGoLib - run this script from a full checkout of the repository." -ForegroundColor Red
    exit 1
}
. $RumGoLib

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

    $GoMin = Get-RumGoMin @((Join-Path $RepoRoot "go.mod"), (Join-Path $RepoRoot "backend\go.mod"))
    $WailsVer = Get-WailsVer
    $arch = Get-CpuArch
    $mgr = Get-PkgMgr
    Info "Detected: windows/$($arch.Go) pkg=$mgr tty=$($script:IsTTY) yes=$Yes prefix=$Prefix"
    Info "Go minimum (from go.mod): $GoMin  |  Wails: $WailsVer  |  Node minimum: $NodeMin"

    $plan = @()
    $null = Select-RumLocalGo $GoMin
    if (-not $script:RumGoBin) { $plan += "Go $GoMin+" }
    if (-not (Test-NodeMeetsMin)) { $plan += "Node.js $NodeMin+ / npm" }
    if (-not (Get-CommandSafe "wails")) { $plan += "wails CLI $WailsVer" }
    if ($plan.Count -gt 0) { Info "Will install: $($plan -join ', ')" } else { Info "All build prerequisites already present" }
    if (Test-Path $Target) { Info "Existing install will be replaced: $Target" } else { Info "Install target: $Target" }
    Info "Log file: $($script:LogFile)"

    Test-RumFreeSpace $env:LOCALAPPDATA 2GB
    Write-RumGoReport $GoMin
    Set-RumGoProxy

    if ($DryRun) { Install-RumGo $GoMin; Info "dry-run complete (no changes). Re-run without -DryRun to install."; return }

    Install-RumGo $GoMin
    Install-RequiredNode
    Set-RumNpmRegistry
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
            if (Use-RumProxyFallback) {
                Warn "wails build failed; retrying with the fallback module proxies"
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
    Ok "Done. Installed $AppName to $Target"
    Write-RumSummary
    Info "Next: launch Rum from the Start Menu or run: `"$Target`""
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
