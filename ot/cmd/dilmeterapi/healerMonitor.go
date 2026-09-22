package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

// Only server-observed HP and recipient conditions are used. Nearby players
// are candidates, not an inferred party roster. Favorites are matched by exact
// character name after a fresh appearance, never by a persisted entity ID.
type healerMemberSelection struct {
	Key            string                    `json:"key"`
	Name           string                    `json:"name"`
	Favorite       bool                      `json:"favorite"`
	HealthSettings *healerHealthSettings     `json:"healthSettings,omitempty"`
	BuffSettings   *healerMemberBuffSettings `json:"buffSettings,omitempty"`
	ID             string                    `json:"id"`
	Included       bool                      `json:"included"`
	Buffs          []uint32                  `json:"buffs"`
	Health         bool                      `json:"health"`
	Overture       bool                      `json:"overture"`
	Vivace         bool                      `json:"vivace"`
}

type healerSound struct {
	Kind    string `json:"kind"`
	SoundID string `json:"soundId"`
	Name    string `json:"name"`
}

type healerPosition struct {
	Enabled bool `json:"enabled"`
	X       int  `json:"x"`
	Y       int  `json:"y"`
}

type healerHealthSettings struct {
	RepeatCount           int            `json:"repeatCount"`
	RepeatIntervalSeconds int            `json:"repeatIntervalSeconds"`
	Threshold             int            `json:"threshold"`
	SoundEnabled          bool           `json:"soundEnabled"`
	Sound                 healerSound    `json:"sound"`
	Overlay               healerPosition `json:"overlay"`
}

type healerMemberBuffSettings struct {
	Enabled      bool             `json:"enabled"`
	SoundEnabled bool             `json:"soundEnabled"`
	Overlay      healerPosition   `json:"overlay"`
	Rules        []healerBuffRule `json:"rules"`
}

type healerBuffRule struct {
	RepeatCount           int          `json:"repeatCount"`
	RepeatIntervalSeconds int          `json:"repeatIntervalSeconds"`
	CCID                  uint32       `json:"ccId"`
	Name                  string       `json:"name"`
	WarningSeconds        int          `json:"warningSeconds"`
	OverlayEnabled        *bool        `json:"overlayEnabled,omitempty"`
	FlashEnabled          *bool        `json:"flashEnabled,omitempty"`
	FlashSeconds          *int         `json:"flashSeconds,omitempty"`
	DurationMode          string       `json:"durationMode"`
	ManualDurationSeconds int          `json:"manualDurationSeconds"`
	Sound                 *healerSound `json:"sound,omitempty"`
}

type healerTextSettings struct {
	Enabled  bool `json:"enabled"`
	FontSize int  `json:"fontSize"`
	X        int  `json:"x"`
	Y        int  `json:"y"`
	Width    int  `json:"width"`
}

// Templates hold reminder preferences only, never a bound character identity.
type healerMemberTemplate struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Health         bool                      `json:"health"`
	HealthSettings *healerHealthSettings     `json:"healthSettings"`
	BuffSettings   *healerMemberBuffSettings `json:"buffSettings"`
}

type healerSettings struct {
	Version            int                     `json:"version"`
	IconSize           int                     `json:"iconSize"`
	OpacityPercent     int                     `json:"opacityPercent"`
	Templates          []healerMemberTemplate  `json:"templates"`
	Enabled            bool                    `json:"enabled"`
	LowHealthPercent   int                     `json:"lowHealthPercent"`
	BuffWarningSeconds int                     `json:"buffWarningSeconds"`
	SoundEnabled       bool                    `json:"soundEnabled"`
	Volume             int                     `json:"volume"`
	HealthSound        healerSound             `json:"healthSound"`
	MusicSound         healerSound             `json:"musicSound"`
	BuffSound          healerSound             `json:"buffSound"`
	Text               healerTextSettings      `json:"text"`
	Buffs              []healerBuffRule        `json:"buffs"`
	OvertureRule       *healerBuffRule         `json:"overtureRule,omitempty"`
	VivaceRule         *healerBuffRule         `json:"vivaceRule,omitempty"`
	Members            []healerMemberSelection `json:"members"`
}

type healerBuffState struct {
	State            string `json:"state"`
	RemainingSeconds *int64 `json:"remainingSeconds"`
}

