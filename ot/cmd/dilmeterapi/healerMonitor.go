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

// Favorites are matched by exact character name after a fresh appearance.
// Automatic selections are scoped to the observed instance.
type healerMemberSelection struct {
	automatic      bool
	AppliedArcana  uint16                     `json:"appliedArcana,omitempty"`
	Self           bool                       `json:"self,omitempty"`
	Key            string                     `json:"key"`
	Name           string                     `json:"name"`
	Favorite       bool                       `json:"favorite"`
	Mana           bool                       `json:"mana,omitempty"`
	ManaSettings   *healerHealthSettings      `json:"manaSettings,omitempty"`
	HealthSettings *healerHealthSettings      `json:"healthSettings,omitempty"`
	SkillSettings  *healerMemberSkillSettings `json:"skillSettings"`
	BuffSettings   *healerMemberBuffSettings  `json:"buffSettings,omitempty"`
	ID             string                     `json:"id"`
	Included       bool                       `json:"included"`
	Buffs          []uint32                   `json:"buffs"`
	Health         bool                       `json:"health"`
	Overture       bool                       `json:"overture"`
	Vivace         bool                       `json:"vivace"`
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
	DeathLoss             *healerDeathLossSettings `json:"deathLoss,omitempty"`
	RepeatCount           int                      `json:"repeatCount"`
	RepeatIntervalSeconds int                      `json:"repeatIntervalSeconds"`
	CCID                  uint32                   `json:"ccId"`
	Name                  string                   `json:"name"`
	WarningSeconds        int                      `json:"warningSeconds"`
	OverlayEnabled        *bool                    `json:"overlayEnabled,omitempty"`
	FlashEnabled          *bool                    `json:"flashEnabled,omitempty"`
	FlashSeconds          *int                     `json:"flashSeconds,omitempty"`
	DurationMode          string                   `json:"durationMode"`
	ManualDurationSeconds int                      `json:"manualDurationSeconds"`
	Sound                 *healerSound             `json:"sound,omitempty"`
}

type healerDeathLossSettings struct {
	Enabled               bool        `json:"enabled"`
	Sound                 healerSound `json:"sound"`
	RepeatCount           int         `json:"repeatCount"`
	RepeatIntervalSeconds int         `json:"repeatIntervalSeconds"`
}

type healerTextSettings struct {
	ScalePercent int  `json:"scalePercent"`
	Enabled      bool `json:"enabled"`
	FontSize     int  `json:"fontSize"`
	X            int  `json:"x"`
	Y            int  `json:"y"`
	Width        int  `json:"width"`
}

// Templates hold reminder preferences only, never a bound character identity.
type healerMemberTemplate struct {
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	Health         bool                       `json:"health"`
	HealthSettings *healerHealthSettings      `json:"healthSettings"`
	SkillSettings  *healerMemberSkillSettings `json:"skillSettings"`
	BuffSettings   *healerMemberBuffSettings  `json:"buffSettings"`
}

type healerSettings struct {
	AutoAddInDungeon   bool                    `json:"autoAddInDungeon"`
	AutoAddTemplateID  string                  `json:"autoAddTemplateId"`
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
	LossReason       string `json:"lossReason,omitempty"`
	State            string `json:"state"`
	RemainingSeconds *int64 `json:"remainingSeconds"`
}

