package main

import (
	"math"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const fighterUppercutSkillID uint16 = 24201
const fighterReverseSkillID uint16 = 24103
const fighterKickSkillID uint16 = 24301

type fighterComboCast struct {
	atMs, reverseAtMs, reverseEndAtMs, readyAtMs, normalEndAtMs int64
	targetID                                                    uint64
	invalid                                                     bool
}

type fighterSpendCast struct {
	atMs    int64
	skillID uint16
}

// Reverse Dragon's animation cancel does not execute an attack: the capture
// has a preparation ACK and (0,1,1,skill) end instead of a 27016 execution.
// Preserve those observations and the explicit kick-ready effect together.
func (t *eventPublisher) publishFighterPacket(p *packet.GamePacket) {
	if p == nil || t.localEntityId == 0 || !t.partySignalActor(p.Id) {
		return
	}
	if signal, ok := parseFighterEnergyPacket(p); ok {
		signal.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
		signal.AtMs = p.At.UnixMilli()
		t.publish(signal)
	}
	if p.Id != t.localEntityId {
		return
	}
	t.observeFighterSpend(p)
	switch p.Op {
	case packet.OpcodeSkillExecute:
		skill, ok := parsePreparedSkillID(p.Msg)
		if !ok {
			return
		}
		if skill == fighterUppercutSkillID {
			t.fighterCombo = nil
			if target, ok := fighterExecuteTarget(p.Msg); ok {
				t.fighterCombo = &fighterComboCast{atMs: p.At.UnixMilli(), targetID: target}
			}
			return
		}
		cast := t.fighterCombo
		if skill == fighterKickSkillID && cast != nil {
			target, targetOK := fighterExecuteTarget(p.Msg)
			count := uint32(0)
			if cast.reverseAtMs > 0 {
				count = 1
			}
			complete := !cast.invalid && targetOK && target == cast.targetID && cast.readyAtMs >= cast.atMs && p.At.UnixMilli() >= cast.readyAtMs
			if count == 1 {
				complete = complete && cast.reverseEndAtMs >= cast.reverseAtMs && cast.readyAtMs >= cast.reverseEndAtMs
			} else {
				complete = complete && cast.normalEndAtMs >= cast.atMs && cast.readyAtMs >= cast.normalEndAtMs
			}
			t.publish(&event.EventArcanaSignal{
				EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)},
				AtMs:      p.At.UnixMilli(), CastAtMs: cast.atMs, ReverseAtMs: cast.reverseAtMs,
				ReverseEndAtMs: cast.reverseEndAtMs, ReadyAtMs: cast.readyAtMs,
				TargetId: strconv.FormatUint(cast.targetID, 10), SkillId: fighterUppercutSkillID,
				Signal: "fighter-combo", Count: count, Complete: complete,
			})
		}
		t.fighterCombo = nil
	case packet.OpcodeSkillPrepareReady:
		skill, ok := parsePreparedSkillID(p.Msg)
		if !ok || t.fighterCombo == nil {
			return
		}
		if skill == fighterReverseSkillID {
			cast := t.fighterCombo
			if p.At.UnixMilli() < cast.atMs || (cast.reverseAtMs > 0 && cast.reverseAtMs != p.At.UnixMilli()) {
				cast.invalid = true
				return
			}
			cast.reverseAtMs = p.At.UnixMilli()
		} else if skill != fighterKickSkillID {
			t.fighterCombo = nil
		}
	case packet.OpcodeSkillPrepareEnd:
		cast := t.fighterCombo
		if cast == nil {
			return
		}
		skill, ok := parseEndedSkillID(p.Msg)
		if !ok {
			return
		}
		if skill == fighterUppercutSkillID {
			t.fighterCombo = nil
			return
		}
		if skill != fighterReverseSkillID {
			return
		}
		msg := p.Msg
		cancel := len(msg) == 4 && msg[0].Type() == packet.MessageElemTypeByte && msg[1].Type() == packet.MessageElemTypeByte && msg[2].Type() == packet.MessageElemTypeByte && msg[0].Data().(uint8) == 0 && msg[1].Data().(uint8) == 1 && msg[2].Data().(uint8) == 1
		if cancel && cast.reverseAtMs >= cast.atMs && p.At.UnixMilli() >= cast.reverseAtMs {
			cast.reverseEndAtMs = p.At.UnixMilli()
		} else {
			cast.invalid = true
		}
	case 27017:
		if skill, ok := parsePreparedSkillID(p.Msg); ok && skill == fighterUppercutSkillID && t.fighterCombo != nil {
			t.fighterCombo.normalEndAtMs = p.At.UnixMilli()
		}
	case packet.OpcodeEffectDelayed:
		if t.fighterCombo != nil && isFighterKickReady(p.Msg) && p.At.UnixMilli() >= t.fighterCombo.atMs {
			t.fighterCombo.readyAtMs = p.At.UnixMilli()
		}
	}
}

func fighterExecuteTarget(msg packet.Message) (uint64, bool) {
	if len(msg) != 2 || msg[0].Type() != packet.MessageElemTypeShort || msg[1].Type() != packet.MessageElemTypeLong {
		return 0, false
	}
	id := msg[1].Data().(uint64)
	return id, id != 0
}