type healerMemberState struct {
	Key           string                     `json:"key"`
	WaitingReason string                     `json:"waitingReason,omitempty"`
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Active        bool                       `json:"active"`
	HealthState   string                     `json:"healthState"`
	HealthPercent *float64                   `json:"healthPercent"`
	Overture      healerBuffState            `json:"overture"`
	Vivace        healerBuffState            `json:"vivace"`
	Buffs         map[uint32]healerBuffState `json:"buffs"`
}

type healerAlert struct {
	Key                   string `json:"key"`
	Name                  string `json:"name"`
	Message               string `json:"message"`
	Category              string `json:"category"`
	CCID                  uint32 `json:"ccId"`
	Title                 string `json:"title"`
	Value                 string `json:"value"`
	Overlay               bool   `json:"overlay"`
	Flash                 bool   `json:"flash"`
	sound                 healerSound
	soundDue              bool
	repeatCount           int
	repeatIntervalSeconds int
	cycleAt               int64
}

type healerCard struct {
	MemberKey string `json:"memberKey"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	State     string `json:"state"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	Value     string `json:"value"`
	CCID      uint32 `json:"ccId"`
	Category  string `json:"category"`
	Flash     bool   `json:"flash"`
}

type healerMonitorState struct {
	Cards     []healerCard        `json:"cards"`
	Settings  healerSettings      `json:"settings"`
	Members   []healerMemberState `json:"members"`
	Alerts    []healerAlert       `json:"alerts"`
	Capturing bool                `json:"capturing"`
	UpdatedAt int64               `json:"updatedAt"`
}

type healerObservation struct {
	healthAtMs   int64
	maximumKnown bool
	visible      bool
	buffs        map[uint32]bool
}

type healerAlertLatch struct {
	sinceMs        int64
	announcedCount int
	lastSoundMs    int64
	cycleAt        int64
}

type healerMonitor struct {
	mu                    sync.Mutex // settings and published snapshots are also read by HTTP
	settings              healerSettings
	state                 healerMonitorState
	observed              map[string]*healerObservation
	alerts                map[string]healerAlertLatch
	lastSoundMs           int64
	lastEventMs           int64
	lastTickMs            int64
	settingsPath          string
	previewUntilMs        int64
	previewText           healerTextSettings
	previewCards          []healerCard
	previewIconSize       int
	previewOpacityPercent int
}

func defaultHealerSettings() healerSettings {
	return healerSettings{Version: 2, IconSize: 30, OpacityPercent: 100, Templates: []healerMemberTemplate{}, LowHealthPercent: 40, BuffWarningSeconds: 10, SoundEnabled: true, Volume: 70, Members: []healerMemberSelection{}, Buffs: []healerBuffRule{},
		HealthSound: healerSound{Kind: "healer-health"}, MusicSound: healerSound{Kind: "healer-music"}, BuffSound: healerSound{Kind: "healer-buff"},
		Text: healerTextSettings{Enabled: true, FontSize: 16, X: 600, Y: 180, Width: 660}}
}

func normalizeHealerSound(sound healerSound, fallback string) healerSound {
	if sound.Kind == "healer-angel" {
		sound.Kind = "healer-health"
	}
	switch sound.Kind {
	case "none", "electronic", "voice", "skill-ready", "healer-health", "healer-music", "healer-buff", "custom":
	default:
		sound.Kind = fallback
	}
	if sound.Kind != "custom" {
		return healerSound{Kind: sound.Kind}
	}
	sound.SoundID = strings.TrimSpace(sound.SoundID)
	if len(sound.SoundID) > 128 {
		sound.SoundID = ""
	}
	sound.Name = string([]rune(sound.Name)[:min(len([]rune(sound.Name)), 100)])
	return sound
}

func normalizeHealerText(s healerTextSettings) healerTextSettings {
	s.FontSize = clampNativeReminderInt(s.FontSize, 12, 72, 24)
	s.Width = clampNativeReminderInt(s.Width, 280, 1600, 660)
	s.X = clampNativeReminderInt(s.X, -32000, 32000, 600)
	s.Y = clampNativeReminderInt(s.Y, -32000, 32000, 180)
	return s
}

