package main

import (
	"fmt"
	"strconv"
	"time"
)

const burstCardSize = 112
const burstCooldownWidth, burstCooldownHeight = 112, 80

// Awakening is limited to ten seconds, independently of how long Power is held.
const powerAwakeningDurationMs int64 = 10000

type burstEffectObservation struct {
	started   int64
	target    *nativeReminderEntity
	condition nativeReminderCondition
	expires   int64
}

func (runtime *nativeReminderRuntime) burstEffect(actor string, skill uint16, nowMs int64) *burstEffectObservation {
	if skill == 58014 {
		e := runtime.entities[actor]
		if e == nil {
			return nil
		}
		state := runtime.partySkills[actor][skill]
		if c, ok := e.Conditions[516]; ok && (state == nil || state.EffectAtMs == 0 || c.At*1000+999 >= state.EffectAtMs) {
			start := c.At * 1000
			end := nativeBuffExpiresAtMs(c, nativeBuffRule{DurationMode: "auto"}, 0)
			if !c.Snapshot && end > 0 && c.DurationMs > 0 && c.DurationMs <= powerAwakeningDurationMs {
				// Preserve millisecond-precision deadlines when CC has a valid duration.
				start = end - c.DurationMs
			}
			if state != nil && state.EffectAtMs > 0 && start+999 >= state.EffectAtMs && start-state.EffectAtMs <= 1000 {
				start = state.EffectAtMs
			}
			// An undated appearance snapshot cannot establish a fresh ten seconds.
			if !c.Snapshot || end > 0 {
				if end == 0 || end > start+powerAwakeningDurationMs {
					end = start + powerAwakeningDurationMs
				}
				if start <= nowMs && end > nowMs {
					return &burstEffectObservation{target: e, condition: c, started: start, expires: end}
				}
				return nil
			}
		}
		if state != nil && state.EffectAtMs > 0 && state.EffectAtMs <= nowMs && nowMs < state.EffectAtMs+powerAwakeningDurationMs {
			return &burstEffectObservation{target: e, condition: nativeReminderCondition{At: state.EffectAtMs / 1000}, started: state.EffectAtMs, expires: state.EffectAtMs + powerAwakeningDurationMs}
		}
		return nil
	}
	var result *burstEffectObservation
	selected := runtime.selectDebuffBoss()
	for _, e := range runtime.entities {
		if !e.Known || !e.Active || e.OwnerID != "" || battleRecordPCRace(e.RaceID) {
			continue
		}
		c, ok := e.Conditions[803]
		if !ok {
			continue
		}
		owner := c.AttackerID
		if _, id, valid := parseConditionSkillMetadata(c.Metadata); valid && id != 0 {
			owner = strconv.FormatUint(id, 10)
		}
		if owner != actor {
			continue
		}
		end := nativeBuffExpiresAtMs(c, nativeBuffRule{DurationMode: "auto"}, 0)
		if end > 0 && end <= nowMs {
			continue
		}
		candidate := &burstEffectObservation{target: e, condition: c, expires: end}
		if selected != nil && e.ID == selected.ID {
			return candidate
		}
		if result == nil || nativeBossLess(e, result.target) || !nativeBossLess(result.target, e) && e.ID < result.target.ID {
			result = candidate
		}
	}
	return result
}

