package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestParseSkillCooldownResetAndReductions(t *testing.T) {
	skillID, err := parseSkillCooldownReset(packet.Message{
		packet.NewMessageElemShort(26002), packet.NewMessageElemByte(0),
	})
	if err != nil || skillID != 26002 {
		t.Fatalf("reset = %d, %v; want skill 26002", skillID, err)
	}
	reductions, err := parseSkillCooldownReductions(packet.Message{
		packet.NewMessageElemInt(3),
		packet.NewMessageElemShort(24101), packet.NewMessageElemInt(800),
		packet.NewMessageElemShort(24301), packet.NewMessageElemInt(1000),
		packet.NewMessageElemShort(59180), packet.NewMessageElemInt(1800),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []skillCooldownReduction{
		{SkillID: 24101, ReduceMs: 800},
		{SkillID: 24301, ReduceMs: 1000},
		{SkillID: 59180, ReduceMs: 1800},
	}
	if len(reductions) != len(want) {
		t.Fatalf("reductions = %#v", reductions)
	}
	for index := range want {
		if reductions[index] != want[index] {
			t.Fatalf("reduction %d = %#v, want %#v", index, reductions[index], want[index])
		}
	}
}

func TestParseAstrologyCooldownSignalUsesConfiguredSkillSeconds(t *testing.T) {
	for skillID, wantMs := range map[uint16]uint32{27201: 3000, 27202: 2000, 27203: 2000, 27204: 3000} {
		msg := packet.Message{
			packet.NewMessageElemByte(2), packet.NewMessageElemShort(1), packet.NewMessageElemByte(1),
			packet.NewMessageElemString("SLST" + strconvSkillID(skillID)),
			packet.NewMessageElemByte(4), packet.NewMessageElemLong(1788267450123),
		}
		gotSkillID, token, reduceMs, ok := parseAstrologyCooldownSignal(msg)
		if !ok || gotSkillID != skillID || token != 1788267450123 || reduceMs != wantMs {
			t.Fatalf("SLST%d = (%d, %d, %d, %t), want reduction %d", skillID, gotSkillID, token, reduceMs, ok, wantMs)
		}
	}
}

func TestPublishAstrologyCooldownSignalDeduplicatesToken(t *testing.T) {
	publisher := &eventPublisher{
		localEntityId:         99,
		recentCooldownSignals: make(map[uint16]uint64),
		lastSentEventAt:       time.Now(),
		pendingEvents:         make([]event.IEvent, 0, 4),
	}
	packet_ := &packet.GamePacket{
		At: time.UnixMilli(1788267450123), Id: 99,
		Msg: packet.Message{
			packet.NewMessageElemByte(2), packet.NewMessageElemShort(1), packet.NewMessageElemByte(1),
			packet.NewMessageElemString("SLST27203"), packet.NewMessageElemByte(4), packet.NewMessageElemLong(1788267450123),
		},
	}
	if err := publisher.publishAstrologyCooldownPacket(packet_); err != nil {
		t.Fatal(err)
	}
	if err := publisher.publishAstrologyCooldownPacket(packet_); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("published %d events, want one", len(publisher.pendingEvents))
	}
	adjustment, ok := publisher.pendingEvents[0].(*event.EventSkillCooldown)
	if !ok || adjustment.SkillId != 27203 || adjustment.ReduceMs != 2000 || adjustment.Reset || adjustment.Signal != "SLST" {
		t.Fatalf("astrology adjustment = %#v", publisher.pendingEvents[0])
	}
}

func strconvSkillID(skillID uint16) string {
	return string([]byte{
		byte('0' + skillID/10000), byte('0' + skillID/1000%10), byte('0' + skillID/100%10),
		byte('0' + skillID/10%10), byte('0' + skillID%10),
	})
}
