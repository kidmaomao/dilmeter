$ErrorActionPreference = "Stop"

$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$workspaceRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot ".."))
$releaseDir = [System.IO.Path]::GetFullPath((Join-Path $workspaceRoot "release-v1.4.0-test"))
$packageVariant = if ([string]::IsNullOrWhiteSpace($env:DILMETER_PACKAGE_VARIANT)) {
    "native-input-r9"
}
else {
    $env:DILMETER_PACKAGE_VARIANT.Trim()
}
if ($packageVariant -notmatch '^[a-z0-9-]+$') {
    throw "Invalid package variant: $packageVariant"
}
$stageRoot = [System.IO.Path]::GetFullPath((Join-Path $releaseDir "stage-$packageVariant"))

foreach ($path in @($releaseDir, $stageRoot)) {
    if (-not $path.StartsWith($workspaceRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Native-input test package path escaped the workspace: $path"
    }
}

# Every CN feature must pass all 14 semantic UI palettes before packaging.
& (Join-Path $PSScriptRoot "assert_ui_color_theme.ps1")

$cnGuideName = (-join (@(0x4F7F, 0x7528, 0x8BF4, 0x660E) | ForEach-Object { [char]$_ })) + ".md"
$rtGuideName = "DilmeterRT-" + $cnGuideName
$testReadmeName = (-join (@(0x539F, 0x751F, 0x8F93, 0x5165, 0x6D4B, 0x8BD5, 0x5FC5, 0x8BFB) | ForEach-Object { [char]$_ })) + ".txt"
$requiredFiles = @(
    "DilmeterCN-v1.4.0.exe",
    "DilmeterRT-v1.4.0.exe",
    $cnGuideName,
    $rtGuideName,
    "native-input-test-readme-v1.4.0.txt",
    "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert.dll",
    "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert64.sys",
    "third_party\windivert\WinDivert-2.2.2-A\LICENSE"
)
foreach ($relativePath in $requiredFiles) {
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot $relativePath) -PathType Leaf)) {
        throw "Missing native-input test package file: $relativePath"
    }
}

function Assert-BinaryMarker {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        [string]$Marker
    )

    $binaryBytes = [System.IO.File]::ReadAllBytes($Path)
    try {
        $binaryText = [System.Text.Encoding]::ASCII.GetString($binaryBytes)
        if ($binaryText.IndexOf($Marker, [System.StringComparison]::Ordinal) -lt 0) {
            throw "Native-input test package binary is missing marker '$Marker': $Path"
        }
    }
    finally {
        $binaryText = $null
        $binaryBytes = $null
    }
}

$nativeInputMarker = $packageVariant
Assert-BinaryMarker -Path (Join-Path $projectRoot "DilmeterCN-v1.4.0.exe") -Marker $nativeInputMarker
Assert-BinaryMarker -Path (Join-Path $projectRoot "DilmeterRT-v1.4.0.exe") -Marker $nativeInputMarker
& (Join-Path $PSScriptRoot "assert_embedded_frontend.ps1") -BinaryPaths @(
    (Join-Path $projectRoot "DilmeterCN-v1.4.0.exe"),
    (Join-Path $projectRoot "DilmeterRT-v1.4.0.exe")
)

if (Test-Path -LiteralPath $stageRoot) {
    Remove-Item -LiteralPath $stageRoot -Recurse -Force
}
$cnStage = Join-Path $stageRoot "CN"
$rtStage = Join-Path $stageRoot "RT"
New-Item -ItemType Directory -Path $cnStage, $rtStage -Force | Out-Null

Copy-Item -LiteralPath (Join-Path $projectRoot "DilmeterCN-v1.4.0.exe") -Destination (Join-Path $cnStage "DilmeterCN-v1.4.0-$packageVariant.exe")
Copy-Item -LiteralPath (Join-Path $projectRoot $cnGuideName) -Destination $cnStage
Copy-Item -LiteralPath (Join-Path $projectRoot "native-input-test-readme-v1.4.0.txt") -Destination (Join-Path $cnStage $testReadmeName)

Copy-Item -LiteralPath (Join-Path $projectRoot "DilmeterRT-v1.4.0.exe") -Destination (Join-Path $rtStage "DilmeterRT-v1.4.0-$packageVariant.exe")
Copy-Item -LiteralPath (Join-Path $projectRoot $rtGuideName) -Destination (Join-Path $rtStage $cnGuideName)
Copy-Item -LiteralPath (Join-Path $projectRoot "native-input-test-readme-v1.4.0.txt") -Destination (Join-Path $rtStage $testReadmeName)
Copy-Item -LiteralPath (Join-Path $projectRoot "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert.dll") -Destination $rtStage
Copy-Item -LiteralPath (Join-Path $projectRoot "third_party\windivert\WinDivert-2.2.2-A\x64\WinDivert64.sys") -Destination $rtStage
Copy-Item -LiteralPath (Join-Path $projectRoot "third_party\windivert\WinDivert-2.2.2-A\LICENSE") -Destination (Join-Path $rtStage "WinDivert-LICENSE.txt")

$cnZip = Join-Path $releaseDir "DilmeterCN-v1.4.0-$packageVariant.zip"
$rtZip = Join-Path $releaseDir "DilmeterRT-v1.4.0-$packageVariant.zip"
& (Join-Path $PSScriptRoot "create_release_zip.ps1") -SourceDirectory $cnStage -ZipPath $cnZip
& (Join-Path $PSScriptRoot "create_release_zip.ps1") -SourceDirectory $rtStage -ZipPath $rtZip

Get-FileHash -LiteralPath $cnZip, $rtZip -Algorithm SHA256