func normalizeHealerBuffRule(rule healerBuffRule, fallback healerSound) healerBuffRule {
	if rule.OverlayEnabled == nil {
		value := true
		rule.OverlayEnabled = &value
	}
	if rule.FlashEnabled == nil {
		value := true
		rule.FlashEnabled = &value
	}
	if rule.FlashSeconds == nil {
		value := rule.WarningSeconds
		rule.FlashSeconds = &value
	}
	value := clampNativeReminderInt(*rule.FlashSeconds, 0, 60, 10)
	rule.FlashSeconds = &value
	rule.WarningSeconds = clampNativeReminderInt(rule.WarningSeconds, 0, 60, 10)
	rule.RepeatCount = clampNativeReminderInt(rule.RepeatCount, 1, 10, 1)
	rule.RepeatIntervalSeconds = clampNativeReminderInt(rule.RepeatIntervalSeconds, 2, 300, 5)
	if rule.DurationMode != "manual" {
		rule.DurationMode = "auto"
	}
	rule.ManualDurationSeconds = clampNativeReminderInt(rule.ManualDurationSeconds, 1, 86400, 60)
	if rule.Sound == nil {
		rule.Sound = &fallback
	}
	sound := normalizeHealerSound(*rule.Sound, fallback.Kind)
	rule.Sound = &sound
	return rule
}

func healerMusicRule(configured *healerBuffRule, ccID uint32, name string, settings healerSettings) healerBuffRule {
	rule := healerBuffRule{WarningSeconds: settings.BuffWarningSeconds}
	if configured != nil {
		rule = *configured
	}
	rule.CCID, rule.Name = ccID, name
	return normalizeHealerBuffRule(rule, settings.MusicSound)
}

func normalizeHealerSettings(s healerSettings) healerSettings {
	s.Version = 2
	s.IconSize = clampNativeReminderInt(s.IconSize, 16, 80, 30)
	s.OpacityPercent = clampNativeReminderInt(s.OpacityPercent, 20, 100, 100)
	s.LowHealthPercent = clampNativeReminderInt(s.LowHealthPercent, 5, 95, 40)
	s.BuffWarningSeconds = clampNativeReminderInt(s.BuffWarningSeconds, 0, 60, 10)
	s.Volume = clampNativeReminderInt(s.Volume, 0, 100, 70)
	s.HealthSound = normalizeHealerSound(s.HealthSound, "healer-health")
	s.MusicSound = normalizeHealerSound(s.MusicSound, "healer-music")
	if s.OvertureRule == nil && s.VivaceRule == nil && s.BuffSound.Kind == "electronic" {
		s.BuffSound.Kind = "healer-buff"
	}
	s.BuffSound = normalizeHealerSound(s.BuffSound, "healer-buff")
	overture := healerMusicRule(s.OvertureRule, 680, "战争序曲", s)
	vivace := healerMusicRule(s.VivaceRule, 192, "活跃进行曲", s)
	s.OvertureRule, s.VivaceRule = &overture, &vivace
	s.Text = normalizeHealerText(s.Text)
	rules := make([]healerBuffRule, 0)
	allowed := map[uint32]bool{}
	for _, rule := range s.Buffs {
		if allowed[rule.CCID] || rule.CCID == 680 || rule.CCID == 192 {
			continue
		}
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			rule.Name = fmt.Sprintf("Buff %d", rule.CCID)
		}
		rule.Name = string([]rune(rule.Name)[:min(len([]rune(rule.Name)), 48)])
		rule.WarningSeconds = clampNativeReminderInt(rule.WarningSeconds, 0, 60, 10)
		rule = normalizeHealerBuffRule(rule, s.BuffSound)
		allowed[rule.CCID] = true
		rules = append(rules, rule)
		if len(rules) == 16 {
			break
		}
	}
	s.Buffs = rules
	members := make([]healerMemberSelection, 0, min(len(s.Members), 32))
	seen := map[string]bool{}
	for _, member := range s.Members {
		member.ID = strings.TrimSpace(member.ID)
		buffs := make([]uint32, 0)
		for _, id := range member.Buffs {
			if allowed[id] && !slices.Contains(buffs, id) {
				buffs = append(buffs, id)
			}
		}
		member.Buffs = buffs
		member = normalizeHealerMember(member, s, len(members))
		if (member.ID == "" && member.Name == "") || len(member.ID) > 128 || seen[member.Key] || !(member.Included || member.Favorite || member.Health || member.Overture || member.Vivace || len(buffs) > 0 || member.BuffSettings.Enabled) {
			continue
		}
		seen[member.Key] = true
		member.Included = true
		members = append(members, member)
		if len(members) == 32 {
			break
		}
	}
	s.Members = members
	s.Templates = normalizeHealerTemplates(s.Templates, s)
	return s
}

