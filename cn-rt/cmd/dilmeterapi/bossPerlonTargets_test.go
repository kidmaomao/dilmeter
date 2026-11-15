package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestPerlonTargetSnapshotAndClearing(t *testing.T) {
	p, batches := perlonPacketTestPublisher()
	for id, race := range map[uint64]uint32{100: perlonRaceID, 101: perlonRaceID, 200: 10001, 201: 8001, 300: 1234} {
		p.entityCache[id] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: id, RaceId: race}}
	}
	selectTarget := func(actor, target uint64, at int64) {
		p.publishPerlonPacket(&packet.GamePacket{Id: actor, Op: packet.OpcodeSetCombatTarget, At: time.UnixMilli(at), Msg: packet.Message{packet.NewMessageElemLong(target)}})
	}
	deploy := func(boss uint64, at int64) *event.EventSkillAction {
		p.publishPerlonPacket(&packet.GamePacket{Id: boss, Op: 37011, At: time.UnixMilli(at), Msg: packet.Message{packet.NewMessageElemInt(967), packet.NewMessageElemInt(1), packet.NewMessageElemLong(uint64(at)), packet.NewMessageElemFloat(97780), packet.NewMessageElemFloat(137046), packet.NewMessageElemFloat(300), packet.NewMessageElemInt(1000), packet.NewMessageElemInt(3000)}})
		select {
		case batch := <-batches:
			return batch[len(batch)-1].(*event.EventSkillAction)
		default:
			t.Fatal("deployment did not flush immediately")
			return nil
		}
	}
	selectTarget(100, 200, 100000)
	first := deploy(100, 110000) // A standing selection is valid until explicitly changed.
	selectTarget(100, 201, 111000)
	selectTarget(200, 300, 112000) // Other actors cannot change Perlon's target.
	selectTarget(100, 200, 100000) // Older packets cannot replace a later selection.
	second := deploy(100, 113000)
	if first.MechanicTargetId != "200" || second.MechanicTargetId != "201" {
		t.Fatalf("target snapshots: %+v %+v", first, second)
	}
	if deploy(101, 114000).MechanicTargetId != "" {
		t.Fatal("different boss inherited selection")
	}
	selectTarget(100, 300, 115000)
	if deploy(100, 116000).MechanicTargetId != "" {
		t.Fatal("monster classified as teammate")
	}
	selectTarget(100, 999, 117000)
	if deploy(100, 118000).MechanicTargetId != "" {
		t.Fatal("unknown target classified as teammate")
	}
	selectTarget(100, 0, 119000)
	if deploy(100, 120000).MechanicTargetId != "" {
		t.Fatal("clear retained target")
	}
	selectTarget(100, 200, 121000)
	p.publishPerlonPacket(&packet.GamePacket{Id: 999, Op: packet.OpcodeEntityDisappear, At: time.UnixMilli(122000), Msg: packet.Message{packet.NewMessageElemLong(200)}})
	if deploy(100, 123000).MechanicTargetId != "" {
		t.Fatal("departed target retained")
	}
	selectTarget(100, 201, 124000)
	p.beginConnectionEpoch(2, time.UnixMilli(125000))
	if p.perlonTarget != (perlonTornadoTarget{}) {
		t.Fatal("reconnect retained target")
	}
}

func TestPerlonTornadoMonitoringCombinations(t *testing.T) {
	for _, self := range []bool{false, true} {
		for _, teammates := range []bool{false, true} {
			for _, target := range []string{"self", "teammate", ""} {
				var sounds []nativeReminderSoundRequest
				r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{Rules: map[string]nativeBossMechanicRule{
					perlonTornadoKey: {TornadoMonitoring: &nativePerlonTornadoMonitoring{Self: self, Teammates: teammates}, ShowCountdown: true, SoundMode: "dedicated"},
				}}}, &sounds)
				r.localID = "self"
				r.onEvent(liveTestAppear("boss", "佩隆", perlonRaceID, 100))
				action := &event.EventSkillAction{EventBase: event.EventBase{Id: "boss", At: 101}, AtMs: 101000, MechanicSignal: perlonTornadoKey, MechanicDurationMs: 1000, MechanicTargetId: target}
				r.onEvent(action)
				r.onEvent(action)
				want := target == "self" && self || target == "teammate" && teammates || target == "" && self && teammates
				if (len(r.perlonOverlay(time.UnixMilli(101200))) == 1) != want || (len(sounds) == 1) != want {
					t.Fatalf("self=%v teammates=%v target=%q got sounds=%d items=%v", self, teammates, target, len(sounds), r.perlonOverlay(time.UnixMilli(101200)))
				}
				if (len(r.perlonOverlay(time.UnixMilli(103999))) == 1) != want {
					t.Fatal("post-start dodge warning lost target filtering")
				}
				if len(r.perlonOverlay(time.UnixMilli(104000))) != 0 {
					t.Fatal("expired tornado retained")
				}
			}
		}
	}
}

func TestPerlonTornadoLegacyMonitoringMigration(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := normalizeNativeReminderSettings(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{Rules: map[string]nativeBossMechanicRule{perlonTornadoKey: {Enabled: enabled}}}})
		rule := s.BossMechanics.Rules[perlonTornadoKey]
		if rule.Enabled != enabled || rule.TornadoMonitoring == nil || rule.TornadoMonitoring.Self != enabled || rule.TornadoMonitoring.Teammates != enabled {
			t.Fatalf("legacy migration %+v", rule)
		}
	}
}
