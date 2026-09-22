$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Speech
$healerVoice = New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
    # Require the requested voice; never silently substitute another speaker.
    $healerVoice.SelectVoice('Microsoft Huihui')
    $healerVoice.Rate = 0
    $healerVoice.Volume = 100
    $healerOutput = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../public/audio/healer-music-huihui.wav'))
    $healerFormat = New-Object System.Speech.AudioFormat.SpeechAudioFormatInfo(44100, [System.Speech.AudioFormat.AudioBitsPerSample]::Sixteen, [System.Speech.AudioFormat.AudioChannel]::Mono)
    $healerVoice.SetOutputToWaveFile($healerOutput, $healerFormat)
    $healerVoice.Speak('队友音乐时间到了')
    $healerVoice.SetOutputToNull()
    Write-Output $healerOutput
} finally { $healerVoice.Dispose() }
