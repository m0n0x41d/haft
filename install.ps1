# Quint Code Installer for Windows
#
# Installs the quint-code binary.
# After installation, run `quint-code init` in each project.
#
# Usage:
#   irm https://raw.githubusercontent.com/russosalv/quint-code-vscode/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "russosalv/quint-code-vscode"
$BinName = "quint-code.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\quint-code"

function Write-Logo {
    Write-Host ""
    Write-Host "    QUINT CODE" -ForegroundColor Red
    Write-Host "    First Principles Framework for AI-assisted engineering" -ForegroundColor DarkGray
    Write-Host ""
}

function Get-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }
}

function Install-QuintCode {
    Write-Logo
    Write-Host "   Installing Quint Code..." -ForegroundColor Cyan
    Write-Host ""

    $arch = Get-Arch
    $osArch = "windows-$arch"

    if (!(Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Try downloading release
    $apiUrl = "https://api.github.com/repos/$Repo/releases/latest"
    $downloadUrl = $null

    try {
        $release = Invoke-RestMethod -Uri $apiUrl -Headers @{ "User-Agent" = "quint-code-installer" }
        $asset = $release.assets | Where-Object { $_.name -like "*$osArch.zip" } | Select-Object -First 1
        if ($asset) {
            $downloadUrl = $asset.browser_download_url
        }
    } catch {
        # No release found, will build from source
    }

    if ($downloadUrl) {
        Write-Host "   Downloading release ($osArch)..." -ForegroundColor DarkGray
        $tmpZip = Join-Path $env:TEMP "quint-code-release.zip"
        $tmpDir = Join-Path $env:TEMP "quint-code-extract"

        Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpZip
        if (Test-Path $tmpDir) { Remove-Item $tmpDir -Recurse -Force }
        Expand-Archive -Path $tmpZip -DestinationPath $tmpDir -Force

        Copy-Item (Join-Path $tmpDir "bin\$BinName") (Join-Path $InstallDir $BinName) -Force
        Remove-Item $tmpZip -Force
        Remove-Item $tmpDir -Recurse -Force

        Write-Host "   [OK] Downloaded release" -ForegroundColor Green
    } else {
        Write-Host "   No release found, building from source..." -ForegroundColor Yellow

        $missing = @()
        if (!(Get-Command git -ErrorAction SilentlyContinue)) { $missing += "Git" }
        if (!(Get-Command go -ErrorAction SilentlyContinue))  { $missing += "Go" }

        if ($missing.Count -gt 0) {
            Write-Host ""
            Write-Host "   [ERROR] No prebuilt release found. Building from source requires: $($missing -join ', ')" -ForegroundColor Red
            Write-Host ""
            if ($missing -contains "Git") {
                Write-Host "   Install Git:  https://git-scm.com/download/win" -ForegroundColor Yellow
                Write-Host "   Or:           winget install Git.Git" -ForegroundColor Yellow
            }
            if ($missing -contains "Go") {
                Write-Host "   Install Go:   https://go.dev/dl/" -ForegroundColor Yellow
                Write-Host "   Or:           winget install GoLang.Go" -ForegroundColor Yellow
            }
            Write-Host ""
            exit 1
        }

        $repoDir = Join-Path $env:TEMP "quint-code-build"
        if (Test-Path $repoDir) { Remove-Item $repoDir -Recurse -Force }

        Write-Host "   Cloning repository..." -ForegroundColor DarkGray
        git clone --depth 1 "https://github.com/$Repo.git" $repoDir 2>&1 | Out-Null

        Write-Host "   Building binary..." -ForegroundColor DarkGray
        Push-Location (Join-Path $repoDir "src\mcp")
        go build -trimpath -o (Join-Path $InstallDir $BinName) .
        Pop-Location

        Remove-Item $repoDir -Recurse -Force
        Write-Host "   [OK] Built from source" -ForegroundColor Green
    }

    Write-Host "   [OK] Installed to $InstallDir\$BinName" -ForegroundColor Green

    # Check PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        Write-Host ""
        Write-Host "   Adding $InstallDir to user PATH..." -ForegroundColor Yellow
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Host "   [OK] Added to PATH (restart terminal for effect)" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "   Installation Complete!" -ForegroundColor Green
    Write-Host ""
    Write-Host "   Next step:" -ForegroundColor White
    Write-Host "   cd \path\to\your\project" -ForegroundColor White
    Write-Host "   quint-code init" -ForegroundColor White
    Write-Host ""
}

Install-QuintCode
