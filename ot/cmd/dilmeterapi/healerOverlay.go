package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type healerOverlayGroup struct {
	Key        string          `json:"key"`
	Kind       string          `json:"kind"`
	X          int             `json:"x"`
	Y          int             `json:"y"`
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	NameWidth  int             `json:"nameWidth,omitempty"`
	CellWidth  int             `json:"cellWidth,omitempty"`
	CellHeight int             `json:"cellHeight,omitempty"`
	Columns    int             `json:"columns,omitempty"`
	Rows       []healerTeamRow `json:"rows,omitempty"`
}

type healerTeamRow struct {
	PartySlot int          `json:"partySlot,omitempty"`
	Key       string       `json:"key"`
	Name      string       `json:"name"`
	Skills    []healerCard `json:"skills,omitempty"`
	Buffs     []healerCard `json:"buffs,omitempty"`
	Health    *healerCard  `json:"health,omitempty"`
	Mana      *healerCard  `json:"mana,omitempty"`
	Curtain   string       `json:"curtain,omitempty"`
}

type healerOverlayFrame struct {
	Groups         []healerOverlayGroup `json:"groups"`
	X              int                  `json:"x"`
	Y              int                  `json:"y"`
	Width          int                  `json:"width"`
	Height         int                  `json:"height"`
	ScalePercent   int                  `json:"scalePercent"`
	DPIPercent     int                  `json:"dpiPercent"`
	FontSize       int                  `json:"fontSize"`
	IconSize       int                  `json:"iconSize"`
	OpacityPercent int                  `json:"opacityPercent"`
	Preview        bool                 `json:"preview"`
	UpdatedAt      int64                `json:"updatedAt"`
}

func currentNativeReminderRuntime() *nativeReminderRuntime {
	nativeReminderRuntimeHolder.RLock()
	defer nativeReminderRuntimeHolder.RUnlock()
	return nativeReminderRuntimeHolder.runtime
}

