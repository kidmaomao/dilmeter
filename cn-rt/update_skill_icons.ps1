param(
    [string]$Destination = (Join-Path $PSScriptRoot "front\public\skill-icons")
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http
$indexUrl = "https://gear.noginogi.sbs/data/prilus-lexicon/search-lite.json?v=20260708-source"
$specialKrIcons = [System.Collections.Generic.HashSet[int]]::new()
[void]$specialKrIcons.Add(58009)
[void]$specialKrIcons.Add(58100)
[void]$specialKrIcons.Add(58101)
[void]$specialKrIcons.Add(58104)

Write-Host "Reading NogiNogi skill index..."
$index = Invoke-RestMethod -Uri $indexUrl -TimeoutSec 90
$skills = @($index.records | Where-Object {
    $_.type -eq "skill" -and $_.cn.image.candidates.Count -gt 0
})

New-Item -ItemType Directory -Force -Path $Destination | Out-Null

$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(45)
$downloaded = 0
$batchSize = 24

try {
    for ($offset = 0; $offset -lt $skills.Count; $offset += $batchSize) {
        $last = [Math]::Min($offset + $batchSize - 1, $skills.Count - 1)
        $requests = foreach ($skill in $skills[$offset..$last]) {
            $skillId = [int]$skill.id
            $url = if ($specialKrIcons.Contains($skillId)) {
                "https://mabires.pril.cc/skillimage/kr/$skillId/$skillId.png"
            } else {
                [string]$skill.cn.image.candidates[0]
            }

            [PSCustomObject]@{
                SkillId = $skillId
                Path = Join-Path $Destination "$skillId.png"
                Task = $client.GetByteArrayAsync($url)
            }
        }

        [System.Threading.Tasks.Task]::WaitAll([System.Threading.Tasks.Task[]]$requests.Task)
        foreach ($request in $requests) {
            [System.IO.File]::WriteAllBytes($request.Path, $request.Task.Result)
            $downloaded++
        }
        Write-Progress -Activity "Downloading skill icons" -Status "$downloaded / $($skills.Count)" -PercentComplete (($downloaded / $skills.Count) * 100)
    }
} finally {
    $client.Dispose()
    Write-Progress -Activity "Downloading skill icons" -Completed
}

Write-Host "Saved $downloaded skill icons to $Destination"
Write-Host "KR overrides: 58009, 58100, 58101, 58104"
