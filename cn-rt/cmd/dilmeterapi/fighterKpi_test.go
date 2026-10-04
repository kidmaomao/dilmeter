package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func fighterBaselineMessage() packet.Message {
	return packet.Message{packet.NewMessageElemFloat(0), packet.NewMessageElemInt(1), packet.NewMessageElemInt(30010), packet.NewMessageElemFloat(3), packet.NewMessageElemInt(math.MaxUint32), packet.NewMessageElemInt(3), packet.NewMessageElemByte(0), packet.NewMessageElemFloat(400), packet.NewMessageElemByte(1)}
}

func fighterReadyMessage() packet.Message {
	return packet.Message{packet.NewMessageElemInt(66), packet.NewMessageElemInt(292), packet.NewMessageElemString("G16_F_Skill_D_kick_ready"), packet.NewMessageElemInt(5000), packet.NewMessageElemFloat(0), packet.NewMessageElemByte(0)}
}

func TestFighterEnergyPackets(t *testing.T) {
	p := &packet.GamePacket{Op: 44893, Msg: fighterBaselineMessage()}
	signal, ok := parseFighterEnergyPacket(p)
	if !ok || signal.Value != 0 || signal.Rate != 3 || signal.Maximum != 400 {
		t.Fatalf("baseline: %#v", signal)
	}
	p.Msg = p.Msg[:8]
	if _, ok := parseFighterEnergyPacket(p); ok {
		t.Fatal("truncated baseline accepted")
	}
	p.Msg = fighterBaselineMessage()
	p.Msg[5] = packet.NewMessageElemInt(2)
	if _, ok := parseFighterEnergyPacket(p); ok {
		t.Fatal("another resource used as fighter baseline")
	}
	p.Msg = fighterBaselineMessage()
	p.Msg[3] = packet.NewMessageElemFloat(float32(math.NaN()))
	if _, ok := parseFighterEnergyPacket(p); ok {
		t.Fatal("NaN regeneration accepted")
	}
	p.Op, p.Msg = 44892, packet.Message{packet.NewMessageElemInt(1), packet.NewMessageElemShort(3), packet.NewMessageElemFloat(-200), packet.NewMessageElemInt(0)}
	signal, ok = parseFighterEnergyPacket(p)
	if !ok || signal.Value != -200 || signal.Signal != "fighter-energy-delta" {
		t.Fatal("signed consumption lost")
	}
	p.Msg[1] = packet.NewMessageElemShort(2)
	signal, ok = parseFighterEnergyPacket(p)
	if !ok || signal.Signal != "fighter-energy-reset" {
		t.Fatal("resource group switch retains fighter energy")
	}
	publisher := newSkillTestPublisher()
	publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
	p.Id, p.Op, p.Msg, p.At = 101, 44893, fighterBaselineMessage(), time.UnixMilli(1000)
	publisher.publishFighterPacket(p)
	if len(publisher.pendingEvents) != 0 {
		t.Fatal("other player's energy included")
	}
}

