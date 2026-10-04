package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestPuppeteerCompleteCharge(t *testing.T) {
	for _, scenario := range []string{"empty", "hit-boss", "hit-other-target", "partial", "no-end", "wrong-owner", "cancelled", "owner-ended-early", "reset"} {
		t.Run(scenario, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
			owner := uint64(100)
			if scenario == "wrong-owner" {
				owner = 101
			}
			publisher.entityCache.add(&packet.EntityInfo{Id: 300, OwnerId: owner}, time.UnixMilli(1000))
			publish := func(id uint64, op packet.OpCode, at int64, msg packet.Message) {
				publisher.publishPuppeteerPacket(&packet.GamePacket{Id: id, Op: op, At: time.UnixMilli(at), Msg: msg})
			}
			publish(100, packet.OpcodeSkillExecute, 1000, packet.Message{packet.NewMessageElemShort(54105), packet.NewMessageElemLong(200), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1), packet.NewMessageElemFloat(0), packet.NewMessageElemFloat(0), packet.NewMessageElemFloat(0), packet.NewMessageElemFloat(1)})
			publish(300, packet.OpcodeSkillExecute, 1100, packet.Message{packet.NewMessageElemShort(54155), packet.NewMessageElemLong(0)})
			if scenario == "owner-ended-early" {
				publish(100, 27017, 1200, packet.Message{packet.NewMessageElemShort(54105)})
			}
			if scenario == "reset" {
				publisher.beginConnectionEpoch(2, time.UnixMilli(1200))
			}
			for i := uint32(1); i <= 3; i++ {
				if scenario == "partial" && i == 3 {
					break
				}
				pack := &packet.CombatActionPackPacket{CombatActionId: i, SubPackets: []*packet.CombatActionPacket{{EntityId: 300, SkillId: 54155, Attacker: &packet.CombatActionPacketAttackerInfo{}}}}
				if i == 2 && (scenario == "hit-boss" || scenario == "hit-other-target") {
					target := uint64(200)
					if scenario == "hit-other-target" {
						target = 201
					}
					pack.SubPackets = append(pack.SubPackets, &packet.CombatActionPacket{EntityId: target, Hit: &packet.CombatActionPacketHitInfo{Damage: 100}})
				}
				p := &packet.GamePacket{At: time.UnixMilli(1300 + int64(i)*300)}
				publisher.observePuppeteerCombat(p, pack)
				publisher.observePuppeteerCombat(p, pack) // repeated action ID is not another attempt
			}
			if scenario != "no-end" {
				op := packet.OpCode(27017)
				if scenario == "cancelled" {
					op = packet.OpcodeSkillPrepareEnd
				}
				publish(300, op, 2500, packet.Message{packet.NewMessageElemShort(54155)})
				publish(300, 27017, 2501, packet.Message{packet.NewMessageElemShort(54155)})
			}
			samples := 0
			for _, raw := range publisher.pendingEvents {
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "act7-sample" {
					samples++
					empty := uint32(1)
					if scenario == "hit-boss" || scenario == "hit-other-target" {
						empty = 0
					}
					if signal.Count != empty || signal.Phase != 3 || signal.TargetId != "200" || signal.Id != "100" || signal.ObjectIds[0] != "300" {
						t.Fatalf("bad charge sample: %#v", signal)
					}
				}
			}
			expected := 0
			if scenario == "empty" || scenario == "hit-boss" || scenario == "hit-other-target" || scenario == "owner-ended-early" {
				expected = 1
			}
			if samples != expected {
				t.Fatalf("samples=%d want=%d", samples, expected)
			}
		})
	}
}

