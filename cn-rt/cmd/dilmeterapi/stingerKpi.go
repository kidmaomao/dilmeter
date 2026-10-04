package main

import (
	"fmt"
	"math"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const hydroPierceSkillID uint16 = 59061

type hydroPierceCast struct {
	atMs     int64
	targetID uint64
	release  *event.EventArcanaSignal
}

// Hydro's execute ACK starts its charge; effect 825 contains the release
// geometry. Preserve the raw range for the full-charge KPI; it does not
// establish intermediate charge percentages or an animation-time conversion.
func (t *eventPublisher) publishStingerPacket(p *packet.GamePacket) {
	t.publishPartyStingerPacket(p)
	if p == nil || t.localEntityId == 0 || p.Id != t.localEntityId {
		return
	}
	switch p.Op {
	case 27012:
		t.hydroPierce = nil
	case packet.OpcodeSkillPrepareReady:
		if skill, ok := parsePreparedSkillID(p.Msg); ok && skill != hydroPierceSkillID {
			t.hydroPierce = nil
		}
	case packet.OpcodeSkillExecute:
		execute, err := packet.ParseSkillExecutePacket(p.Msg)
		if err != nil || execute.SkillId != hydroPierceSkillID {
			t.hydroPierce = nil
			return
		}
		// The verified seven-field target form is required; other forms
		// cannot establish which boss this charge belongs to.
		if len(p.Msg) != 7 || p.Msg[1].Type() != packet.MessageElemTypeLong || execute.ActionId != 0 || p.Msg[2].Type() != packet.MessageElemTypeInt || p.Msg[3].Type() != packet.MessageElemTypeInt || p.Msg[2].Data().(uint32) != 0 || p.Msg[3].Data().(uint32) != 1 {
			t.hydroPierce = nil
			return
		}
		for _, field := range p.Msg[4:] {
			if field.Type() != packet.MessageElemTypeShort {
				t.hydroPierce = nil
				return
			}
		}
		target := p.Msg[1].Data().(uint64)
		if target == 0 {
			t.hydroPierce = nil
			return
		}
		if cast := t.hydroPierce; cast != nil && cast.atMs == p.At.UnixMilli() && cast.targetID == target {
			return
		}
		t.hydroPierce = &hydroPierceCast{atMs: p.At.UnixMilli(), targetID: target}
	case 37011:
		signal, target, ok := parseHydroRelease(p.Msg)
		cast := t.hydroPierce
		if !ok || cast == nil || target != cast.targetID || p.At.UnixMilli() < cast.atMs || cast.release != nil {
			return
		}
		signal.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
		signal.AtMs, signal.CastAtMs = p.At.UnixMilli(), cast.atMs
		signal.TargetId = strconv.FormatUint(target, 10)
		cast.release = signal
	case packet.OpcodeSkillPrepareEnd:
		if skill, ok := parseEndedSkillID(p.Msg); ok && skill == hydroPierceSkillID && !isExecutionPreparationEnd(p.Msg) {
			t.hydroPierce = nil
		}
	case 27017:
		if len(p.Msg) != 1 {
			return
		}
		if skill, ok := parsePreparedSkillID(p.Msg); !ok || skill != hydroPierceSkillID {
			return
		}
		cast := t.hydroPierce
		if cast != nil && cast.release != nil && p.At.UnixMilli() >= cast.release.AtMs {
			cast.release.Complete = true
			t.publish(cast.release)
		}
		t.hydroPierce = nil
	}
}

// Teammates do not broadcast the private execute/end ACKs. The 14:34 capture
// verifies one delayed effect 821 per Blazing release (three damage actions),
// and an effect 825 charge start followed by the same release geometry used
// locally. These public messages retain the actual caster and target.
func (t *eventPublisher) publishPartyStingerPacket(p *packet.GamePacket) {
	if p == nil || t.localEntityId == 0 || p.Id == t.localEntityId || !t.partySignalActor(p.Id) {
		return
	}
	msg := p.Msg
	switch p.Op {
	case packet.OpcodeEffectDelayed:
		// Delay and effect parameters are not skill identities. Validate the
		// typed release form without restricting it to this player's 600/795.
		if len(msg) != 4 || msg[0].Type() != packet.MessageElemTypeInt || msg[1].Type() != packet.MessageElemTypeInt || msg[2].Type() != packet.MessageElemTypeLong || msg[3].Type() != packet.MessageElemTypeInt || msg[1].Data().(uint32) != 821 || msg[2].Data().(uint64) == 0 {
			return
		}
		if t.recentBossSkillAt == nil {
			t.recentBossSkillAt = make(map[string]int64)
		}
		key := fmt.Sprintf("party-blazing-release:%d", p.Id)
		if t.recentBossSkillAt[key] == p.At.UnixMilli() {
			return
		}
		t.recentBossSkillAt[key] = p.At.UnixMilli()
		id := strconv.FormatUint(p.Id, 10)
		t.publish(&event.EventSkillAction{EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: p.At.Unix(), Id: id}, SkillId: 59060, AtMs: p.At.UnixMilli(), SourceId: id, IsLocal: false})
	case 28006:
		if len(msg) == 1 && msg[0].Type() == packet.MessageElemTypeByte && msg[0].Data().(uint8) == 0 {
			delete(t.partyHydroPierce, p.Id)
		}
	case 37011:
		if len(msg) < 2 || msg[0].Type() != packet.MessageElemTypeInt || msg[0].Data().(uint32) != 825 {
			return
		}
		if len(msg) == 3 && msg[1].Type() == packet.MessageElemTypeByte && msg[2].Type() == packet.MessageElemTypeByte && msg[1].Data().(uint8) == 1 && msg[2].Data().(uint8) == 0 {
			if t.partyHydroPierce == nil {
				t.partyHydroPierce = make(map[uint64]int64)
			}
			t.partyHydroPierce[p.Id] = p.At.UnixMilli()
			return
		}
		// A four-field cancellation is not a release. Clear malformed feedback
		// too, so it cannot leave a stale charge for a later truncated sample.
		castAt := t.partyHydroPierce[p.Id]
		delete(t.partyHydroPierce, p.Id)
		signal, target, ok := parseHydroRelease(msg)
		if !ok || castAt <= 0 || p.At.UnixMilli() < castAt {
			return
		}
		signal.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
		signal.AtMs, signal.CastAtMs = p.At.UnixMilli(), castAt
		signal.TargetId, signal.Complete = strconv.FormatUint(target, 10), true
		t.publish(signal)
	}
}

