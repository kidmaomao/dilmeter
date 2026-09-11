//go:build windows

package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestWindowsLocalTTSIntegration(t *testing.T) {
	if os.Getenv("DILMETER_TEST_LOCAL_TTS") != "1" {
		t.Skip("set DILMETER_TEST_LOCAL_TTS=1 to test installed Windows voices")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	voices, err := listPlatformTTSVoices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) == 0 {
		t.Fatal("no installed Windows voices")
	}
	wave, selected, err := synthesizePlatformTTS(ctx, "本地语音测试成功", voices[0].Name, 0)
	if err != nil {
		t.Fatal(err)
	}
	if selected == "" || len(wave) < 44 || string(wave[:4]) != "RIFF" || string(wave[8:12]) != "WAVE" {
		t.Fatal("local TTS did not produce a valid WAV")
	}
	t.Logf("voice=%s bytes=%d", selected, len(wave))
}
