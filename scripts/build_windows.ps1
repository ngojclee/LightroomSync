param(
    [string]$Version = "2.0.0.0",
    [string]$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path,
    [string]$OutputDir = "build/bin",
    [string]$ReleaseDir = "build/release",
    [ValidateSet("harness", "wails")]
    [string]$UIRuntime = "harness",
    [switch]$AllowHarnessFallback,
    [switch]$SkipTests,
    [switch]$SkipReleaseAssets
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ($Version -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    throw "Version must match x.y.z.k format, received: $Version"
}

if (-not (Test-Path -LiteralPath $ProjectRoot)) {
    throw "ProjectRoot not found: $ProjectRoot"
}

$UIRuntime = $UIRuntime.Trim().ToLowerInvariant()

$resolvedOutputDir = Join-Path $ProjectRoot $OutputDir
New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

$resolvedReleaseDir = Join-Path $ProjectRoot $ReleaseDir
if (-not $SkipReleaseAssets) {
    New-Item -ItemType Directory -Force -Path $resolvedReleaseDir | Out-Null
}

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
    $uiRuntimeEffective = $UIRuntime
    $uiBuildWarnings = New-Object System.Collections.Generic.List[string]

    function Build-HarnessUI {
        param(
            [Parameter(Mandatory = $true)][string]$OutPath,
            [Parameter(Mandatory = $true)][string]$Ldflags
        )
        & go build -trimpath -ldflags $Ldflags -o $OutPath ./cmd/ui
        if ($LASTEXITCODE -ne 0) {
            throw "ui build failed"
        }
    }

    function Build-WailsUI {
        param(
            [Parameter(Mandatory = $true)][string]$OutPath,
            [Parameter(Mandatory = $true)][string]$Ldflags,
            [Parameter(Mandatory = $true)][string]$Root
        )

        $wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
        $previousCgo = $env:CGO_ENABLED
        $env:CGO_ENABLED = "1"
        $wailsBuildError = $null
        try {
            if ($wailsCmd) {
                $wailsArgs = @(
                    "build",
                    "-clean",
                    "-platform", "windows/amd64",
                    "-tags", "wails",
                    "-ldflags", $Ldflags
                )
                & $wailsCmd.Source @wailsArgs
                if ($LASTEXITCODE -eq 0) {
                    $candidates = @(
                        (Join-Path $Root "build\bin\LightroomSyncUI.exe"),
                        (Join-Path $Root "build\bin\LightroomSyncUI")
                    )
                    $builtPath = $null
                    foreach ($candidate in $candidates) {
                        if (Test-Path -LiteralPath $candidate) {
                            $builtPath = (Resolve-Path -LiteralPath $candidate).Path
                            break
                        }
                    }
                    if (-not $builtPath) {
                        throw "wails build succeeded but output binary not found in expected locations."
                    }

                    $builtFullPath = [System.IO.Path]::GetFullPath($builtPath)
                    $targetFullPath = [System.IO.Path]::GetFullPath($OutPath)
                    if ($targetFullPath -ne $builtFullPath) {
                        Copy-Item -LiteralPath $builtPath -Destination $OutPath -Force
                    }
                    return
                }
                $wailsBuildError = "wails build failed"
            } else {
                $wailsBuildError = "wails CLI not found in PATH."
            }

            Write-Warning "[build] Wails CLI path failed: $wailsBuildError. Attempting direct go build -tags wails..."
            & go build -trimpath -tags wails -ldflags $Ldflags -o $OutPath ./cmd/ui
            if ($LASTEXITCODE -ne 0) {
                throw "wails build failed; direct go build -tags wails also failed"
            }
            Write-Warning "[build] Built Wails runtime via direct go build fallback (without Wails CLI packaging)."
        } finally {
            $env:CGO_ENABLED = $previousCgo
        }
    }

    Write-Host "[build] Building Agent..."
    & go build -trimpath -ldflags $ldflags -o $agentPath ./cmd/agent
    if ($LASTEXITCODE -ne 0) {
        throw "agent build failed"
    }

    Write-Host "[build] Building UI (requested runtime=$UIRuntime)..."
    if ($UIRuntime -eq "wails") {
        try {
            Build-WailsUI -OutPath $uiPath -Ldflags $ldflags -Root $ProjectRoot
        } catch {
            if (-not $AllowHarnessFallback) {
                throw
            }
            $warningText = "Wails UI build failed, falling back to harness UI: $($_.Exception.Message)"
            $uiBuildWarnings.Add($warningText) | Out-Null
            Write-Warning $warningText
            Build-HarnessUI -OutPath $uiPath -Ldflags $ldflags
            $uiRuntimeEffective = "harness"
        }
    } else {
        Build-HarnessUI -OutPath $uiPath -Ldflags $ldflags
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
        version                = $Version
        built_at_utc           = (Get-Date).ToUniversalTime().ToString("o")
        git_commit             = $gitCommit
        go_version             = (& go version | Out-String).Trim()
        ui_runtime_requested   = $UIRuntime
        ui_runtime_effective   = $uiRuntimeEffective
        ui_runtime_fallback    = ($UIRuntime -ne $uiRuntimeEffective)
        ui_runtime_warnings    = @($uiBuildWarnings)
        files                  = @(
            (Get-BinaryMetadata -Path $agentPath),
            (Get-BinaryMetadata -Path $uiPath)
        )
    }

    $metadataPath = Join-Path $resolvedOutputDir "build-metadata.json"
    $buildMetadata | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $metadataPath -Encoding UTF8

    if (-not $SkipReleaseAssets) {
        $releasePrefix = "LightroomSync-v$Version-windows-amd64"
        $releaseAgentPath = Join-Path $resolvedReleaseDir "LightroomSyncAgent-v$Version-windows-amd64.exe"
        $releaseUIPath = Join-Path $resolvedReleaseDir "LightroomSyncUI-v$Version-windows-amd64.exe"
        $releaseMetadataPath = Join-Path $resolvedReleaseDir "$releasePrefix-build-metadata.json"
        $releaseZipPath = Join-Path $resolvedReleaseDir "$releasePrefix.zip"

        Copy-Item -LiteralPath $agentPath -Destination $releaseAgentPath -Force
        Copy-Item -LiteralPath $uiPath -Destination $releaseUIPath -Force
        Copy-Item -LiteralPath $metadataPath -Destination $releaseMetadataPath -Force

        $zipInputs = @($releaseAgentPath, $releaseUIPath, $releaseMetadataPath)
        if (Test-Path -LiteralPath $releaseZipPath) {
            Remove-Item -LiteralPath $releaseZipPath -Force
        }
        Compress-Archive -LiteralPath $zipInputs -DestinationPath $releaseZipPath -CompressionLevel Optimal
    }

    Write-Host "[build] OK"
    Write-Host "  Agent : $agentPath"
    Write-Host "  UI    : $uiPath (requested=$UIRuntime, effective=$uiRuntimeEffective)"
    Write-Host "  Meta  : $metadataPath"
    if (-not $SkipReleaseAssets) {
        Write-Host "  Release Dir : $resolvedReleaseDir"
    }
} finally {
    Pop-Location
}