type healerMemberState struct {
	PartySlot     int    `json:"partySlot,omitempty"`
	Arcana        string `json:"arcana,omitempty"`
	Automatic     bool   `json:"automatic,omitempty"`
	CurtainAmount *int64 `json:"curtainAmount,omitempty"`
	showHealth    bool
	showMana      bool
	Key           string                     `json:"key"`
	WaitingReason string                     `json:"waitingReason,omitempty"`
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Active        bool                       `json:"active"`
	ManaState     string                     `json:"manaState,omitempty"`
	ManaPercent   *float64                   `json:"manaPercent,omitempty"`
	ManaCurrent   *float64                   `json:"manaCurrent,omitempty"`
	ManaMaximum   float64                    `json:"manaMaximum,omitempty"`
	HealthState   string                     `json:"healthState"`
	HealthPercent *float64                   `json:"healthPercent"`
	Overture      healerBuffState            `json:"overture"`
	Vivace        healerBuffState            `json:"vivace"`
	Skills        map[uint16]healerBuffState `json:"skills"`
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
	SkillID   uint16 `json:"skillId,omitempty"`
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
	DungeonSession   uint64                  `json:"dungeonSession"`
	TemporaryMembers []healerMemberSelection `json:"temporaryMembers,omitempty"`
	InDungeon        bool                    `json:"inDungeon"`
	Cards            []healerCard            `json:"cards"`
	Settings         healerSettings          `json:"settings"`
	Members          []healerMemberState     `json:"members"`
	Alerts           []healerAlert           `json:"alerts"`
	Capturing        bool                    `json:"capturing"`
	UpdatedAt        int64                   `json:"updatedAt"`
}

type healerObservation struct {
	appearedAt          int64
	regionID            uint32
	curtain             healerCurtain
	lastDeathMs         int64
	removals            map[uint32]healerBuffRemoval
	manaAtMs            int64
	manaCurrent         float64
	manaReportedMaximum float64
	manaBase            float64
	manaBonus           float64
	manaBaseKnown       bool
	manaBonusKnown      bool
	healthBonus         float64
	healthAtMs          int64
	maximumKnown        bool
	visible             bool
	buffs               map[uint32]bool
}

type healerBuffRemoval struct {
	atMs      int64
	condition nativeReminderCondition
	death     bool
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
	regionID              uint32
	inDungeon             bool
	localReliable         bool
	party                 event.EventPartyRoster
	dungeonSession        uint64
	instanceMembers       map[string]healerInstanceMember
	arcana                map[string]healerArcanaObservation
	settingsPath          string
	previewUntilMs        int64
	previewText           healerTextSettings
	previewCards          []healerCard
	previewIconSize       int
	previewOpacityPercent int
}

const (
	healerSettingsVersion = 4
	healerDefaultIconSize = 28
)

func defaultHealerSettings() healerSettings {
	return healerSettings{Version: healerSettingsVersion, IconSize: healerDefaultIconSize, OpacityPercent: 100, Templates: []healerMemberTemplate{}, LowHealthPercent: 40, BuffWarningSeconds: 10, SoundEnabled: true, Volume: 70, Members: []healerMemberSelection{}, Buffs: []healerBuffRule{},
		HealthSound: healerSound{Kind: "healer-health"}, MusicSound: healerSound{Kind: "healer-music"}, BuffSound: healerSound{Kind: "healer-buff"},
		Text: healerTextSettings{Enabled: true, FontSize: 16, ScalePercent: 100, X: 600, Y: 180, Width: 660}}
}

