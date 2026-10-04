package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

type partySkillObservation struct {
	PowerSignalActive                                     bool
	EffectAtMs                                            int64
	UsedAtMs, CastAtMs, CastUntilMs, ReducedMs, ResetAtMs int64
	ActionID                                              uint32
	SoundCastAtMs, SoundReadyUsedAtMs                     int64
	ConfirmedAtMs                                         int64
}

type healerSkillRule struct {
	SkillID         uint16      `json:"skillId"`
	Name            string      `json:"name"`
	CooldownSeconds float64     `json:"cooldownSeconds"`
	Sound           healerSound `json:"sound"`
}
type healerMemberSkillSettings struct {
	Enabled      bool              `json:"enabled"`
	SoundEnabled bool              `json:"soundEnabled"`
	Overlay      healerPosition    `json:"overlay"`
	Rules        []healerSkillRule `json:"rules"`
}

func normalizeHealerSkills(value *healerMemberSkillSettings, index int) *healerMemberSkillSettings {
	if value == nil {
		value = &healerMemberSkillSettings{SoundEnabled: true, Overlay: healerPosition{Enabled: true, X: 40, Y: 240 + index*72}}
	}
	result := *value
	result.Overlay = normalizeHealerPosition(result.Overlay)
	result.Rules = []healerSkillRule{}
	seen := map[uint16]bool{}
	for _, rule := range value.Rules {
		if rule.SkillID == 0 || rule.SkillID == 58014 || seen[rule.SkillID] {
			continue
		}
		seen[rule.SkillID] = true
		if rule.Name == "" {
			rule.Name = fmt.Sprintf("技能 %d", rule.SkillID)
		}
		rule.Name = string([]rune(rule.Name)[:min(48, len([]rune(rule.Name)))])
		rule.CooldownSeconds = partySeconds(rule.CooldownSeconds, 30)
		rule.Sound = normalizeHealerSound(rule.Sound, "skill-ready")
		result.Rules = append(result.Rules, rule)
		if len(result.Rules) >= 16 {
			break
		}
	}
	return &result
}

func partySeconds(value, fallback float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return fallback
	}
	return math.Max(.1, math.Min(86400, value))
}

func partyReadyAt(state *partySkillObservation, seconds float64) int64 {
	if state == nil || state.UsedAtMs == 0 {
		return 0
	}
	if state.ResetAtMs >= state.UsedAtMs {
		return state.ResetAtMs
	}
	return max(state.UsedAtMs, state.UsedAtMs+int64(seconds*1000)-state.ReducedMs)
}

func (runtime *nativeReminderRuntime) partyPlayerActive(id string) bool {
	entity := runtime.entities[id]
	if entity == nil || !entity.Known || entity.OwnerID != "" || !battleRecordPCRace(entity.RaceID) {
		return false
	}
	if entity.Active {
		return true
	}
	if runtime.healer == nil {
		return false
	}
	runtime.healer.mu.Lock()
	defer runtime.healer.mu.Unlock()
	return healerEntityActive(entity, runtime.healer.observed[id])
}

func (runtime *nativeReminderRuntime) partyObservation(actor string, skill uint16) *partySkillObservation {
	if runtime.partySkills == nil {
		runtime.partySkills = map[string]map[uint16]*partySkillObservation{}
	}
	if runtime.partySkills[actor] == nil {
		runtime.partySkills[actor] = map[uint16]*partySkillObservation{}
	}
	states := runtime.partySkills[actor]
	if state := states[skill]; state != nil {
		return state
	}
	if len(states) >= 128 {
		return nil
	}
	states[skill] = &partySkillObservation{}
	return states[skill]
}

