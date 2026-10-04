package main

import (
	"math"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const gunnerSniperSkillID uint16 = 59123
const gunnerDomainSkillID uint16 = 59125

type gunnerSniperCast struct {
	atMs     int64
	targetID uint64
	executed bool
	count    uint32
}

type gunnerHeavyCast struct {
	atMs         int64
	executedAtMs int64
	targetID     uint64
	linked       *event.EventArcanaSignal
}

// 27073 has no skill ID. Only interpret it inside a confirmed local Fatal
// Scope lifecycle, so unrelated aim/status packets cannot become shot counts.
func (t *eventPublisher) publishGunnerPacket(p *packet.GamePacket) {
	if p == nil || t.localEntityId == 0 || p.Id != t.localEntityId {
		return
	}
	switch p.Op {
	case 27012:
		// The 918 list precedes the prepare request, rather than its ACK.
		// Consume it only for the next local preparation; never reuse a list.
		t.gunnerHeavy = nil
		t.gunnerSniper = nil
		if len(p.Msg) > 0 && p.Msg[0].Type() == packet.MessageElemTypeShort && p.Msg[0].Data().(uint16) == 59121 {
			t.gunnerHeavy = &gunnerHeavyCast{atMs: p.At.UnixMilli(), linked: t.gunnerPendingDomains}
		}
		t.gunnerPendingDomains = nil
	case packet.OpcodeSkillPrepareReady:
		if len(p.Msg) == 0 || p.Msg[0].Type() != packet.MessageElemTypeShort {
			return
		}
		t.gunnerSniper = nil
		if p.Msg[0].Data().(uint16) != 59121 {
			t.gunnerHeavy = nil
			t.gunnerPendingDomains = nil
		}
		if p.Msg[0].Data().(uint16) == gunnerSniperSkillID {
			t.gunnerSniper = &gunnerSniperCast{atMs: p.At.UnixMilli()}
			if len(p.Msg) > 2 && p.Msg[2].Type() == packet.MessageElemTypeLong {
				t.gunnerSniper.targetID = p.Msg[2].Data().(uint64)
			}
		}
	case packet.OpcodeSkillExecute:
		if len(p.Msg) == 0 || p.Msg[0].Type() != packet.MessageElemTypeShort {
			return
		}
		if p.Msg[0].Data().(uint16) != gunnerSniperSkillID {
			t.gunnerSniper = nil
		}
		if p.Msg[0].Data().(uint16) == 59121 {
			targetForm := len(p.Msg) == 2 || (len(p.Msg) == 4 && p.Msg[2].Type() == packet.MessageElemTypeInt && p.Msg[3].Type() == packet.MessageElemTypeInt && p.Msg[2].Data().(uint32) == 0 && p.Msg[3].Data().(uint32) == 1)
			if cast := t.gunnerHeavy; targetForm && cast != nil && cast.linked != nil && cast.linked.AtMs <= cast.atMs && p.At.UnixMilli() >= cast.atMs && p.Msg[1].Type() == packet.MessageElemTypeLong {
				cast.executedAtMs = p.At.UnixMilli()
				cast.targetID = p.Msg[1].Data().(uint64)
			}
			return
		}
		if p.Msg[0].Data().(uint16) != gunnerSniperSkillID {
			t.gunnerHeavy = nil
			t.gunnerPendingDomains = nil
			return
		}
		if t.gunnerSniper == nil {
			t.gunnerSniper = &gunnerSniperCast{atMs: p.At.UnixMilli()}
		}
		t.gunnerSniper.executed = true
		if len(p.Msg) == 2 && p.Msg[1].Type() == packet.MessageElemTypeLong {
			t.gunnerSniper.targetID = p.Msg[1].Data().(uint64)
		}
	case 27017, packet.OpcodeSkillPrepareEnd:
		// The captured 27028 (0,1,0,skill) follows execution immediately:
		// preparation ended, but the sustained shot sequence is still active.
		endedSkill, ok := parseEndedSkillID(p.Msg)
		if !ok {
			return
		}
		preparedEnd := p.Op == packet.OpcodeSkillPrepareEnd && len(p.Msg) == 4 && p.Msg[0].Type() == packet.MessageElemTypeByte && p.Msg[1].Type() == packet.MessageElemTypeByte && p.Msg[2].Type() == packet.MessageElemTypeByte && p.Msg[0].Data().(uint8) == 0 && p.Msg[1].Data().(uint8) == 1 && p.Msg[2].Data().(uint8) == 0
		if preparedEnd && ((endedSkill == gunnerSniperSkillID && t.gunnerSniper != nil && t.gunnerSniper.executed) || (endedSkill == 59121 && t.gunnerHeavy != nil && t.gunnerHeavy.executedAtMs > 0)) {
			return
		}
		if endedSkill == 59121 {
			if cast := t.gunnerHeavy; p.Op == 27017 && cast != nil && cast.linked != nil && cast.executedAtMs > 0 && cast.targetID != 0 && p.At.UnixMilli() >= cast.executedAtMs {
				sample := *cast.linked
				sample.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: cast.executedAtMs / 1000, Id: strconv.FormatUint(p.Id, 10)}
				sample.Signal, sample.AtMs, sample.CastAtMs, sample.Complete = "domain-sample", cast.executedAtMs, cast.atMs, true
				sample.TargetId = strconv.FormatUint(cast.targetID, 10)
				t.publish(&sample)
			}
			t.gunnerHeavy = nil
			t.gunnerPendingDomains = nil
		}
		if endedSkill == gunnerSniperSkillID {
			t.gunnerSniper = nil
		}
	case 27073:
		phase, count, ok := parseGunnerSniperCounter(p.Msg)
		cast := t.gunnerSniper
		if !ok || cast == nil || p.At.UnixMilli() < cast.atMs || count < cast.count || (count > 0 && !cast.executed) {
			return
		}
		cast.count = count
		targetID := ""
		if cast.targetID != 0 {
			targetID = strconv.FormatUint(cast.targetID, 10)
		}
		t.publish(&event.EventArcanaSignal{
			EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)},
			AtMs:      p.At.UnixMilli(), SkillId: gunnerSniperSkillID, Signal: "sniper-counter",
			CastAtMs: cast.atMs, TargetId: targetID, Count: count, Phase: phase,
			Complete: phase == 7 && cast.executed,
		})
		if phase == 7 {
			t.gunnerSniper = nil
		}
	case 37011:
		if linked, ok := parseGunnerDomainLinks(p.Msg); ok {
			linked.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
			linked.AtMs = p.At.UnixMilli()
			t.gunnerPendingDomains = linked
			t.publish(linked)
			return
		}
		if removed, ok := parseGunnerDomainRemoval(p.Msg); ok {
			removed.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
			removed.AtMs = p.At.UnixMilli()
			t.gunnerPendingDomains = nil
			t.publish(removed)
			return
		}
		if extra, ok := parseGunnerHitExtra(p.Msg); ok {
			extra.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
			extra.AtMs = p.At.UnixMilli()
			t.publish(extra)
			return
		}
		zone, ok := parseGunnerDomainEffect(p.Msg)
		if !ok {
			return
		}
		zone.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
		zone.AtMs = p.At.UnixMilli()
		t.publish(zone)
	}
}