func newHealerMonitor(path string) *healerMonitor {
	s := defaultHealerSettings()
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &s) != nil {
			s = defaultHealerSettings()
		}
	}
	// Restore only named favorites; reconnect using fresh appearance data.
	s.Members = healerFavoriteMembers(s.Members)
	return &healerMonitor{settings: normalizeHealerSettings(s), observed: map[string]*healerObservation{}, alerts: map[string]healerAlertLatch{}, settingsPath: path,
		state: healerMonitorState{Members: []healerMemberState{}, Alerts: []healerAlert{}},
	}
}

func (h *healerMonitor) observation(id string) *healerObservation {
	if h.observed[id] == nil {
		h.observed[id] = &healerObservation{buffs: map[uint32]bool{}}
	}
	return h.observed[id]
}

func (h *healerMonitor) onEvent(current event.IEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if base, ok := current.(interface{ GetEventBase() *event.EventBase }); ok {
		h.lastEventMs = max(h.lastEventMs, base.GetEventBase().At*1000)
	}
	switch v := current.(type) {
	case *event.EventLocalEntity:
		if v.Reset {
			clear(h.observed)
			clear(h.alerts)
			h.settings.Members = healerFavoriteMembers(h.settings.Members)
		}
	case *event.EventEntityAppear:
		h.observation(v.Id).visible = true
	case *event.EventEntityDisappear:
		id := v.Id
		delete(h.observed, id)
		for key := range h.alerts {
			if strings.HasPrefix(key, id+":") {
				delete(h.alerts, key)
			}
		}
	case *event.EventFinish:
		// Death is not loss of visibility. Keep Buff observations and quotas:
		// removal packets can precede/follow duplicate death notifications.
		// Fresh HP must still confirm revival before health alerts resume.
		if observed := h.observed[v.Id]; observed != nil {
			observed.healthAtMs = 0
		}
		delete(h.alerts, v.Id+":health")
	case *event.EventStatUpdate:
		for _, stat := range v.Stats {
			if math.IsNaN(stat.Value) || math.IsInf(stat.Value, 0) {
				continue
			}
			if stat.StatId == 28 {
				h.observation(v.Id).healthAtMs = v.At * 1000
			}
			if stat.StatId == 30 {
				h.observation(v.Id).maximumKnown = stat.Value > 0
			}
		}
	case *event.EventCharacterConditionEnable:
		h.observation(v.Id).buffs[v.CCId] = true
	case *event.EventCharacterConditionDisable:
		h.observation(v.Id).buffs[v.CCId] = true
	}
}

func healerBuff(entity *nativeReminderEntity, observed *healerObservation, rule healerBuffRule, nowMs int64) healerBuffState {
	ccID, warning := rule.CCID, max(rule.WarningSeconds, *rule.FlashSeconds)
	state := healerBuffState{State: "unknown"}
	if observed == nil || !observed.buffs[ccID] {
		return state
	}
	condition, ok := entity.Conditions[ccID]
	if !ok {
		state.State = "missing"
		return state
	}
	expires := nativeBuffExpiresAtMs(condition, nativeBuffRule{DurationMode: rule.DurationMode, ManualDurationSeconds: rule.ManualDurationSeconds}, 0)
	state.State = "active"
	if expires > 0 {
		remaining := max(int64(0), (expires-nowMs+999)/1000)
		state.RemainingSeconds = &remaining
		if expires <= nowMs {
			state.State = "missing"
		} else if expires-nowMs <= int64(warning)*1000 {
			state.State = "expiring"
		}
	}
	return state
}

func healerBuffAlertKey(id string, ccID uint32) string {
	switch ccID {
	case 680:
		return id + ":overture"
	case 192:
		return id + ":vivace"
	default:
		return fmt.Sprintf("%s:buff:%d", id, ccID)
	}
}