// Track confirmed casts per actual player, independently of the local skill timers.
// Damage-phase fallback events cannot restart an already cooling skill.
func (runtime *nativeReminderRuntime) observePartyEvent(current event.IEvent) {
	if gone, ok := current.(*event.EventFinish); ok {
		for _, state := range runtime.partySkills[gone.Id] {
			state.EffectAtMs, state.CastUntilMs = 0, 0
			state.PowerSignalActive = false
		}
		return
	}
	if local, ok := current.(*event.EventLocalEntity); ok && (local.Reset || local.Id != runtime.partyLocalID) {
		runtime.partySkills = nil
		runtime.partyLocalID = local.Id
		return
	}
	if gone, ok := current.(*event.EventEntityDisappear); ok {
		delete(runtime.partySkills, gone.Id)
		return
	}
	if v, ok := current.(*event.EventCharacterConditionDisable); ok && v.CCId == 516 {
		// Ignore the old removal of a same-frame condition replacement.
		if entity := runtime.entities[v.Id]; entity != nil {
			if _, active := entity.Conditions[516]; active {
				return
			}
		}
		if state := runtime.partySkills[v.Id][58014]; state != nil {
			state.EffectAtMs = 0
		}
		return
	}
	if v, ok := current.(*event.EventCharacterConditionEnable); ok {
		actor, skill := v.Id, uint16(58014)
		if v.CCId == 803 {
			actor, skill = v.AttackerId, 59005
			if _, source, valid := parseConditionSkillMetadata(v.Metadata); valid && source != 0 {
				actor = strconv.FormatUint(source, 10)
			}
		} else if v.CCId != 516 {
			return
		}
		if runtime.partyPlayerActive(actor) {
			if state := runtime.partyObservation(actor, skill); state != nil {
				// CC803 names the actual caster. This confirms a release even when
				// that teammate's private execute packet was not broadcast.
				if v.CCId == 803 && !v.Snapshot && (state.UsedAtMs == 0 || partyReadyAt(state, runtime.partyCooldownSeconds(actor, skill)) <= v.At*1000) {
					state.UsedAtMs, state.ReducedMs, state.ResetAtMs, state.ActionID = v.At*1000, 0, 0, 0
				}
				state.ConfirmedAtMs = max(state.ConfirmedAtMs, v.At*1000)
				if state.CastAtMs <= v.At*1000+999 {
					state.CastUntilMs = 0
				}
			}
		}
		return
	}
	var actor string
	var skill uint16
	var atMs int64
	var action *event.EventSkillAction
	switch v := current.(type) {
	case *event.EventSkillAction:
		actor, skill, atMs, action = v.SourceId, v.SkillId, nativeEventAtMs(v.At, v.AtMs), v
		if actor == "" {
			actor = v.Id
		}
	case *event.EventSkillState:
		if v.Scope != "prepare" && !(v.Scope == "burst-effect" && v.SkillId == 58014) && !(v.Scope == "burst-release" && v.SkillId == 59005) {
			return
		}
		actor, skill, atMs = v.Id, v.SkillId, nativeEventAtMs(v.At, v.AtMs)
	case *event.EventSkillCooldown:
		actor, skill, atMs = v.Id, v.SkillId, nativeEventAtMs(v.At, v.AtMs)
	default:
		return
	}
	if !runtime.partyPlayerActive(actor) || skill == 0 {
		return
	}
	state := runtime.partyObservation(actor, skill)
	if state == nil {
		return
	}
	if v, ok := current.(*event.EventSkillCooldown); ok {
		if skill == 58014 || state.UsedAtMs == 0 || atMs < state.UsedAtMs {
			return
		}
		if v.Reset {
			state.ResetAtMs = atMs
		} else {
			state.ReducedMs += int64(v.ReduceMs)
		}
		return
	}
	if v, ok := current.(*event.EventSkillState); ok {
		if v.Scope == "burst-release" {
			if !v.Active {
				return
			}
			state.CastUntilMs = 0
			state.ConfirmedAtMs = max(state.ConfirmedAtMs, atMs)
			if state.UsedAtMs == 0 || atMs-state.UsedAtMs >= 1000 {
				state.UsedAtMs, state.ReducedMs, state.ResetAtMs, state.ActionID = atMs, 0, 0, 0
			}
			return
		}
		if v.Scope == "burst-effect" {
			state.CastUntilMs = 0
			// 615-on anchors Awakening's fixed ten seconds. 615-off only
			// ends the held skill; it does not end or extend Awakening.
			if v.Active && !state.PowerSignalActive {
				state.EffectAtMs = atMs
				state.ConfirmedAtMs = atMs
			}
			state.PowerSignalActive = v.Active
			return
		}
		if !v.Active {
			state.CastUntilMs = 0
			return
		}
		if rule, exists := runtime.settings.Burst.Rules[skill]; exists && rule.Cast.Enabled {
			if state.CastUntilMs > atMs || state.ConfirmedAtMs+999 >= atMs {
				return
			}
			state.CastAtMs, state.CastUntilMs = atMs, atMs+int64(rule.CastSeconds*1000)
			if skill == 58014 {
				state.PowerSignalActive = false
			}
		}
		return
	}
	if state.UsedAtMs > 0 && (atMs <= state.UsedAtMs || atMs-state.UsedAtMs < 1000 || action.CombatActionId != 0 && action.CombatActionId == state.ActionID) {
		return
	}
	if action.IsFallback && partyReadyAt(state, runtime.partyCooldownSeconds(actor, skill)) > atMs {
		return
	}
	state.UsedAtMs, state.ReducedMs, state.ResetAtMs, state.ActionID = atMs, 0, 0, action.CombatActionId
	if rule, exists := runtime.settings.Burst.Rules[skill]; exists && rule.Cast.Enabled && state.CastUntilMs <= atMs && state.ConfirmedAtMs+999 < atMs {
		state.CastAtMs, state.CastUntilMs = atMs, atMs+int64(rule.CastSeconds*1000)
	}
}

