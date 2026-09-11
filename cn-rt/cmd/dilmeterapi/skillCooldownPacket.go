package main

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

type skillCooldownReduction struct {
	SkillID  uint16
	ReduceMs uint32
}

var astrologyCriticalCooldownMs = map[uint16]uint32{
	27201: 3000, // 星之引爆 / 星之爆破
	27202: 2000, // 星界突破
	27203: 2000, // 迴旋斬擊
	27204: 3000, // 星象輪迴
}

func parseSkillCooldownReset(msg packet.Message) (uint16, error) {
	if len(msg) < 1 || msg[0].Type() != packet.MessageElemTypeShort {
		return 0, fmt.Errorf("expected leading skill-id short")
	}
	skillID := msg[0].Data().(uint16)
	if skillID == 0 {
		return 0, fmt.Errorf("invalid zero skill id")
	}
	return skillID, nil
}

func parseSkillCooldownReductions(msg packet.Message) ([]skillCooldownReduction, error) {
	if len(msg) < 1 || msg[0].Type() != packet.MessageElemTypeInt {
		return nil, fmt.Errorf("expected leading reduction count")
	}
	count := int(msg[0].Data().(uint32))
	if count <= 0 || count > 256 || len(msg) < 1+count*2 {
		return nil, fmt.Errorf("invalid reduction count %d for %d elements", count, len(msg))
	}
	result := make([]skillCooldownReduction, 0, count)
	for index := 0; index < count; index++ {
		base := 1 + index*2
		if msg[base].Type() != packet.MessageElemTypeShort || msg[base+1].Type() != packet.MessageElemTypeInt {
			return nil, fmt.Errorf("invalid reduction entry %d", index)
		}
		skillID := msg[base].Data().(uint16)
		reduceMs := msg[base+1].Data().(uint32)
		if skillID == 0 || reduceMs == 0 {
			continue
		}
		result = append(result, skillCooldownReduction{SkillID: skillID, ReduceMs: reduceMs})
	}
	return result, nil
}

func parseAstrologyCooldownSignal(msg packet.Message) (uint16, uint64, uint32, bool) {
	if len(msg) < 6 ||
		msg[0].Type() != packet.MessageElemTypeByte ||
		msg[1].Type() != packet.MessageElemTypeShort ||
		msg[2].Type() != packet.MessageElemTypeByte ||
		msg[3].Type() != packet.MessageElemTypeString ||
		msg[4].Type() != packet.MessageElemTypeByte ||
		msg[5].Type() != packet.MessageElemTypeLong {
		return 0, 0, 0, false
	}
	key := msg[3].Data().(string)
	if !strings.HasPrefix(key, "SLST") || len(key) <= 4 {
		return 0, 0, 0, false
	}
	parsed, err := strconv.ParseUint(key[4:], 10, 16)
	if err != nil || parsed == 0 {
		return 0, 0, 0, false
	}
	skillID := uint16(parsed)
	reduceMs, configured := astrologyCriticalCooldownMs[skillID]
	if !configured {
		return 0, 0, 0, false
	}
	return skillID, msg[5].Data().(uint64), reduceMs, true
}

func (t *eventPublisher) acceptsLocalCooldownPacket(p *packet.GamePacket) bool {
	return p != nil && (t.localEntityId == 0 || p.Id == t.localEntityId)
}

func (t *eventPublisher) publishSkillCooldownResetPacket(p *packet.GamePacket) error {
	if !t.acceptsLocalCooldownPacket(p) {
		return nil
	}
	skillID, err := parseSkillCooldownReset(p.Msg)
	if err != nil {
		return err
	}
	t.publishSkillCooldownEvent(p, skillID, true, 0, "27049")
	return nil
}

func (t *eventPublisher) publishSkillCooldownReducePacket(p *packet.GamePacket) error {
	if !t.acceptsLocalCooldownPacket(p) {
		return nil
	}
	reductions, err := parseSkillCooldownReductions(p.Msg)
	if err != nil {
		return err
	}
	for _, reduction := range reductions {
		t.publishSkillCooldownEvent(p, reduction.SkillID, false, reduction.ReduceMs, "27069")
	}
	return nil
}

func (t *eventPublisher) publishAstrologyCooldownPacket(p *packet.GamePacket) error {
	if !t.acceptsLocalCooldownPacket(p) {
		return nil
	}
	skillID, token, reduceMs, ok := parseAstrologyCooldownSignal(p.Msg)
	if !ok {
		return nil
	}
	if t.recentCooldownSignals == nil {
		t.recentCooldownSignals = make(map[uint16]uint64, 8)
	}
	if previous := t.recentCooldownSignals[skillID]; previous == token && token != 0 {
		return nil
	}
	t.recentCooldownSignals[skillID] = token
	t.publishSkillCooldownEvent(p, skillID, false, reduceMs, "SLST")
	return nil
}

func (t *eventPublisher) publishSkillCooldownEvent(p *packet.GamePacket, skillID uint16, reset bool, reduceMs uint32, signal string) {
	entityID := p.Id
	if t.localEntityId != 0 {
		entityID = t.localEntityId
	}
	t.publish(&event.EventSkillCooldown{
		EventBase: event.EventBase{
			EventId: event.EventIdSkillCooldown,
			At:      p.At.Unix(),
			Id:      strconv.FormatUint(entityID, 10),
		},
		AtMs: p.At.UnixMilli(), SkillId: skillID,
		Reset: reset, ReduceMs: reduceMs, Signal: signal,
	})
}
