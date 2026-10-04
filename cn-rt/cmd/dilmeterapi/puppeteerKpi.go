package main

import (
	"math"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const puppeteerAct7SkillID uint16 = 54105
const puppetAct7SkillID uint16 = 54155
const puppeteerInterludeSkillID uint16 = 59165

type puppeteerPositionGroup struct {
	atMs  int64
	count uint32
}

type puppeteerInterludeCast struct {
	atMs, firstAttackAtMs, lastAttackAtMs int64
	positions                             uint32
	actions                               map[uint32]bool
	targets                               map[uint64]bool
}

type puppeteerAct7Cast struct {
	atMs, puppetAtMs, firstAttackAtMs, lastAttackAtMs int64
	targetID, puppetID                                uint64
	actions                                           map[uint32]bool
	hit                                               bool
	invalid                                           bool
}

func (t *eventPublisher) publishPuppeteerPacket(p *packet.GamePacket) {
	if p == nil || t.localEntityId == 0 {
		return
	}
	t.publishInterludePacket(p)
	if p.Op == packet.OpcodeSkillExecute {
		skill, ok := parsePreparedSkillID(p.Msg)
		if !ok {
			return
		}
		if p.Id == t.localEntityId && skill == puppeteerAct7SkillID {
			t.puppeteerAct7 = nil
			// The owner ACK carries the selected target and four movement
			// floats. Its early end is not the end of the puppet's charge.
			if len(p.Msg) != 8 || p.Msg[1].Type() != packet.MessageElemTypeLong || p.Msg[2].Type() != packet.MessageElemTypeInt || p.Msg[3].Type() != packet.MessageElemTypeInt || p.Msg[2].Data().(uint32) != 0 || p.Msg[3].Data().(uint32) != 1 {
				return
			}
			for _, field := range p.Msg[4:] {
				if field.Type() != packet.MessageElemTypeFloat {
					return
				}
			}
			target := p.Msg[1].Data().(uint64)
			if target == 0 {
				return
			}
			t.puppeteerAct7 = &puppeteerAct7Cast{atMs: p.At.UnixMilli(), targetID: target, actions: make(map[uint32]bool)}
		}
		if skill == puppetAct7SkillID && p.Id != t.localEntityId && resolveEntityOwner(p.Id, t.entityCache) == t.localEntityId {
			cast := t.puppeteerAct7
			if cast == nil || p.At.UnixMilli() < cast.atMs || p.At.UnixMilli()-cast.atMs > 1000 {
				return
			}
			if cast.puppetID != 0 && cast.puppetID != p.Id {
				cast.invalid = true
				return
			}
			if len(p.Msg) != 2 || p.Msg[1].Type() != packet.MessageElemTypeLong || p.Msg[1].Data().(uint64) != 0 {
				cast.invalid = true
				return
			}
			cast.puppetID, cast.puppetAtMs = p.Id, p.At.UnixMilli()
		}
		return
	}
	if p.Op != 27017 && p.Op != packet.OpcodeSkillPrepareEnd {
		return
	}
	skill, ok := parseEndedSkillID(p.Msg)
	cast := t.puppeteerAct7
	if !ok || cast == nil {
		return
	}
	if p.Id == t.localEntityId && skill == puppeteerAct7SkillID {
		if p.Op == packet.OpcodeSkillPrepareEnd && !isExecutionPreparationEnd(p.Msg) {
			t.puppeteerAct7 = nil
		}
		return
	}
	if p.Id != cast.puppetID || skill != puppetAct7SkillID {
		return
	}
	if p.Op == packet.OpcodeSkillPrepareEnd && isExecutionPreparationEnd(p.Msg) {
		return
	}
	// All seven captured charges have three attack attempts. A short capture
	// with no damage is not sufficient evidence of an empty charge.
	if p.Op == 27017 && !cast.invalid && len(cast.actions) == 3 && cast.firstAttackAtMs >= cast.puppetAtMs && p.At.UnixMilli() >= cast.lastAttackAtMs && resolveEntityOwner(p.Id, t.entityCache) == t.localEntityId {
		empty := uint32(0)
		if !cast.hit {
			empty = 1
		}
		t.publish(&event.EventArcanaSignal{
			EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: cast.lastAttackAtMs / 1000, Id: strconv.FormatUint(t.localEntityId, 10)},
			AtMs:      cast.lastAttackAtMs, CastAtMs: cast.atMs, FirstHitAtMs: cast.firstAttackAtMs,
			SkillId: puppeteerAct7SkillID, Signal: "act7-sample", TargetId: strconv.FormatUint(cast.targetID, 10),
			Count: empty, Phase: 3, Complete: true, ObjectIds: []string{strconv.FormatUint(cast.puppetID, 10)},
		})
	}
	t.puppeteerAct7 = nil
}