func parseGunnerDomainLinks(msg packet.Message) (*event.EventArcanaSignal, bool) {
	if len(msg) < 3 || msg[0].Type() != packet.MessageElemTypeInt || msg[1].Type() != packet.MessageElemTypeByte || msg[2].Type() != packet.MessageElemTypeInt || msg[0].Data().(uint32) != 918 || msg[1].Data().(uint8) != 1 {
		return nil, false
	}
	count := msg[2].Data().(uint32)
	if count > 32 || len(msg) != 3+int(count) {
		return nil, false
	}
	ids := make([]string, 0, count)
	seen := make(map[uint64]bool, count)
	for _, field := range msg[3:] {
		if field.Type() != packet.MessageElemTypeLong {
			return nil, false
		}
		id := field.Data().(uint64)
		if id == 0 || seen[id] {
			return nil, false
		}
		seen[id] = true
		ids = append(ids, strconv.FormatUint(id, 10))
	}
	return &event.EventArcanaSignal{SkillId: 59121, Signal: "domain-linked", Count: count, ObjectIds: ids}, true
}

// Retain the observed 922 end form without assigning it to an object ID or
// assuming that the monster spent all of the zone's lifetime inside it.
func parseGunnerDomainRemoval(msg packet.Message) (*event.EventArcanaSignal, bool) {
	if len(msg) != 4 || msg[0].Type() != packet.MessageElemTypeInt || msg[1].Type() != packet.MessageElemTypeByte || msg[2].Type() != packet.MessageElemTypeInt || msg[3].Type() != packet.MessageElemTypeInt || msg[0].Data().(uint32) != 922 || msg[1].Data().(uint8) != 1 {
		return nil, false
	}
	kind := msg[2].Data().(uint32)
	if kind < 1 || kind > 3 {
		return nil, false
	}
	return &event.EventArcanaSignal{SkillId: gunnerDomainSkillID, Signal: "domain-remove-signal", Kind: kind, Count: msg[3].Data().(uint32)}, true
}