func normalizeHealerSound(sound healerSound, fallback string) healerSound {
	if sound.Kind == "healer-angel" {
		sound.Kind = "healer-health"
	}
	switch sound.Kind {
	case "none", "electronic", "voice", "skill-ready", "healer-health", "healer-music", "healer-buff", "healer-death", "custom":
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
	s.ScalePercent = clampNativeReminderInt(s.ScalePercent, 50, 200, 100)
	s.FontSize = clampNativeReminderInt(s.FontSize, 12, 72, 24)
	s.Width = clampNativeReminderInt(s.Width, 280, 1600, 660)
	s.X = clampNativeReminderInt(s.X, -32000, 32000, 600)
	s.Y = clampNativeReminderInt(s.Y, -32000, 32000, 180)
	return s
}

func normalizeHealerBuffRule(rule healerBuffRule, fallback healerSound) healerBuffRule {
	death := healerDeathLossSettings{Enabled: true, Sound: healerSound{Kind: "healer-death"}, RepeatCount: 1, RepeatIntervalSeconds: 5}
	if rule.DeathLoss != nil {
		death = *rule.DeathLoss
	}
	death.Sound = normalizeHealerSound(death.Sound, "healer-death")
	death.RepeatCount = clampNativeReminderInt(death.RepeatCount, 1, 10, 1)
	death.RepeatIntervalSeconds = clampNativeReminderInt(death.RepeatIntervalSeconds, 2, 300, 5)
	rule.DeathLoss = &death
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
	// Previous defaults were 30 px through v2 and 20 px in v3.
	if (s.Version <= 2 && s.IconSize == 30) || (s.Version == 3 && s.IconSize == 20) {
		s.IconSize = healerDefaultIconSize
	}
	s.Version = healerSettingsVersion
	s.IconSize = clampNativeReminderInt(s.IconSize, 16, 80, healerDefaultIconSize)
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
		if (member.ID == "" && member.Name == "") || len(member.ID) > 128 || seen[member.Key] || !(member.Included || member.Favorite || member.Health || member.Mana || member.Overture || member.Vivace || len(buffs) > 0 || member.BuffSettings.Enabled || member.SkillSettings.Enabled) {
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
	s.AutoAddTemplateID = strings.TrimSpace(s.AutoAddTemplateID)
	return s
}

func newHealerMonitor(path string) *healerMonitor {
	s := defaultHealerSettings()
	if data, err := os.ReadFile(path); err == nil {
		s.Version = 0 // Files predating the version field use the legacy defaults.
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
		h.observed[id] = &healerObservation{buffs: map[uint32]bool{}, removals: map[uint32]healerBuffRemoval{}}
	}
	return h.observed[id]
}

func (h *healerMonitor) onEvent(current event.IEvent, runtime *nativeReminderRuntime) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if base, ok := current.(interface{ GetEventBase() *event.EventBase }); ok {
		h.lastEventMs = max(h.lastEventMs, base.GetEventBase().At*1000)
	}
	switch v := current.(type) {
	case *event.EventLocalEntity:
		h.localReliable = v.Reliable && v.Id != ""
		if v.Reset || v.Id != runtime.localID {
			h.regionID, h.inDungeon = 0, false
			h.party = event.EventPartyRoster{}
			h.resetDungeonMembers()
			clear(h.arcana)
		}
		if v.Reset {
			clear(h.observed)
			clear(h.alerts)
			h.settings.Members = healerFavoriteMembers(h.settings.Members)
		}
	case *event.EventPartyRoster:
		if v.Id == runtime.localID && h.localReliable {
			old := h.party
			h.party = *v
			if len(old.Members) > 0 && (len(v.Members) == 0 || old.PartyID != "" && v.PartyID != "" && old.PartyID != v.PartyID) {
				h.resetDungeonMembers()
			} else {
				for _, member := range old.Members {
					if h.partyMember(member.ID).ID == "" {
						delete(h.instanceMembers, member.ID)
					}
				}
			}
		}
	case *event.EventLocalRegion:
		if v.Id == runtime.localID {
			if !v.Dungeon && len(h.party.Members) == 0 && (h.inDungeon || v.RegionID != h.regionID) {
				h.resetDungeonMembers()
			}
			if v.RegionID != h.regionID {
				for _, observed := range h.observed {
					// A destination's entity snapshots can precede the player's
					// warp confirmation within the same map-load sequence.
					if observed.regionID == v.RegionID && (h.regionID == 0 || observed.appearedAt >= v.At-2) {
						continue
					}
					observed.regionID = 0
					observed.curtain = healerCurtain{}
				}
			}
			h.regionID, h.inDungeon = v.RegionID, v.Dungeon
		}
	case *event.EventEntityAppear:
		observed := h.observation(v.Id)
		observed.visible = true
		observed.regionID = v.RegionID
		observed.appearedAt = v.At
	case *event.EventEntityDisappear:
		id := v.Id
		if id == runtime.localID {
			h.regionID, h.inDungeon = 0, false
		}
		previous := h.observed[id]
		delete(h.observed, id)
		if id == runtime.localID && previous != nil && previous.manaBaseKnown {
			// Keep the verified base when the same player moves between maps.
			// The current MP and equipment modifier need a fresh sample.
			observed := h.observation(id)
			observed.manaBase, observed.manaBaseKnown = previous.manaBase, true
		}
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
			observed.curtain = healerCurtain{}
			observed.healthAtMs = 0
			observed.manaAtMs = 0
			observed.lastDeathMs = v.At * 1000
			// The server may remove Buffs just before announcing death.
			for ccID, removal := range observed.removals {
				if removal.atMs <= observed.lastDeathMs && observed.lastDeathMs-removal.atMs <= 3000 {
					removal.death = true
					observed.removals[ccID] = removal
				}
			}
		}
		delete(h.alerts, v.Id+":health")
		delete(h.alerts, v.Id+":mana")
	case *event.EventStatUpdate:
		atMs := nativeEventAtMs(v.At, v.AtMs)
		observed := h.observation(v.Id)
		isLocal := v.Id != "" && v.Id == runtime.localID
		for _, stat := range v.Stats {
			if math.IsNaN(stat.Value) || math.IsInf(stat.Value, 0) {
				continue
			}
			if isLocal {
				observed.observeManaStat(stat, atMs)
			}
			if stat.StatId == 31 {
				observed.healthBonus = stat.Value
			}
			if stat.StatId == 28 {
				observed.healthAtMs = atMs
			}
			if stat.StatId == 30 {
				observed.maximumKnown = stat.Value > 0
			}
		}
		if isLocal {
			observed.observeManaMaximum(v.ManaMaximum)
		}
	case *event.EventCharacterConditionEnable:
		observed := h.observation(v.Id)
		observed.buffs[v.CCId] = true
		delete(observed.removals, v.CCId)
		if v.CCId == lifeCurtainConditionID {
			if curtain, ok := readHealerCurtain(v); ok && curtain.observedAt >= observed.curtain.observedAt {
				observed.curtain = curtain
			}
		}
	case *event.EventCharacterConditionDisable:
		observed := h.observation(v.Id)
		observed.buffs[v.CCId] = true
		if v.CCId == lifeCurtainConditionID {
			atMs := nativeEventAtMs(v.At, v.AtMs)
			if v.At < observed.curtain.observedAt/1000 || v.AtMs > 0 && atMs < observed.curtain.observedAt {
				break
			}
			observed.curtain = healerCurtain{observedAt: max(atMs, observed.curtain.observedAt)}
		}
		if entity := runtime.entities[v.Id]; entity != nil {
			if condition, exists := entity.Conditions[v.CCId]; exists {
				// Match the runtime's stale-removal guard after a real refresh.
				if refreshAt, guarded := entity.RefreshGuard[v.CCId]; guarded && refreshAt == condition.At && v.At >= refreshAt && v.At-refreshAt <= 1 {
					break
				}
				atMs := v.At * 1000
				observed.removals[v.CCId] = healerBuffRemoval{atMs: atMs, condition: condition, death: observed.lastDeathMs > 0 && atMs >= observed.lastDeathMs && atMs-observed.lastDeathMs <= 3000}
			}
		}
	}
	h.observeArcana(current, runtime)
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
		if removal, exists := observed.removals[ccID]; exists && removal.death {
			// Use the server's lifetime, not a user-supplied countdown. Events
			// have second precision, so expiry in the same second is not early loss.
			expires := nativeBuffExpiresAtMs(removal.condition, nativeBuffRule{DurationMode: "auto"}, 0)
			if expires == 0 || expires > removal.atMs+1000 {
				state.LossReason = "death"
			}
		}
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
	if nowMs-h.lastTickMs < 100 {
		return
	}
	h.lastTickMs = nowMs
	streamCapturing := capturing
	capturing = capturing && h.lastEventMs > 0 && nowMs-h.lastEventMs <= 30_000
	state := healerMonitorState{DungeonSession: h.dungeonSession, InDungeon: streamCapturing && h.inDungeon, Capturing: capturing, UpdatedAt: nowMs, Members: []healerMemberState{}, Alerts: []healerAlert{}}
	type entry struct {
		id      string
		choice  healerMemberSelection
		watched bool
	}
	entries := []entry{}
	selected := map[string]bool{}
	for _, choice := range h.selectedMembers(runtime, streamCapturing) {
		entries = append(entries, entry{id: choice.ID, choice: choice, watched: true})
		if choice.ID != "" {
			selected[choice.ID] = true
		}
	}
	local := runtime.entities[runtime.localID]
	selfPinned := streamCapturing && runtime.localID != "" && (h.partyMember(runtime.localID).ID != "" ||
		h.localReliable && local != nil && local.Known && local.Name != "" && local.OwnerID == "" && battleRecordPCRace(local.RaceID))
	if !selected[runtime.localID] && runtime.localID != "" {
		observed := h.observed[runtime.localID]
		hasCurtain := streamCapturing && local != nil && !local.Defeated && observed != nil && observed.visible && observed.curtain.remaining(nowMs) != nil
		if selfPinned || hasCurtain {
			entries = append(entries, entry{id: runtime.localID, choice: healerMemberSelection{ID: runtime.localID, Self: true}})
			selected[runtime.localID] = true
		}
	}
	if streamCapturing {
		for _, player := range h.party.Members {
			if !selected[player.ID] {
				entries = append(entries, entry{id: player.ID, choice: healerMemberSelection{ID: player.ID, Name: player.Name}})
				selected[player.ID] = true
			}
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
		if choice.automatic {
			if h.instanceMembers == nil {
				h.instanceMembers = make(map[string]healerInstanceMember)
			}
			if h.instanceMembers[id].name != choice.Name {
				h.instanceMembers[id] = healerInstanceMember{name: choice.Name}
			}
			state.TemporaryMembers = append(state.TemporaryMembers, choice)
		}
		health, buffSettings := choice.HealthSettings, choice.BuffSettings
		rules := buffSettings.Rules
		entity := runtime.entities[id]
		observation := h.observed[id]
		member := healerMemberState{Key: choice.Key, ID: id, Name: choice.Name, Automatic: choice.automatic, HealthState: "unknown", Overture: healerBuffState{State: "unknown"}, Vivace: healerBuffState{State: "unknown"}, Buffs: map[uint32]healerBuffState{}, Skills: map[uint16]healerBuffState{}}
		available := entity != nil && observation != nil && observation.visible && capturing
		skillAvailable := entity != nil && observation != nil && observation.visible && streamCapturing
		skillAlive := skillAvailable && !entity.Defeated && healerEntityActive(entity, observation)
		member.Active = available && healerEntityActive(entity, observation)
		player := event.PartyMember{}
		if streamCapturing {
			player = h.partyMember(id)
			member.PartySlot = player.Slot
			if player.Name != "" {
				member.Name = player.Name
			}
		}
		if member.PartySlot == 0 && (member.Name == "" || choice.Self) && entity != nil && entity.Name != "" {
			member.Name = entity.Name
		}
		pinned := member.PartySlot > 0 || choice.Self && selfPinned
		member.showHealth = pinned && (choice.Self || health.Overlay.Enabled)
		if job := h.memberArcana(id, member.Name); job != 0 {
			member.Arcana = healerArcanaPresets[job-1].name
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
			if observation.healthAtMs > 0 && observation.maximumKnown && entity.MaximumHealth+observation.healthBonus > 0 && !math.IsInf(entity.MaximumHealth, 0) && !math.IsNaN(entity.CurrentHealth) && !math.IsInf(entity.CurrentHealth, 0) {
				if nowMs-observation.healthAtMs > 30_000 {
					member.HealthState = "stale"
				} else {
					percent := math.Max(0, math.Min(100, entity.CurrentHealth/(entity.MaximumHealth+observation.healthBonus)*100))
					member.HealthPercent, member.HealthState = &percent, "normal"
					_, wasLow := h.alerts[id+":health"]
					if percent <= float64(health.Threshold) || wasLow && percent < float64(health.Threshold+5) {
						member.HealthState = "low"
					}
				}
			}
		}
		// The party packet provides an initial life percentage before full stats.
		// Low-resource alerts still depend on the actual recent stat sample.
		if member.HealthPercent == nil && player.HealthAtMs > 0 && nowMs-player.HealthAtMs <= 30_000 {
			percent := float64(player.HealthPercent)
			member.HealthPercent = &percent
		}
		if skillAlive && entry.watched {
			member.showHealth = member.showHealth || health.Overlay.Enabled && (choice.Health || member.HealthPercent != nil)
		}
		if choice.Self {
			mana := choice.ManaSettings
			member.showMana = (pinned || skillAlive) && entry.watched && choice.Mana && mana.Overlay.Enabled
			member.ManaState = "unknown"
			if !member.Active {
				member.WaitingReason = "等待自身角色数据；请登录或切换地图完成识别"
				member.ManaState = "unavailable"
			}
			if skillAlive {
				member.ManaState, member.ManaPercent = h.manaState(id, observation, mana, nowMs)
				if observation.manaAtMs > 0 {
					current := observation.manaCurrent
					member.ManaCurrent = &current
					member.ManaMaximum = observation.observedManaMaximum()
				}
			}
		}
		// A timed absorption pool may still be active while the game is idle
		// or before the first health/mana sample for this player.
		if streamCapturing && observation != nil && (observation.visible || pinned) &&
			(entry.watched || pinned || choice.Self) && (entity == nil || !entity.Defeated) {
			member.CurtainAmount = observation.curtain.remaining(nowMs)
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
		for _, rule := range choice.SkillSettings.Rules {
			skillState := healerBuffState{State: "unknown"}
			observed := runtime.partySkills[id][rule.SkillID]
			readyAt := partyReadyAt(observed, rule.CooldownSeconds)
			if skillAvailable && readyAt > 0 {
				remaining := int64(math.Ceil(float64(max(0, readyAt-nowMs)) / 1000))
				skillState.RemainingSeconds = &remaining
				skillState.State = "cooling"
				if remaining == 0 {
					skillState.State = "ready"
				}
			}
			member.Skills[rule.SkillID] = skillState
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
		if choice.Mana && (member.ManaState == "unavailable" || member.ManaState == "stale" || member.ManaState == "unknown") {
			activeAlerts[id+":mana"] = true
		}
		// A known cooldown can finish while the party stands idle. It depends on
		// capture/visibility and the observed use, not on fresh health/Buff traffic.
		if choice.SkillSettings.Enabled && skillAvailable {
			for _, rule := range choice.SkillSettings.Rules {
				status := member.Skills[rule.SkillID]
				key := fmt.Sprintf("%s:skill:%d", id, rule.SkillID)
				value := "未观测"
				if status.State == "cooling" {
					value = fmt.Sprintf("%ds", *status.RemainingSeconds)
				}
				if status.State == "ready" {
					value = "就绪"
				}
				if skillAlive && status.State == "ready" {
					sound := rule.Sound
					if !choice.SkillSettings.SoundEnabled {
						sound = healerSound{Kind: "none"}
					}
					addAlert(healerAlert{Key: key, Name: member.Name, Message: rule.Name + "冷却结束", Category: "skill", Title: rule.Name, Value: value, sound: sound, soundDue: true, repeatCount: 1, repeatIntervalSeconds: 5, cycleAt: runtime.partySkills[id][rule.SkillID].UsedAtMs}, 0)
				} else if !skillAlive {
					activeAlerts[key] = true
				}
				if choice.SkillSettings.Overlay.Enabled {
					state.Cards = append(state.Cards, healerCard{Key: key, MemberKey: choice.Key, Name: member.Name, Title: rule.Name, Value: value, Category: "skill", SkillID: rule.SkillID, State: status.State, X: choice.BuffSettings.Overlay.X, Y: choice.BuffSettings.Overlay.Y})
				}
			}
		}
		if !available {
			for _, rule := range choice.SkillSettings.Rules {
				activeAlerts[fmt.Sprintf("%s:skill:%d", id, rule.SkillID)] = true
			}
			if buffSettings.Enabled {
				for _, rule := range rules {
					activeAlerts[healerBuffAlertKey(id, rule.CCID)] = true
					activeAlerts[healerBuffAlertKey(id, rule.CCID)+":death"] = true
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

		if choice.Mana && member.ManaState == "low" {
			mana := choice.ManaSettings
			sound := mana.Sound
			if !mana.SoundEnabled {
				sound = healerSound{Kind: "none"}
			}
			alert := healerAlert{Key: id + ":mana", Name: member.Name, Message: fmt.Sprintf("魔法值≤%d%%（当前 %.0f%%）", mana.Threshold, *member.ManaPercent), Category: "mana", Overlay: mana.Overlay.Enabled, Flash: true, Title: fmt.Sprintf("魔法值≤%d%%", mana.Threshold), Value: healerManaPercentText(*member.ManaPercent), sound: sound, soundDue: true, repeatCount: mana.RepeatCount, repeatIntervalSeconds: mana.RepeatIntervalSeconds}
			if addAlert(alert, 500) && mana.Overlay.Enabled {
				state.Cards = append(state.Cards, healerCard{Key: alert.Key, MemberKey: choice.Key, Name: member.Name, Title: alert.Title, Value: alert.Value, Category: "mana", State: "low", Flash: true, X: mana.Overlay.X, Y: mana.Overlay.Y})
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
				activeAlerts[key+":death"] = true
				// Keep configured slots visible as soon as the teammate appears.
				// An unknown observation has its own badge and does not consume a reminder episode.
				if buffSettings.Overlay.Enabled && *rule.OverlayEnabled {
					state.Cards = append(state.Cards, healerCard{Key: key, MemberKey: choice.Key, Name: member.Name, Title: rule.Name, Value: "未观测", CCID: rule.CCID, Category: category, State: "unknown", Flash: *rule.FlashEnabled, X: buffSettings.Overlay.X, Y: buffSettings.Overlay.Y})
				}
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
			deathLoss := buff.LossReason == "death"
			// Briefly wait for a following death packet before choosing the
			// normal removal voice. The icon can still update immediately.
			if removal, exists := observation.removals[rule.CCID]; !deathLoss && buff.State == "missing" && exists && nowMs-removal.atMs <= 3000 {
				soundDue = false
			}
			if deathLoss {
				activeAlerts[key] = true // Preserve the normal episode's independent quota.
				key += ":death"
				value = "死亡丢失"
			}
			if soundDue || visualDue {
				message := rule.Name + "即将结束"
				if buff.State == "missing" {
					message = rule.Name + "已结束，需要补充"
				}
				sound := *rule.Sound
				repeatCount, repeatInterval := rule.RepeatCount, rule.RepeatIntervalSeconds
				cycleAt := entity.Conditions[rule.CCID].At
				if deathLoss {
					message = rule.Name + "因死亡消失，需要补充"
					sound = rule.DeathLoss.Sound
					if !rule.DeathLoss.Enabled {
						sound = healerSound{Kind: "none"}
					}
					repeatCount, repeatInterval = rule.DeathLoss.RepeatCount, rule.DeathLoss.RepeatIntervalSeconds
					cycleAt = observation.removals[rule.CCID].atMs
				}
				if !buffSettings.SoundEnabled {
					sound = healerSound{Kind: "none"}
				}
				matured = addAlert(healerAlert{Key: key, Name: member.Name, Message: message, Category: category, CCID: rule.CCID, Title: rule.Name, Value: value, Overlay: buffSettings.Overlay.Enabled && *rule.OverlayEnabled, Flash: *rule.FlashEnabled && visualDue, sound: sound, soundDue: soundDue, repeatCount: repeatCount, repeatIntervalSeconds: repeatInterval, cycleAt: cycleAt}, 1200)
			}
			if buffSettings.Overlay.Enabled && *rule.OverlayEnabled {
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
		for _, category := range []string{"health", "mana", "music", "buff", "skill"} {
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
		if state.DungeonSession != h.dungeonSession || !h.settings.AutoAddInDungeon || !h.inDungeon && len(h.party.Members) == 0 {
			state.TemporaryMembers = nil
		}
		state.DungeonSession = h.dungeonSession
		state.InDungeon = state.InDungeon && h.inDungeon
		_ = json.NewEncoder(w).Encode(state)
	case http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		request := struct {
			healerSettings
			DungeonSession   uint64                  `json:"dungeonSession"`
			TemporaryMembers []healerMemberSelection `json:"temporaryMembers"`
		}{healerSettings: defaultHealerSettings()}
		request.Version = 0
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "invalid healer settings", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			http.Error(w, "invalid trailing data", http.StatusBadRequest)
			return
		}
		settings := normalizeHealerSettings(request.healerSettings)
		if settings.AutoAddInDungeon && settings.automaticTemplate() == nil {
			http.Error(w, "请为自动添加的队友选择一个通用模板", http.StatusBadRequest)
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		overrides := make(map[string]healerInstanceMember)
		sounds := []healerSound{}
		if request.DungeonSession == h.dungeonSession && h.settings.AutoAddInDungeon && settings.AutoAddInDungeon {
			for _, member := range request.TemporaryMembers[:min(32, len(request.TemporaryMembers))] {
				known := h.instanceMembers[member.ID]
				if known.name == "" || known.name != member.Name {
					continue
				}
				member.automatic, member.Included, member.Self, member.Favorite = true, true, false, false
				member = normalizeHealerMember(member, settings, len(settings.Members))
				known.override = &member
				overrides[member.ID] = known
				sounds = appendHealerSounds(sounds, member.HealthSettings, nil, member.BuffSettings, member.SkillSettings)
			}
		}
		for _, member := range settings.Members {
			sounds = appendHealerSounds(sounds, member.HealthSettings, member.ManaSettings, member.BuffSettings, member.SkillSettings)
		}
		for _, template := range settings.Templates {
			sounds = appendHealerSounds(sounds, template.HealthSettings, nil, template.BuffSettings, template.SkillSettings)
		}
		for _, sound := range sounds {
			if sound.Kind == "custom" {
				if _, err := resolveCustomAudio(sound.SoundID); err != nil {
					http.Error(w, "自定义音效不可用，请重新选择文件", http.StatusBadRequest)
					return
				}
			}
		}
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
		for id, member := range overrides {
			h.instanceMembers[id] = member
		}
		h.lastTickMs = 0
		if len(request.TemporaryMembers) > 0 && request.DungeonSession != h.dungeonSession {
			w.Header().Set("X-Dilmeter-Dungeon-Changed", "1")
		}
		_ = json.NewEncoder(w).Encode(settings)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func appendHealerSounds(sounds []healerSound, health, mana *healerHealthSettings, buffs *healerMemberBuffSettings, skills *healerMemberSkillSettings) []healerSound {
	sounds = append(sounds, health.Sound)
	if mana != nil {
		sounds = append(sounds, mana.Sound)
	}
	for _, rule := range skills.Rules {
		sounds = append(sounds, rule.Sound)
	}
	for _, rule := range buffs.Rules {
		sounds = append(sounds, *rule.Sound, rule.DeathLoss.Sound)
	}
	return sounds
}

func healerSettingsPath() string { return filepath.Join(appDataDir(), "healer-monitor.json") }
