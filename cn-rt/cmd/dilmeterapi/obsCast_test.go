package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestObsExecuteIndependentOfCooldownDedupe(t *testing.T) {
	for _, skill := range []uint16{59005, 59047, 59064, 59085, 59106, 59123, 59143, 59163, 59187} {
		for _, actor := range []uint64{100, 200} {
			p := newSkillTestPublisher()
			p.localEntityId = 100
			p.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, RaceId: 10001}}
			p.lastSentEventAt = time.Now()
			// An earlier fallback consumed this cooldown action ID. It must not
			// swallow a real animation release, including a later reused token.
			p.acceptSkillAction(99)
			for i := 0; i < 2; i++ {
				p.publishObsExecute(&packet.GamePacket{Id: actor, At: time.Unix(100+int64(i)*60, 0), Msg: packet.Message{packet.NewMessageElemShort(skill), packet.NewMessageElemInt(99)}})
			}
			if len(p.pendingEvents) != 2 {
				t.Fatalf("skill %d actor %d: missing release: %+v", skill, actor, p.pendingEvents)
			}
			for _, raw := range p.pendingEvents {
				e := raw.(*event.EventSkillState)
				if e.Scope != "obs-execute" || e.SkillId != skill || !e.Active {
					t.Fatalf("wrong release: %+v", e)
				}
			}
		}
	}
}

func TestObsPublicReleaseRejectsHitPhasesAndOwnedSources(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId = 100
	p.lastSentEventAt = time.Now()
	p.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, RaceId: 10001}}
	p.entityCache[300] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 300, RaceId: 10001, OwnerId: 100}}
	at := &packet.GamePacket{At: time.Unix(100, 0)}
	for _, actor := range []uint64{100, 200, 300, 400} {
		for _, skill := range []uint16{59005, 59047, 59064, 59085, 59106, 59123, 59143, 59163, 59187, 59169, 20001} {
			before := len(p.pendingEvents)
			a := &packet.CombatActionPacket{EntityId: actor, SkillId: skill, Type: 0x72, Attacker: &packet.CombatActionPacketAttackerInfo{}}
			// No target entity or damage is required, and multiple records in a
			// pack remain one release per player/skill.
			p.publishObsCombat(at, &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{a, a}})
			want := 0
			if actor <= 200 && obsUltimateSkill(skill) {
				want = 1
			}
			if len(p.pendingEvents)-before != want {
				t.Fatalf("actor %d skill %d", actor, skill)
			}
		}
	}
	before := len(p.pendingEvents)
	for _, flags := range []packet.CombatActionType{0x42, 0x52, 0x62, 0x32, 1} {
		p.publishObsCombat(at, &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{{EntityId: 200, SkillId: 59085, Type: flags, Attacker: &packet.CombatActionPacketAttackerInfo{}}}})
	}
	p.publishObsCombat(at, &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{{EntityId: 200, SkillId: 59106, SubSkillId: 2, Type: 0x72, Attacker: &packet.CombatActionPacketAttackerInfo{}}, {EntityId: 200, SkillId: 59085, Type: 0x72, Attacker: &packet.CombatActionPacketAttackerInfo{}, Hit: &packet.CombatActionPacketHitInfo{}}}})
	p.publishObsExecute(&packet.GamePacket{Id: 300, At: at.At, Msg: packet.Message{packet.NewMessageElemShort(59163)}})
	if len(p.pendingEvents) != before {
		t.Fatal("hit/late phase/pet became a release")
	}
}
