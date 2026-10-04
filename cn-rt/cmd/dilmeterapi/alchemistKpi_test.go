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

func TestChemicalRawCountersAndOwnership(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
	msg := packet.Message{packet.NewMessageElemInt(850), packet.NewMessageElemInt(924), packet.NewMessageElemLong(200), packet.NewMessageElemInt(350), packet.NewMessageElemInt(150), packet.NewMessageElemInt(5), packet.NewMessageElemInt(150)}
	p := &packet.GamePacket{Id: 100, Op: packet.OpcodeEffectDelayed, At: time.UnixMilli(1000), Msg: msg}
	publisher.publishAlchemistPacket(p)
	p.Id = 999
	publisher.publishAlchemistPacket(p)
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("other player's chemical effect included")
	}
	signal := publisher.pendingEvents[0].(*event.EventArcanaSignal)
	if signal.Count != 5 || signal.Complete || signal.TargetId != "200" {
		t.Fatalf("raw effect relabelled as additional attack: %#v", signal)
	}
	if _, ok := parseChemicalEffectCount(msg[:6]); ok {
		t.Fatal("truncated feedback accepted")
	}
	unrelated := append(packet.Message{}, msg...)
	unrelated[1] = packet.NewMessageElemInt(925)
	if _, ok := parseChemicalEffectCount(unrelated); ok {
		t.Fatal("another effect treated as chemical")
	}
	pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{
		{EntityId: 100, SkillId: 59144, Attacker: &packet.CombatActionPacketAttackerInfo{}},
		{EntityId: 200, Hit: &packet.CombatActionPacketHitInfo{Options: packet.CombatActionHitOptionsMultiHit, MultiHitCount: 5}},
	}}
	publisher.publishAlchemistHitCounts(p, pack)
	if len(publisher.pendingEvents) != 2 || publisher.pendingEvents[1].(*event.EventArcanaSignal).Count != 5 {
		t.Fatal("bundled five-hit count lost")
	}
	pack.SubPackets[0].EntityId = 999
	publisher.publishAlchemistHitCounts(p, pack)
	if len(publisher.pendingEvents) != 2 {
		t.Fatal("other caster's hits included")
	}
}

func TestReplayAlchemistKpiCapture(t *testing.T) {
	file := os.Getenv("DILMETER_ALCHEMIST_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_ALCHEMIST_TEST_PCAP to replay the four-full-orb capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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
	if destination := os.Getenv("DILMETER_ALCHEMIST_TEST_EVENTS"); destination != "" {
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
	counts := map[string]uint32{}
	casts := map[uint16]int{}
	last := time.Now()
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
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
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.SkillId == 59144 {
					if signal.TargetId != "4767482421021708" {
						t.Fatalf("wrong chemical target: %s", signal.TargetId)
					}
					counts[signal.Signal] = signal.Count
				}
				if action, ok := raw.(*event.EventSkillAction); ok && !action.IsFallback {
					casts[action.SkillId]++
				}
			}
		case <-timer.C:
			if len(counts) > 0 && time.Since(last) > time.Second {
				_, _, errors := reader.GetStats()
				if errors != 0 {
					t.Fatalf("decode errors %d", errors)
				}
				if counts["chemical-effect-count"] != 5 || counts["chemical-hit-count"] != 5 || counts["chemical-sample"] != 5 || casts[59145] != 5 || casts[59144] != 1 {
					t.Fatalf("chemical capture counters=%v casts=%v", counts, casts)
				}
				t.Logf("raw chemical counts=%v confirmed casts=%v", counts, casts)
				return
			}
		case <-ctx.Done():
			t.Fatal("alchemist capture replay timed out")
		}
	}
}

func TestChemicalCompletedCast(t *testing.T) {
	for _, scenario := range []string{"complete", "base-only", "cancelled", "mismatch", "other-target", "other-owner", "stale-effect", "connection-reset"} {
		t.Run(scenario, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
			count := uint32(5)
			if scenario == "base-only" {
				count = 1
			}
			at := int64(1000)
			if scenario == "stale-effect" {
				at = 999
			}
			publisher.publishAlchemistPacket(&packet.GamePacket{Id: 100, Op: packet.OpcodeEffectDelayed, At: time.UnixMilli(at), Msg: packet.Message{packet.NewMessageElemInt(850), packet.NewMessageElemInt(924), packet.NewMessageElemLong(200), packet.NewMessageElemInt(350), packet.NewMessageElemInt(150), packet.NewMessageElemInt(count), packet.NewMessageElemInt(150)}})
			publisher.publishAlchemistPacket(&packet.GamePacket{Id: 100, Op: packet.OpcodeSkillExecute, At: time.UnixMilli(1000), Msg: packet.Message{packet.NewMessageElemShort(59144), packet.NewMessageElemLong(200), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1), packet.NewMessageElemShort(2)}})
			publisher.publishAlchemistPacket(&packet.GamePacket{Id: 100, Op: packet.OpcodeSkillPrepareEnd, At: time.UnixMilli(1001), Msg: packet.Message{packet.NewMessageElemByte(0), packet.NewMessageElemByte(1), packet.NewMessageElemByte(0), packet.NewMessageElemShort(59144)}})
			caster, target := uint64(100), uint64(200)
			if scenario == "other-target" {
				target = 201
			}
			if scenario == "other-owner" {
				caster = 101
			}
			if scenario == "connection-reset" {
				publisher.beginConnectionEpoch(2, time.UnixMilli(1100))
			}
			pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{
				{EntityId: caster, SkillId: 59144, Attacker: &packet.CombatActionPacketAttackerInfo{}},
				{EntityId: target, Hit: &packet.CombatActionPacketHitInfo{}},
			}}
			p := &packet.GamePacket{At: time.UnixMilli(1800)}
			publisher.publishAlchemistHitCounts(p, pack)
			if scenario != "base-only" {
				pack.SubPackets[1].Hit.Options = packet.CombatActionHitOptionsMultiHit
				pack.SubPackets[1].Hit.MultiHitCount = count
				if scenario == "mismatch" {
					pack.SubPackets[1].Hit.MultiHitCount = 4
				}
				p.At = time.UnixMilli(2000)
				publisher.publishAlchemistHitCounts(p, pack)
			}
			end := packet.OpCode(27017)
			if scenario == "cancelled" {
				end = packet.OpcodeSkillPrepareEnd
			}
			publisher.publishAlchemistPacket(&packet.GamePacket{Id: 100, Op: end, At: time.UnixMilli(2200), Msg: packet.Message{packet.NewMessageElemShort(59144)}})
			publisher.publishAlchemistPacket(&packet.GamePacket{Id: 100, Op: 27017, At: time.UnixMilli(2201), Msg: packet.Message{packet.NewMessageElemShort(59144)}})
			samples := 0
			for _, raw := range publisher.pendingEvents {
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "chemical-sample" {
					samples++
					if !signal.Complete || signal.Count != count || signal.FirstHitAtMs != 1800 || signal.CastAtMs != 1000 {
						t.Fatalf("invalid completed sample: %#v", signal)
					}
				}
			}
			expected := 0
			if scenario == "complete" || scenario == "base-only" {
				expected = 1
			}
			if samples != expected {
				t.Fatalf("samples=%d want=%d", samples, expected)
			}
		})
	}
}
