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

func TestDarkMageRepeatedThunderReleases(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
	for _, at := range []int64{1000, 2000, 3000} {
		if err := publisher.publishSkillExecutePacket(&packet.GamePacket{Id: 100, Op: packet.OpcodeSkillExecute, At: time.UnixMilli(at),
			Msg: packet.Message{packet.NewMessageElemShort(30102), packet.NewMessageElemInt(100), packet.NewMessageElemInt(1)}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(publisher.pendingEvents) != 3 {
		t.Fatalf("repeated Thunder executions suppressed: %d", len(publisher.pendingEvents))
	}
	for _, raw := range publisher.pendingEvents {
		action, ok := raw.(*event.EventSkillAction)
		if !ok || action.IsFallback || action.SkillId != 30102 || action.CombatActionId != 0 {
			t.Fatalf("invalid canonical Thunder release: %#v", raw)
		}
	}
}

func TestLightningChainStatePackets(t *testing.T) {
	on := packet.Message{packet.NewMessageElemInt(814), packet.NewMessageElemInt(1), packet.NewMessageElemLong(200), packet.NewMessageElemFloat(3200)}
	off := packet.Message{packet.NewMessageElemInt(814), packet.NewMessageElemInt(0)}
	for _, duration := range []int64{12000, 17000} {
		publisher := newSkillTestPublisher()
		publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
		publisher.publishDarkMagePacket(&packet.GamePacket{Id: 100, Op: 37011, At: time.UnixMilli(1000), Msg: on})
		publisher.publishDarkMagePacket(&packet.GamePacket{Id: 100, Op: 37011, At: time.UnixMilli(1000 + duration), Msg: off})
		if len(publisher.pendingEvents) != 2 {
			t.Fatalf("state transitions: %d", len(publisher.pendingEvents))
		}
		start := publisher.pendingEvents[0].(*event.EventArcanaSignal)
		end := publisher.pendingEvents[1].(*event.EventArcanaSignal)
		if start.Signal != "lightning-chain-state" || start.SkillId != 59041 || start.Kind != 814 || start.Count != 1 || start.TargetId != "200" || !start.Complete || end.Count != 0 || !end.Complete || end.AtMs-start.AtMs != duration {
			t.Fatalf("actual lifetime not retained: %+v %+v", start, end)
		}
	}
	for _, scenario := range []string{"other-owner", "unknown-local", "cooldown-opcode", "malformed", "non-finite", "unknown-state"} {
		t.Run(scenario, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
			p := &packet.GamePacket{Id: 100, Op: 37011, At: time.UnixMilli(1000), Msg: append(packet.Message{}, on...)}
			switch scenario {
			case "other-owner":
				p.Id = 101
			case "unknown-local":
				publisher.localEntityId = 0
			case "cooldown-opcode":
				p.Op = 27020
			case "malformed":
				p.Msg = p.Msg[:3]
			case "non-finite":
				p.Msg[3] = packet.NewMessageElemFloat(float32(math.NaN()))
			case "unknown-state":
				p.Msg[1] = packet.NewMessageElemInt(2)
			}
			publisher.publishDarkMagePacket(p)
			if len(publisher.pendingEvents) != 0 {
				t.Fatal("unverified state accepted")
			}
		})
	}
}

func TestReplayDarkMageKpiCapture(t *testing.T) {
	file := os.Getenv("DILMETER_DARK_MAGE_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_DARK_MAGE_TEST_PCAP to replay the 01:30 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	if destination := os.Getenv("DILMETER_DARK_MAGE_TEST_EVENTS"); destination != "" {
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
	casts := map[uint16]int{}
	var states []*event.EventArcanaSignal
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
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "lightning-chain-state" {
					states = append(states, signal)
				}
			}
		case <-tick.C:
			if len(states) > 0 && time.Since(last) > time.Second {
				_, _, errors := reader.GetStats()
				if errors != 0 || len(states) != 12 {
					t.Fatalf("chain transitions=%d, decode errors=%d", len(states), errors)
				}
				for skill, count := range map[uint16]int{59040: 11, 30102: 87, 59045: 4} {
					if casts[skill] != count {
						t.Fatalf("skill %d has %d releases, expected %d", skill, casts[skill], count)
					}
				}
				for i := 0; i < len(states); i += 2 {
					start, end := states[i], states[i+1]
					if start.Id != "4503599631566088" || start.TargetId != "4767482419865510" || start.Count != 1 || end.Count != 0 || end.AtMs-start.AtMs < 17000 || end.AtMs-start.AtMs > 17100 {
						t.Fatalf("unexpected captured state pair: %+v %+v", start, end)
					}
				}
				t.Log("11 Dragon releases, 87 Thunder releases, 4 Mana Seal releases, and six actual Lightning Chain windows")
				return
			}
		case <-ctx.Done():
			t.Fatal("dark mage capture replay timed out")
		}
	}
}
