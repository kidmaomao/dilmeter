package main

import (
	"encoding/json"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestLanceMountFrozenAtRelease(t *testing.T) {
	p := newPartyStingerTestPublisher()
	actor := p.entityCache[101]
	actor.characterConditionMap = map[uint32]*packet.EntityCharacterCondition{975: {CCId: 975, DisableAt: 21, DisableAtMs: 20500}}
	action := func(at int64) *event.EventSkillAction {
		return &event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "101", At: at / 1000}, AtMs: at, SkillId: 59106, SourceId: "101"}
	}
	early, late := action(20100), action(20600)
	p.publish(early)
	p.publish(late)
	if early.LanceMounted == nil || !*early.LanceMounted || late.LanceMounted == nil || *late.LanceMounted {
		t.Fatal("millisecond expiry not applied at release")
	}
	delete(actor.characterConditionMap, 975)
	p.captureLanceMount(early)
	if !*early.LanceMounted {
		t.Fatal("later removal changed an already captured release")
	}
	missing, pet := action(20200), action(20200)
	missing.Id, missing.SourceId = "999", "999"
	pet.SourceId = "103"
	p.captureLanceMount(missing)
	p.captureLanceMount(pet)
	if missing.LanceMounted != nil || pet.LanceMounted != nil {
		t.Fatal("unknown or owned source must not create a caster snapshot")
	}
	raw, err := json.Marshal(late)
	var restored event.EventSkillAction
	if err != nil || json.Unmarshal(raw, &restored) != nil || restored.LanceMounted == nil || *restored.LanceMounted {
		t.Fatal("an observed false must survive JSON export")
	}
}

func TestLancerPublicGainAndPhaseDedup(t *testing.T) {
	p := newPartyStingerTestPublisher()
	for i, skill := range []uint16{59101, 59102, 59103, 59104, 59105, 59106} {
		a := &packet.CombatActionPacket{EntityId: 101, SkillId: skill, SubSkillId: 1, Type: 0x72,
			CombatActionId: uint32(i + 1), Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 200}}
		pack := &packet.CombatActionPackPacket{CombatActionId: a.CombatActionId, SubPackets: []*packet.CombatActionPacket{a}}
		p.publishPartyCombatAction(&packet.GamePacket{At: time.UnixMilli(1000 + int64(i))}, pack)
		p.publishPartyCombatAction(&packet.GamePacket{At: time.UnixMilli(1000 + int64(i))}, pack)
		if skill >= 59105 {
			a.SubSkillId, a.CombatActionId = 2, uint32(i+100)
			p.publishPartyCombatAction(&packet.GamePacket{At: time.UnixMilli(1400 + int64(i))}, pack)
		}
	}
	confirmed := 0
	for _, raw := range p.pendingEvents {
		if a, ok := raw.(*event.EventSkillAction); ok && !a.IsFallback {
			confirmed++
			if a.SubSkillId != 1 {
				t.Fatal("damage phase became another release")
			}
		}
	}
	if confirmed != 6 {
		t.Fatalf("got %d releases, want 6", confirmed)
	}
}

func TestLancerHeatCheckpointBeyondIdleBuffer(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("player", "测试骑士枪", 9001, 100))
	mounted := true
	cast := func(skill uint16, at int64) {
		f.add(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "player", At: at}, AtMs: at * 1000,
			SkillId: skill, SourceId: "player", LanceMounted: &mounted})
	}
	cast(59106, 100)
	cast(59101, 110)
	cast(59102, 250)
	f.add(liveTestAppear("boss", "首领", 7603, 500))
	f.add(liveTestHP("boss", 500))
	f.add(liveTestDamage("boss", 500))
	cast(59106, 501)
	_, rows := f.read("session=" + f.index.current.key)
	var skills []uint16
	for _, row := range rows {
		if row["EventId"] == float64(10) {
			skills = append(skills, uint16(row["SkillId"].(float64)))
			if row["LanceMounted"] != true {
				t.Fatal("checkpoint lost release-time mount state")
			}
		}
	}
	if len(skills) != 4 || skills[0] != 59106 || skills[1] != 59101 || skills[2] != 59102 || skills[3] != 59106 {
		t.Fatalf("full heat cycle not retained: %v", skills)
	}
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 502}, Reset: true})
	if len(f.index.actors) != 0 {
		t.Fatal("connection reset retained prior heat state")
	}
}

func TestLancerHeatCheckpointKeepsIncompleteEvidence(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("player", "测试骑士枪", 9001, 100))
	f.add(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "player", At: 100}, SkillId: 59106, SourceId: "player"})
	f.add(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "player", At: 101}, SkillId: 59102, SourceId: "player", IsFallback: true})
	f.add(liveTestAppear("boss", "首领", 7603, 500))
	f.add(liveTestHP("boss", 500))
	f.add(liveTestDamage("boss", 500))
	_, rows := f.read("session=" + f.index.current.key)
	found := false
	for _, row := range rows {
		if row["EventId"] == float64(10) && row["SkillId"] == float64(59102) && row["IsFallback"] == true {
			found = true
		}
	}
	if !found {
		t.Fatal("checkpoint hid the missing release and would falsely establish a complete heat cycle")
	}
}
