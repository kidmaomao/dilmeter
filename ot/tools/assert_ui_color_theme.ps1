$ErrorActionPreference = "Stop"

$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$frontRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot "front"))
if (-not $frontRoot.StartsWith($projectRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "UI colour verification path escaped the project."
}

$npmCommand = Get-Command npm.cmd -ErrorAction Stop
Push-Location $frontRoot
try {
    & $npmCommand.Source run verify:ui-color-theme
    if ($LASTEXITCODE -ne 0) {
        throw "CN packaging stopped because UI colour adaptation verification failed."
    }
}
finally {
    Pop-Location
}
