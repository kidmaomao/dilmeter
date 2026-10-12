package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"testing"
	"time"
)

func TestObsHolyConfirmedExecuteNeedsNoLiveTarget(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId = 100
	p.lastSentEventAt = time.Now()
	p.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, RaceId: 10001}}
	// No target entity, HP or damage event exists in this fixture.
	err := p.publishSkillExecutePacket(&packet.GamePacket{Id: 200, At: time.Unix(100, 0), Msg: packet.Message{packet.NewMessageElemShort(59085), packet.NewMessageElemInt(90)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.pendingEvents) != 1 {
		t.Fatalf("confirmed execute lost: %v", p.pendingEvents)
	}
	e, ok := p.pendingEvents[0].(*event.EventSkillAction)
	if !ok || e.SkillId != 59085 || e.IsFallback || e.IsLocal || e.SourceId != "200" {
		t.Fatalf("incorrect confirmed event: %+v", e)
	}
}

func TestObsHolyPublicFallbackCarriesEvidenceWithoutPromotion(t *testing.T) {
	for _, target := range []uint64{0, 400} {
		p := newSkillTestPublisher()
		p.localEntityId = 100
		p.lastSentEventAt = time.Now()
		p.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, RaceId: 10001}}
		flags := packet.CombatActionType(packet.CombatActionTypeAttacker | packet.CombatActionTypeSkillActive | packet.CombatActionTypeSkillSuccess | packet.CombatActionTypeSkillPlayerCharacter)
		p.publishPartyCombatAction(&packet.GamePacket{At: time.Unix(100, 0)}, &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{{EntityId: 200, SkillId: 59085, CombatActionId: 90, Type: flags, Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: target}}}})
		if len(p.pendingEvents) != 1 {
			t.Fatal("public diagnostic missing")
		}
		e := p.pendingEvents[0].(*event.EventSkillAction)
		if !e.IsFallback || e.ObservedActionType != uint8(flags) || e.ObservedTargetId == "" {
			t.Fatalf("fallback promoted or evidence lost: %+v", e)
		}
	}
}
