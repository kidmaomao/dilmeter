param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$')]
    [string]$Version
)

$ErrorActionPreference = "Stop"
$numericVersion = (($Version -split '[-+]', 2)[0]) + ".0"
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$targets = @(
    @{ Path = "cn-rt\cmd\dilmeterapi\winres\winres.json"; App = "DilmeterCN" },
    @{ Path = "cn-rt\cmd\dilmeterapi\winres-rt\winres.json"; App = "DilmeterRT" },
    @{ Path = "ot\cmd\dilmeterapi\winres\winres.json"; App = "DilmeterOT" }
)

foreach ($target in $targets) {
    $path = Join-Path $repoRoot $target.Path
    $document = Get-Content -LiteralPath $path -Raw -Encoding UTF8 | ConvertFrom-Json
    $versionInfo = $document.RT_VERSION.'#1'.'0000'
    $versionInfo.fixed.file_version = $numericVersion
    $versionInfo.fixed.product_version = $numericVersion
    $strings = $versionInfo.info.'0804'
    $strings.FileVersion = $numericVersion
    $strings.ProductVersion = $numericVersion
    $strings.OriginalFilename = "$($target.App)-v$Version.exe"
    $json = $document | ConvertTo-Json -Depth 20
    [System.IO.File]::WriteAllText($path, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
}

Write-Host "Updated Windows version metadata to $Version."
