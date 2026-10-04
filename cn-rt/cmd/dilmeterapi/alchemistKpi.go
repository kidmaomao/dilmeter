package main

import (
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const chemicalCarnivalSkillID uint16 = 59144

type chemicalCarnivalCast struct {
	atMs, firstHitAtMs, lastHitAtMs int64
	targetID                        uint64
	effectCount, hitCount           uint32
	invalid                         bool
}

// The confirmed count includes one base attack. Publish a complete sample
// only after matching the planned count, target hits and normal skill end.
func (t *eventPublisher) publishAlchemistPacket(p *packet.GamePacket) {
	if p == nil || t.localEntityId == 0 || p.Id != t.localEntityId {
		return
	}
	switch p.Op {
	case 27012, packet.OpcodeSkillPrepareReady:
		t.chemicalCast, t.chemicalPendingEffect = nil, nil
	case packet.OpcodeSkillExecute:
		t.chemicalCast = nil
		effect := t.chemicalPendingEffect
		t.chemicalPendingEffect = nil
		if len(p.Msg) != 5 || p.Msg[0].Type() != packet.MessageElemTypeShort || p.Msg[0].Data().(uint16) != chemicalCarnivalSkillID || p.Msg[1].Type() != packet.MessageElemTypeLong || p.Msg[2].Type() != packet.MessageElemTypeInt || p.Msg[3].Type() != packet.MessageElemTypeInt || p.Msg[4].Type() != packet.MessageElemTypeShort || p.Msg[2].Data().(uint32) != 0 || p.Msg[3].Data().(uint32) != 1 || p.Msg[4].Data().(uint16) != 2 {
			return
		}
		cast := &chemicalCarnivalCast{atMs: p.At.UnixMilli(), targetID: p.Msg[1].Data().(uint64)}
		// The effect precedes the execution ACK at the same capture timestamp.
		// Never carry an old effect into a later cast.
		if effect != nil && effect.AtMs == cast.atMs && effect.TargetId == strconv.FormatUint(cast.targetID, 10) {
			cast.effectCount = effect.Count
		}
		t.chemicalCast = cast
	case packet.OpcodeEffectDelayed:
		signal, ok := parseChemicalEffectCount(p.Msg)
		if !ok {
			return
		}
		signal.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
		signal.AtMs = p.At.UnixMilli()
		if cast := t.chemicalCast; cast != nil && signal.AtMs == cast.atMs && signal.TargetId == strconv.FormatUint(cast.targetID, 10) {
			signal.CastAtMs = cast.atMs
			if cast.effectCount != 0 && cast.effectCount != signal.Count {
				cast.invalid = true
			}
			cast.effectCount = signal.Count
		} else {
			t.chemicalPendingEffect = signal
		}
		t.publish(signal)
	case 27017, packet.OpcodeSkillPrepareEnd:
		skill, ok := parseEndedSkillID(p.Msg)
		if !ok || skill != chemicalCarnivalSkillID {
			return
		}
		cast := t.chemicalCast
		if p.Op == packet.OpcodeSkillPrepareEnd && isExecutionPreparationEnd(p.Msg) && cast != nil {
			return
		}
		if p.Op == 27017 && cast != nil && !cast.invalid && cast.targetID != 0 && cast.effectCount > 0 && cast.firstHitAtMs >= cast.atMs && cast.lastHitAtMs > 0 && p.At.UnixMilli() >= cast.lastHitAtMs && ((cast.effectCount == 1 && cast.hitCount == 0) || cast.hitCount == cast.effectCount) {
			t.publish(&event.EventArcanaSignal{
				EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: cast.lastHitAtMs / 1000, Id: strconv.FormatUint(p.Id, 10)},
				AtMs:      cast.lastHitAtMs, CastAtMs: cast.atMs, FirstHitAtMs: cast.firstHitAtMs,
				SkillId: chemicalCarnivalSkillID, Signal: "chemical-sample", TargetId: strconv.FormatUint(cast.targetID, 10), Count: cast.effectCount, Complete: true,
			})
		}
		t.chemicalCast, t.chemicalPendingEffect = nil, nil
	}
}

func isExecutionPreparationEnd(msg packet.Message) bool {
	return len(msg) == 4 && msg[0].Type() == packet.MessageElemTypeByte && msg[1].Type() == packet.MessageElemTypeByte && msg[2].Type() == packet.MessageElemTypeByte && msg[3].Type() == packet.MessageElemTypeShort && msg[0].Data().(uint8) == 0 && msg[1].Data().(uint8) == 1 && msg[2].Data().(uint8) == 0
}

func parseChemicalEffectCount(msg packet.Message) (*event.EventArcanaSignal, bool) {
	if len(msg) != 7 {
		return nil, false
	}
	for i, field := range msg {
		expected := packet.MessageElemTypeInt
		if i == 2 {
			expected = packet.MessageElemTypeLong
		}
		if field.Type() != expected {
			return nil, false
		}
	}
	count, target := msg[5].Data().(uint32), msg[2].Data().(uint64)
	if msg[1].Data().(uint32) != 924 || count == 0 || count > 100 || target == 0 {
		return nil, false
	}
	return &event.EventArcanaSignal{SkillId: chemicalCarnivalSkillID, Signal: "chemical-effect-count", TargetId: strconv.FormatUint(target, 10), Count: count, Kind: 924}, true
}

func (t *eventPublisher) publishAlchemistHitCounts(p *packet.GamePacket, pack *packet.CombatActionPackPacket) {
	if p == nil || pack == nil || t.localEntityId == 0 {
		return
	}
	var caster *packet.CombatActionPacket
	for _, action := range pack.SubPackets {
		if action.Attacker != nil && action.Hit == nil {
			// Multiple attackers would make the hit attribution ambiguous.
			if caster != nil {
				return
			}
			caster = action
		}
	}
	if caster == nil || caster.EntityId != t.localEntityId || caster.SkillId != chemicalCarnivalSkillID {
		return
	}
	for _, action := range pack.SubPackets {
		if action.Hit == nil || action.EntityId == 0 {
			continue
		}
		if cast := t.chemicalCast; cast != nil && action.EntityId == cast.targetID && p.At.UnixMilli() >= cast.atMs {
			if cast.firstHitAtMs == 0 {
				cast.firstHitAtMs = p.At.UnixMilli()
			}
			cast.lastHitAtMs = p.At.UnixMilli()
			if action.Hit.Options&packet.CombatActionHitOptionsMultiHit != 0 {
				count := action.Hit.MultiHitCount
				if count == 0 || count > 100 || (cast.hitCount != 0 && cast.hitCount != count) {
					cast.invalid = true
				}
				cast.hitCount = count
			}
		}
		if action.Hit.Options&packet.CombatActionHitOptionsMultiHit == 0 || action.Hit.MultiHitCount == 0 || action.Hit.MultiHitCount > 100 {
			continue
		}
		castAtMs := int64(0)
		if t.chemicalCast != nil && action.EntityId == t.chemicalCast.targetID {
			castAtMs = t.chemicalCast.atMs
		}
		t.publish(&event.EventArcanaSignal{
			EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(caster.EntityId, 10)},
			AtMs:      p.At.UnixMilli(), SkillId: chemicalCarnivalSkillID, Signal: "chemical-hit-count",
			TargetId: strconv.FormatUint(action.EntityId, 10), Count: action.Hit.MultiHitCount, CastAtMs: castAtMs,
		})
	}
}
