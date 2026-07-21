# GAIA Installer for Windows
# Run this script to install GAIA on your system

param(
    [string]$InstallDir = "$env:USERPROFILE\.gaia",
    [string]$Mode = "menu",     # "full", "client", "menu"
    [switch]$AddToPath = $true
)

$ErrorActionPreference = "Stop"

# ─── UI Helpers ──────────────────────────────────────────

function Write-Step { param([string]$M) Write-Host "  >> $M" -ForegroundColor Cyan }
function Write-OK   { param([string]$M) Write-Host "  OK $M" -ForegroundColor Green }
function Write-Warn { param([string]$M) Write-Host "  !! $M" -ForegroundColor Yellow }

function Show-Menu {
    Clear-Host
    Write-Host @"

  ╔══════════════════════════════════════════╗
  ║         GAIA — Installation Mode          ║
  ╚══════════════════════════════════════════╝

  Choose how you want to install GAIA:

"@ -ForegroundColor Magenta
    Write-Host "  [1]  Full Install" -ForegroundColor Green
    Write-Host "       GAIA runs on THIS machine (default)."
    Write-Host "       Installs the agent + all tools + TUI."
    Write-Host ""
    Write-Host "  [2]  Remote Client" -ForegroundColor Cyan
    Write-Host "       GAIA runs on a remote server (VPS/cloud)."
    Write-Host "       Installs only the desktop client to connect remotely."
    Write-Host ""
    $choice = Read-Host "  Select [1] or [2]"
    if ($choice -eq "2") { return "client" }
    return "full"
}

# ─── Installation Mode ───────────────────────────────────

if ($Mode -eq "menu") {
    $Mode = Show-Menu
}

Write-Host ""
Write-Host "  Mode: $Mode" -ForegroundColor Yellow
Write-Host ""

# ─── Full Installation ───────────────────────────────────

if ($Mode -eq "full") {
    Write-Step "Full installation selected"

    # 1. Find or build binary
    $binaryName = "gaia.exe"
    $sourceBinary = Join-Path $PSScriptRoot $binaryName
    $targetDir = Join-Path $InstallDir "bin"
    $targetBinary = Join-Path $targetDir $binaryName

    if (Test-Path $sourceBinary) {
        Write-Step "Found pre-built binary"
    } else {
        Write-Step "No binary found. Building from source..."
        $goCheck = Get-Command "go" -ErrorAction SilentlyContinue
        if (-not $goCheck) {
            Write-Warn "Go is not installed."
            Write-Warn "Download from: https://go.dev/dl/"
            Write-Warn "Then run this script again."
            exit 1
        }
        Push-Location $PSScriptRoot
        go build -ldflags="-s -w" -o $binaryName ./cmd/gaia/
        if ($LASTEXITCODE -ne 0) { throw "Build failed" }
        Pop-Location
        Write-OK "Built gaia.exe from source"
    }

    # 2. Create directories
    Write-Step "Creating directories..."
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
    New-Item -ItemType Directory -Force -Path "$InstallDir\skills" | Out-Null
    New-Item -ItemType Directory -Force -Path "$InstallDir\taps" | Out-Null
    Write-OK "Directories created"

    # 3. Copy binary
    Copy-Item -Path $sourceBinary -Destination $targetBinary -Force
    Write-OK "Binary installed to $targetBinary"

    # 4. Default config
    $configPath = "$InstallDir\config.yaml"
    if (-not (Test-Path $configPath)) {
@"
api_keys:
  openai: ""
  anthropic: ""
  copilot: ""
llm:
  provider: copilot
  model: claude-sonnet-4-20250514
  trust_mode: always
budget:
  max_iterations: 25
  compaction_threshold: 50
  keep_recent_messages: 20
"@ | Set-Content -Path $configPath -Encoding UTF8
        Write-OK "Created $configPath"
    } else {
        Write-Warn "Config exists, skipping"
    }

    # 5. PATH
    if ($AddToPath) {
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        if ($userPath -notlike "*$targetDir*") {
            [Environment]::SetEnvironmentVariable("PATH", "$targetDir;$userPath", "User")
            Write-OK "Added to PATH"
        }
    }

    # 6. Shortcut
    try {
        $scPath = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\GAIA.lnk"
        $shell = New-Object -ComObject WScript.Shell
        $sc = $shell.CreateShortcut($scPath)
        $sc.TargetPath = $targetBinary
        $sc.Description = "GAIA — Go AI Agent"
        $sc.Save()
        Write-OK "Created Start Menu shortcut"
    } catch { Write-Warn "Could not create shortcut" }

    # 7. Summary
    Write-Host @"

  ── Installation Complete (Full) ──

  Binary:    gaia.exe
  Config:    $configPath
  PATH:      Added

  Next steps:
    1. Edit your API keys:     notepad $configPath
    2. Run GAIA:               gaia
    3. Skills:                 gaia skills list
    4. Remote server:          gaia serve 8080

"@ -ForegroundColor Green
}

# ─── Client-Only Installation ────────────────────────────

if ($Mode -eq "client") {
    Write-Step "Remote client installation"

    $targetDir = Join-Path $InstallDir "bin"
    $targetBinary = Join-Path $targetDir "gaia.exe"

    # 1. Copy binary (same process, just different config)
    $binaryName = "gaia.exe"
    $sourceBinary = Join-Path $PSScriptRoot $binaryName

    if (-not (Test-Path $sourceBinary)) {
        Push-Location $PSScriptRoot
        go build -ldflags="-s -w" -o $binaryName ./cmd/gaia/
        if ($LASTEXITCODE -ne 0) { throw "Build failed" }
        Pop-Location
    }

    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
    Copy-Item -Path $sourceBinary -Destination $targetBinary -Force
    Write-OK "Binary installed"

    # 2. Client config — prompts for remote URL
    $configPath = "$InstallDir\config.yaml"
    $remoteUrl = Read-Host "  Enter your remote GAIA server URL (e.g. http://your-vps:8080)"
    if ($remoteUrl -eq "") { $remoteUrl = "http://localhost:8080" }

@"
remote:
  enabled: true
  url: $remoteUrl
llm:
  provider: remote
  model: remote
  trust_mode: never
"@ | Set-Content -Path $configPath -Encoding UTF8
    Write-OK "Created client config pointing to $remoteUrl"

    # 3. PATH
    if ($AddToPath) {
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        if ($userPath -notlike "*$targetDir*") {
            [Environment]::SetEnvironmentVariable("PATH", "$targetDir;$userPath", "User")
            Write-OK "Added to PATH"
        }
    }

    # 4. Shortcut — desktop app connecting to remote
    try {
        $scPath = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\GAIA Remote.lnk"
        $shell = New-Object -ComObject WScript.Shell
        $sc = $shell.CreateShortcut($scPath)
        $sc.TargetPath = $targetBinary
        $sc.Arguments = "serve $remoteUrl"
        $sc.Description = "GAIA — Remote Client"
        $sc.Save()
        Write-OK "Created Start Menu shortcut"
    } catch { Write-Warn "Could not create shortcut" }

    # 5. Summary
    Write-Host @"

  ── Installation Complete (Remote Client) ──

  Binary:    gaia.exe
  Server:    $remoteUrl
  Config:    $configPath

  Run the client:
    gaia

  Or use the Start Menu shortcut "GAIA Remote".

"@ -ForegroundColor Cyan
}

# ─── Done ────────────────────────────────────────────────

$runNow = Read-Host "  Run GAIA now? (Y/n)"
if ($runNow -ne "n") {
    Write-Host "  Starting GAIA..." -ForegroundColor Cyan
    & $targetBinary
}
