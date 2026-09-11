//go:build !windows

package main

import (
	"context"
	"errors"
)

func listPlatformTTSVoices(_ context.Context) ([]localTTSVoice, error) {
	return nil, errors.New("本地 TTS 仅支持 Windows")
}

func synthesizePlatformTTS(_ context.Context, _, _ string, _ int) ([]byte, string, error) {
	return nil, "", errors.New("本地 TTS 仅支持 Windows")
}
