param(
    [Parameter(Mandatory = $true)]
    [string]$SourceDirectory,

    [Parameter(Mandatory = $true)]
    [string]$ZipPath
)

$ErrorActionPreference = "Stop"
$source = [IO.Path]::GetFullPath($SourceDirectory)
$destination = [IO.Path]::GetFullPath($ZipPath)
if (-not (Test-Path -LiteralPath $source -PathType Container)) {
    throw "Source directory does not exist: $source"
}

Add-Type -AssemblyName System.IO.Compression
$parent = Split-Path -Parent $destination
New-Item -ItemType Directory -Force -Path $parent | Out-Null
if (Test-Path -LiteralPath $destination) {
    Remove-Item -LiteralPath $destination -Force
}

$stream = [IO.File]::Open($destination, [IO.FileMode]::CreateNew, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
$archive = New-Object IO.Compression.ZipArchive($stream, [IO.Compression.ZipArchiveMode]::Create, $false, [Text.Encoding]::UTF8)
try {
    foreach ($file in Get-ChildItem -LiteralPath $source -File -Recurse | Sort-Object FullName) {
        $relative = $file.FullName.Substring($source.Length).TrimStart([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
        $entryName = $relative.Replace([IO.Path]::DirectorySeparatorChar, "/")
        $entry = $archive.CreateEntry($entryName, [IO.Compression.CompressionLevel]::Optimal)
        $entry.LastWriteTime = $file.LastWriteTime
        $input = [IO.File]::OpenRead($file.FullName)
        $output = $entry.Open()
        try {
            $input.CopyTo($output)
        }
        finally {
            $output.Dispose()
            $input.Dispose()
        }
    }
}
finally {
    $archive.Dispose()
    $stream.Dispose()
}

# ZipArchive writes UTF-8 filenames but only sets the language-encoding flag
# when a name contains characters outside CP437. On Simplified-Chinese Windows,
# Explorer/Expand-Archive may still reinterpret such entries through the active
# ANSI code page. Force bit 11 in every central and local header so the archive
# is unambiguous to every extractor used by Dilmeter users.
$zipBytes = [IO.File]::ReadAllBytes($destination)
function Set-Utf8Flag([int]$flagsOffset) {
    $flags = [BitConverter]::ToUInt16($zipBytes, $flagsOffset) -bor 0x0800
    $encodedFlags = [BitConverter]::GetBytes([uint16]$flags)
    $zipBytes[$flagsOffset] = $encodedFlags[0]
    $zipBytes[$flagsOffset + 1] = $encodedFlags[1]
}

$eocdOffset = -1
$minimumEocdOffset = [Math]::Max(0, $zipBytes.Length - 65557)
for ($candidate = $zipBytes.Length - 22; $candidate -ge $minimumEocdOffset; $candidate--) {
    if ([BitConverter]::ToUInt32($zipBytes, $candidate) -eq 0x06054b50) {
        $eocdOffset = $candidate
        break
    }
}
if ($eocdOffset -lt 0) {
    throw "ZIP end-of-central-directory record was not found: $destination"
}

$entryCount = [BitConverter]::ToUInt16($zipBytes, $eocdOffset + 10)
$centralOffset = [int][BitConverter]::ToUInt32($zipBytes, $eocdOffset + 16)
for ($entryIndex = 0; $entryIndex -lt $entryCount; $entryIndex++) {
    if ($centralOffset -gt $zipBytes.Length - 46 -or [BitConverter]::ToUInt32($zipBytes, $centralOffset) -ne 0x02014b50) {
        throw "Invalid ZIP central-directory entry $entryIndex in $destination"
    }

    Set-Utf8Flag ($centralOffset + 8)
    $localOffset = [int][BitConverter]::ToUInt32($zipBytes, $centralOffset + 42)
    if ($localOffset -gt $zipBytes.Length - 30 -or [BitConverter]::ToUInt32($zipBytes, $localOffset) -ne 0x04034b50) {
        throw "Invalid ZIP local header for entry $entryIndex in $destination"
    }
    Set-Utf8Flag ($localOffset + 6)

    $nameLength = [BitConverter]::ToUInt16($zipBytes, $centralOffset + 28)
    $extraLength = [BitConverter]::ToUInt16($zipBytes, $centralOffset + 30)
    $commentLength = [BitConverter]::ToUInt16($zipBytes, $centralOffset + 32)
    $centralOffset += 46 + $nameLength + $extraLength + $commentLength
}
[IO.File]::WriteAllBytes($destination, $zipBytes)

Write-Host "Created UTF-8 ZIP: $destination"