func TestReplayPuppeteerKpiCapture(t *testing.T) {
	file := os.Getenv("DILMETER_PUPPETEER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PUPPETEER_TEST_PCAP to replay the reworked puppet capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	ch := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, ch)
	var encoder *json.Encoder
	if destination := os.Getenv("DILMETER_PUPPETEER_TEST_EVENTS"); destination != "" {
		output, err := os.Create(destination)
		if err != nil {
			t.Fatal(err)
		}
		defer output.Close()
		encoder = json.NewEncoder(output)
	}
	if err = reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	last := time.Now()
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	samples, empty := 0, uint32(0)
	ownerCharges := 0
	var interludeCounts []uint32
	for {
		select {
		case batch := <-ch:
			last = time.Now()
			for _, raw := range batch {
				if encoder != nil {
					if err = encoder.Encode(raw); err != nil {
						t.Fatal(err)
					}
				}
				if action, ok := raw.(*event.EventSkillAction); ok && action.SkillId == 54105 && action.SourceId == "4503599631566088" && !action.IsFallback && action.MechanicSignal == "" {
					ownerCharges++
				}
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "act7-sample" {
					if signal.TargetId != "4767482421050579" || signal.Phase != 3 || !signal.Complete {
						t.Fatalf("invalid charge: %#v", signal)
					}
					samples++
					empty += signal.Count
				}
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "interlude-sample" {
					if signal.TargetId != "4767482421050579" || !signal.Complete {
						t.Fatalf("invalid interlude: %#v", signal)
					}
					interludeCounts = append(interludeCounts, signal.Count)
				}
			}
		case <-timer.C:
			if samples > 0 && time.Since(last) > time.Second {
				_, _, errors := reader.GetStats()
				if errors != 0 || samples != 7 || empty != 2 || ownerCharges != 7 {
					t.Fatalf("samples=%d empty=%d ownerCharges=%d decode errors=%d", samples, empty, ownerCharges, errors)
				}
				expected := []uint32{1, 3, 3, 3, 2, 2}
				if len(interludeCounts) != len(expected) {
					t.Fatalf("interlude counts=%v", interludeCounts)
				}
				for i, count := range expected {
					if interludeCounts[i] != count {
						t.Fatalf("interlude counts=%v", interludeCounts)
					}
				}
				t.Logf("completed Act 7 charges=%d empty=%d", samples, empty)
				t.Logf("Interlude participating puppets=%v", interludeCounts)
				return
			}
		case <-ctx.Done():
			t.Fatal("puppeteer capture replay timed out")
		}
	}
}

func TestInterludeParticipatingPuppets(t *testing.T) {
	for _, scenario := range []string{"complete", "positions-after-ack", "partial", "duplicate-position", "stale-position", "cancelled", "other-owner", "next-skill", "reset"} {
		t.Run(scenario, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
			publish := func(op packet.OpCode, at int64, msg packet.Message) {
				publisher.publishPuppeteerPacket(&packet.GamePacket{Id: 100, Op: op, At: time.UnixMilli(at), Msg: msg})
			}
			execute := func() {
				publish(packet.OpcodeSkillExecute, 1000, packet.Message{packet.NewMessageElemShort(59165), packet.NewMessageElemLong(0)})
			}
			if scenario == "positions-after-ack" {
				execute()
			}
			positions := 3
			if scenario == "duplicate-position" {
				positions = 4
			}
			at := int64(1000)
			if scenario == "stale-position" {
				at = 999
			}
			for i := 0; i < positions; i++ {
				// Puppets can occupy identical coordinates. Count confirmed
				// feedback entries, rather than distinct positions or targets.
				publish(37011, at, packet.Message{packet.NewMessageElemInt(959), packet.NewMessageElemInt(1), packet.NewMessageElemFloat(100), packet.NewMessageElemFloat(200)})
			}
			if scenario != "positions-after-ack" {
				execute()
			}
			if scenario == "next-skill" {
				publish(packet.OpcodeSkillPrepareReady, 1100, packet.Message{packet.NewMessageElemShort(54101)})
			}
			if scenario == "reset" {
				publisher.beginConnectionEpoch(2, time.UnixMilli(1100))
			}
			for i := uint32(1); i <= 3; i++ {
				if scenario == "partial" && i == 3 {
					break
				}
				caster := uint64(100)
				if scenario == "other-owner" {
					caster = 101
				}
				pack := &packet.CombatActionPackPacket{CombatActionId: i, SubPackets: []*packet.CombatActionPacket{
					{EntityId: caster, SkillId: 59165, Attacker: &packet.CombatActionPacketAttackerInfo{}},
					{EntityId: 200, Hit: &packet.CombatActionPacketHitInfo{Damage: 100}},
					{EntityId: 201, Hit: &packet.CombatActionPacketHitInfo{Damage: 100}},
				}}
				p := &packet.GamePacket{At: time.UnixMilli(1100 + int64(i))}
				publisher.observePuppeteerCombat(p, pack)
				publisher.observePuppeteerCombat(p, pack)
			}
			end := packet.OpCode(27017)
			if scenario == "cancelled" {
				end = packet.OpcodeSkillPrepareEnd
			}
			publish(end, 1400, packet.Message{packet.NewMessageElemShort(59165)})
			publish(27017, 1401, packet.Message{packet.NewMessageElemShort(59165)})
			samples := 0
			for _, raw := range publisher.pendingEvents {
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "interlude-sample" {
					samples++
					if signal.Count != 3 || !signal.Complete {
						t.Fatalf("target/damage count used instead of puppet positions: %#v", signal)
					}
				}
			}
			expected := 0
			if scenario == "complete" || scenario == "positions-after-ack" {
				expected = 2
			}
			if samples != expected {
				t.Fatalf("samples=%d want=%d", samples, expected)
			}
		})
	}
}