// The controlled 1/2/3-zone capture adds one typed scalar to each Rapid Fire
// hit feedback. Retain that scalar without interpreting its entry count as
// the number of overlapping zones.
func parseGunnerHitExtra(msg packet.Message) (*event.EventArcanaSignal, bool) {
	if len(msg) != 12 {
		return nil, false
	}
	want := []packet.MessageElemType{packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeInt,
		packet.MessageElemTypeInt, packet.MessageElemTypeByte, packet.MessageElemTypeByte, packet.MessageElemTypeInt,
		packet.MessageElemTypeLong, packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeFloat, packet.MessageElemTypeFloat}
	for i, field := range msg {
		if field.Type() != want[i] {
			return nil, false
		}
	}
	value := msg[11].Data().(float32)
	kind := float64(msg[10].Data().(float32))
	if msg[0].Data().(uint32) != 917 || msg[1].Data().(uint32) != 3 || msg[9].Data().(uint32) != 1 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || kind < 0 || kind > math.MaxUint32 || math.Trunc(kind) != kind || math.IsNaN(kind) || math.IsInf(kind, 0) {
		return nil, false
	}
	return &event.EventArcanaSignal{SkillId: 59120, Signal: "gunner-hit-extra", Count: 1,
		TargetId: strconv.FormatUint(msg[7].Data().(uint64), 10), Kind: uint32(kind), Value: value}, true
}

func parseGunnerSniperCounter(msg packet.Message) (uint32, uint32, bool) {
	if len(msg) != 3 || msg[0].Type() != packet.MessageElemTypeInt || msg[1].Type() != packet.MessageElemTypeFloat || msg[2].Type() != packet.MessageElemTypeInt {
		return 0, 0, false
	}
	phase, count := msg[0].Data().(uint32), msg[2].Data().(uint32)
	progress := float64(msg[1].Data().(float32))
	return phase, count, (phase == 2 || phase == 5 || phase == 6 || phase == 7) && !math.IsNaN(progress) && !math.IsInf(progress, 0) && progress >= 0 && progress <= 100 && count <= 100
}

func parseGunnerDomainEffect(msg packet.Message) (*event.EventArcanaSignal, bool) {
	if len(msg) != 11 {
		return nil, false
	}
	want := []packet.MessageElemType{packet.MessageElemTypeInt, packet.MessageElemTypeByte, packet.MessageElemTypeInt,
		packet.MessageElemTypeFloat, packet.MessageElemTypeInt, packet.MessageElemTypeFloat, packet.MessageElemTypeFloat,
		packet.MessageElemTypeFloat, packet.MessageElemTypeFloat, packet.MessageElemTypeInt, packet.MessageElemTypeInt}
	for i, field := range msg {
		if field.Type() != want[i] {
			return nil, false
		}
	}
	kind, duration := msg[2].Data().(uint32), msg[4].Data().(uint32)
	if msg[0].Data().(uint32) != 922 || msg[1].Data().(uint8) != 0 || kind < 1 || kind > 3 || duration == 0 || duration > 10*60*1000 {
		return nil, false
	}
	for _, index := range []int{3, 5, 6, 7, 8} {
		value := float64(msg[index].Data().(float32))
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
	}
	return &event.EventArcanaSignal{SkillId: gunnerDomainSkillID, Signal: "domain-created", Kind: kind,
		DurationMs: duration, Range: msg[3].Data().(float32), X: msg[5].Data().(float32), Y: msg[6].Data().(float32)}, true
}
