$ErrorActionPreference = "Stop"

$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$workspaceRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot ".."))
$version = if ($env:DILMETER_APP_VERSION) { $env:DILMETER_APP_VERSION.Trim() } else { "1.5.1" }
if ($version -notmatch '^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$') {
    throw "Invalid DILMETER_APP_VERSION: $version"
}
$releaseDir = [System.IO.Path]::GetFullPath((Join-Path $workspaceRoot "artifacts\release-v$version"))
$cnExe = "DilmeterCN-v$version.exe"
$rtExe = "DilmeterRT-v$version.exe"
if (-not $releaseDir.StartsWith($workspaceRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Release directory is outside the workspace."
}

# Every CN feature must pass all 14 semantic UI palettes before packaging.
& (Join-Path $PSScriptRoot "assert_ui_color_theme.ps1")
& (Join-Path $PSScriptRoot "assert_embedded_frontend.ps1") -BinaryPaths @(
    (Join-Path $projectRoot $cnExe),
    (Join-Path $projectRoot $rtExe)
)

$requiredFiles = @(
    $cnExe,
    $rtExe,
    "release-notes-current.txt",
    "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert.dll",
    "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert64.sys",
    "third_party\windivert\WinDivert-2.2.2-A\LICENSE"
)
foreach ($relativePath in $requiredFiles) {
    $fullPath = Join-Path $projectRoot $relativePath
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        throw "Missing release file: $relativePath"
    }
}

$cnGuide = Get-ChildItem -LiteralPath $projectRoot -Filter "*.md" -File |
    Where-Object { $_.Name -notlike "README*" -and $_.Name -notlike "DilmeterRT-*" } |
    Sort-Object Length -Descending |
    Select-Object -First 1
$rtGuide = Get-ChildItem -LiteralPath $projectRoot -Filter "DilmeterRT-*.md" -File | Select-Object -First 1
if ($null -eq $cnGuide -or $null -eq $rtGuide) {
    throw "Release usage guides were not found."
}

$cnStage = Join-Path $releaseDir "CN"
$rtStage = Join-Path $releaseDir "RT"
New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null
foreach ($stage in @($cnStage, $rtStage)) {
    $resolvedStage = [System.IO.Path]::GetFullPath($stage)
    if (-not $resolvedStage.StartsWith($releaseDir + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Invalid staging directory."
    }
    if (Test-Path -LiteralPath $resolvedStage) {
        Remove-Item -LiteralPath $resolvedStage -Recurse -Force
    }
    New-Item -ItemType Directory -Path $resolvedStage | Out-Null
}

Copy-Item -LiteralPath (Join-Path $projectRoot $cnExe) -Destination $cnStage -Force
Copy-Item -LiteralPath $cnGuide.FullName -Destination (Join-Path $cnStage $cnGuide.Name) -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "release-notes-current.txt") -Destination (Join-Path $cnStage "release-notes.txt") -Force
& (Join-Path $PSScriptRoot "create_release_zip.ps1") -SourceDirectory $cnStage -ZipPath (Join-Path $releaseDir "DilmeterCN.zip")
& (Join-Path $PSScriptRoot "make_update_manifest.ps1") -AppName "DilmeterCN" -Version $version -ZipPath (Join-Path $releaseDir "DilmeterCN.zip") -NotesPath (Join-Path $projectRoot "release-notes-current.txt")

Copy-Item -LiteralPath (Join-Path $projectRoot $rtExe) -Destination $rtStage -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert.dll") -Destination $rtStage -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert64.sys") -Destination $rtStage -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "third_party\windivert\WinDivert-2.2.2-A\LICENSE") -Destination (Join-Path $rtStage "WinDivert-LICENSE.txt") -Force
Copy-Item -LiteralPath $rtGuide.FullName -Destination (Join-Path $rtStage $cnGuide.Name) -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "release-notes-current.txt") -Destination (Join-Path $rtStage "release-notes.txt") -Force
& (Join-Path $PSScriptRoot "create_release_zip.ps1") -SourceDirectory $rtStage -ZipPath (Join-Path $releaseDir "DilmeterRT.zip")
& (Join-Path $PSScriptRoot "make_update_manifest.ps1") -AppName "DilmeterRT" -Version $version -ZipPath (Join-Path $releaseDir "DilmeterRT.zip") -NotesPath (Join-Path $projectRoot "release-notes-current.txt")

Write-Host "Created DilmeterCN.zip, DilmeterRT.zip, and update manifests in $releaseDir"