func (h *healerMonitor) evaluate(runtime *nativeReminderRuntime, now time.Time, capturing bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	nowMs := now.UnixMilli()
	if nowMs-h.lastTickMs < 250 {
		return
	}
	h.lastTickMs = nowMs
	capturing = capturing && h.lastEventMs > 0 && nowMs-h.lastEventMs <= 30_000
	state := healerMonitorState{Capturing: capturing, UpdatedAt: nowMs, Members: []healerMemberState{}, Alerts: []healerAlert{}}
	h.resolveMembers(runtime)
	type entry struct {
		id      string
		choice  healerMemberSelection
		watched bool
	}
	entries := []entry{}
	selected := map[string]bool{}
	for _, choice := range h.settings.Members {
		entries = append(entries, entry{id: choice.ID, choice: choice, watched: true})
		if choice.ID != "" {
			selected[choice.ID] = true
		}
	}
	candidates := []string{}
	for id, entity := range runtime.entities {
		observed := h.observed[id]
		if !selected[id] && id != runtime.localID && entity.Known && battleRecordPCRace(entity.RaceID) && entity.OwnerID == "" && observed != nil && observed.visible && healerEntityActive(entity, observed) {
			candidates = append(candidates, id)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return runtime.entities[candidates[i]].Name < runtime.entities[candidates[j]].Name
	})
	for _, id := range candidates {
		entries = append(entries, entry{id: id, choice: healerMemberSelection{ID: id, Name: runtime.entities[id].Name}})
		if len(entries) >= 96 {
			break
		}
	}
	activeAlerts := map[string]bool{}
	addAlert := func(alert healerAlert, delay int64) bool {
		key := alert.Key
		activeAlerts[key] = true
		latch, exists := h.alerts[key]
		// A renewed Buff begins a new episode, even when it stays within the
		// warning window between two evaluations. Expiry/removal is the same episode.
		if !exists || alert.cycleAt > 0 && latch.cycleAt > 0 && alert.cycleAt > latch.cycleAt {
			latch = healerAlertLatch{sinceMs: nowMs}
		}
		if alert.cycleAt > 0 {
			latch.cycleAt = alert.cycleAt
		}
		if nowMs-latch.sinceMs >= delay {
			state.Alerts = append(state.Alerts, alert)
		}
		h.alerts[key] = latch
		return nowMs-latch.sinceMs >= delay
	}
	for index, entry := range entries {
		id := entry.id
		choice := normalizeHealerMember(entry.choice, h.settings, index)
		health, buffSettings := choice.HealthSettings, choice.BuffSettings
		rules := buffSettings.Rules
		entity := runtime.entities[id]
		observation := h.observed[id]
		member := healerMemberState{Key: choice.Key, ID: id, Name: choice.Name, HealthState: "unknown", Overture: healerBuffState{State: "unknown"}, Vivace: healerBuffState{State: "unknown"}, Buffs: map[uint32]healerBuffState{}}
		available := entity != nil && observation != nil && observation.visible && capturing
		member.Active = available && healerEntityActive(entity, observation)
		if member.Name == "" && entity != nil {
			member.Name = entity.Name
		}
		for _, rule := range rules {
			member.Buffs[rule.CCID] = healerBuffState{State: "unknown"}
		}
		if !member.Active {
			member.HealthState = "unavailable"
			if !capturing {
				member.WaitingReason = "等待实时数据；开启软件后请让队友切换一次地图"
			} else if available {
				member.WaitingReason = "队友已倒下，等待复活；继续监测 Buff"
			} else {
				member.WaitingReason = "等待识别／不在视野；请让队友切换地图"
			}
		} else {
			if observation.healthAtMs > 0 && observation.maximumKnown && entity.MaximumHealth > 0 && !math.IsInf(entity.MaximumHealth, 0) && !math.IsNaN(entity.CurrentHealth) && !math.IsInf(entity.CurrentHealth, 0) {
				if nowMs-observation.healthAtMs > 30_000 {
					member.HealthState = "stale"
				} else {
					percent := math.Max(0, math.Min(100, entity.CurrentHealth/entity.MaximumHealth*100))
					member.HealthPercent, member.HealthState = &percent, "normal"
					_, wasLow := h.alerts[id+":health"]
					if percent <= float64(health.Threshold) || wasLow && percent < float64(health.Threshold+5) {
						member.HealthState = "low"
					}
				}
			}
		}
		// A visible fallen teammate can lose music before its countdown ends.
		// Evaluate confirmed Buff changes independently of living/health state.
		if available {
			for _, rule := range rules {
				buff := healerBuff(entity, observation, rule, nowMs)
				member.Buffs[rule.CCID] = buff
				if rule.CCID == 680 {
					member.Overture = buff
				}
				if rule.CCID == 192 {
					member.Vivace = buff
				}
			}
		}
		state.Members = append(state.Members, member)
		if !entry.watched || !h.settings.Enabled {
			continue
		}
		// Missing fresh data pauses output, not the reminder episode. Otherwise
		// unrelated traffic after an idle gap would replenish an exhausted quota.
		if choice.Health && (member.HealthState == "unavailable" || member.HealthState == "stale" || member.HealthState == "unknown") {
			activeAlerts[id+":health"] = true
		}
		if !available {
			if buffSettings.Enabled {
				for _, rule := range rules {
					activeAlerts[healerBuffAlertKey(id, rule.CCID)] = true
				}
			}
			continue
		}
		if choice.Health && member.HealthState == "low" {
			message := fmt.Sprintf("血量≤%d%%（当前 %.0f%%）", health.Threshold, *member.HealthPercent)
			if *member.HealthPercent > float64(health.Threshold) {
				message = fmt.Sprintf("血量 %.0f%%，尚未恢复安全范围", *member.HealthPercent)
			}
			sound := health.Sound
			if !health.SoundEnabled {
				sound = healerSound{Kind: "none"}
			}
			alert := healerAlert{Key: id + ":health", Name: member.Name, Message: message, Category: "health", Overlay: health.Overlay.Enabled, Flash: true, Title: fmt.Sprintf("血量≤%d%%", health.Threshold), Value: fmt.Sprintf("%.0f%%", *member.HealthPercent), sound: sound, soundDue: true, repeatCount: health.RepeatCount, repeatIntervalSeconds: health.RepeatIntervalSeconds}
			if addAlert(alert, 500) && health.Overlay.Enabled {
				state.Cards = append(state.Cards, healerCard{Key: alert.Key, MemberKey: choice.Key, Name: member.Name, Title: alert.Title, Value: alert.Value, Category: "health", State: "low", Flash: true, X: health.Overlay.X, Y: health.Overlay.Y})
			}
		}
		if !buffSettings.Enabled {
			continue
		}
		for _, rule := range rules {
			buff, category, key := member.Buffs[rule.CCID], "buff", healerBuffAlertKey(id, rule.CCID)
			if rule.CCID == 680 || rule.CCID == 192 {
				category = "music"
			}
			if buff.State == "unknown" {
				activeAlerts[key] = true
				continue
			}
			soundDue := buff.State == "missing" || buff.RemainingSeconds != nil && *buff.RemainingSeconds <= int64(rule.WarningSeconds)
			visualDue := buff.State == "missing" || buff.RemainingSeconds != nil && *buff.RemainingSeconds <= int64(*rule.FlashSeconds)
			value := "生效"
			if buff.RemainingSeconds != nil {
				value = fmt.Sprintf("%ds", *buff.RemainingSeconds)
			}
			if buff.State == "missing" {
				value = "补充"
			}
			matured := true
			if soundDue || visualDue {
				message := rule.Name + "即将结束"
				if buff.State == "missing" {
					message = rule.Name + "已结束，需要补充"
				}
				sound := *rule.Sound
				if !buffSettings.SoundEnabled {
					sound = healerSound{Kind: "none"}
				}
				cycleAt := entity.Conditions[rule.CCID].At
				matured = addAlert(healerAlert{Key: key, Name: member.Name, Message: message, Category: category, CCID: rule.CCID, Title: rule.Name, Value: value, Overlay: buffSettings.Overlay.Enabled && *rule.OverlayEnabled, Flash: *rule.FlashEnabled && visualDue, sound: sound, soundDue: soundDue, repeatCount: rule.RepeatCount, repeatIntervalSeconds: rule.RepeatIntervalSeconds, cycleAt: cycleAt}, 1200)
			}
			if buffSettings.Overlay.Enabled && *rule.OverlayEnabled && (buff.State != "missing" || matured) {
				state.Cards = append(state.Cards, healerCard{Key: key, MemberKey: choice.Key, Name: member.Name, Title: rule.Name, Value: value, CCID: rule.CCID, Category: category, State: buff.State, Flash: *rule.FlashEnabled && visualDue && matured, X: buffSettings.Overlay.X, Y: buffSettings.Overlay.Y})
			}
		}
	}
	for key := range h.alerts {
		if !activeAlerts[key] {
			delete(h.alerts, key)
		}
	}
	// Count only accepted sounds. First warnings precede repeats; the shared
	// audio gap may delay a reminder but must not consume its per-rule quota.
	due := func(alert healerAlert) bool {
		latch := h.alerts[alert.Key]
		return alert.soundDue && h.settings.SoundEnabled && h.settings.Volume > 0 && alert.sound.Kind != "none" &&
			latch.announcedCount < alert.repeatCount &&
			(latch.announcedCount == 0 || nowMs-latch.lastSoundMs >= int64(alert.repeatIntervalSeconds)*1000)
	}
	for pass := 0; pass < 2; pass++ {
		for _, category := range []string{"health", "music", "buff"} {
			for _, alert := range state.Alerts {
				first := h.alerts[alert.Key].announcedCount == 0
				if alert.Category != category || first != (pass == 0) || !due(alert) || nowMs-h.lastSoundMs < 2000 {
					continue
				}
				if runtime.playSound == nil || !runtime.playSound(nativeReminderSoundRequest{Kind: alert.sound.Kind, SoundID: alert.sound.SoundID, Volume: h.settings.Volume}) {
					continue
				}
				h.lastSoundMs = nowMs
				// One identical clip can cover several currently due warnings, but
				// never advance a muted rule, an exhausted count or a pending interval.
				for _, other := range state.Alerts {
					if due(other) && other.sound.Kind == alert.sound.Kind && other.sound.SoundID == alert.sound.SoundID {
						latch := h.alerts[other.Key]
						latch.announcedCount++
						latch.lastSoundMs = nowMs
						h.alerts[other.Key] = latch
					}
				}
			}
		}
	}
	h.state = state
}