func TestFighterAnimationCancel(t *testing.T) {
	for _, scenario := range []string{"cancel", "natural", "no-ready", "wrong-end", "other-target", "interrupted", "other-owner", "reset"} {
		t.Run(scenario, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
			publish := func(op packet.OpCode, at int64, msg packet.Message) {
				publisher.publishFighterPacket(&packet.GamePacket{Id: 100, Op: op, At: time.UnixMilli(at), Msg: msg})
			}
			publish(packet.OpcodeSkillExecute, 1000, packet.Message{packet.NewMessageElemShort(24201), packet.NewMessageElemLong(200)})
			if scenario == "natural" {
				publish(27017, 2300, packet.Message{packet.NewMessageElemShort(24201), packet.NewMessageElemLong(200)})
			} else {
				id := uint64(100)
				if scenario == "other-owner" {
					id = 101
				}
				publisher.publishFighterPacket(&packet.GamePacket{Id: id, Op: packet.OpcodeSkillPrepareReady, At: time.UnixMilli(1300), Msg: packet.Message{packet.NewMessageElemShort(24103)}})
				flag := uint8(1)
				if scenario == "wrong-end" {
					flag = 0
				}
				publish(packet.OpcodeSkillPrepareEnd, 1380, packet.Message{packet.NewMessageElemByte(0), packet.NewMessageElemByte(1), packet.NewMessageElemByte(flag), packet.NewMessageElemShort(24103)})
			}
			readyAt, kickAt := int64(1400), int64(1410)
			if scenario == "natural" {
				readyAt, kickAt = 2500, 2510
			}
			if scenario != "no-ready" {
				publish(packet.OpcodeEffectDelayed, readyAt, fighterReadyMessage())
			}
			if scenario == "interrupted" {
				publish(packet.OpcodeSkillExecute, 1405, packet.Message{packet.NewMessageElemShort(24101), packet.NewMessageElemLong(200)})
			}
			if scenario == "reset" {
				publisher.beginConnectionEpoch(2, time.UnixMilli(1405))
			}
			target := uint64(200)
			if scenario == "other-target" {
				target = 201
			}
			publish(packet.OpcodeSkillExecute, kickAt, packet.Message{packet.NewMessageElemShort(24301), packet.NewMessageElemLong(target)})
			publish(packet.OpcodeSkillExecute, kickAt+1, packet.Message{packet.NewMessageElemShort(24301), packet.NewMessageElemLong(target)})
			complete := 0
			for _, raw := range publisher.pendingEvents {
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "fighter-combo" && signal.Complete {
					complete++
					if signal.TargetId != "200" || signal.CastAtMs != 1000 || signal.ReadyAtMs != readyAt {
						t.Fatalf("bad combo: %#v", signal)
					}
					if scenario == "cancel" && (signal.Count != 1 || signal.ReverseAtMs != 1300) {
						t.Fatal("prepare-only cancel omitted")
					}
					if scenario == "natural" && signal.Count != 0 {
						t.Fatal("normal uppercut considered a reverse attempt")
					}
				}
			}
			expected := 0
			if scenario == "cancel" || scenario == "natural" {
				expected = 1
			}
			if complete != expected {
				t.Fatalf("complete=%d want=%d", complete, expected)
			}
		})
	}
}

func TestReplayFighterKpiCapture(t *testing.T) {
	file := os.Getenv("DILMETER_FIGHTER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_FIGHTER_TEST_PCAP to replay fighter capture")
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
	if destination := os.Getenv("DILMETER_FIGHTER_TEST_EVENTS"); destination != "" {
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
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	baseline, deltas, combos, reverse := 0, 0, 0, 0
	casts := map[uint16]int{}
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
				if action, ok := raw.(*event.EventSkillAction); ok && action.SourceId == "4503599631566088" && !action.IsFallback {
					casts[action.SkillId]++
				}
				if signal, ok := raw.(*event.EventArcanaSignal); ok {
					switch signal.Signal {
					case "fighter-energy-baseline":
						baseline++
						if signal.Value != 0 || signal.Maximum != 400 || signal.Rate != 3 {
							t.Fatalf("baseline=%#v", signal)
						}
					case "fighter-energy-delta":
						deltas++
					case "fighter-combo":
						if !signal.Complete || signal.TargetId != "4767482419126404" {
							t.Fatalf("combo=%#v", signal)
						}
						combos++
						if signal.Count == 1 {
							reverse++
							if signal.ReverseAtMs-signal.CastAtMs > 500 {
								t.Fatal("unexpected late cancel")
							}
						}
					}
				}
			}
		case <-tick.C:
			if combos > 0 && time.Since(last) > time.Second {
				_, _, errors := reader.GetStats()
				if errors != 0 || baseline != 1 || deltas != 97 || combos != 17 || reverse != 14 || casts[24201] != 17 || casts[24301] != 17 {
					t.Fatalf("baseline=%d deltas=%d combos=%d reverse=%d casts=%v errors=%d", baseline, deltas, combos, reverse, casts, errors)
				}
				t.Logf("baseline=0 maximum=400 rate=3, deltas=%d combos=%d reverse=%d", deltas, combos, reverse)
				return
			}
		case <-ctx.Done():
			t.Fatal("fighter capture replay timed out")
		}
	}
}
