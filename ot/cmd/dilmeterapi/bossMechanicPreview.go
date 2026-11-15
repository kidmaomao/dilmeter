package main

import (
	"encoding/json"
	"math"
	"net/http"
	"time"
)

var bossPreviewNames = map[string]string{
	perlonTornadoKey:   "尔格龙卷风",
	perlonResonanceKey: "吸收 · 靠近区域",
	brontasRockKey:     "二王 50% 落石",
	"miel-orb":         "缠绕的神迹（环绕球）",
	"miel-laser":       "神剑（射线）",
	greenCurtainKey:    "四王绿幕",
}

type bossMechanicPreviewRequest struct {
	TornadoTarget   string  `json:"tornadoTarget"`
	ResonanceMode   string  `json:"resonanceMode"`
	AreaWarning     bool    `json:"areaWarning"`
	Rotation        int     `json:"rotation"`
	Facing          string  `json:"facing"`
	GuidanceEnabled bool    `json:"guidanceEnabled"`
	PassiveStyle    string  `json:"passiveStyle"`
	OpacityPercent  int     `json:"opacityPercent"`
	Key             string  `json:"key"`
	DurationSeconds float64 `json:"durationSeconds"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	ScalePercent    int     `json:"scalePercent"`
}

// Caller holds skillOverlayState's lock. A preview owns its deadline, while
// each live frame continues to own the actual encounter and other overlays.
func mergeBossMechanicPreviewsLocked(message nativeSkillOverlayMessage, nowMs int64) nativeSkillOverlayMessage {
	for key, item := range skillOverlayState.bossPreviews {
		if item.PreviewExpiresAtMs <= nowMs {
			delete(skillOverlayState.bossPreviews, key)
		}
	}
	items := make([]nativeBossMechanicOverlayItem, 0, len(message.Mechanics)+len(skillOverlayState.bossPreviews))
	for _, item := range message.Mechanics {
		_, previewing := skillOverlayState.bossPreviews[item.Key]
		if previewing || (bossPreviewNames[item.Key] != "" && item.PreviewExpiresAtMs > 0) {
			continue // Never reuse an expired preview echoed by an older frame.
		}
		items = append(items, item)
	}
	for _, key := range []string{greenCurtainKey, "miel-orb", "miel-laser", brontasRockKey, perlonTornadoKey, perlonResonanceKey} {
		if item, ok := skillOverlayState.bossPreviews[key]; ok {
			if key == brontasRockKey && item.BrontasGuide != nil {
				item.BrontasGuide = brontasStationPreview(item.StartedAtMs, nowMs, item.BrontasGuide.Clock, item.BrontasGuide.Facing)
			} else if key == brontasRockKey && nowMs < item.StartedAtMs+2500 {
				item.Rocks = nil // Preview the static map before confirming locations.
			}
			items = append(items, item)
		}
	}
	message.Mechanics = items
	return message
}

func setBossMechanicPreview(request bossMechanicPreviewRequest, nowMs int64) {
	icon := "mdi-laser-pointer"
	if request.Key == "miel-orb" {
		icon = "mdi-orbit"
	}
	if request.Key == greenCurtainKey {
		icon = "mdi-shield-outline"
	}
	endsAtMs := nowMs + int64(math.Round(request.DurationSeconds*1000))
	item := nativeBossMechanicOverlayItem{
		Key: request.Key, Name: bossPreviewNames[request.Key], Icon: icon,
		StartedAtMs: nowMs, EndsAtMs: endsAtMs, PreviewExpiresAtMs: endsAtMs,
		Generation: uint64(nowMs), X: max(-32000, min(32000, request.X)),
		Y: max(-32000, min(32000, request.Y)), ScalePercent: max(50, min(200, request.ScalePercent)),
	}
	if request.Key == perlonTornadoKey || request.Key == perlonResonanceKey {
		item.OpacityPercent = clampNativeReminderInt(request.OpacityPercent, 20, 100, 100)
	}
	if request.Key == perlonTornadoKey {
		item.TornadoStartsAtMs = min(endsAtMs, nowMs+1000)
		item.TornadoTarget = "self"
		if request.TornadoTarget == "teammate" {
			item.TornadoTarget = "teammate"
		}
	}
	if request.Key == perlonResonanceKey {
		item.ResonanceMode = "absorb"
		if request.ResonanceMode == "release" {
			item.ResonanceMode = "release"
			item.Name = "释放 · 远离区域"
			if request.AreaWarning {
				item.AreaWarningUntilMs = endsAtMs
			}
		}
	}
	if request.Key == greenCurtainKey && (request.PassiveStyle == "timer" || request.PassiveStyle == "bar") {
		item.PassiveStyle = request.PassiveStyle
		item.OpacityPercent = clampNativeReminderInt(request.OpacityPercent, 20, 100, 100)
	}
	if request.Key == brontasRockKey {
		item.Rocks = []nativeBrontasRockMarker{{Clock: 12}, {Clock: 9}}
		for i := range item.Rocks {
			if request.Rotation == 90 || request.Rotation == 180 || request.Rotation == 270 {
				item.Rocks[i].Clock = (item.Rocks[i].Clock+request.Rotation/30-1)%12 + 1
			}
		}
		item.HideCountdown = true
		if request.GuidanceEnabled {
			clock := 9
			item.BrontasGuide = brontasStationPreview(nowMs, nowMs, clock, request.Facing)
		}
	}
	skillOverlayState.Lock()
	if skillOverlayState.bossPreviews == nil {
		skillOverlayState.bossPreviews = make(map[string]nativeBossMechanicOverlayItem)
	}
	skillOverlayState.bossPreviews[request.Key] = item
	composeSkillOverlayStateLocked(nowMs)
	skillOverlayState.Unlock()
	setNativeSkillOverlayActive(true)
	refreshWebViewReminderHitRegions()
}

func handleBossMechanicPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request bossMechanicPreviewRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request) != nil ||
		bossPreviewNames[request.Key] == "" || math.IsNaN(request.DurationSeconds) || math.IsInf(request.DurationSeconds, 0) ||
		request.DurationSeconds < 1 || request.DurationSeconds > 120 ||
		(request.ResonanceMode != "" && request.ResonanceMode != "absorb" && request.ResonanceMode != "release") ||
		(request.Key == greenCurtainKey && request.DurationSeconds > 62) {
		http.Error(w, "invalid boss mechanic preview", http.StatusBadRequest)
		return
	}
	setBossMechanicPreview(request, time.Now().UnixMilli())
	w.WriteHeader(http.StatusNoContent)
}
