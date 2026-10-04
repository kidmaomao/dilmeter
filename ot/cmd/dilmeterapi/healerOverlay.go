package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type healerOverlayGroup struct {
	Key           string                 `json:"key"`
	Name          string                 `json:"name"`
	Kind          string                 `json:"kind"`
	X             int                    `json:"x"`
	Y             int                    `json:"y"`
	Width         int                    `json:"width"`
	Height        int                    `json:"height"`
	CellWidth     int                    `json:"cellWidth"`
	NameWidth     int                    `json:"nameWidth"`
	Cards         []healerCard           `json:"cards"`
	Columns       int                    `json:"columns"`
	CellHeight    int                    `json:"cellHeight"`
	LabelWidth    int                    `json:"labelWidth"`
	LabelFontSize int                    `json:"labelFontSize"`
	Sections      []healerOverlaySection `json:"sections"`
}

type healerOverlaySection struct {
	Kind  string       `json:"kind"`
	Cards []healerCard `json:"cards"`
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
			for index, rule := range member.SkillSettings.Rules {
				h.previewCards = append(h.previewCards, healerCard{Key: fmt.Sprintf("preview-skill-%d", rule.SkillID), MemberKey: member.Key, Name: member.Name, Title: rule.Name, Value: []string{"就绪", "24s", "未观测"}[index%3], Category: "skill", State: "ready", SkillID: rule.SkillID, X: member.BuffSettings.Overlay.X, Y: member.BuffSettings.Overlay.Y})
			}
			if len(h.previewCards) == 0 {
				http.Error(w, "请先添加需要显示的 Buff 或技能", http.StatusBadRequest)
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
			layoutHealerStatusGroup(group, frame.FontSize, frame.IconSize)
		}
	}
	// Saved coordinates are anchors. If wrapped status panels overlap, move the
	// lower panel down for this frame only; never rewrite the user's settings.
	statusIndices := []int{}
	for i, group := range frame.Groups {
		if group.Kind != "health" {
			statusIndices = append(statusIndices, i)
		}
	}
	sort.SliceStable(statusIndices, func(i, j int) bool {
		return frame.Groups[statusIndices[i]].Y < frame.Groups[statusIndices[j]].Y
	})
	for position, index := range statusIndices {
		group := &frame.Groups[index]
		for _, previous := range statusIndices[:position] {
			above := frame.Groups[previous]
			if group.X < above.X+above.Width && group.X+group.Width > above.X && group.Y < above.Y+above.Height+6 {
				group.Y = above.Y + above.Height + 6
			}
		}
	}
	for i, group := range frame.Groups {
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

// Keep this geometry in sync with healerStatusLayout in healerMonitorTypes.ts:
// the native window must contain every wrapped row drawn by the WebView.
func layoutHealerStatusGroup(group *healerOverlayGroup, fontSize, iconSize int) {
	group.NameWidth = min(max(120, fontSize*8), max(healerLabelWidth("队友", min(12, fontSize)), healerLabelWidth(group.Name, fontSize)))
	group.LabelFontSize = max(10, min(16, fontSize*3/4))
	group.LabelWidth = healerLabelWidth("Buff", group.LabelFontSize)
	group.CellWidth = iconSize
	group.CellHeight = iconSize + 2 + (fontSize*6+4)/5
	group.Columns = 1
	group.Sections = []healerOverlaySection{}
	for _, kind := range []string{"skill", "buff"} {
		section := healerOverlaySection{Kind: kind, Cards: []healerCard{}}
		for _, card := range group.Cards {
			if (card.Category == "skill") == (kind == "skill") {
				section.Cards = append(section.Cards, card)
				group.CellWidth = max(group.CellWidth, healerLabelWidth(card.Value, fontSize))
			}
		}
		if len(section.Cards) == 0 {
			continue
		}
		// Reserve normal status/digit widths so ticking does not move the icons.
		baseline := "00000" // Buff durations may contain five digits.
		if kind == "skill" {
			baseline = "未观测"
		}
		group.CellWidth = max(group.CellWidth, healerLabelWidth(baseline, fontSize))
		group.Columns = max(group.Columns, min(4, len(section.Cards)))
		group.Sections = append(group.Sections, section)
	}
	contentHeight := 0
	for index, section := range group.Sections {
		rows := (len(section.Cards) + group.Columns - 1) / group.Columns
		contentHeight += rows*group.CellHeight + (rows-1)*6
		if index > 0 {
			contentHeight += 7 // 3px space on each side of the divider.
		}
	}
	group.Width = 18 + group.NameWidth + 6 + group.LabelWidth + 4 + group.Columns*group.CellWidth + (group.Columns-1)*4
	group.Height = 14 + max(group.CellHeight, contentHeight)
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
