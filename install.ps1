# ==============================================================================
# hssh Windows PowerShell Installer
# Run: irm https://raw.githubusercontent.com/ilhammmaulana/hssh/main/install.ps1 | iex
# ==============================================================================

$ErrorActionPreference = "Stop"

function Write-Color([string]$text, [string]$color = "White") {
    Write-Host $text -ForegroundColor $color
}

function Show-Banner {
    Write-Host @"
  _               _     
 | |__  ___  ___ | |__  
 | '_ \/ __|/ __|| '_ \ 
 | | | \__ \\__ \| | | |
 |_| |_|___/|___/|_| |_|
"@ -ForegroundColor Cyan
    Write-Host "  Fast & Seamless OpenSSH Connection Manager (Windows)" -ForegroundColor Yellow
    Write-Host "  ====================================================" -ForegroundColor Blue
    Write-Host ""
}

Show-Banner

# 1. Check OpenSSH Client on Windows
Write-Color "Checking Windows OpenSSH Client..." "Gray"
$sshPath = Get-Command "ssh.exe" -ErrorAction SilentlyContinue

if (-not $sshPath) {
    Write-Color "[!] OpenSSH Client not found on your Windows system." "Yellow"
    Write-Color "Attempting to enable Windows OpenSSH Capability (requires Administrator if prompted)..." "Cyan"
    try {
        Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0 -ErrorAction Stop
        Write-Color "[✓] Windows OpenSSH Client enabled successfully!" "Green"
    } catch {
        Write-Color "[!] Could not automatically enable OpenSSH Client. Please run in Admin PowerShell:" "Red"
        Write-Color "    Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0" "White"
    }
} else {
    Write-Color "[✓] OpenSSH Client detected: $($sshPath.Source)" "Green"
}

# 2. Determine Installation Path
$InstallDir = Join-Path $HOME "bin"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TargetExe = Join-Path $InstallDir "hssh.exe"

# 3. Download or Copy Binary
Write-Color "`nInstalling hssh into $InstallDir..." "White"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path -ErrorAction SilentlyContinue
$LocalDist = Join-Path $ScriptDir "dist\hssh-windows-amd64.exe"

if (Test-Path $LocalDist) {
    Copy-Item -Path $LocalDist -Destination $TargetExe -Force
} else {
    $DownloadUrl = "https://github.com/ilhammmaulana/hssh/releases/latest/download/hssh-windows-amd64.exe"
    Write-Color "Downloading latest release binary from GitHub..." "Cyan"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TargetExe -UseBasicParsing
}

Write-Color "[✓] Binary placed at: $TargetExe" "Green"

# 4. Configure User PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Color "Adding $InstallDir to User PATH..." "Cyan"
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Color "[✓] PATH updated successfully." "Green"
}

# 5. Ensure SSH config
$sshDir = Join-Path $HOME ".ssh"
if (-not (Test-Path $sshDir)) {
    New-Item -ItemType Directory -Path $sshDir -Force | Out-Null
}
$sshConfig = Join-Path $sshDir "config"
if (-not (Test-Path $sshConfig)) {
    New-Item -ItemType File -Path $sshConfig -Force | Out-Null
}

Write-Host @"

====================================================
[✓] hssh has been successfully installed!
====================================================

Restart PowerShell or run:
  `$env:Path = [System.Environment]::GetEnvironmentVariable("Path","User")

Get started:
  hssh             # Launch interactive menu
  hssh add         # Add server & setup passwordless key
  hssh list        # View servers & ping status
  hssh help        # Full help manual

"@ -ForegroundColor Green