func healerEntityActive(entity *nativeReminderEntity, observed *healerObservation) bool {
	return entity.Active || observed != nil && observed.visible && observed.healthAtMs > 0 && entity.CurrentHealth > 0
}

func handleHealerMonitor(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "local access only", http.StatusForbidden)
		return
	}
	nativeReminderRuntimeHolder.RLock()
	runtime := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.RUnlock()
	if runtime == nil || runtime.healer == nil {
		http.Error(w, "healer monitor unavailable", http.StatusServiceUnavailable)
		return
	}
	h := runtime.healer
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		h.mu.Lock()
		defer h.mu.Unlock()
		state := h.state
		state.Settings = h.settings
		_ = json.NewEncoder(w).Encode(state)
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		settings := defaultHealerSettings()
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&settings); err != nil {
			http.Error(w, "invalid healer settings", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			http.Error(w, "invalid trailing data", http.StatusBadRequest)
			return
		}
		settings = normalizeHealerSettings(settings)
		sounds := []healerSound{}
		for _, member := range settings.Members {
			sounds = append(sounds, member.HealthSettings.Sound)
			for _, rule := range member.BuffSettings.Rules {
				sounds = append(sounds, *rule.Sound)
			}
		}
		for _, template := range settings.Templates {
			sounds = append(sounds, template.HealthSettings.Sound)
			for _, rule := range template.BuffSettings.Rules {
				sounds = append(sounds, *rule.Sound)
			}
		}
		for _, sound := range sounds {
			if sound.Kind == "custom" {
				if _, err := resolveCustomAudio(sound.SoundID); err != nil {
					http.Error(w, "自定义音效不可用，请重新选择文件", http.StatusBadRequest)
					return
				}
			}
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		for index := range settings.Members {
			member := &settings.Members[index]
			if member.Name == "" {
				for _, current := range h.state.Members {
					if current.ID == member.ID && current.Name != "" {
						member.Name = current.Name
						break
					}
				}
			}
			member.Key = healerMemberKey(*member)
			if member.Favorite && member.Name == "" {
				http.Error(w, "请等待队友被识别后再保存为常用", http.StatusBadRequest)
				return
			}
		}
		stored := settings
		stored.Members = healerFavoriteMembers(settings.Members)
		data, err := json.MarshalIndent(stored, "", "  ")
		if err == nil {
			err = os.WriteFile(h.settingsPath, data, 0600)
		}
		if err != nil {
			http.Error(w, "无法保存圣歌监测设置", http.StatusInternalServerError)
			return
		}
		h.settings = settings
		h.lastTickMs = 0
		_ = json.NewEncoder(w).Encode(settings)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func healerSettingsPath() string { return filepath.Join(appDataDir(), "healer-monitor.json") }
