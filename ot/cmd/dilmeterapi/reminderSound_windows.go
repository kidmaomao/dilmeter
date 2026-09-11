//go:build windows

package main

import (
	"sync"
	"time"
)

const nativeReminderMaximumSoundDelay = 4 * time.Second

var nativeReminderSoundQueue = make(chan nativeReminderSoundRequest, 64)
var nativeBossSoundQueue = make(chan nativeReminderSoundRequest, 8)
var nativeReminderSoundWorkerOnce sync.Once
var nativeBossSoundWorkerOnce sync.Once

func queueNativeReminderSound(request nativeReminderSoundRequest) bool {
	if request.Volume <= 0 {
		return false
	}
	request.EnqueuedAtMs = time.Now().UnixMilli()
	queue := nativeReminderSoundQueue
	if request.Priority {
		queue = nativeBossSoundQueue
		nativeBossSoundWorkerOnce.Do(func() { go runNativeReminderSoundWorker(queue, true) })
	} else {
		nativeReminderSoundWorkerOnce.Do(func() { go runNativeReminderSoundWorker(queue, false) })
	}
	select {
	case queue <- request:
		return true
	default:
		logger.Println("native reminder sound queue full; dropping newest request", request.Kind)
		return false
	}
}

func runNativeReminderSoundWorker(queue <-chan nativeReminderSoundRequest, bossChannel bool) {
	for request := range queue {
		// A reminder that spent several seconds behind a long dedicated timeline
		// is no longer useful and must not create a second, delayed warning wave.
		if time.Now().UnixMilli()-request.EnqueuedAtMs > nativeReminderMaximumSoundDelay.Milliseconds() {
			continue
		}
		var err error
		if bossChannel {
			err = playBossReminderSound(request.Kind, request.Volume, request.SoundID)
		} else {
			err = playBuffSound(request.Kind, request.Volume, request.SoundID)
		}
		if err != nil {
			logger.Println("native reminder sound playback failed:", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
