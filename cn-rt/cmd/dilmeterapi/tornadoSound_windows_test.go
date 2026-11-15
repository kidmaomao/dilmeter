//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func tornadoTestAudio(t *testing.T) string {
	t.Helper()
	dir, oldDir := t.TempDir(), customAudioDir
	customAudioDir = func() string { return dir }
	t.Cleanup(func() { customAudioDir = oldDir })
	id := strings.Repeat("a", 64)
	if err := os.WriteFile(filepath.Join(dir, id+".mp3"), []byte("ID3"), 0600); err != nil {
		t.Fatal(err)
	}
	return id
}

func captureTornadoCommands(t *testing.T) *[]string {
	t.Helper()
	oldSender, oldResult := mciCommandSender, mciCommandResultSender
	var commands []string
	mciCommandSender = func(command string) error { commands = append(commands, command); return nil }
	mciCommandResultSender = func(command string) (string, error) {
		commands = append(commands, "query "+command)
		return "60000", nil
	}
	t.Cleanup(func() { mciCommandSender, mciCommandResultSender = oldSender, oldResult })
	return &commands
}

func TestTornadoSoundPreloadAndRestartWithoutClipWait(t *testing.T) {
	id := tornadoTestAudio(t)
	commands := captureTornadoCommands(t)
	player := tornadoSoundPlayer{}
	defer player.close()
	request := nativeReminderSoundRequest{Kind: "custom", SoundID: id, Volume: 73}
	if err := player.apply(request, true); err != nil {
		t.Fatal(err)
	}
	if len(*commands) != 2 || !strings.HasPrefix((*commands)[1], "open ") {
		t.Fatalf("preload should open without playing: %v", *commands)
	}
	// Once prepared, even a large custom WAV would never be read/normalized
	// again on the urgent path. A changed sound ID still resolves a new file.
	customAudioDir = func() string { t.Error("preloaded file was resolved again on the urgent path"); return "" }
	for i := 0; i < 2; i++ {
		if err := player.apply(request, false); err != nil {
			t.Fatal(err)
		}
	}
	if len(*commands) != 8 || player.path == "" {
		t.Fatalf("clip was reopened/closed instead of retained: %v", *commands)
	}
	for i := 2; i < 8; i += 3 {
		if (*commands)[i] != "stop "+tornadoSoundAlias ||
			(*commands)[i+1] != "setaudio "+tornadoSoundAlias+" volume to 730" ||
			(*commands)[i+2] != "play "+tornadoSoundAlias+" from 0" {
			t.Fatalf("playback should restart asynchronously at the requested volume: %v", *commands)
		}
	}
}

func TestTornadoSoundWorkerIndependentFromBusyChannels(t *testing.T) {
	id := tornadoTestAudio(t)
	commands := captureTornadoCommands(t)
	oldMuted := traySoundMuted.Load()
	traySoundMuted.Store(false)
	t.Cleanup(func() { traySoundMuted.Store(oldMuted) })
	queue := make(chan tornadoSoundCommand, 5)
	now := time.Now().UnixMilli()
	request := nativeReminderSoundRequest{Kind: "custom", SoundID: id, Volume: 80, EnqueuedAtMs: now}
	queue <- tornadoSoundCommand{Request: request, Preload: true}
	stale := request
	stale.EnqueuedAtMs = now - tornadoMaximumSoundDelay.Milliseconds() - 100
	queue <- tornadoSoundCommand{Request: stale}
	silent := request
	silent.Volume = 0
	queue <- tornadoSoundCommand{Request: silent}
	queue <- tornadoSoundCommand{Request: request}
	queue <- tornadoSoundCommand{Request: request}
	close(queue)
	buffSoundPlaybackMu.Lock()
	bossSoundPlaybackMu.Lock()
	done := make(chan struct{})
	go func() { runTornadoSoundWorker(queue); close(done) }()
	blocked := false
	select {
	case <-done:
	case <-time.After(time.Second):
		blocked = true
	}
	bossSoundPlaybackMu.Unlock()
	buffSoundPlaybackMu.Unlock()
	if blocked {
		<-done
		t.Fatal("tornado waited for another playback channel")
	}
	plays := 0
	for _, command := range *commands {
		if strings.HasPrefix(command, "play ") {
			plays++
		}
		if strings.Contains(command, " wait") || strings.HasPrefix(command, "query ") {
			t.Fatalf("tornado waited for clip duration: %s", command)
		}
	}
	if plays != 2 {
		t.Fatalf("wanted two fresh alerts and no stale/silent sounds: %v", *commands)
	}
	traySoundMuted.Store(true)
	muted := make(chan tornadoSoundCommand, 1)
	muted <- tornadoSoundCommand{Request: request}
	close(muted)
	before := len(*commands)
	runTornadoSoundWorker(muted)
	for _, command := range (*commands)[before:] {
		if strings.HasPrefix(command, "play ") {
			t.Fatal("tornado ignored tray mute")
		}
	}
}

func TestTornadoSoundReopensInvalidDevice(t *testing.T) {
	id := tornadoTestAudio(t)
	commands := captureTornadoCommands(t)
	sender := mciCommandSender
	plays := 0
	mciCommandSender = func(command string) error {
		_ = sender(command)
		if strings.HasPrefix(command, "play ") {
			plays++
			if plays == 1 {
				return &mciCommandError{Code: 263, Command: "play"}
			}
		}
		return nil
	}
	player := tornadoSoundPlayer{}
	defer player.close()
	if err := player.apply(nativeReminderSoundRequest{Kind: "custom", SoundID: id, Volume: 50}, false); err != nil {
		t.Fatal(err)
	}
	opens := 0
	for _, command := range *commands {
		if strings.HasPrefix(command, "open ") {
			opens++
		}
	}
	if plays != 2 || opens != 2 || player.path == "" {
		t.Fatalf("device recovery failed: %v", *commands)
	}
}