func (t *eventPublisher) observePuppeteerCombat(p *packet.GamePacket, pack *packet.CombatActionPackPacket) {
	t.observeInterludeCombat(p, pack)
	cast := t.puppeteerAct7
	if p == nil || pack == nil || cast == nil || cast.puppetID == 0 || p.At.UnixMilli() < cast.puppetAtMs {
		return
	}
	var attacker *packet.CombatActionPacket
	for _, action := range pack.SubPackets {
		if action.Attacker != nil && action.Hit == nil {
			if attacker != nil {
				return
			}
			attacker = action
		}
	}
	if attacker == nil || attacker.SkillId != puppetAct7SkillID || attacker.EntityId != cast.puppetID || resolveEntityOwner(attacker.EntityId, t.entityCache) != t.localEntityId || pack.CombatActionId == 0 {
		return
	}
	if cast.firstAttackAtMs == 0 {
		cast.firstAttackAtMs = p.At.UnixMilli()
	}
	cast.lastAttackAtMs = p.At.UnixMilli()
	cast.actions[pack.CombatActionId] = true
	for _, action := range pack.SubPackets {
		// Hitting a different monster is still a successful charge. Never
		// infer an empty charge merely from no damage to the selected boss.
		if action.Hit != nil {
			cast.hit = true
		}
	}
}

// The user confirmed that the six casts and 959 position feedback counts
// (1,3,3,3,2,2) represent Interlude Slash's participating puppets. Require
// its own complete execution, matching attack attempts and target hits.
func (t *eventPublisher) publishInterludePacket(p *packet.GamePacket) {
	if p.Id != t.localEntityId {
		return
	}
	switch p.Op {
	case 27012, packet.OpcodeSkillPrepareReady:
		t.puppeteerInterlude, t.puppeteerPositions = nil, nil
	case 37011:
		if !isInterludePosition(p.Msg) {
			return
		}
		at := p.At.UnixMilli()
		if cast := t.puppeteerInterlude; cast != nil && cast.atMs == at {
			cast.positions++
		} else {
			if t.puppeteerPositions == nil || t.puppeteerPositions.atMs != at {
				t.puppeteerPositions = &puppeteerPositionGroup{atMs: at}
			}
			t.puppeteerPositions.count++
		}
	case packet.OpcodeSkillExecute:
		t.puppeteerInterlude = nil
		positions := t.puppeteerPositions
		t.puppeteerPositions = nil
		if len(p.Msg) != 2 || p.Msg[0].Type() != packet.MessageElemTypeShort || p.Msg[0].Data().(uint16) != puppeteerInterludeSkillID || p.Msg[1].Type() != packet.MessageElemTypeLong || p.Msg[1].Data().(uint64) != 0 {
			return
		}
		cast := &puppeteerInterludeCast{atMs: p.At.UnixMilli(), actions: make(map[uint32]bool), targets: make(map[uint64]bool)}
		if positions != nil && positions.atMs == cast.atMs {
			cast.positions = positions.count
		}
		t.puppeteerInterlude = cast
	case 27017, packet.OpcodeSkillPrepareEnd:
		skill, ok := parseEndedSkillID(p.Msg)
		cast := t.puppeteerInterlude
		if !ok || skill != puppeteerInterludeSkillID || cast == nil {
			return
		}
		if p.Op == packet.OpcodeSkillPrepareEnd && isExecutionPreparationEnd(p.Msg) {
			return
		}
		if p.Op == 27017 && cast.positions > 0 && cast.positions <= 100 && len(cast.actions) == int(cast.positions) && cast.firstAttackAtMs >= cast.atMs && p.At.UnixMilli() >= cast.lastAttackAtMs {
			for targetID := range cast.targets {
				t.publish(&event.EventArcanaSignal{
					EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: cast.lastAttackAtMs / 1000, Id: strconv.FormatUint(p.Id, 10)},
					AtMs:      cast.lastAttackAtMs, CastAtMs: cast.atMs, FirstHitAtMs: cast.firstAttackAtMs,
					SkillId: puppeteerInterludeSkillID, Signal: "interlude-sample", TargetId: strconv.FormatUint(targetID, 10), Count: cast.positions, Complete: true,
				})
			}
		}
		t.puppeteerInterlude, t.puppeteerPositions = nil, nil
	}
}

func isInterludePosition(msg packet.Message) bool {
	if len(msg) != 4 || msg[0].Type() != packet.MessageElemTypeInt || msg[1].Type() != packet.MessageElemTypeInt || msg[2].Type() != packet.MessageElemTypeFloat || msg[3].Type() != packet.MessageElemTypeFloat || msg[0].Data().(uint32) != 959 || msg[1].Data().(uint32) != 1 {
		return false
	}
	for _, field := range msg[2:] {
		value := float64(field.Data().(float32))
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func (t *eventPublisher) observeInterludeCombat(p *packet.GamePacket, pack *packet.CombatActionPackPacket) {
	cast := t.puppeteerInterlude
	if p == nil || pack == nil || cast == nil || p.At.UnixMilli() < cast.atMs || pack.CombatActionId == 0 {
		return
	}
	var attacker *packet.CombatActionPacket
	for _, action := range pack.SubPackets {
		if action.Attacker != nil && action.Hit == nil {
			if attacker != nil {
				return
			}
			attacker = action
		}
	}
	if attacker == nil || attacker.SkillId != puppeteerInterludeSkillID || attacker.EntityId != t.localEntityId {
		return
	}
	if cast.firstAttackAtMs == 0 {
		cast.firstAttackAtMs = p.At.UnixMilli()
	}
	cast.lastAttackAtMs = p.At.UnixMilli()
	cast.actions[pack.CombatActionId] = true
	for _, action := range pack.SubPackets {
		if action.Hit != nil && action.EntityId != 0 {
			cast.targets[action.EntityId] = true
		}
	}
}