// Cast and effect share a card; the cooldown changes size at the alert threshold.
// A real condition is
// sufficient to display "active" even if its expiration was not broadcast.
// Preparation alone never invents a resulting condition or its duration.
func (runtime *nativeReminderRuntime) burstOverlay(now time.Time) ([]nativeBossMechanicOverlayItem, []nativeEffectTimerOverlayItem) {
	items := []nativeBossMechanicOverlayItem{}
	if !runtime.settings.Burst.Enabled {
		return items, nil
	}
	nowMs := now.UnixMilli()
	actors := runtime.burstActors()
	for _, skill := range []uint16{59005, 58014} {
		rule := runtime.settings.Burst.Rules[skill]
		if skill == 59005 {
			if cooldown := runtime.burstCooldownOverlay(actors, rule, nowMs); cooldown != nil {
				items = append(items, separateBurstPopup(items, *cooldown))
			}
		}
		for index, id := range actors {
			state := runtime.partySkills[id][skill]
			phase := ""
			started, ends := int64(0), int64(0)
			unknown := false
			target := ""
			if effect := runtime.burstEffect(id, skill, nowMs); rule.Effect.Enabled && effect != nil && (state == nil || state.CastUntilMs <= nowMs || effect.condition.At*1000+999 >= state.CastAtMs) {
				phase, started, ends, target = "effect", effect.condition.At*1000, effect.expires, effect.target.ID
				if effect.started > 0 {
					started = effect.started
				}
				if ends == 0 {
					unknown = true
					ends = nowMs + 2000
				}
			} else if state != nil && rule.Cast.Enabled && state.CastAtMs > 0 && state.CastAtMs <= nowMs && state.CastUntilMs > nowMs {
				phase, started, ends = "cast", state.CastAtMs, state.CastUntilMs
			}
			if phase == "" {
				continue
			}
			generation := started
			if phase == "effect" && state != nil && state.CastAtMs > 0 && started >= state.CastAtMs-999 && started-state.CastAtMs <= int64(rule.CastSeconds*1000)+3000 {
				generation = state.CastAtMs
			}
			name := runtime.burstActorName(id)
			label := name + " · " + rule.Name
			if phase == "cast" {
				label += "吟唱"
			} else {
				label += "生效"
			}
			item := nativeBossMechanicOverlayItem{Key: fmt.Sprintf("burst:%d:%s", skill, id), Name: label, Label: label, ActorID: id, ActorName: name, SkillID: skill, SkillName: rule.Name, Phase: phase, TimingUnknown: unknown, TargetID: target, Orientation: rule.Orientation,
				Icon: "mdi-alert-decagram", StartedAtMs: started, EndsAtMs: ends, Generation: uint64(generation), X: rule.Cast.X, Y: rule.Cast.Y + index*(burstCardSize+8)*rule.Cast.ScalePercent/100, ScalePercent: rule.Cast.ScalePercent}
			items = append(items, separateBurstPopup(items, item))
			if state == nil {
				continue
			}
			display := rule.Cast
			announced := state.SoundCastAtMs == started
			if phase == "cast" && !announced && display.SoundEnabled && runtime.settings.Burst.Volume > 0 && runtime.playSound != nil && runtime.playSound(nativeReminderSoundRequest{Kind: "skill-ready", Volume: runtime.settings.Burst.Volume}) {
				state.SoundCastAtMs = started
			}
		}
	}
	return items, nil
}

