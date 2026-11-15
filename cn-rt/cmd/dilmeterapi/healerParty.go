package main

import (
	"strconv"
	"strings"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const (
	healerPartyCreateReply = 36501
	healerPartySlotUpdate  = 36502
	healerPartyJoinReply   = 36504
	healerPartyMemberJoin  = 36505
	healerPartyLeaveReply  = 36507
	healerPartyMemberLeave = 36508
	healerPartyRemoved     = 36511
	healerMaxPartySlots    = 32
)

type healerPartyTracker struct {
	localID uint64
	partyID uint64
	slots   [healerMaxPartySlots + 1]event.PartyMember
}

// Game party slots are carried by 36502 and 36505. The 44790 ready-check
// participants may arrive in a different order and do not define these slots.
func (t *eventPublisher) publishHealerParty(p *packet.GamePacket) {
	switch p.Op {
	case healerPartyCreateReply, healerPartyJoinReply, healerPartySlotUpdate, healerPartyMemberJoin,
		healerPartyLeaveReply, healerPartyMemberLeave, healerPartyRemoved:
	default:
		return
	}
	localID := t.localEntityId
	if localID == 0 || !t.localEntityReliable || p.Id != localID && p.Id != 0 {
		return
	}
	tracker := t.healerParty
	if tracker.localID != localID {
		tracker = healerPartyTracker{localID: localID}
	}
	msg := p.Msg
	switch p.Op {
	case healerPartyCreateReply, healerPartyJoinReply:
		if p.Id != localID {
			return
		}
		partyID, slots, ok := readHealerPartyRoster(msg, localID, p.At.UnixMilli())
		if !ok {
			return
		}
		tracker.partyID, tracker.slots = partyID, slots
	case healerPartySlotUpdate:
		if len(msg) < 2 || msg[0].Type() != packet.MessageElemTypeLong {
			return
		}
		member, ok := readHealerPartyMember(msg[1:])
		if !ok || msg[0].Data().(uint64) != member.id {
			return
		}
		tracker.setMember(member, p.At.UnixMilli())
	case healerPartyMemberJoin:
		member, ok := readHealerPartyMember(msg)
		if !ok {
			return
		}
		tracker.setMember(member, p.At.UnixMilli())
	case healerPartyMemberLeave:
		if !healerPacketTypes(msg, packet.MessageElemTypeLong) {
			return
		}
		id := msg[0].Data().(uint64)
		if id == localID {
			tracker = healerPartyTracker{localID: localID}
		} else {
			tracker.removeMember(strconv.FormatUint(id, 10))
		}
	case healerPartyLeaveReply:
		if p.Id != localID || !healerPacketTypes(msg, packet.MessageElemTypeByte) || msg[0].Data().(uint8) != 1 {
			return
		}
		tracker = healerPartyTracker{localID: localID}
	case healerPartyRemoved:
		if p.Id != localID || len(msg) != 0 {
			return
		}
		tracker = healerPartyTracker{localID: localID}
	}
	if tracker == t.healerParty {
		return
	}
	t.healerParty = tracker
	members := []event.PartyMember{}
	for _, member := range tracker.slots {
		if member.ID != "" {
			members = append(members, member)
		}
	}
	partyID := ""
	if tracker.partyID != 0 {
		partyID = strconv.FormatUint(tracker.partyID, 10)
	}
	t.publish(&event.EventPartyRoster{
		EventBase: event.EventBase{EventId: event.EventIdPartyRoster, At: p.At.Unix(), Id: strconv.FormatUint(localID, 10)},
		PartyID:   partyID,
		Members:   members,
	})
}

type healerPacketPartyMember struct {
	healthPercent int
	healthKnown   bool
	id            uint64
	slot          int
	name          string
}

// The player record begins with slot, actor ID, name, presence and location.
// Its trailing status payload is not needed to identify the member.
func readHealerPartyMember(msg packet.Message) (healerPacketPartyMember, bool) {
	const prefixLength = 7
	if len(msg) < prefixLength || !healerPacketTypes(msg[:prefixLength], packet.MessageElemTypeInt,
		packet.MessageElemTypeLong, packet.MessageElemTypeString, packet.MessageElemTypeByte,
		packet.MessageElemTypeInt, packet.MessageElemTypeInt, packet.MessageElemTypeInt) {
		return healerPacketPartyMember{}, false
	}
	slot := int(msg[0].Data().(uint32))
	id := msg[1].Data().(uint64)
	name := strings.TrimSpace(msg[2].Data().(string))
	if slot < 1 || slot > healerMaxPartySlots || id == 0 || name == "" || len([]rune(name)) > 100 ||
		msg[3].Data().(uint8) > 1 {
		return healerPacketPartyMember{}, false
	}
	member := healerPacketPartyMember{id: id, slot: slot, name: name}
	// Party status carries life and wound percentages after the position.
	// Use that life reading as a display fallback before a full stat update.
	if len(msg) >= 10 && msg[3].Data().(uint8) == 1 &&
		healerPacketTypes(msg[7:10], packet.MessageElemTypeByte, packet.MessageElemTypeInt, packet.MessageElemTypeInt) {
		life, maximum := msg[8].Data().(uint32), msg[9].Data().(uint32)
		if life <= 100 && maximum <= 100 {
			member.healthPercent, member.healthKnown = int(life), true
		}
	}
	return member, true
}

func (tracker *healerPartyTracker) removeMember(id string) {
	for i, member := range tracker.slots {
		if member.ID == id {
			tracker.slots[i] = event.PartyMember{}
		}
	}
}

func (tracker *healerPartyTracker) setMember(member healerPacketPartyMember, atMs int64) {
	id := strconv.FormatUint(member.id, 10)
	tracker.removeMember(id)
	player := event.PartyMember{ID: id, Slot: member.slot, Name: member.name}
	if member.healthKnown {
		player.HealthPercent, player.HealthAtMs = member.healthPercent, atMs
	}
	tracker.slots[member.slot] = player
}

func readHealerPartyRoster(msg packet.Message, localID uint64, atMs int64) (uint64, [healerMaxPartySlots + 1]event.PartyMember, bool) {
	var roster healerPartyTracker
	if len(msg) < 20 || !healerPacketTypes(msg[:4], packet.MessageElemTypeByte,
		packet.MessageElemTypeLong, packet.MessageElemTypeString, packet.MessageElemTypeLong) ||
		msg[0].Data().(uint8) != 1 || msg[1].Data().(uint64) == 0 {
		return 0, roster.slots, false
	}
	namesAt := 10
	if msg[namesAt].Type() == packet.MessageElemTypeInt {
		namesAt++ // The current CN roster has an extra field before the two strings.
	}
	if !healerPacketTypes(msg[namesAt:namesAt+3], packet.MessageElemTypeString, packet.MessageElemTypeString, packet.MessageElemTypeInt) {
		return 0, roster.slots, false
	}
	count := int(msg[namesAt+2].Data().(uint32))
	if count < 1 || count > healerMaxPartySlots {
		return 0, roster.slots, false
	}
	ids := make(map[uint64]bool, count)
	for i := namesAt + 3; i < len(msg); i++ {
		member, ok := readHealerPartyMember(msg[i:])
		if !ok {
			continue
		}
		if ids[member.id] || roster.slots[member.slot].ID != "" {
			return 0, roster.slots, false
		}
		ids[member.id] = true
		roster.setMember(member, atMs)
		i += 6 // Advance past this record's checked prefix.
	}
	return msg[1].Data().(uint64), roster.slots, len(ids) == count && ids[localID] && ids[msg[3].Data().(uint64)]
}
