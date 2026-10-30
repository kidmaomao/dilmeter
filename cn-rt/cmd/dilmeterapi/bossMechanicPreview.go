package main

import (
	"encoding/json"
	"math"
	"net/http"
	"time"
)

var bossPreviewNames = map[string]string{
	brontasRockKey:  "二王 50% 落石",
	"miel-orb":      "缠绕的神迹（环绕球）",
	"miel-laser":    "神剑（射线）",
	greenCurtainKey: "四王绿幕",
}

type bossMechanicPreviewRequest struct {
	Rotation        int     `json:"rotation"`
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
	for _, key := range []string{greenCurtainKey, "miel-orb", "miel-laser", brontasRockKey} {
		if item, ok := skillOverlayState.bossPreviews[key]; ok {
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
			zero, width, _ := brontasGuidePreviewGrid().rayGeometry("vertical", 1)
			f := brontasGuideFrame{Active: true, Clock: 12, ObservedAtMs: nowMs, GeometryComplete: true, Player: &brontasGuidePlayer{brontasPoint: brontasPoint{X: 0, Y: -.35}, Radius: .08, Speed: 1.2}, Rays: []brontasGuideRay{{ID: "preview", Axis: "vertical", WarningAtMs: nowMs, LethalAtMs: nowMs + 1600, EndsAtMs: nowMs + 3000, Offset: &zero, HalfWidth: &width}}}
			item.BrontasGuide = planBrontasGuide(f, nowMs)
			item.BrontasGuide.Detail = "预览数据 · 非实战识别"
			rotateBrontasGuide(item.BrontasGuide, request.Rotation)
		}
	}
	skillOverlayState.Lock()
	var message nativeSkillOverlayMessage
	_ = json.Unmarshal(skillOverlayState.data, &message)
	if skillOverlayState.bossPreviews == nil {
		skillOverlayState.bossPreviews = make(map[string]nativeBossMechanicOverlayItem)
	}
	skillOverlayState.bossPreviews[request.Key] = item
	message = mergeBossMechanicPreviewsLocked(message, nowMs)
	message.Type, message.AtMs = "skill-cooldown-state", nowMs
	skillOverlayState.data, _ = json.Marshal(message)
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
		(request.Key == greenCurtainKey && request.DurationSeconds > 62) {
		http.Error(w, "invalid boss mechanic preview", http.StatusBadRequest)
		return
	}
	setBossMechanicPreview(request, time.Now().UnixMilli())
	w.WriteHeader(http.StatusNoContent)
}
