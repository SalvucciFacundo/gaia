# GAIA Installer for Windows
# Run this script to install GAIA on your system

param(
    [string]$InstallDir = "$env:USERPROFILE\.gaia",
    [switch]$AddToPath = $true
)

$ErrorActionPreference = "Stop"
$Host.UI.RawUI.WindowTitle = "GAIA — Installing..."

function Write-Step {
    param([string]$Message, [string]$Status = ">>>")
    Write-Host " $Status $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "  DONE $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "  WARN $Message" -ForegroundColor Yellow
}

# 1. Banner
Write-Host @"

    ╔══════════════════════════════════════════╗
    ║           GAIA — Go AI Agent             ║
    ║     Programming-first autonomous agent    ║
    ╚══════════════════════════════════════════╝

"@ -ForegroundColor Magenta

# 2. Detect architecture
Write-Step "Detecting system..."
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$os = "windows"
Write-Success "$os / $arch"

# 3. Find or build the binary
$binaryName = "gaia.exe"
$sourceBinary = Join-Path $PSScriptRoot $binaryName
$targetDir = Join-Path $InstallDir "bin"
$targetBinary = Join-Path $targetDir $binaryName

if (Test-Path $sourceBinary) {
    Write-Step "Found pre-built binary: $sourceBinary"
} else {
    Write-Step "No pre-built binary found. Building from source..."
    $goCheck = Get-Command "go" -ErrorAction SilentlyContinue
    if (-not $goCheck) {
        Write-Warning "Go is not installed. Attempting to download binary..."
        Write-Warning "Please install Go from https://go.dev/dl/ first, then run this script again."
        Write-Host "  Or download the latest GAIA release from:"
        Write-Host "  https://github.com/SalvucciFacundo/gaia/releases" -ForegroundColor Cyan
        exit 1
    }
    
    Push-Location $PSScriptRoot
    try {
        go build -o $binaryName ./cmd/gaia/
        if ($LASTEXITCODE -ne 0) { throw "Build failed" }
        Write-Success "Built gaia.exe from source"
    } finally {
        Pop-Location
    }
}

# 4. Create directories
Write-Step "Creating directories..."
New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallDir\skills" | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallDir\taps" | Out-Null
Write-Success "Created $InstallDir"

# 5. Copy binary
Write-Step "Installing binary..."
Copy-Item -Path $sourceBinary -Destination $targetBinary -Force
Write-Success "Installed to $targetBinary"

# 6. Create default config
Write-Step "Creating default configuration..."
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
"@ | Set-Content -Path $configPath
    Write-Success "Created $configPath"
} else {
    Write-Warning "Config already exists, skipping: $configPath"
}

# 7. Add to PATH
if ($AddToPath) {
    Write-Step "Adding to PATH..."
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($userPath -notlike "*$targetDir*") {
        $newPath = "$targetDir;$userPath"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Success "Added $targetDir to PATH"
        Write-Warning "Restart your terminal or run: `$env:PATH = `"$targetDir;`$env:PATH`""
    } else {
        Write-Success "Already in PATH"
    }
}

# 8. Create start menu shortcut
Write-Step "Creating Start Menu shortcut..."
try {
    $shortcutPath = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\GAIA.lnk"
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $targetBinary
    $shortcut.WorkingDirectory = "%USERPROFILE%"
    $shortcut.Description = "GAIA — Go AI Agent"
    $shortcut.Save()
    Write-Success "Created Start Menu shortcut"
} catch {
    Write-Warning "Could not create shortcut: $_"
}

# 9. Summary
Write-Host @"

    ─── Installation Complete ───

  GAIA is installed at:
    $InstallDir

  Binary:
    $targetBinary

"@ -ForegroundColor Green

Write-Host "  Quick Start:" -ForegroundColor Yellow
Write-Host "   1. Edit your API keys:   notepad $configPath" -ForegroundColor White
Write-Host "   2. Run the wizard:       gaia" -ForegroundColor White
Write-Host "   3. List skills:          gaia skills list" -ForegroundColor White
Write-Host "   4. Check health:         gaia doctor" -ForegroundColor White
Write-Host "   5. Start gateway:        gaia gateway start" -ForegroundColor White
Write-Host "   6. Remote server:        gaia serve 8080" -ForegroundColor White
Write-Host ""

# 10. Offer to run
$runNow = Read-Host "  Run GAIA now? (Y/n)"
if ($runNow -ne "n") {
    Write-Host "`n  Starting GAIA...`n" -ForegroundColor Cyan
    & $targetBinary
}
