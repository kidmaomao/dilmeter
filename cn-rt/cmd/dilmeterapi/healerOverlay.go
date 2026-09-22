package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type healerOverlayGroup struct {
	Key       string       `json:"key"`
	Name      string       `json:"name"`
	Kind      string       `json:"kind"`
	X         int          `json:"x"`
	Y         int          `json:"y"`
	Width     int          `json:"width"`
	Height    int          `json:"height"`
	CellWidth int          `json:"cellWidth"`
	NameWidth int          `json:"nameWidth"`
	Cards     []healerCard `json:"cards"`
}

type healerOverlayFrame struct {
	Groups         []healerOverlayGroup `json:"groups"`
	X              int                  `json:"x"`
	Y              int                  `json:"y"`
	Width          int                  `json:"width"`
	Height         int                  `json:"height"`
	FontSize       int                  `json:"fontSize"`
	IconSize       int                  `json:"iconSize"`
	OpacityPercent int                  `json:"opacityPercent"`
	Preview        bool                 `json:"preview"`
	UpdatedAt      int64                `json:"updatedAt"`
}

func healerRuntimeMonitor() *healerMonitor {
	nativeReminderRuntimeHolder.RLock()
	defer nativeReminderRuntimeHolder.RUnlock()
	if nativeReminderRuntimeHolder.runtime == nil {
		return nil
	}
	return nativeReminderRuntimeHolder.runtime.healer
}

