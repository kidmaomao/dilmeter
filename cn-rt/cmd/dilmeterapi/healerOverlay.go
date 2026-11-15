package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type healerOverlayGroup struct {
	Key       string          `json:"key"`
	Kind      string          `json:"kind"`
	X         int             `json:"x"`
	Y         int             `json:"y"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	NameWidth int             `json:"nameWidth,omitempty"`
	CellWidth int             `json:"cellWidth,omitempty"`
	Columns   int             `json:"columns,omitempty"`
	Rows      []healerTeamRow `json:"rows,omitempty"`
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
	h.previewIconSize = clampNativeReminderInt(request.IconSize, 16, 80, 20)
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
				{Key: "sample-health", MemberKey: "self", Name: "自身角色", Title: "血量", Category: "health", Value: "76%"},
				{Key: "sample-mana", MemberKey: "self", Name: "自身角色", Title: "魔法值", Category: "mana", Value: "62%"},
				{Key: "sample-skill", MemberKey: "sample-2", Name: "小月亮", Title: "崩坏波动", Category: "skill", SkillID: 59005, Value: "24s", State: "cooling"},
				{Key: "sample-curtain", MemberKey: "sample-2", Name: "小月亮", Category: "curtain", Value: "2480"},
				{Key: "sample-buff", MemberKey: "sample-2", Name: "小月亮", Title: "战争序曲", Category: "music", CCID: 680, Value: "132s", State: "active"},
				{Key: "sample-health-2", MemberKey: "sample-2", Name: "小月亮", Title: "血量", Category: "health", Value: "93%"},
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
		FontSize:     clampNativeReminderInt(settings.FontSize, 12, 72, 16), IconSize: clampNativeReminderInt(iconSize, 16, 80, 20),
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

	// The WebView scales the entire panel from its top left. Give the native
	// window the matching scaled dimensions, plus room for its shadow.
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
	const panelPaddingX, panelPaddingY, rowPaddingY = 8, 4, 4
	const nameGap, cellGap, vitalsGap = 8, 4, 3
	textSize := max(10, fontSize*3/4)
	group.CellWidth = iconSize + cellGap + healerLabelWidth("死亡丢失", textSize)
	maxCards := 0
	for _, row := range group.Rows {
		name := row.Name
		if row.PartySlot > 0 {
			name = strconv.Itoa(row.PartySlot) + "  " + name
		}
		group.NameWidth = max(group.NameWidth, healerLabelWidth(name, fontSize))
		maxCards = max(maxCards, len(row.Skills)+len(row.Buffs))
	}
	group.NameWidth = min(group.NameWidth, fontSize*8)
	group.Width = panelPaddingX*2 + group.NameWidth
	group.Height = panelPaddingY * 2
	group.Columns = min(max(1, maxCards), 4, max(1, (widthLimit-group.Width-nameGap+cellGap)/(group.CellWidth+cellGap)))
	if maxCards > 0 {
		group.Width += nameGap + group.Columns*group.CellWidth + (group.Columns-1)*cellGap
	}
	for _, row := range group.Rows {
		cards := len(row.Skills) + len(row.Buffs)
		height := fontSize + 4
		if cards > 0 {
			rows := (cards + group.Columns - 1) / group.Columns
			height = rows*max(iconSize, height) + (rows-1)*cellGap
		}
		vitals := 0
		if row.Health != nil || row.Curtain != "" {
			vitals++
		}
		if row.Mana != nil {
			vitals++
		}
		if vitals > 0 {
			group.Width = max(group.Width, 156)
			height += cellGap + vitals*max(14, textSize+4) + (vitals-1)*vitalsGap
		}
		if row.Curtain != "" {
			commas := (len(row.Curtain) - 1) / 3
			group.Width = max(group.Width, panelPaddingX*2+healerLabelWidth(row.Curtain+strings.Repeat(",", commas), textSize))
		}
		group.Height += rowPaddingY*2 + height
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
