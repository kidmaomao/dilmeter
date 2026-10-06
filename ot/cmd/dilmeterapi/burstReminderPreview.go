package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const burstPreviewDurationMs int64 = 8000

// Caller owns skillOverlayState's lock. Live updates never own the preview lifetime.
func mergeBurstPreviewLocked(message nativeSkillOverlayMessage, nowMs int64) nativeSkillOverlayMessage {
	mechanics := make([]nativeBossMechanicOverlayItem, 0, len(message.Mechanics)+1)
	for _, item := range message.Mechanics {
		if !strings.HasPrefix(item.Key, "burst-preview") {
			mechanics = append(mechanics, item)
		}
	}
	if preview := skillOverlayState.burstPreview; preview != nil {
		if preview.PreviewExpiresAtMs > nowMs {
			mechanics = append(mechanics, *preview)
		} else {
			skillOverlayState.burstPreview = nil
		}
	}
	message.Mechanics = mechanics
	return message
}

func setBurstReminderPreview(item nativeBossMechanicOverlayItem, nowMs int64) {
	item.Key = "burst-preview-" + item.Phase
	item.StartedAtMs, item.EndsAtMs = nowMs, nowMs+burstPreviewDurationMs
	item.PreviewExpiresAtMs, item.Generation = item.EndsAtMs, uint64(nowMs)
	item.X, item.Y = max(-32000, min(32000, item.X)), max(-32000, min(32000, item.Y))
	item.ScalePercent = max(50, min(200, item.ScalePercent))
	skillOverlayState.Lock()
	var message nativeSkillOverlayMessage
	_ = json.Unmarshal(skillOverlayState.data, &message)
	skillOverlayState.burstPreview = &item
	message = mergeBurstPreviewLocked(message, nowMs)
	message.Type, message.AtMs = "skill-cooldown-state", nowMs
	skillOverlayState.data, _ = json.Marshal(message)
	skillOverlayState.Unlock()
	setNativeSkillOverlayActive(true)
	refreshWebViewReminderHitRegions()
}

func handleBurstReminderPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var item nativeBossMechanicOverlayItem
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&item) != nil ||
		(item.SkillID != 59005 && item.SkillID != 58014) ||
		(item.Phase != "cast" && item.Phase != "effect" && item.Phase != "ready" && item.Phase != "cooldown") {
		http.Error(w, "invalid burst preview", http.StatusBadRequest)
		return
	}
	setBurstReminderPreview(item, time.Now().UnixMilli())
	w.WriteHeader(http.StatusNoContent)
}
