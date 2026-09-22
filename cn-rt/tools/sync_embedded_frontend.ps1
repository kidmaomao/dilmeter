$ErrorActionPreference = "Stop"
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$sourceRoot = [IO.Path]::GetFullPath((Join-Path $projectRoot "front/dist"))
$targetRoot = [IO.Path]::GetFullPath((Join-Path $projectRoot "cmd/dilmeterapi/static_v130_release"))
$projectPrefix = $projectRoot.TrimEnd('\') + '\'
if (-not $targetRoot.StartsWith($projectPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Embedded frontend directory is outside the project."
}
if (-not (Test-Path -LiteralPath (Join-Path $sourceRoot "index.html") -PathType Leaf)) {
    throw "Build the frontend before synchronizing embedded resources."
}

New-Item -ItemType Directory -Force -Path $targetRoot | Out-Null
# Refuse linked trees before copying or pruning generated files.
foreach ($root in @($sourceRoot, $targetRoot)) {
    $entries = @((Get-Item -LiteralPath $root)) + @(Get-ChildItem -LiteralPath $root -Recurse -Force)
    if ($entries | Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint }) {
        throw "Cannot synchronize a frontend directory containing links: $root"
    }
}
$files = @(Get-ChildItem -LiteralPath $sourceRoot -File -Recurse -Force)
if (-not ($files | Where-Object { $_.Extension -eq '.js' }) -or -not ($files | Where-Object { $_.Extension -eq '.css' })) {
    throw "Frontend output is incomplete; embedded resources were not changed."
}
$expected = @{}
foreach ($file in $files) {
    $relative = $file.FullName.Substring($sourceRoot.Length).TrimStart('\', '/')
    $expected[$relative] = $true
    $destination = Join-Path $targetRoot $relative
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
    Copy-Item -LiteralPath $file.FullName -Destination $destination -Force
}
$targetPrefix = $targetRoot.TrimEnd('\') + '\'
$removedBytes = 0L
$removedCount = 0
foreach ($file in Get-ChildItem -LiteralPath $targetRoot -File -Recurse -Force) {
    $fullPath = [IO.Path]::GetFullPath($file.FullName)
    if (-not $fullPath.StartsWith($targetPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to prune a path outside the embedded frontend directory: $fullPath"
    }
    $relative = $fullPath.Substring($targetPrefix.Length)
    if (-not $expected.ContainsKey($relative)) {
        $removedBytes += $file.Length
        $removedCount++
        Remove-Item -LiteralPath $fullPath -Force
    }
}
Write-Output ("Embedded frontend synchronized: {0} current files; removed {1} stale files ({2:N2} MB)." -f $files.Count, $removedCount, ($removedBytes / 1000000))
