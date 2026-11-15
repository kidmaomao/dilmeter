package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// Preview state is separate from live combat state. Each key owns a fixed
// deadline; another preview or a live refresh cannot erase or extend it.
type reminderPreviewRequest struct {
	AtMs         int64                          `json:"atMs"`
	DurationMs   int64                          `json:"durationMs"`
	Items        []nativeSkillOverlayItem       `json:"items"`
	AimReminder  *nativeAimReminderOverlayItem  `json:"aimReminder,omitempty"`
	EffectTimers []nativeEffectTimerOverlayItem `json:"effectTimers"`
	StackAlerts  []nativeBuffStackOverlayItem   `json:"stackAlerts"`
	TargetHealth *nativeTargetHealthOverlayItem `json:"targetHealth,omitempty"`
}

type reminderPreviewState struct {
	ExpiresAtMs int64
	Content     reminderPreviewRequest
}

func mergeReminderPreviewsLocked(message nativeSkillOverlayMessage, nowMs int64) nativeSkillOverlayMessage {
	keys := make([]string, 0, len(skillOverlayState.previews))
	for key, preview := range skillOverlayState.previews {
		if preview.ExpiresAtMs <= nowMs {
			delete(skillOverlayState.previews, key)
		} else {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		preview := skillOverlayState.previews[key].Content
		for _, item := range preview.Items {
			items := make([]nativeSkillOverlayItem, 0, len(message.Items)+1)
			for _, live := range message.Items {
				if live.SkillID != item.SkillID {
					items = append(items, live)
				}
			}
			message.Items = append(items, item)
		}
		if preview.AimReminder != nil {
			message.AimReminder = preview.AimReminder
		}
		for _, item := range preview.EffectTimers {
			items := make([]nativeEffectTimerOverlayItem, 0, len(message.EffectTimers)+1)
			for _, live := range message.EffectTimers {
				if live.Key != item.Key {
					items = append(items, live)
				}
			}
			message.EffectTimers = append(items, item)
		}
		for _, item := range preview.StackAlerts {
			items := make([]nativeBuffStackOverlayItem, 0, len(message.StackAlerts)+1)
			for _, live := range message.StackAlerts {
				if live.CCID != item.CCID || live.SkillID != item.SkillID {
					items = append(items, live)
				}
			}
			message.StackAlerts = append(items, item)
		}
		if preview.TargetHealth != nil {
			message.TargetHealth = preview.TargetHealth
		}
	}
	return message
}

// Caller holds the state lock. Always compose from the most recent live frame,
// never a frame already decorated with previews (which could leak after expiry).
func composeSkillOverlayStateLocked(nowMs int64) nativeSkillOverlayMessage {
	var message nativeSkillOverlayMessage
	_ = json.Unmarshal(skillOverlayState.liveData, &message)
	message = mergeBurstPreviewLocked(message, nowMs)
	message = mergeBossMechanicPreviewsLocked(message, nowMs)
	message = mergeTargetHealthPreviewLocked(message, nowMs)
	message = mergeReminderPreviewsLocked(message, nowMs)
	message.Type = "skill-cooldown-state"
	if message.AtMs == 0 {
		message.AtMs = nowMs
	}
	skillOverlayState.revision++
	message.Revision = skillOverlayState.revision
	skillOverlayState.data, _ = json.Marshal(message)
	publishSkillOverlaySnapshotLocked(skillOverlayState.data)
	pushNativeSkillOverlayState(skillOverlayState.data)
	return message
}

func skillOverlayPreviewNeedsRefresh() bool {
	skillOverlayState.RLock()
	defer skillOverlayState.RUnlock()
	return len(skillOverlayState.previews) > 0 || len(skillOverlayState.bossPreviews) > 0 ||
		skillOverlayState.burstPreview != nil || skillOverlayState.healthPreview != nil
}

func setReminderPreview(request reminderPreviewRequest, nowMs int64) {
	expires := nowMs + request.DurationMs
	shift := func(at int64) int64 {
		if at == 0 {
			return 0
		}
		return nowMs + max(int64(-120000), min(int64(120000), at-request.AtMs))
	}
	skillOverlayState.Lock()
	if skillOverlayState.previews == nil {
		skillOverlayState.previews = make(map[string]reminderPreviewState)
	}
	put := func(key string, content reminderPreviewRequest) {
		skillOverlayState.previews[key] = reminderPreviewState{ExpiresAtMs: expires, Content: content}
	}
	for _, item := range request.Items {
		item.UsedAtMs, item.ReadyAtMs = shift(item.UsedAtMs), shift(item.ReadyAtMs)
		item.Generation, item.PreviewExpiresAtMs = uint64(nowMs), expires
		item.X, item.Y = max(-32000, min(32000, item.X)), max(-32000, min(32000, item.Y))
		put("skill:"+strconv.Itoa(int(item.SkillID)), reminderPreviewRequest{Items: []nativeSkillOverlayItem{item}})
	}
	if source := request.AimReminder; source != nil {
		item := *source
		item.StartedAtMs, item.ReadyAtMs = shift(item.StartedAtMs), shift(item.ReadyAtMs)
		item.Generation, item.PreviewExpiresAtMs = uint64(nowMs), expires
		put("aim", reminderPreviewRequest{AimReminder: &item})
	}
	for _, item := range request.EffectTimers {
		item.StartedAtMs, item.EndsAtMs = shift(item.StartedAtMs), shift(item.EndsAtMs)
		item.Generation, item.PreviewExpiresAtMs, item.Enabled = uint64(nowMs), expires, true
		put("effect:"+item.Key, reminderPreviewRequest{EffectTimers: []nativeEffectTimerOverlayItem{item}})
	}
	for _, item := range request.StackAlerts {
		item.StartedAtMs, item.EndsAtMs = nowMs, expires
		item.Generation, item.PreviewExpiresAtMs = uint64(nowMs), expires
		put("stack:"+strconv.Itoa(int(item.CCID))+":"+strconv.Itoa(int(item.SkillID)), reminderPreviewRequest{StackAlerts: []nativeBuffStackOverlayItem{item}})
	}
	if source := request.TargetHealth; source != nil {
		item := *source
		item.Generation, item.PreviewEndsAt = uint64(nowMs), expires
		put("health", reminderPreviewRequest{TargetHealth: &item})
	}
	composeSkillOverlayStateLocked(nowMs)
	skillOverlayState.Unlock()
	setNativeSkillOverlayActive(true)
	refreshWebViewReminderHitRegions()
}

func handleReminderPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request reminderPreviewRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&request) != nil ||
		request.AtMs <= 0 || request.DurationMs < 100 || request.DurationMs > 120000 ||
		len(request.Items)+len(request.EffectTimers)+len(request.StackAlerts) > 32 ||
		(len(request.Items)+len(request.EffectTimers)+len(request.StackAlerts) == 0 && request.AimReminder == nil && request.TargetHealth == nil) {
		http.Error(w, "invalid reminder preview", http.StatusBadRequest)
		return
	}
	setReminderPreview(request, time.Now().UnixMilli())
	w.WriteHeader(http.StatusNoContent)
}
