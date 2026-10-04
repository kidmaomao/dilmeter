package main

import (
	"math"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// Lightning Chain is an independently enabled skill. Its effect 814 carries
// the actual player's on/off state; target CC 948 is not that lifetime.
// Preserve transitions rather than assuming a fixed duration or cooldown.
func (t *eventPublisher) publishDarkMagePacket(p *packet.GamePacket) {
	if p == nil || t.localEntityId == 0 || !t.partySignalActor(p.Id) || p.Op != 37011 {
		return
	}
	msg := p.Msg
	if len(msg) < 2 || msg[0].Type() != packet.MessageElemTypeInt || msg[0].Data().(uint32) != 814 || msg[1].Type() != packet.MessageElemTypeInt {
		return
	}
	active := msg[1].Data().(uint32)
	var target uint64
	var distance float32
	if active == 1 {
		if len(msg) != 4 || msg[2].Type() != packet.MessageElemTypeLong || msg[3].Type() != packet.MessageElemTypeFloat {
			return
		}
		target, distance = msg[2].Data().(uint64), msg[3].Data().(float32)
		if target == 0 || distance <= 0 || math.IsNaN(float64(distance)) || math.IsInf(float64(distance), 0) {
			return
		}
	} else if active != 0 || len(msg) != 2 {
		return
	}
	signal := &event.EventArcanaSignal{
		EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)},
		AtMs:      p.At.UnixMilli(), SkillId: 59041, Signal: "lightning-chain-state", Kind: 814, Count: active, Complete: true, Range: distance,
	}
	if target != 0 {
		signal.TargetId = strconv.FormatUint(target, 10)
	}
	t.publish(signal)
}
