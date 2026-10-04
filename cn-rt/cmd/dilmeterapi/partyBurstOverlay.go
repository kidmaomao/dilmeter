package main

import (
	"fmt"
	"strconv"
	"time"
)

const burstCardSize = 112

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

// All phases share one compact card and one coordinate. A real condition is
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
		for index, id := range actors {
			e, state := runtime.entities[id], runtime.partySkills[id][skill]
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
			} else if state != nil && skill != 58014 && rule.Ready.Enabled && state.UsedAtMs > 0 {
				ready := partyReadyAt(state, rule.CooldownSeconds)
				if ready <= nowMs && nowMs < ready+3000 {
					phase, started, ends = "ready", ready, ready+3000
				}
			}
			if phase == "" {
				continue
			}
			generation := started
			if phase == "effect" && state != nil && state.CastAtMs > 0 && started >= state.CastAtMs-999 && started-state.CastAtMs <= int64(rule.CastSeconds*1000)+3000 {
				generation = state.CastAtMs
			}
			label := e.Name + " · " + rule.Name
			if phase == "ready" {
				label += "已就绪"
			} else if phase == "cast" {
				label += "吟唱"
			} else {
				label += "生效"
			}
			item := nativeBossMechanicOverlayItem{Key: fmt.Sprintf("burst:%d:%s", skill, id), Name: label, Label: label, ActorID: id, ActorName: e.Name, SkillID: skill, SkillName: rule.Name, Phase: phase, HideCountdown: phase == "ready", TimingUnknown: unknown, TargetID: target, Orientation: rule.Orientation,
				Icon: "mdi-alert-decagram", StartedAtMs: started, EndsAtMs: ends, Generation: uint64(generation), X: rule.Cast.X, Y: rule.Cast.Y + index*(burstCardSize+8)*rule.Cast.ScalePercent/100, ScalePercent: rule.Cast.ScalePercent}
			items = append(items, separateBurstPopup(items, item))
			if state == nil {
				continue
			}
			display := rule.Cast
			announced := state.SoundCastAtMs == started
			if phase == "ready" {
				display = rule.Ready
				announced = state.SoundReadyUsedAtMs == state.UsedAtMs
			}
			if phase != "effect" && !announced && display.SoundEnabled && runtime.settings.Burst.Volume > 0 && runtime.playSound != nil && runtime.playSound(nativeReminderSoundRequest{Kind: "skill-ready", Volume: runtime.settings.Burst.Volume}) {
				if phase == "ready" {
					state.SoundReadyUsedAtMs = state.UsedAtMs
				} else {
					state.SoundCastAtMs = started
				}
			}
		}
	}
	return items, nil
}

func separateBurstPopup(existing []nativeBossMechanicOverlayItem, popup nativeBossMechanicOverlayItem) nativeBossMechanicOverlayItem {
	size := burstCardSize * popup.ScalePercent / 100
	for attempts := 0; attempts <= len(existing); attempts++ {
		moved := false
		for _, other := range existing {
			otherSize := burstCardSize * other.ScalePercent / 100
			if popup.X < other.X+otherSize && popup.X+size > other.X && popup.Y < other.Y+otherSize && popup.Y+size > other.Y {
				popup.Y = other.Y + otherSize + 8
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	return popup
}