func isPublicMagnumRelease(action *packet.CombatActionPacket) bool {
	const releaseFlags = packet.CombatActionTypeAttacker | packet.CombatActionTypeSkillActive | packet.CombatActionTypeSkillSuccess | packet.CombatActionTypeSkillPlayerCharacter
	return action != nil && action.SkillId == 21002 && action.Hit == nil && action.Attacker != nil && action.Attacker.TargetId != 0 && action.Type&releaseFlags == releaseFlags
}

func parseHydroRelease(msg packet.Message) (*event.EventArcanaSignal, uint64, bool) {
	if len(msg) != 15 || msg[0].Type() != packet.MessageElemTypeInt || msg[0].Data().(uint32) != 825 {
		return nil, 0, false
	}
	for _, field := range msg[1:4] {
		if field.Type() != packet.MessageElemTypeByte || field.Data().(uint8) != 0 {
			return nil, 0, false
		}
	}
	for index := 4; index < 15; index++ {
		if index == 10 {
			if msg[index].Type() != packet.MessageElemTypeLong {
				return nil, 0, false
			}
			continue
		}
		if msg[index].Type() != packet.MessageElemTypeFloat {
			return nil, 0, false
		}
		value := float64(msg[index].Data().(float32))
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, 0, false
		}
	}
	angle, distance := msg[8].Data().(float32), msg[7].Data().(float32)
	target := msg[10].Data().(uint64)
	if angle <= 0 || angle > 180 || distance <= 0 || target == 0 {
		return nil, 0, false
	}
	return &event.EventArcanaSignal{SkillId: hydroPierceSkillID, Signal: "hydro-charge-sample", Kind: 825, Count: 1, Value: angle, Range: distance}, target, true
}