// Display one known cooldown, independently of the cast/effect cards. A caster's
// explicit teammate setting overrides the shared estimate, including shorter CDs.
func (runtime *nativeReminderRuntime) burstCooldownOverlay(actors []string, rule nativeBurstRule, nowMs int64) *nativeBossMechanicOverlayItem {
	always, alert := burstCooldownAlwaysVisible(rule), burstCooldownAlertEnabled(rule)
	if !rule.Ready.Enabled || !always && !alert {
		return nil
	}
	overrides := map[string]float64{}
	if runtime.healer != nil {
		runtime.healer.mu.Lock()
		for _, member := range runtime.healer.settings.Members {
			if member.SkillSettings == nil {
				continue
			}
			for _, skill := range member.SkillSettings.Rules {
				if skill.SkillID == 59005 {
					overrides[member.ID] = skill.CooldownSeconds
				}
			}
		}
		runtime.healer.mu.Unlock()
	}
	actor, readyAt := "", int64(0)
	completedActor, completedAt := "", int64(0)
	readyActors := []nativeBurstReadyActor{}
	recentReady := false
	announce := []*partySkillObservation{}
	for _, id := range actors {
		state, entity := runtime.partySkills[id][59005], runtime.entities[id]
		if state == nil || state.UsedAtMs <= 0 || state.UsedAtMs > nowMs || entity == nil || entity.Defeated || entity.HealthKnown && entity.CurrentHealth <= 0 {
			continue
		}
		seconds := rule.CooldownSeconds
		if override, exists := overrides[id]; exists {
			seconds = partySeconds(override, seconds)
		}
		ready := partyReadyAt(state, seconds)
		if ready <= nowMs {
			readyActors = append(readyActors, nativeBurstReadyActor{ActorID: id, ActorName: runtime.burstActorName(id)})
			if completedActor == "" || ready > completedAt || ready == completedAt && id < completedActor {
				completedActor, completedAt = id, ready
			}
			if nowMs < ready+3000 {
				recentReady = true
				if state.SoundReadyUsedAtMs != state.UsedAtMs {
					announce = append(announce, state)
				}
			}
			continue
		}
		if actor == "" || ready < readyAt || ready == readyAt && id < actor {
			actor, readyAt = id, ready
		}
	}
	if actor == "" {
		actor, readyAt = completedActor, completedAt
	}

	if actor == "" {
		// Keep the requested always-visible frame without inventing a cooldown.
		if always && len(actors) > 0 {
			id := actors[0]
			for _, candidate := range actors {
				if candidate == runtime.localID {
					id = candidate
					break
				}
			}
			name := runtime.burstActorName(id)
			return &nativeBossMechanicOverlayItem{Key: "burst:cooldown:59005", Name: rule.Name, Label: rule.Name, ActorID: id, ActorName: name, SkillID: 59005, SkillName: rule.Name, Phase: "cooldown", Compact: true, TimingUnknown: true, StartedAtMs: nowMs, EndsAtMs: nowMs + 2000, X: rule.Cast.X, Y: rule.Cast.Y, ScalePercent: rule.Cast.ScalePercent}
		}
		return nil
	}
	nextSoon := readyAt > nowMs && readyAt-nowMs <= int64(rule.CooldownLeadSeconds*1000)
	alertWindow := alert && (nextSoon || recentReady)
	if !always && !alertWindow {
		return nil
	}
	state := runtime.partySkills[actor][59005]
	name := runtime.burstActorName(actor)
	phase, start, end := "cooldown", state.UsedAtMs, readyAt
	label := name + " · " + rule.Name + "冷却中"
	if readyAt <= nowMs {
		phase, start, end, label = "ready", readyAt, readyAt+3000, name+" · "+rule.Name+"已就绪"
		if always {
			end = nowMs + 2000
		}
	}
	// A batch of simultaneous completions produces one sound, without repeating on each tick.
	if len(announce) > 0 && rule.Ready.SoundEnabled && runtime.settings.Burst.Volume > 0 && runtime.playSound != nil && runtime.playSound(nativeReminderSoundRequest{Kind: "skill-ready", Volume: runtime.settings.Burst.Volume}) {
		for _, completed := range announce {
			completed.SoundReadyUsedAtMs = completed.UsedAtMs
		}
	}
	return &nativeBossMechanicOverlayItem{Key: "burst:cooldown:59005", Name: label, Label: label, ActorID: actor, ActorName: name, ReadyActors: readyActors, NextReadySoon: nextSoon, SkillID: 59005, SkillName: rule.Name, Phase: phase, Compact: !alertWindow, HideCountdown: phase == "ready", Orientation: rule.Orientation, Icon: "mdi-timer-outline", StartedAtMs: start, EndsAtMs: end, Generation: uint64(state.UsedAtMs), X: rule.Cast.X, Y: rule.Cast.Y, ScalePercent: rule.Cast.ScalePercent}
}

func burstCooldownAlwaysVisible(rule nativeBurstRule) bool {
	if rule.CooldownAlwaysVisible != nil {
		return *rule.CooldownAlwaysVisible
	}
	return rule.CooldownMode == "always"
}

func burstCooldownAlertEnabled(rule nativeBurstRule) bool {
	return rule.CooldownAlertEnabled == nil || *rule.CooldownAlertEnabled
}

func (runtime *nativeReminderRuntime) burstActorName(id string) string {
	if entity := runtime.entities[id]; entity != nil && entity.Name != "" {
		return entity.Name
	}
	if id == runtime.localID {
		return "本人"
	}
	return "未知使用者"
}

func burstPopupDimensions(item nativeBossMechanicOverlayItem) (int, int) {
	if item.SkillID == 59005 && (item.Phase == "cooldown" || item.Phase == "ready") || item.Compact {
		rows := len(item.ReadyActors)
		if item.Phase != "ready" || rows == 0 {
			rows++
		}
		if item.Compact {
			return burstCooldownWidth, max(burstCooldownHeight, 48+16*rows)
		}
		return 128, max(112, 80+16*rows)
	}
	return burstCardSize, burstCardSize
}

func burstPopupSize(item nativeBossMechanicOverlayItem) (int, int) {
	width, height := burstPopupDimensions(item)
	return width * item.ScalePercent / 100, height * item.ScalePercent / 100
}

func separateBurstPopup(existing []nativeBossMechanicOverlayItem, popup nativeBossMechanicOverlayItem) nativeBossMechanicOverlayItem {
	width, height := burstPopupSize(popup)
	for attempts := 0; attempts <= len(existing); attempts++ {
		moved := false
		for _, other := range existing {
			otherWidth, otherHeight := burstPopupSize(other)
			if popup.X < other.X+otherWidth && popup.X+width > other.X && popup.Y < other.Y+otherHeight && popup.Y+height > other.Y {
				popup.Y = other.Y + otherHeight + 8
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	return popup
}