func handleHealerOverlay(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "local access only", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h := healerRuntimeMonitor()
	if h == nil {
		http.Error(w, "healer monitor unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(h.overlaySnapshot(time.Now().UnixMilli()))
}

// Previews never change selected members, saved settings, or sound latches.
func handleHealerTextPreview(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "local access only", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h := healerRuntimeMonitor()
	if h == nil {
		http.Error(w, "healer monitor unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 24<<10)
	var request struct {
		Text           healerTextSettings     `json:"text"`
		IconSize       int                    `json:"iconSize"`
		OpacityPercent *int                   `json:"opacityPercent"`
		Member         *healerMemberSelection `json:"member"`
		Kind           string                 `json:"kind"`
		Stop           bool                   `json:"stop"`
	}
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid preview", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.previewText = normalizeHealerText(request.Text)
	h.previewIconSize = clampNativeReminderInt(request.IconSize, 16, 80, 30)
	h.previewOpacityPercent = h.settings.OpacityPercent
	if request.OpacityPercent != nil {
		h.previewOpacityPercent = clampNativeReminderInt(*request.OpacityPercent, 20, 100, 100)
	}
	h.previewUntilMs = time.Now().Add(8 * time.Second).UnixMilli()
	h.previewCards = nil
	if request.Member != nil {
		member := normalizeHealerMember(*request.Member, h.settings, 0)
		if member.Name == "" {
			member.Name = "Oneforall"
		}
		if request.Kind == "health" {
			h.previewCards = []healerCard{{Key: "preview-health", MemberKey: member.Key, Name: member.Name, Title: "低血量提醒", Value: "20%", Category: "health", State: "low", X: member.HealthSettings.Overlay.X, Y: member.HealthSettings.Overlay.Y, Flash: true}}
		} else {
			for index, rule := range member.BuffSettings.Rules {
				if !*rule.OverlayEnabled {
					continue
				}
				h.previewCards = append(h.previewCards, healerCard{Key: "preview-" + rule.Name, MemberKey: member.Key, Name: member.Name, Title: rule.Name, Value: []string{"1380s", "24s", "生效"}[index%3], Category: "buff", State: "active", CCID: rule.CCID, X: member.BuffSettings.Overlay.X, Y: member.BuffSettings.Overlay.Y})
			}
			if len(h.previewCards) == 0 {
				http.Error(w, "请先添加并勾选需要显示的 Buff 图标", http.StatusBadRequest)
				h.previewUntilMs = 0
				return
			}
		}
	}
	if request.Stop {
		h.previewUntilMs = 0
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *healerMonitor) textSnapshot(nowMs int64) (healerTextSettings, []healerCard, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.textSnapshotLocked(nowMs)
}
func (h *healerMonitor) textSnapshotLocked(nowMs int64) (healerTextSettings, []healerCard, bool) {
	if nowMs < h.previewUntilMs {
		cards := h.previewCards
		if cards == nil {
			cards = []healerCard{{Key: "preview-health", MemberKey: "preview", Name: "Oneforall", Title: "血量≤25%", Value: "20%", Category: "health", X: h.previewText.X, Y: h.previewText.Y}, {Key: "preview-music", MemberKey: "preview", Name: "Oneforall", Title: "战争序曲", Value: "11s", CCID: 680, Category: "music", X: h.previewText.X, Y: h.previewText.Y + 120}}
		}
		return h.previewText, append([]healerCard(nil), cards...), true
	}
	if !h.settings.Enabled || !h.settings.Text.Enabled || nowMs-h.state.UpdatedAt > 1500 {
		return h.settings.Text, nil, false
	}
	return h.settings.Text, append([]healerCard(nil), h.state.Cards...), false
}
func (h *healerMonitor) overlaySnapshot(nowMs int64) healerOverlayFrame {
	h.mu.Lock()
	defer h.mu.Unlock()
	settings, cards, preview := h.textSnapshotLocked(nowMs)
	iconSize := h.settings.IconSize
	if preview {
		iconSize = h.previewIconSize
	}
	frame := buildHealerOverlayFrame(settings.FontSize, iconSize, cards, preview, nowMs)
	frame.OpacityPercent = h.settings.OpacityPercent
	if preview {
		frame.OpacityPercent = h.previewOpacityPercent
	}
	frame.OpacityPercent = clampNativeReminderInt(frame.OpacityPercent, 20, 100, 100)
	return frame
}
func buildHealerOverlayFrame(fontSize, iconSize int, cards []healerCard, preview bool, nowMs int64) healerOverlayFrame {
	frame := healerOverlayFrame{OpacityPercent: 100, Groups: []healerOverlayGroup{}, FontSize: clampNativeReminderInt(fontSize, 12, 72, 24), IconSize: clampNativeReminderInt(iconSize, 16, 80, 30), Preview: preview, UpdatedAt: nowMs, Width: 1, Height: 1}
	indices := map[string]int{}
	for _, card := range cards {
		kind := "buff"
		if card.Category == "health" {
			kind = "health"
		}
		key := card.MemberKey + ":" + kind
		index, exists := indices[key]
		if !exists {
			index = len(frame.Groups)
			indices[key] = index
			frame.Groups = append(frame.Groups, healerOverlayGroup{Key: key, Name: card.Name, Kind: kind, X: card.X, Y: card.Y, Cards: []healerCard{}})
		}
		card.Value = strings.TrimSuffix(card.Value, "s")
		frame.Groups[index].Cards = append(frame.Groups[index].Cards, card)
	}
	for i := range frame.Groups {
		group := &frame.Groups[i]
		if group.Kind == "health" {
			group.Width = max(220, frame.FontSize*13)
			group.Height = frame.FontSize*4 + 24
		} else {
			group.NameWidth = min(max(220, frame.FontSize*12), max(72, healerLabelWidth(group.Name, frame.FontSize)))
			group.CellWidth = frame.IconSize
			for _, card := range group.Cards {
				group.CellWidth = max(group.CellWidth, healerLabelWidth(card.Value, frame.FontSize))
			}
			group.Width = group.NameWidth + 20 + len(group.Cards)*(group.CellWidth+6) + 12
			group.Height = frame.IconSize + frame.FontSize + 20
		}
		if i == 0 {
			frame.X, frame.Y = group.X-12, group.Y-12
		} else {
			frame.X = min(frame.X, group.X-12)
			frame.Y = min(frame.Y, group.Y-12)
		}
	}
	for _, group := range frame.Groups {
		frame.Width = max(frame.Width, group.X+group.Width+12-frame.X)
		frame.Height = max(frame.Height, group.Y+group.Height+12-frame.Y)
	}
	return frame
}

func healerLabelWidth(text string, fontSize int) int {
	units := 0
	for _, char := range text {
		if char < 128 {
			units += 6
		} else {
			units += 10
		}
	}
	return (units*fontSize+9)/10 + 4
}
