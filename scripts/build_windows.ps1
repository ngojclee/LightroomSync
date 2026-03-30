param(
    [string]$Version = "2.0.0.0",
    [string]$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path,
    [string]$OutputDir = "build/bin",
    [switch]$SkipTests
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ($Version -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    throw "Version must match x.y.z.k format, received: $Version"
}

if (-not (Test-Path -LiteralPath $ProjectRoot)) {
    throw "ProjectRoot not found: $ProjectRoot"
}

$resolvedOutputDir = Join-Path $ProjectRoot $OutputDir
New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

function Resolve-DefaultGoPath {
    param([string]$Root)

    if ($env:USERPROFILE) {
        return (Join-Path $env:USERPROFILE "go")
    }
    if ($env:HOMEDRIVE -and $env:HOMEPATH) {
        return (Join-Path ($env:HOMEDRIVE + $env:HOMEPATH) "go")
    }

    $existing = Get-ChildItem -Path "C:\Users" -Directory -ErrorAction SilentlyContinue |
        ForEach-Object { Join-Path $_.FullName "go\pkg\mod" } |
        Where-Object { Test-Path -LiteralPath $_ } |
        Select-Object -First 1
    if ($existing) {
        return (Split-Path -Path (Split-Path -Path $existing -Parent) -Parent)
    }

    return (Join-Path $Root ".cache\gopath")
}

if (-not $env:GOPATH) {
    $env:GOPATH = Resolve-DefaultGoPath -Root $ProjectRoot
}
if (-not $env:GOMODCACHE) {
    $env:GOMODCACHE = Join-Path $env:GOPATH "pkg\mod"
}
if (-not $env:GOCACHE) {
    if ($env:LOCALAPPDATA) {
        $env:GOCACHE = Join-Path $env:LOCALAPPDATA "go-build"
    } else {
        $env:GOCACHE = Join-Path $env:TEMP "go-build"
    }
}

New-Item -ItemType Directory -Force -Path $env:GOMODCACHE | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null

Write-Host "[build] GOPATH=$($env:GOPATH)"
Write-Host "[build] GOMODCACHE=$($env:GOMODCACHE)"
Write-Host "[build] GOCACHE=$($env:GOCACHE)"

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Push-Location $ProjectRoot
try {
    if (-not $SkipTests) {
        Write-Host "[build] Running tests..."
        & go test ./... -count=1
        if ($LASTEXITCODE -ne 0) {
            throw "go test failed"
        }
    }

    $ldflags = "-s -w -X main.Version=$Version"

    $agentPath = Join-Path $resolvedOutputDir "LightroomSyncAgent.exe"
    $uiPath = Join-Path $resolvedOutputDir "LightroomSyncUI.exe"

    Write-Host "[build] Building Agent..."
    & go build -trimpath -ldflags $ldflags -o $agentPath ./cmd/agent
    if ($LASTEXITCODE -ne 0) {
        throw "agent build failed"
    }

    Write-Host "[build] Building UI..."
    & go build -trimpath -ldflags $ldflags -o $uiPath ./cmd/ui
    if ($LASTEXITCODE -ne 0) {
        throw "ui build failed"
    }

    $agentVersion = (& $agentPath --version 2>$null | Out-String).Trim()
    $uiVersion = (& $uiPath --version 2>$null | Out-String).Trim()
    if ($agentVersion -ne $Version) {
        throw "Agent version mismatch: expected=$Version actual=$agentVersion"
    }
    if ($uiVersion -ne $Version) {
        throw "UI version mismatch: expected=$Version actual=$uiVersion"
    }

    function Get-BinaryMetadata {
        param([Parameter(Mandatory = $true)][string]$Path)

        $file = Get-Item -LiteralPath $Path
        $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $Path

        [ordered]@{
            name           = $file.Name
            path           = $file.FullName
            size_bytes     = [int64]$file.Length
            sha256         = $hash.Hash.ToLowerInvariant()
            built_at_utc   = $file.LastWriteTimeUtc.ToString("o")
            file_version   = $file.VersionInfo.FileVersion
            product_version = $file.VersionInfo.ProductVersion
            verified_version_flag = if ($file.Name -like "*Agent*") { $agentVersion } else { $uiVersion }
        }
    }

    $gitCommit = "unknown"
    try {
        $candidate = (& git -C $ProjectRoot rev-parse --short HEAD 2>$null | Out-String).Trim()
        if ($candidate) {
            $gitCommit = $candidate
        }
    } catch {
    }

    $buildMetadata = [ordered]@{
        version      = $Version
        built_at_utc = (Get-Date).ToUniversalTime().ToString("o")
        git_commit   = $gitCommit
        go_version   = (& go version | Out-String).Trim()
        files        = @(
            (Get-BinaryMetadata -Path $agentPath),
            (Get-BinaryMetadata -Path $uiPath)
        )
    }

    $metadataPath = Join-Path $resolvedOutputDir "build-metadata.json"
    $buildMetadata | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $metadataPath -Encoding UTF8

    Write-Host "[build] OK"
    Write-Host "  Agent : $agentPath"
    Write-Host "  UI    : $uiPath"
    Write-Host "  Meta  : $metadataPath"
} finally {
    Pop-Location
}
