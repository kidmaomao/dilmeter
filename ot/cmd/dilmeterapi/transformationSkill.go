package main

import (
	"fmt"

	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// The captured Fury of Connous cast has no 27016 execution. After enabling its
// passive skills, the server sends 27020 [short(43001), string("")]. Only the
// transformation activation skills use this extra confirmation path: treating
// every completion or passive-enable packet as a cast would restart timers.
func (t *eventPublisher) publishTransformationComplete(p *packet.GamePacket) {
	if p == nil || p.Op != packet.OpcodeSkillComplete {
		return
	}
	skillID, ok := t.transformationResultSkill(p)
	if !ok {
		return
	}
	if t.recentBossSkillAt == nil {
		t.recentBossSkillAt = make(map[string]int64)
	}
	key := fmt.Sprintf("transform-complete:%d:%d", p.Id, skillID)
	if previous, exists := t.recentBossSkillAt[key]; exists && p.At.UnixMilli() <= previous {
		return
	}
	// Reuse the established action pipeline so both duration reminders and
	// cooldown tracking receive the same local, server-confirmed event.
	if err := t.publishSkillExecutePacket(p); err == nil {
		t.recentBossSkillAt[key] = p.At.UnixMilli()
	}
}

func isTransformationSkill(skillID uint32) bool {
	switch skillID {
	case 40001, 41001, 42001, 43001, 44001:
		return true
	}
	return false
}

func (t *eventPublisher) transformationResultSkill(p *packet.GamePacket) (uint16, bool) {
	if p == nil || p.Id == 0 || p.Id != t.localEntityId || len(p.Msg) != 2 ||
		p.Msg[0].Type() != packet.MessageElemTypeShort ||
		p.Msg[1].Type() != packet.MessageElemTypeString || p.Msg[1].Data().(string) != "" {
		return 0, false
	}
	skillID := p.Msg[0].Data().(uint16)
	return skillID, isTransformationSkill(uint32(skillID))
}

// The cancellation capture ends the transformation with 27021 [skill, ""],
// after disabling its passive skills. This is state removal, never another
// cast: leave the original cooldown intact.
func (t *eventPublisher) publishTransformationCancel(p *packet.GamePacket) {
	if p == nil || p.Op != packet.OpcodeSkillCancel {
		return
	}
	if skillID, ok := t.transformationResultSkill(p); ok {
		t.publishSkillState(p, skillID, "active", false, 0)
	}
}

func (runtime *nativeReminderRuntime) endTransformationEffectTimers(skillID uint16, atMs int64) {
	for key, rule := range runtime.settings.EffectTimers.Rules {
		if rule.SourceType != "skill" || rule.TargetMode != "self" ||
			!isTransformationSkill(rule.SourceID) || (skillID != 0 && rule.SourceID != uint32(skillID)) {
			continue
		}
		if state, exists := runtime.effectTimers[key]; exists && state.TargetID == runtime.localID &&
			(atMs == 0 || atMs >= state.StartedAtMs) {
			delete(runtime.effectTimers, key)
		}
	}
}
