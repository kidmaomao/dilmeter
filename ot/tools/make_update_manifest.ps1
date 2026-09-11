param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$')]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [ValidateSet('DilmeterCN', 'DilmeterRT', 'DilmeterOT')]
    [string]$AppName,

    [Parameter(Mandatory = $true)]
    [string]$ZipPath,

    [string]$Notes = '',

    [string]$NotesPath = ''
)

if ($NotesPath) {
    $resolvedNotes = Resolve-Path -LiteralPath $NotesPath -ErrorAction Stop
    $Notes = [System.IO.File]::ReadAllText($resolvedNotes.Path, [System.Text.Encoding]::UTF8).Trim()
}

$resolvedZip = Resolve-Path -LiteralPath $ZipPath -ErrorAction Stop
$zip = Get-Item -LiteralPath $resolvedZip.Path
$manifestPath = Join-Path $zip.DirectoryName "$AppName.json"
$sha256 = [System.Security.Cryptography.SHA256]::Create()
$zipStream = [System.IO.File]::OpenRead($zip.FullName)
try {
    $hashBytes = $sha256.ComputeHash($zipStream)
}
finally {
    $zipStream.Dispose()
    $sha256.Dispose()
}
$hash = ([System.BitConverter]::ToString($hashBytes)).Replace("-", "").ToLowerInvariant()
$manifest = [ordered]@{
    version = $Version
    url = "https://github.com/kidmaomao/dilmeter/releases/latest/download/$AppName.zip"
    sha256 = $hash
    size = $zip.Length
    notes = $Notes
}

$json = $manifest | ConvertTo-Json
[System.IO.File]::WriteAllText($manifestPath, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
Write-Host "Created $manifestPath"
