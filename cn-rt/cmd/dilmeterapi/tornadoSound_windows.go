//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const tornadoSoundAlias = "dilmeter_tornado_alert"
const tornadoMaximumSoundDelay = 500 * time.Millisecond

type tornadoSoundCommand struct {
	Request nativeReminderSoundRequest
	Preload bool
}

var tornadoSoundQueue = make(chan tornadoSoundCommand, 8)
var tornadoSoundWorkerOnce sync.Once

func enqueueTornadoSound(request nativeReminderSoundRequest, preload bool) bool {
	tornadoSoundWorkerOnce.Do(func() { go runTornadoSoundWorker(tornadoSoundQueue) })
	select {
	case tornadoSoundQueue <- tornadoSoundCommand{Request: request, Preload: preload}:
		return true
	default:
		logger.Println("tornado sound queue full")
		return false
	}
}

// Warm the selected file/device when enabling or changing the rule. No sound
// is played, and disk/device preparation never blocks the combat event loop.
func prepareNativeTornadoSound(settings nativeReminderSettings) {
	rule := settings.BossMechanics.Rules[perlonTornadoKey]
	if !rule.Enabled {
		return
	}
	request := nativeReminderSoundRequest{Volume: settings.BossMechanics.Volume, Tornado: true}
	switch {
	case rule.SoundMode == "dedicated":
		request.Kind = "skill-ready"
	case rule.SoundMode == "custom" && rule.CustomSoundID != "":
		request.Kind, request.SoundID = "custom", rule.CustomSoundID
	default:
		return
	}
	enqueueTornadoSound(request, true)
}

func runTornadoSoundWorker(queue <-chan tornadoSoundCommand) {
	// MCI aliases belong to the opening thread. Keep this thread and its
	// device alive between asynchronous plays instead of sleeping for the
	// whole clip then closing it. A new warning can restart even a long clip.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	player := tornadoSoundPlayer{}
	defer player.close()
	for command := range queue {
		request := command.Request
		if !command.Preload && (traySoundMuted.Load() || request.Volume <= 0 ||
			time.Now().UnixMilli()-request.EnqueuedAtMs > tornadoMaximumSoundDelay.Milliseconds()) {
			continue
		}
		if err := player.apply(request, command.Preload); err != nil {
			logger.Println("tornado sound failed:", err)
		}
	}
}

type tornadoSoundPlayer struct {
	path, kind, soundID string
}

func (p *tornadoSoundPlayer) close() {
	_ = mciCommandSender("close " + tornadoSoundAlias)
	p.path, p.kind, p.soundID = "", "", ""
}

func (p *tornadoSoundPlayer) apply(request nativeReminderSoundRequest, preload bool) error {
	path := p.path
	var err error
	if path == "" || request.Kind != p.kind || request.SoundID != p.soundID {
		switch request.Kind {
		case "skill-ready":
			// MP3 volume is set on the device; reuse one file at all volumes.
			path, err = extractBuffSound("skill-ready-pop.mp3", 100)
		case "custom":
			path, err = resolveCustomAudio(request.SoundID)
		default:
			return fmt.Errorf("unsupported tornado sound kind")
		}
	}
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 2; attempt++ {
		if p.path != path {
			p.close()
			mediaType := ""
			if strings.EqualFold(filepath.Ext(path), ".mp3") {
				mediaType = " type mpegvideo"
			}
			err = mciCommandSender(fmt.Sprintf("open \"%s\"%s alias %s", strings.ReplaceAll(path, "\"", ""), mediaType, tornadoSoundAlias))
			if err == nil {
				p.path, p.kind, p.soundID = path, request.Kind, request.SoundID
			}
		}
		if err == nil && !preload {
			_ = mciCommandSender("stop " + tornadoSoundAlias)
			_ = mciCommandSender(fmt.Sprintf("setaudio %s volume to %d", tornadoSoundAlias, max(0, min(100, request.Volume))*10))
			err = mciCommandSender("play " + tornadoSoundAlias + " from 0")
		}
		if attempt == 0 && isMCIErrorCode(err, 263) {
			p.close()
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return err
	}
	return err
}
