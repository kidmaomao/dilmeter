package main

import (
	"strconv"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const (
	healerWarpOpcode          = 26009
	healerRaidReadyOpcode     = 44790
	healerRegionProfileOpcode = 110000
	healerTransferWindow      = 4 * time.Second
)

type healerRegionTracker struct {
	localID     uint64
	region      uint32
	warpAt      time.Time
	raidReadyAt time.Time
	instance    uint32
	instanceAt  time.Time
	dungeon     bool
}

// 26009 confirms the local destination. Pair the transfer with the raid
// readiness roster (44790/1) or the destination's instance profile (110000)
// seen during map load.
func (t *eventPublisher) publishHealerRegion(p *packet.GamePacket) {
	switch p.Op {
	case healerWarpOpcode, healerRaidReadyOpcode, healerRegionProfileOpcode:
	default:
		return
	}
	if t.localEntityId == 0 || !t.localEntityReliable {
		return
	}
	tracker := &t.healerRegion
	if tracker.localID != t.localEntityId {
		*tracker = healerRegionTracker{localID: t.localEntityId}
	}
	msg := p.Msg
	switch p.Op {
	case healerRaidReadyOpcode:
		if p.Id == tracker.localID && healerRaidReady(msg) {
			tracker.raidReadyAt = p.At
		}
		return
	case healerRegionProfileOpcode:
		if !healerPacketTypes(msg, packet.MessageElemTypeByte, packet.MessageElemTypeInt, packet.MessageElemTypeByte, packet.MessageElemTypeInt, packet.MessageElemTypeByte) ||
			msg[3].Data().(uint32) != 0x80000001 {
			return
		}
		tracker.instance, tracker.instanceAt = msg[1].Data().(uint32), p.At
		if tracker.dungeon || tracker.instance != tracker.region || !healerInstanceRegion(tracker.region) || !healerSameTransfer(tracker.warpAt, p.At) {
			return
		}
		tracker.dungeon = true
	case healerWarpOpcode:
		if p.Id != tracker.localID ||
			!healerPacketTypes(msg, packet.MessageElemTypeByte, packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeInt) ||
			msg[0].Data().(uint8) != 1 {
			return
		}
		region := msg[1].Data().(uint32)
		if region == 0 {
			return
		}
		tracker.dungeon = healerInstanceRegion(region) &&
			(tracker.region == region && tracker.dungeon ||
				healerSameTransfer(tracker.raidReadyAt, p.At) ||
				tracker.instance == region && healerSameTransfer(tracker.instanceAt, p.At))
		tracker.region, tracker.warpAt, tracker.raidReadyAt = region, p.At, time.Time{}
	}
	t.publish(&event.EventLocalRegion{
		EventBase: event.EventBase{EventId: event.EventIdLocalRegion, At: p.At.Unix(), Id: strconv.FormatUint(tracker.localID, 10)},
		RegionID:  tracker.region,
		Dungeon:   tracker.dungeon,
	})
}

func healerInstanceRegion(region uint32) bool {
	// Raid staging and combat regions in the observed 35000-series instances.
	return region >= 35000 && region < 36000
}

func healerSameTransfer(earlier, later time.Time) bool {
	return !earlier.IsZero() && !later.Before(earlier) && later.Sub(earlier) <= healerTransferWindow
}

func healerPacketTypes(msg packet.Message, types ...packet.MessageElemType) bool {
	if len(msg) != len(types) {
		return false
	}
	for i, typ := range types {
		if msg[i].Type() != typ {
			return false
		}
	}
	return true
}

func healerRaidReady(msg packet.Message) bool {
	if len(msg) < 6 || !healerPacketTypes(msg[:6], packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeByte, packet.MessageElemTypeInt) ||
		msg[0].Data().(uint32) != 1 || msg[1].Data().(uint32) != 1 || msg[2].Data().(uint32) == 0 {
		return false
	}
	count := uint64(msg[5].Data().(uint32))
	if count*2 != uint64(len(msg)-6) {
		return false
	}
	for i := 6; i < len(msg); i += 2 {
		if !healerPacketTypes(msg[i:i+2], packet.MessageElemTypeLong, packet.MessageElemTypeByte) {
			return false
		}
	}
	return true
}
