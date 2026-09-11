//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const listLocalTTSVoicesScript = `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[System.Text.Encoding]::UTF8
Add-Type -AssemblyName System.Speech
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
  $voices = @($synth.GetInstalledVoices() | Where-Object { $_.Enabled } | ForEach-Object {
    [PSCustomObject]@{
      name = $_.VoiceInfo.Name
      culture = $_.VoiceInfo.Culture.Name
      gender = $_.VoiceInfo.Gender.ToString()
      description = $_.VoiceInfo.Description
    }
  })
  ConvertTo-Json -InputObject $voices -Compress
} finally { $synth.Dispose() }`

const synthesizeLocalTTSScript = `$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Speech
$text = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($env:DILMETER_TTS_TEXT))
$voice = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($env:DILMETER_TTS_VOICE))
$rate = [int]$env:DILMETER_TTS_RATE
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
  if ($voice) { $synth.SelectVoice($voice) }
  $synth.Rate = $rate
  $synth.Volume = 100
  $synth.SetOutputToWaveFile($env:DILMETER_TTS_OUTPUT)
  $synth.Speak($text)
  $synth.SetOutputToNull()
} finally { $synth.Dispose() }`

func listPlatformTTSVoices(ctx context.Context) ([]localTTSVoice, error) {
	command := hiddenPowerShellCommand(ctx, listLocalTTSVoicesScript)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("System.Speech unavailable: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || trimmed == "null" {
		return nil, errors.New("Windows 没有安装可用语音")
	}
	var voices []localTTSVoice
	if err := json.Unmarshal([]byte(trimmed), &voices); err != nil {
		var single localTTSVoice
		if singleErr := json.Unmarshal([]byte(trimmed), &single); singleErr != nil {
			return nil, fmt.Errorf("解析系统语音失败：%w", err)
		}
		voices = []localTTSVoice{single}
	}
	if len(voices) == 0 {
		return nil, errors.New("Windows 没有安装可用语音")
	}
	return voices, nil
}

func synthesizePlatformTTS(ctx context.Context, text, voice string, rate int) ([]byte, string, error) {
	voices, err := listPlatformTTSVoices(ctx)
	if err != nil {
		return nil, "", err
	}
	selectedVoice := voice
	if selectedVoice == "" {
		selectedVoice = voices[0].Name
	} else {
		found := false
		for _, item := range voices {
			if item.Name == selectedVoice {
				found = true
				break
			}
		}
		if !found {
			return nil, "", errors.New("所选系统语音已经不可用")
		}
	}
	dir := customAudioDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, "", err
	}
	temporary, err := os.CreateTemp(dir, ".local-tts-*.wav")
	if err != nil {
		return nil, "", err
	}
	outputPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return nil, "", err
	}
	_ = os.Remove(outputPath)
	defer os.Remove(outputPath)
	command := hiddenPowerShellCommand(ctx, synthesizeLocalTTSScript)
	command.Env = append(os.Environ(),
		"DILMETER_TTS_TEXT="+base64.StdEncoding.EncodeToString([]byte(text)),
		"DILMETER_TTS_VOICE="+base64.StdEncoding.EncodeToString([]byte(selectedVoice)),
		fmt.Sprintf("DILMETER_TTS_RATE=%d", rate),
		"DILMETER_TTS_OUTPUT="+outputPath,
	)
	if output, err := command.CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("Windows 本地语音生成失败：%w (%s)", err, strings.TrimSpace(string(output)))
	}
	data, err := readLimitedTTSWave(outputPath)
	return data, selectedVoice, err
}

func hiddenPowerShellCommand(ctx context.Context, script string) *exec.Cmd {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	executable := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	command := exec.CommandContext(ctx, executable, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return command
}