func TestTornadoSoundRoutesOutsideFullBossQueue(t *testing.T) {
	// Keep this routing test deterministic: drain its private queue manually.
	oldTornado, oldBoss := tornadoSoundQueue, nativeBossSoundQueue
	tornadoSoundQueue = make(chan tornadoSoundCommand, 8)
	nativeBossSoundQueue = make(chan nativeReminderSoundRequest, 1)
	nativeBossSoundQueue <- nativeReminderSoundRequest{}
	tornadoSoundWorkerOnce.Do(func() {})
	t.Cleanup(func() {
		tornadoSoundQueue, nativeBossSoundQueue = oldTornado, oldBoss
		tornadoSoundWorkerOnce = sync.Once{}
	})
	settings := nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{Volume: 60, Rules: map[string]nativeBossMechanicRule{
		perlonTornadoKey: {Enabled: true, SoundMode: "dedicated"},
	}}}
	prepareNativeTornadoSound(settings)
	preload := <-tornadoSoundQueue
	if !preload.Preload || preload.Request.Kind != "skill-ready" {
		t.Fatal("dedicated sound was not prepared")
	}
	request := nativeReminderSoundRequest{Kind: "skill-ready", Volume: 60, Priority: true, Tornado: true}
	if !queueNativeReminderSound(request) {
		t.Fatal("full boss queue blocked tornado")
	}
	queued := <-tornadoSoundQueue
	if queued.Preload || queued.Request.EnqueuedAtMs == 0 || len(nativeBossSoundQueue) != 1 {
		t.Fatal("urgent sound used the wrong channel")
	}
	settings.BossMechanics.Rules[perlonTornadoKey] = nativeBossMechanicRule{Enabled: true, SoundMode: "custom", CustomSoundID: "selected-file"}
	prepareNativeTornadoSound(settings)
	custom := <-tornadoSoundQueue
	if !custom.Preload || custom.Request.SoundID != "selected-file" {
		t.Fatal("changed custom sound was not prepared")
	}
	for _, rule := range []nativeBossMechanicRule{{}, {Enabled: true, SoundMode: "off"}, {Enabled: true, SoundMode: "custom"}} {
		settings.BossMechanics.Rules[perlonTornadoKey] = rule
		prepareNativeTornadoSound(settings)
	}
	request.Volume = 0
	if queueNativeReminderSound(request) || len(tornadoSoundQueue) != 0 {
		t.Fatal("disabled/silent rules queued audio")
	}
}

// Opt-in device check: the custom WAV contains silence, and the MP3 device is
// explicitly muted before playback. This does not change system/tray volume.
func TestTornadoSoundDeviceSmoke(t *testing.T) {
	if os.Getenv("DILMETER_TORNADO_AUDIO_QA") != "1" {
		t.Skip("set DILMETER_TORNADO_AUDIO_QA=1 for the local Windows device check")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// MCI waveaudio rejects the long path produced by t.TempDir plus a
	// content-addressed filename; use a short temporary directory for QA.
	dir, err := os.MkdirTemp("", "tornado-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)
	oldDir := customAudioDir
	customAudioDir = func() string { return dir }
	defer func() { customAudioDir = oldDir }()
	id := strings.Repeat("b", 64)
	silence, err := normalizeWaveToPCM16(makeFloatWave(make([]float32, 44100*3)))
	if err != nil {
		t.Fatal(err)
	}
	wavePath := filepath.Join(dir, id+".wav")
	defer os.Remove(wavePath)
	if err := os.WriteFile(wavePath, silence, 0600); err != nil {
		t.Fatal(err)
	}
	const background = "dilmeter_tornado_qa_background"
	if err := sendMCICommand(`open "` + wavePath + `" alias ` + background); err != nil {
		t.Fatal(err)
	}
	defer sendMCICommand("close " + background)
	if err := sendMCICommand("play " + background + " from 0"); err != nil {
		t.Fatal(err)
	}
	for _, request := range []nativeReminderSoundRequest{{Kind: "custom", SoundID: id}, {Kind: "skill-ready"}} {
		player := tornadoSoundPlayer{}
		defer player.close()
		if err := player.apply(request, true); err != nil {
			t.Fatal(err)
		}
		if request.Kind == "skill-ready" {
			// Abort before playing any non-silent file if device muting fails.
			if err := sendMCICommand("setaudio " + tornadoSoundAlias + " volume to 0"); err != nil {
				t.Fatal(err)
			}
		}
		for attempt := 0; attempt < 2; attempt++ {
			started := time.Now()
			if err := player.apply(request, false); err != nil {
				t.Fatal(err)
			}
			elapsed := time.Since(started)
			_ = sendMCICommand("set " + tornadoSoundAlias + " time format milliseconds")
			time.Sleep(80 * time.Millisecond)
			positionText, err := sendMCICommandResult("status " + tornadoSoundAlias + " position")
			if err != nil {
				t.Fatal(err)
			}
			position, _ := strconv.Atoi(strings.TrimSpace(positionText))
			if position <= 0 {
				t.Fatalf("asynchronous %s playback never advanced: %q", request.Kind, positionText)
			}
			t.Logf("%s play %d: command returned in %s; playback position=%d ms", request.Kind, attempt+1, elapsed, position)
		}
		player.close()
	}
}
