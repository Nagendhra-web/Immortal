# Immortal — one-line installer (Windows PowerShell)
#
#   irm https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.ps1 | iex
#
# Tries, in order:
#   1. Download a pre-built release binary from GitHub Releases.
#   2. Fall back to `go install` (requires Go 1.25+ on PATH).
#
# Env overrides:
#   $env:IMMORTAL_VERSION   pin a specific release tag (default: latest)
#   $env:IMMORTAL_INSTALL   target directory (default: $env:LOCALAPPDATA\immortal\bin)

$ErrorActionPreference = "Stop"

$Repo    = "Nagendhra-web/Immortal"
$Version = if ($env:IMMORTAL_VERSION) { $env:IMMORTAL_VERSION } else { "latest" }
$Install = if ($env:IMMORTAL_INSTALL) { $env:IMMORTAL_INSTALL } else { "$env:LOCALAPPDATA\immortal\bin" }

New-Item -ItemType Directory -Force -Path $Install | Out-Null

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { Write-Error "unsupported arch: $env:PROCESSOR_ARCHITECTURE"; exit 1 }
}

function Try-Release {
    $url = if ($Version -eq "latest") {
        "https://github.com/$Repo/releases/latest/download/immortal_windows_${Arch}.zip"
    } else {
        "https://github.com/$Repo/releases/download/$Version/immortal_windows_${Arch}.zip"
    }
    Write-Host "→ Fetching $url"

    $tmp = New-TemporaryFile
    try {
        Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing -ErrorAction Stop
    } catch {
        Write-Host "  no release binary available at $url"
        Remove-Item $tmp -ErrorAction SilentlyContinue
        return $false
    }

    $extract = Join-Path ([System.IO.Path]::GetTempPath()) "immortal-install-$(Get-Random)"
    Expand-Archive -Path $tmp -DestinationPath $extract -Force
    Copy-Item -Path (Join-Path $extract "immortal.exe") -Destination (Join-Path $Install "immortal.exe") -Force
    Remove-Item $tmp
    Remove-Item $extract -Recurse -Force

    Write-Host "✓ installed → $Install\immortal.exe"
    return $true
}

function Try-GoInstall {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "go not found — cannot fall back to source build"
        return $false
    }

    $tag = if ($Version -eq "latest") { "@latest" } else { "@$Version" }
    Write-Host "→ Building via: go install github.com/$Repo/cmd/immortal$tag"

    $env:GOBIN = $Install
    & go install "github.com/$Repo/cmd/immortal$tag"
    Write-Host "✓ installed → $Install\immortal.exe"
    return $true
}

Write-Host "Installing Immortal ($Version) for windows/$Arch"

if (Try-Release) { }
elseif (Try-GoInstall) { }
else {
    Write-Error "Neither a release binary nor a local Go toolchain is available."
    Write-Error "Install Go (https://go.dev/dl/) and rerun, or clone + make build."
    exit 1
}

if ($env:Path -notlike "*$Install*") {
    Write-Host ""
    Write-Host "⚠  $Install is not on your PATH."
    Write-Host "   Add it (current user, persistent):"
    Write-Host "     [Environment]::SetEnvironmentVariable('Path', `"`$env:Path;$Install`", 'User')"
}

Write-Host ""
Write-Host "Get started:"
Write-Host "  immortal start --pqaudit --twin --agentic --causal --topology --formal"
