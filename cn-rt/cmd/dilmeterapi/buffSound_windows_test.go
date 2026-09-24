//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEmbeddedSkillReadySound(t *testing.T) {
	data, err := staticFiles.ReadFile(filepath.ToSlash(filepath.Join(embeddedStaticDir, "audio", "skill-ready-pop.mp3")))
	if err != nil {
		t.Fatalf("read embedded skill-ready sound: %v", err)
	}
	const expectedSHA256 = "5280619d7b8c0b5aad59030e1a744cb7af566cd185d24a929908848781fe8121"
	if actual := fmt.Sprintf("%x", sha256.Sum256(data)); actual != expectedSHA256 {
		t.Fatalf("unexpected embedded skill-ready sound: got %s", actual)
	}
}

func TestEmbeddedHealerDeathXiaoxiaoSound(t *testing.T) {
	data, err := staticFiles.ReadFile(filepath.ToSlash(filepath.Join(embeddedStaticDir, "audio", "healer-death-xiaoxiao.mp3")))
	if err != nil {
		t.Fatal(err)
	}
	const expected = "b98bb5e2d1f35616471fc9c96d6cb059cd25a83c77a7e3a63e387a4cb569ea58"
	if fmt.Sprintf("%x", sha256.Sum256(data)) != expected {
		t.Fatal("death voice is absent or differs from the verified Xiaoxiao recording")
	}
}

func TestHandleBuffSoundAcceptsStoredCustomAudio(t *testing.T) {
	dir := t.TempDir()
	previousDir := customAudioDir
	customAudioDir = func() string { return dir }
	t.Cleanup(func() { customAudioDir = previousDir })

	soundID := string(bytes.Repeat([]byte{'a'}, sha256.Size*2))
	if err := os.WriteFile(filepath.Join(dir, soundID+".mp3"), []byte("ID3"), 0600); err != nil {
		t.Fatalf("write custom sound: %v", err)
	}
	body, err := json.Marshal(buffSoundRequest{Kind: "custom", Volume: 0, SoundID: soundID})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/buff_sound", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handleBuffSound(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleBuffSoundRejectsCustomPath(t *testing.T) {
	body, err := json.Marshal(buffSoundRequest{Kind: "custom", Volume: 50, SoundID: `C:\alert.mp3`})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/buff_sound", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handleBuffSound(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestMCICommandErrorCanBeMatchedByCode(t *testing.T) {
	err := fmt.Errorf("play alert audio: %w", &mciCommandError{Code: 263, Command: "play"})
	if !isMCIErrorCode(err, 263) {
		t.Fatal("wrapped MCI error 263 should be recoverable")
	}
	if isMCIErrorCode(err, 275) {
		t.Fatal("different MCI error code must not match")
	}
	if got := err.Error(); got != "play alert audio: MCI error 263 (play)" {
		t.Fatalf("unexpected diagnostic: %q", got)
	}
}

func TestMCICommandVerbDoesNotExposeAudioPath(t *testing.T) {
	if got := mciCommandVerb(`open "C:\\Users\\player\\private alert.mp3" type mpegvideo alias alert`); got != "open" {
		t.Fatalf("unexpected command verb: %q", got)
	}
	if got := mciCommandVerb("  "); got != "command" {
		t.Fatalf("unexpected empty command verb: %q", got)
	}
}

func TestPlayMCIFileReopensEntireTransactionAfterError263(t *testing.T) {
	previousSender := mciCommandSender
	previousResultSender := mciCommandResultSender
	t.Cleanup(func() {
		mciCommandSender = previousSender
		mciCommandResultSender = previousResultSender
	})

	commands := make([]string, 0, 12)
	playAttempts := 0
	mciCommandSender = func(command string) error {
		commands = append(commands, command)
		if command == "play dilmeter_buff_alert from 0 wait" {
			playAttempts++
			if playAttempts == 1 {
				return &mciCommandError{Code: 263, Command: "play"}
			}
		}
		return nil
	}
	mciCommandResultSender = func(string) (string, error) { return "", nil }

	const openCommand = `open "C:\\audio\\alert.wav" alias dilmeter_buff_alert`
	if err := playMCIFile(openCommand, false, "custom", 75); err != nil {
		t.Fatalf("retrying MCI transaction failed: %v", err)
	}
	expected := []string{
		"close dilmeter_buff_alert",
		openCommand,
		"setaudio dilmeter_buff_alert volume to 750",
		"play dilmeter_buff_alert from 0 wait",
		"close dilmeter_buff_alert",
		"close dilmeter_buff_alert",
		openCommand,
		"setaudio dilmeter_buff_alert volume to 750",
		"play dilmeter_buff_alert from 0 wait",
		"close dilmeter_buff_alert",
	}
	if !reflect.DeepEqual(commands, expected) {
		t.Fatalf("unexpected MCI recovery sequence:\n got: %#v\nwant: %#v", commands, expected)
	}
}
