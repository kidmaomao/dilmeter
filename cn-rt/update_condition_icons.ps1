param(
    [string]$ConditionData = (Join-Path $PSScriptRoot "front\src\data\condition.ts"),
    [string]$Destination = (Join-Path $PSScriptRoot "front\public\condition-icons"),
    [switch]$Force
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http

if (-not (Test-Path -LiteralPath $ConditionData -PathType Leaf)) {
    throw "Condition data was not found: $ConditionData"
}

$ids = @(
    # The checked-in name table includes the known sparse/high IDs.
    Select-String -LiteralPath $ConditionData -Pattern '^\s*"(\d+)"\s*:' | ForEach-Object {
        [int]$_.Matches[0].Groups[1].Value
    }
    # CN can introduce a condition before its name table is refreshed. Keep the
    # contiguous live CC range bundled as well (for example 1159/1161/1225).
    0..1300
) | Sort-Object -Unique

New-Item -ItemType Directory -Force -Path $Destination | Out-Null

$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(15)
$batchSize = 64
$downloaded = 0
$kept = 0
$missing = [System.Collections.Generic.List[int]]::new()

try {
    for ($offset = 0; $offset -lt $ids.Count; $offset += $batchSize) {
        $last = [Math]::Min($offset + $batchSize - 1, $ids.Count - 1)
        $requests = foreach ($ccId in $ids[$offset..$last]) {
            $path = Join-Path $Destination "$ccId.png"
            if (-not $Force -and (Test-Path -LiteralPath $path -PathType Leaf)) {
                $kept++
                continue
            }

            [PSCustomObject]@{
                CCId = $ccId
                Path = $path
                Task = $client.GetAsync("https://mabires.pril.cc/characterconditionimage/cn/$ccId/$ccId.png")
            }
        }

        if ($requests.Count -gt 0) {
            try {
                [System.Threading.Tasks.Task]::WaitAll([System.Threading.Tasks.Task[]]$requests.Task)
            } catch [System.AggregateException] {
                # Individual transient failures are handled below so one unavailable
                # icon cannot abort the complete offline icon set.
            }
            foreach ($request in $requests) {
                if ($request.Task.IsFaulted -or $request.Task.IsCanceled) {
                    $missing.Add($request.CCId)
                    continue
                }
                $response = $request.Task.Result
                try {
                    if (-not $response.IsSuccessStatusCode) {
                        $missing.Add($request.CCId)
                        continue
                    }
                    $bytesTask = $response.Content.ReadAsByteArrayAsync()
                    $bytesTask.Wait()
                    [System.IO.File]::WriteAllBytes($request.Path, $bytesTask.Result)
                    $downloaded++
                } finally {
                    $response.Dispose()
                }
            }
        }

        $completed = [Math]::Min($last + 1, $ids.Count)
        Write-Progress -Activity "Downloading CN Buff icons" -Status "$completed / $($ids.Count)" -PercentComplete (($completed / $ids.Count) * 100)
    }
} finally {
    $client.Dispose()
    Write-Progress -Activity "Downloading CN Buff icons" -Completed
}

$manifest = [ordered]@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    source = "https://mabires.pril.cc/characterconditionimage/cn/{id}/{id}.png"
    requested = $ids.Count
    available = (Get-ChildItem -LiteralPath $Destination -Filter "*.png" -File).Count
    unavailable = $missing.ToArray()
}
$manifest | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $Destination "manifest.json") -Encoding utf8

Write-Host "CN Buff icons: downloaded $downloaded, kept $kept, unavailable $($missing.Count)."
Write-Host "Saved to $Destination"