func isFighterKickReady(msg packet.Message) bool {
	return len(msg) == 6 && msg[0].Type() == packet.MessageElemTypeInt && msg[1].Type() == packet.MessageElemTypeInt && msg[2].Type() == packet.MessageElemTypeString && msg[3].Type() == packet.MessageElemTypeInt && msg[4].Type() == packet.MessageElemTypeFloat && msg[5].Type() == packet.MessageElemTypeByte && msg[1].Data().(uint32) == 292 && msg[2].Data().(string) == "G16_F_Skill_D_kick_ready" && msg[3].Data().(uint32) == 5000 && msg[5].Data().(uint8) == 0
}

// 44893 carries the current gauge, permanent regeneration and maximum. The
// user's controlled fighter capture confirms maximum 400 and spend costs
// 100/100/200. Read the actual maximum/rate rather than hardcoding them.
func parseFighterEnergyPacket(p *packet.GamePacket) (*event.EventArcanaSignal, bool) {
	msg := p.Msg
	if p.Op == 44893 {
		if len(msg) == 1 && msg[0].Type() == packet.MessageElemTypeInt && msg[0].Data().(uint32) == 0 {
			return &event.EventArcanaSignal{Signal: "fighter-energy-reset"}, true
		}
		want := []packet.MessageElemType{packet.MessageElemTypeFloat, packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeFloat, packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeByte, packet.MessageElemTypeFloat, packet.MessageElemTypeByte}
		if len(msg) != len(want) {
			return nil, false
		}
		for i, field := range msg {
			if field.Type() != want[i] {
				return nil, false
			}
		}
		if msg[1].Data().(uint32) != 1 || msg[2].Data().(uint32) != 30010 || msg[4].Data().(uint32) != math.MaxUint32 || msg[5].Data().(uint32) != 3 || msg[6].Data().(uint8) != 0 || msg[8].Data().(uint8) != 1 {
			return nil, false
		}
		value, rate, maximum := msg[0].Data().(float32), msg[3].Data().(float32), msg[7].Data().(float32)
		if !finiteFighterValue(value) || !finiteFighterValue(rate) || !finiteFighterValue(maximum) || value < 0 || maximum <= 0 || value > maximum || rate < 0 {
			return nil, false
		}
		return &event.EventArcanaSignal{Signal: "fighter-energy-baseline", Kind: 3, Value: value, Rate: rate, Maximum: maximum}, true
	}
	if p.Op == 44892 && len(msg) == 4 && msg[0].Type() == packet.MessageElemTypeInt && msg[1].Type() == packet.MessageElemTypeShort && msg[2].Type() == packet.MessageElemTypeFloat && msg[3].Type() == packet.MessageElemTypeInt && msg[0].Data().(uint32) == 1 && msg[3].Data().(uint32) == 0 {
		if msg[1].Data().(uint16) != 3 {
			return &event.EventArcanaSignal{Signal: "fighter-energy-reset"}, true
		}
		value := msg[2].Data().(float32)
		if finiteFighterValue(value) {
			return &event.EventArcanaSignal{Signal: "fighter-energy-delta", Kind: 3, Value: value}, true
		}
	}
	return nil, false
}

func finiteFighterValue(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func (t *eventPublisher) observeFighterSpend(p *packet.GamePacket) {
	end := func() {
		if cast := t.fighterSpend; cast != nil && p.At.UnixMilli() >= cast.atMs {
			t.publish(&event.EventArcanaSignal{EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}, AtMs: p.At.UnixMilli(), CastAtMs: cast.atMs, SkillId: cast.skillID, Signal: "fighter-spend-end", Complete: true})
		}
		t.fighterSpend = nil
	}
	if p.Op == packet.OpcodeSkillExecute {
		skill, ok := parsePreparedSkillID(p.Msg)
		if !ok {
			return
		}
		end()
		if skill != 59185 && skill != 59186 && skill != 59187 {
			return
		}
		if len(p.Msg) != 4 || p.Msg[1].Type() != packet.MessageElemTypeLong || p.Msg[2].Type() != packet.MessageElemTypeInt || p.Msg[3].Type() != packet.MessageElemTypeInt || p.Msg[2].Data().(uint32) != 0 || p.Msg[3].Data().(uint32) != 1 {
			return
		}
		t.fighterSpend = &fighterSpendCast{atMs: p.At.UnixMilli(), skillID: skill}
		t.publish(&event.EventArcanaSignal{EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}, AtMs: p.At.UnixMilli(), CastAtMs: p.At.UnixMilli(), SkillId: skill, Signal: "fighter-spend-start"})
	} else if p.Op == 27017 {
		if skill, ok := parsePreparedSkillID(p.Msg); ok && t.fighterSpend != nil && skill == t.fighterSpend.skillID {
			end()
		}
	} else if p.Op == packet.OpcodeSkillPrepareEnd {
		if skill, ok := parseEndedSkillID(p.Msg); ok && t.fighterSpend != nil && skill == t.fighterSpend.skillID && !isExecutionPreparationEnd(p.Msg) {
			end()
		}
	}
}
