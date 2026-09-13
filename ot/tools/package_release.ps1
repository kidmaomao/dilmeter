$ErrorActionPreference = "Stop"

$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$workspaceRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot ".."))
$version = if ($env:DILMETER_APP_VERSION) { $env:DILMETER_APP_VERSION.Trim() } else { "1.4.3" }
if ($version -notmatch '^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$') {
    throw "Invalid DILMETER_APP_VERSION: $version"
}
$releaseDir = [System.IO.Path]::GetFullPath((Join-Path $workspaceRoot "artifacts\release-v$version"))
$stageDir = [System.IO.Path]::GetFullPath((Join-Path $releaseDir "OT"))
$otExe = "DilmeterOT-v$version.exe"

if (-not $releaseDir.StartsWith($workspaceRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Release directory is outside the workspace."
}
if (-not $stageDir.StartsWith($releaseDir + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Invalid OT staging directory."
}

& (Join-Path $PSScriptRoot "assert_ui_color_theme.ps1")
$otServerMarker =
    (-join (@(0x670D, 0x52A1, 0x5668) | ForEach-Object { [char]$_ })) +
    " IP / CIDR " +
    (-join (@(0x7F51, 0x6BB5) | ForEach-Object { [char]$_ }))
& (Join-Path $PSScriptRoot "assert_embedded_frontend.ps1") -BinaryPaths @(
    (Join-Path $projectRoot $otExe)
) -AdditionalRequiredMarkers @("DilmeterOT", $otServerMarker)

$requiredFiles = @(
    $otExe,
    "release-notes-current.txt",
    "RESOURCE_PACK.md"
)
foreach ($relativePath in $requiredFiles) {
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot $relativePath) -PathType Leaf)) {
        throw "Missing OT release file: $relativePath"
    }
}

$guide = Get-ChildItem -LiteralPath $projectRoot -Filter "*.md" -File |
    Where-Object { $_.Name -notlike "README*" -and $_.Name -notlike "design-*" } |
    Sort-Object Length -Descending |
    Select-Object -First 1
if ($null -eq $guide) {
    throw "OT usage guide was not found."
}

New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null
if (Test-Path -LiteralPath $stageDir) {
    Remove-Item -LiteralPath $stageDir -Recurse -Force
}
New-Item -ItemType Directory -Path $stageDir | Out-Null

Copy-Item -LiteralPath (Join-Path $projectRoot $otExe) -Destination $stageDir -Force
Copy-Item -LiteralPath $guide.FullName -Destination (Join-Path $stageDir $guide.Name) -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "release-notes-current.txt") -Destination (Join-Path $stageDir "release-notes.txt") -Force
Copy-Item -LiteralPath (Join-Path $projectRoot "RESOURCE_PACK.md") -Destination (Join-Path $stageDir "RESOURCE_PACK.md") -Force

$zipPath = Join-Path $releaseDir "DilmeterOT.zip"
& (Join-Path $PSScriptRoot "create_release_zip.ps1") -SourceDirectory $stageDir -ZipPath $zipPath
& (Join-Path $PSScriptRoot "make_update_manifest.ps1") -AppName "DilmeterOT" -Version $version -ZipPath $zipPath -NotesPath (Join-Path $projectRoot "release-notes-current.txt")

Write-Host "Created DilmeterOT.zip and update manifest in $releaseDir"