func (runtime *nativeReminderRuntime) healerOverlaySnapshot(nowMs int64) healerOverlayFrame {
	runtime.settingsMu.RLock()
	configuredDPI := runtime.settings.Buff.DPIPercent
	runtime.settingsMu.RUnlock()
	frame := runtime.healer.overlaySnapshot(nowMs)
	frame.DPIPercent = healerOverlayDPI(configuredDPI)
	return frame
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
	runtime := currentNativeReminderRuntime()
	if runtime == nil || runtime.healer == nil {
		http.Error(w, "healer monitor unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(runtime.healerOverlaySnapshot(time.Now().UnixMilli()))
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
	runtime := currentNativeReminderRuntime()
	if runtime == nil || runtime.healer == nil {
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
	h := runtime.healer
	h.mu.Lock()
	defer h.mu.Unlock()
	if request.Stop {
		h.previewUntilMs, h.previewCards = 0, nil
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if request.Kind == "mana" && (request.Member == nil || !request.Member.Self) {
		http.Error(w, "仅支持预览自身魔法值", http.StatusBadRequest)
		return
	}
	h.previewText = normalizeHealerText(request.Text)
	h.previewIconSize = clampNativeReminderInt(request.IconSize, 16, 80, healerDefaultIconSize)
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
		} else if request.Kind == "mana" {
			h.previewCards = []healerCard{{Key: "preview-mana", MemberKey: member.Key, Name: member.Name, Title: "低魔法值提醒", Value: "20%", Category: "mana", State: "low", X: member.ManaSettings.Overlay.X, Y: member.ManaSettings.Overlay.Y, Flash: true}}
		} else {
			for index, rule := range member.BuffSettings.Rules {
				if !*rule.OverlayEnabled {
					continue
				}
				h.previewCards = append(h.previewCards, healerCard{Key: fmt.Sprintf("preview-buff-%d", rule.CCID), MemberKey: member.Key, Name: member.Name, Title: rule.Name, Value: []string{"1380s", "24s", "生效"}[index%3], Category: "buff", State: "active", CCID: rule.CCID, X: member.BuffSettings.Overlay.X, Y: member.BuffSettings.Overlay.Y})
			}
			skillExamples := [...]struct{ value, state string }{
				{"就绪", "ready"}, {"24s", "cooling"}, {"未观测", "unknown"},
			}
			for index, rule := range member.SkillSettings.Rules {
				example := skillExamples[index%len(skillExamples)]
				h.previewCards = append(h.previewCards, healerCard{Key: fmt.Sprintf("preview-skill-%d", rule.SkillID), MemberKey: member.Key, Name: member.Name, Title: rule.Name, Value: example.value, Category: "skill", State: example.state, SkillID: rule.SkillID, X: member.BuffSettings.Overlay.X, Y: member.BuffSettings.Overlay.Y})
			}
			if len(h.previewCards) == 0 {
				http.Error(w, "请先添加需要显示的 Buff 或技能", http.StatusBadRequest)
				h.previewUntilMs = 0
				return
			}
		}
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
			cards = []healerCard{
				{Key: "sample-skill", MemberKey: "sample-2", Name: "七罪", Title: "崩坏波动", Category: "skill", SkillID: 59005, Value: "未观测", State: "unknown"},
				{Key: "sample-buff", MemberKey: "sample-2", Name: "七罪", Title: "战争序曲", Category: "music", CCID: 680, Value: "32s", State: "active"},
				{Key: "sample-buff-3", MemberKey: "sample-3", Name: "月牙", Title: "战争序曲", Category: "music", CCID: 680, Value: "125s", State: "active"},
			}
		}
		return h.previewText, append([]healerCard(nil), cards...), true
	}
	if !h.liveOverlay(nowMs) {
		return h.settings.Text, nil, false
	}
	return h.settings.Text, append([]healerCard(nil), h.state.Cards...), false
}

func (h *healerMonitor) liveOverlay(nowMs int64) bool {
	return h.settings.Enabled && h.settings.Text.Enabled && nowMs-h.state.UpdatedAt <= 1500
}

func (h *healerMonitor) overlaySnapshot(nowMs int64) healerOverlayFrame {
	h.mu.Lock()
	defer h.mu.Unlock()
	settings, cards, preview := h.textSnapshotLocked(nowMs)
	iconSize := h.settings.IconSize
	var members []healerMemberState
	if preview {
		iconSize = h.previewIconSize
		if h.previewCards == nil {
			mana, health := 99.9, 100.0
			members = []healerMemberState{
				{Key: "self", Name: "小月亮", PartySlot: 1, showHealth: true, showMana: true, ManaPercent: &mana},
				{Key: "sample-2", Name: "七罪", PartySlot: 2, showHealth: true, HealthPercent: &health},
				{Key: "sample-3", Name: "月牙", PartySlot: 3, showHealth: true},
			}
		}
	} else if h.liveOverlay(nowMs) {
		members = h.state.Members
	}
	frame := buildHealerOverlayFrame(settings, iconSize, cards, members, preview, nowMs)
	frame.OpacityPercent = h.settings.OpacityPercent
	if preview {
		frame.OpacityPercent = h.previewOpacityPercent
	}
	frame.OpacityPercent = clampNativeReminderInt(frame.OpacityPercent, 20, 100, 100)
	return frame
}
func buildHealerOverlayFrame(settings healerTextSettings, iconSize int, cards []healerCard, members []healerMemberState, preview bool, nowMs int64) healerOverlayFrame {
	frame := healerOverlayFrame{
		OpacityPercent: 100, Groups: []healerOverlayGroup{}, Preview: preview, UpdatedAt: nowMs, Width: 1, Height: 1,
		ScalePercent: clampNativeReminderInt(settings.ScalePercent, 50, 200, 100),
		FontSize:     clampNativeReminderInt(settings.FontSize, 12, 72, 16), IconSize: clampNativeReminderInt(iconSize, 16, 80, healerDefaultIconSize),
	}
	team := healerOverlayGroup{Key: "team", Kind: "team", X: settings.X, Y: settings.Y}
	indices := make(map[string]int)
	rowFor := func(key, name string) *healerTeamRow {
		index, ok := indices[key]
		if !ok {
			index = len(team.Rows)
			indices[key] = index
			team.Rows = append(team.Rows, healerTeamRow{Key: key, Name: name})
		}
		return &team.Rows[index]
	}

	for _, member := range members {
		row := rowFor(member.Key, member.Name)
		row.PartySlot = member.PartySlot
		if member.showHealth {
			row.Health = healerResourceBar(member.HealthPercent, "血量")
		}
		if member.showMana {
			row.Mana = &healerCard{Title: "魔法值", Value: "—", State: member.ManaState}
			if member.ManaPercent != nil {
				row.Mana.Value = healerManaPercentText(*member.ManaPercent)
				if member.ManaCurrent != nil {
					row.Mana.Title = fmt.Sprintf("魔法值 %.0f / %.0f", *member.ManaCurrent, member.ManaMaximum)
				}
			} else if member.ManaCurrent != nil {
				row.Mana.Value = fmt.Sprintf("%.0f", *member.ManaCurrent)
				row.Mana.Title = "已同步当前魔法值，等待上限"
			}
		}
		if member.CurtainAmount != nil {
			row.Curtain = strconv.FormatInt(*member.CurtainAmount, 10)
		}
	}
	for _, card := range cards {
		row := rowFor(card.MemberKey, card.Name)
		switch card.Category {
		case "health":
			row.Health = &card
		case "mana":
			row.Mana = &card
		case "curtain":
			row.Curtain = card.Value
		case "skill":
			row.Skills = append(row.Skills, card)
		default:
			row.Buffs = append(row.Buffs, card)
		}
	}
	visible := team.Rows[:0]
	for _, row := range team.Rows {
		if row.PartySlot > 0 || len(row.Skills)+len(row.Buffs) > 0 || row.Health != nil || row.Mana != nil || row.Curtain != "" {
			visible = append(visible, row)
		}
	}
	team.Rows = visible
	if len(team.Rows) == 0 {
		return frame
	}
	sort.SliceStable(team.Rows, func(i, j int) bool {
		return healerTeamRowOrder(team.Rows[i]) < healerTeamRowOrder(team.Rows[j])
	})
	layoutHealerTeam(&team, frame.FontSize, frame.IconSize, settings.Width)
	frame.Groups = append(frame.Groups, team)

	// Layout dimensions and shadow margins are logical pixels. The team X/Y
	// anchor stays in physical desktop coordinates, including on secondary monitors.
	const windowMargin = 12
	frame.X, frame.Y = team.X-windowMargin, team.Y-windowMargin
	frame.Width = (team.Width*frame.ScalePercent+99)/100 + 2*windowMargin
	frame.Height = (team.Height*frame.ScalePercent+99)/100 + 2*windowMargin
	return frame
}

func healerTeamRowOrder(row healerTeamRow) int {
	if row.PartySlot > 0 {
		return row.PartySlot
	}
	if row.Key == "self" {
		return healerMaxPartySlots + 1
	}
	return healerMaxPartySlots + 2
}

func healerResourceBar(percent *float64, title string) *healerCard {
	value := "—"
	if percent != nil {
		value = fmt.Sprintf("%.0f%%", *percent)
	}
	return &healerCard{Title: title, Value: value}
}

// Match the padding, row heights and gaps in HealerOverlayVisual.vue so the
// native window contains all rows, including status grids that wrap.
func layoutHealerTeam(group *healerOverlayGroup, fontSize, iconSize, widthLimit int) {
	const panelPaddingX, panelPaddingY, rowPaddingY = 6, 3, 4
	const nameplatePadding, nameGap, cellGap, statusGap, manaGap = 4, 4, 2, 4, 2
	const defaultWidth, preferredColumns = 224, 6
	textSize := max(10, fontSize*3/4)
	// Width depends only on appearance settings, including the space for a full shield value.
	preferredWidth := max(defaultWidth, 2*panelPaddingX+fontSize+preferredColumns*iconSize+(preferredColumns-1)*cellGap)
	shieldWidth := healerLabelWidth("9,223,372,036,854,775,807", textSize)
	group.Width = max(min(preferredWidth, clampNativeReminderInt(widthLimit, 280, 1600, 660)), 2*(panelPaddingX+nameplatePadding)+fontSize+nameGap+shieldWidth)
	innerWidth := group.Width - 2*panelPaddingX
	group.NameWidth = innerWidth - 2*nameplatePadding
	group.CellWidth = iconSize
	// The dark caption spans the bottom edge, leaving most of the square artwork visible.
	group.CellHeight = iconSize + min(iconSize/4, (textSize+2)/2)
	group.Columns = max(1, (innerWidth-fontSize+cellGap)/(iconSize+cellGap))
	group.Height = 2 * panelPaddingY
	for _, row := range group.Rows {
		height := fontSize + 8
		if row.Mana != nil {
			height += manaGap + textSize + 6
		}
		if count := len(row.Skills) + len(row.Buffs); count > 0 {
			lines := (count + group.Columns - 1) / group.Columns
			height += statusGap + lines*group.CellHeight + (lines-1)*cellGap
		}
		group.Height += 2*rowPaddingY + height
	}
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
