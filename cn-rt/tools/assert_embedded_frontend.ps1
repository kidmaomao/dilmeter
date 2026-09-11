param(
    [Parameter(Mandatory = $true)]
    [string[]]$BinaryPaths,

    [string[]]$AdditionalRequiredMarkers = @()
)

$ErrorActionPreference = "Stop"

$skillBarTitle = -join (@(0x989D, 0x5916, 0x6280, 0x80FD, 0x680F, 0x28, 0x6D4B, 0x8BD5, 0x29) | ForEach-Object { [char]$_ })
$aimSummaryLabel = -join (@(0x7A7F, 0x5FC3, 0x5E73, 0x5747, 0x7784, 0x51C6, 0x7387) | ForEach-Object { [char]$_ })
$oldSkillBarTitle =
    (-join (@(0x72EC, 0x7ACB, 0x6280, 0x80FD, 0x680F) | ForEach-Object { [char]$_ })) +
    " $([char]0x00B7) v1.4.0 " +
    (-join (@(0x539F, 0x751F, 0x8F93, 0x5165) | ForEach-Object { [char]$_ })) +
    " r9"

$requiredMarkers = @(
    $skillBarTitle,
    $aimSummaryLabel,
    "aim-reminder-save skill-cooldown-save"
) + $AdditionalRequiredMarkers
$forbiddenMarkers = @($oldSkillBarTitle)

foreach ($path in $BinaryPaths) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Embedded frontend verification file is missing: $path"
    }
    $binaryBytes = [System.IO.File]::ReadAllBytes($path)
    try {
        $binaryText = [System.Text.Encoding]::UTF8.GetString($binaryBytes)
        foreach ($marker in $requiredMarkers) {
            if ($binaryText.IndexOf($marker, [System.StringComparison]::Ordinal) -lt 0) {
                throw "Embedded frontend is stale; missing marker '$marker': $path"
            }
        }
        foreach ($marker in $forbiddenMarkers) {
            if ($binaryText.IndexOf($marker, [System.StringComparison]::Ordinal) -ge 0) {
                throw "Embedded frontend is stale; forbidden marker '$marker' remains: $path"
            }
        }
    }
    finally {
        $binaryText = $null
        $binaryBytes = $null
    }
}

Write-Output "Embedded frontend markers verified in $($BinaryPaths.Count) binary file(s)."
