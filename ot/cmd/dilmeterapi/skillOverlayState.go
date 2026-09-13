package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

const emptySkillOverlayState = `{"type":"skill-state","at":0,"items":[]}`

var skillOverlayState = struct {
	sync.RWMutex
	data []byte
}{data: []byte(emptySkillOverlayState)}

func setSkillOverlayState(data []byte) bool {
	if len(data) == 0 || !json.Valid(data) {
		return false
	}
	skillOverlayState.Lock()
	skillOverlayState.data = append(skillOverlayState.data[:0], data...)
	skillOverlayState.Unlock()
	var message nativeSkillOverlayMessage
	if json.Unmarshal(data, &message) == nil {
		setNativeSkillOverlayActive(nativeSkillOverlayMessageVisible(message, time.Now().UnixMilli()))
		refreshWebViewReminderHitRegions()
	}
	return true
}

func nativeSkillOverlayMessageVisible(message nativeSkillOverlayMessage, nowMs int64) bool {
	if target := message.TargetHealth; target != nil && target.MaximumHealth > 0 && target.CurrentHealth >= 0 &&
		(target.PreviewEndsAt == 0 || target.PreviewEndsAt > nowMs) {
		return true
	}
	if !message.Settings.OverlayEnabled && message.Settings.Opacity != 0 {
		return false
	}
	if message.AimReminder != nil && (message.AimReminder.Active || message.AimReminder.AlwaysVisible) {
		return true
	}
	for _, item := range message.EffectTimers {
		if item.Enabled && (item.AlwaysVisible || item.EndsAtMs > nowMs) {
			return true
		}
	}
	for _, item := range message.Mechanics {
		if item.EndsAtMs > nowMs {
			return true
		}
	}
	for _, item := range message.StackAlerts {
		if item.Persistent || item.EndsAtMs > nowMs {
			return true
		}
	}
	for _, item := range message.Items {
		if item.BarOnly {
			continue
		}
		if item.ProgressObserved != nil {
			progress, threshold := 0.0, 95.0
			if item.ProgressPercent != nil {
				progress = *item.ProgressPercent
			}
			if item.ProgressThresholdPercent != nil {
				threshold = *item.ProgressThresholdPercent
			}
			if item.AlwaysVisible || (*item.ProgressObserved && progress >= threshold && progress < 100) {
				return true
			}
			continue
		}
		if item.AlwaysVisible || (item.ReadyAtMs > 0 && nowMs >= item.ReadyAtMs && nowMs-item.ReadyAtMs < 2600) {
			return true
		}
	}
	return false
}

// handleSkillOverlayState bridges the main report and native overlay window.
// Keeping the latest state server-side lets rendering resume without waiting
// for the next skill event.
func handleSkillOverlayState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		skillOverlayState.RLock()
		data := append([]byte(nil), skillOverlayState.data...)
		skillOverlayState.RUnlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPut:
		data, err := io.ReadAll(io.LimitReader(r.Body, 256*1024))
		if err != nil || !setSkillOverlayState(data) {
			http.Error(w, "invalid overlay state", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
