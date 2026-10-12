package main

import (
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func obsUltimateSkill(skill uint16) bool {
	switch skill {
	case 59005, 59047, 59064, 59085, 59106, 59123, 59143, 59163, 59187:
		return true
	}
	return false
}

func obsActionEvidence(e *event.EventSkillAction, a *packet.CombatActionPacket) *event.EventSkillAction {
	if obsUltimateSkill(e.SkillId) && a != nil {
		e.ObservedActionType = uint8(a.Type)
		if a.Attacker != nil {
			e.ObservedTargetId = strconv.FormatUint(a.Attacker.TargetId, 10)
		}
	}
	return e
}

// Animation release evidence is independent of cooldown fallback/action-ID
// deduplication. ACK payloads may contain reused targets or mode values, and a
// combat fallback may already have consumed the same cooldown action ID.
// Star merges ACK/public confirmations within each skill's cast window.
func (t *eventPublisher) publishObsRelease(p *packet.GamePacket, actor uint64, skill uint16, scope string) {
	if p == nil || !obsUltimateSkill(skill) || !t.partySignalActor(actor) {
		return
	}
	t.publish(&event.EventSkillState{
		EventBase: event.EventBase{EventId: event.EventIdSkillState, At: p.At.Unix(), Id: strconv.FormatUint(actor, 10)},
		AtMs:      p.At.UnixMilli(), SkillId: skill, Scope: scope, Active: true,
	})
}

func (t *eventPublisher) publishObsExecute(p *packet.GamePacket) {
	if p == nil {
		return
	}
	action, err := packet.ParseSkillExecutePacket(p.Msg)
	if err == nil {
		t.publishObsRelease(p, p.Id, action.SkillId, "obs-execute")
	}
}

func (t *eventPublisher) publishObsCombat(p *packet.GamePacket, pack *packet.CombatActionPackPacket) {
	if p == nil || pack == nil {
		return
	}
	seen := map[[2]uint64]bool{}
	const flags = packet.CombatActionTypeAttacker | packet.CombatActionTypeSkillActive | packet.CombatActionTypeSkillSuccess | packet.CombatActionTypeSkillPlayerCharacter
	for _, a := range pack.SubPackets {
		if a == nil || !obsUltimateSkill(a.SkillId) || a.Hit != nil || a.Attacker == nil || a.Type&flags != flags {
			continue
		}
		// Later lance explosion phases are not another release. A target may
		// already be dead or absent; target HP is not release evidence.
		if a.SkillId == 59106 && a.SubSkillId > 1 {
			continue
		}
		key := [2]uint64{a.EntityId, uint64(a.SkillId)}
		if !seen[key] {
			seen[key] = true
			t.publishObsRelease(p, a.EntityId, a.SkillId, "obs-public-release")
		}
	}
}