func (runtime *nativeReminderRuntime) partyCooldownSeconds(actor string, skill uint16) float64 {
	if skill == 58014 {
		return 0
	}
	seconds := 30.0
	if rule, ok := runtime.settings.Burst.Rules[skill]; ok {
		seconds = rule.CooldownSeconds
	}
	if runtime.healer != nil {
		runtime.healer.mu.Lock()
		defer runtime.healer.mu.Unlock()
		for _, member := range runtime.healer.settings.Members {
			if member.ID == actor && member.SkillSettings != nil {
				for _, rule := range member.SkillSettings.Rules {
					if rule.SkillID == skill {
						seconds = max(seconds, rule.CooldownSeconds)
					}
				}
			}
		}
	}
	return seconds
}

type nativeBurstDisplay struct {
	Enabled      bool `json:"enabled"`
	X            int  `json:"x"`
	Y            int  `json:"y"`
	ScalePercent int  `json:"scalePercent"`
	SoundEnabled bool `json:"soundEnabled"`
}
type nativeBurstRule struct {
	SkillID         uint16             `json:"skillId"`
	CCID            uint32             `json:"ccId"`
	Name            string             `json:"name"`
	CooldownSeconds float64            `json:"cooldownSeconds"`
	CastSeconds     float64            `json:"castSeconds"`
	Ready           nativeBurstDisplay `json:"ready"`
	Cast            nativeBurstDisplay `json:"cast"`
	Effect          nativeBurstDisplay `json:"effect"`
	Orientation     string             `json:"orientation"`
}
type nativeBurstSettings struct {
	Enabled          bool                       `json:"enabled"`
	IncludeSelf      bool                       `json:"includeSelf"`
	IncludeTeammates bool                       `json:"includeTeammates"`
	Volume           int                        `json:"volume"`
	Rules            map[uint16]nativeBurstRule `json:"rules"`
}

func normalizeNativeBurstSettings(settings nativeBurstSettings) nativeBurstSettings {
	settings.Volume = clampNativeReminderInt(settings.Volume, 0, 100, 80)
	rules := map[uint16]nativeBurstRule{}
	for index, skill := range []uint16{59005, 58014} {
		rule, exists := settings.Rules[skill]
		name, cc, cd, cast := "崩坏波动", uint32(803), 60.0, 2.0
		if skill == 58014 {
			name, cc, cd, cast = "万钧之力", 516, 0, 5
		}
		// Presentation follows the selected OT resource region. IDs and timing
		// remain fixed; accept only the two verified TW names from the UI.
		if skill == 59005 && rule.Name == "崩壞的波動" || skill == 58014 && rule.Name == "力量團聚" {
			name = rule.Name
		}
		if !exists {
			rule = nativeBurstRule{CooldownSeconds: cd, CastSeconds: cast, Orientation: "horizontal",
				Ready:  nativeBurstDisplay{Enabled: skill == 59005, X: 600, Y: 260 + index*120, ScalePercent: 100, SoundEnabled: true},
				Cast:   nativeBurstDisplay{Enabled: true, X: 600, Y: 350 + index*120, ScalePercent: 100, SoundEnabled: true},
				Effect: nativeBurstDisplay{Enabled: true, X: 600, Y: 450 + index*120, ScalePercent: 100}}
		}
		rule.SkillID, rule.CCID, rule.Name = skill, cc, name
		rule.CooldownSeconds, rule.CastSeconds = partySeconds(rule.CooldownSeconds, cd), math.Min(60, partySeconds(rule.CastSeconds, cast))
		if skill == 58014 {
			rule.CooldownSeconds, rule.Ready.Enabled = 0, false
		}
		if rule.Orientation != "vertical" {
			rule.Orientation = "horizontal"
		}
		for _, display := range []*nativeBurstDisplay{&rule.Ready, &rule.Cast, &rule.Effect} {
			display.ScalePercent = clampNativeReminderInt(display.ScalePercent, 50, 200, 100)
			display.X = clampNativeReminderInt(display.X, -32000, 32000, 600)
			display.Y = clampNativeReminderInt(display.Y, -32000, 32000, 260)
		}
		rule.Ready.X, rule.Ready.Y, rule.Ready.ScalePercent = rule.Cast.X, rule.Cast.Y, rule.Cast.ScalePercent
		rule.Effect.X, rule.Effect.Y, rule.Effect.ScalePercent = rule.Cast.X, rule.Cast.Y, rule.Cast.ScalePercent
		rules[skill] = rule
	}
	settings.Rules = rules
	return settings
}

func (runtime *nativeReminderRuntime) burstActors() []string {
	ids := map[string]bool{}
	if runtime.settings.Burst.IncludeSelf && runtime.localID != "" {
		ids[runtime.localID] = true
	}
	if runtime.settings.Burst.IncludeTeammates && runtime.healer != nil {
		runtime.healer.mu.Lock()
		runtime.healer.resolveMembers(runtime)
		for _, member := range runtime.healer.settings.Members {
			if member.Included && member.ID != "" {
				ids[member.ID] = true
			}
		}
		runtime.healer.mu.Unlock()
	}
	result := []string{}
	for id := range ids {
		if runtime.partyPlayerActive(id) {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}
