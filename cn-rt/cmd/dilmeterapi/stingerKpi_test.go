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

func TestRepeatedStingerAndMagnumExecute(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
	for index, skill := range []uint16{21001, 21002, 59061, 59060, 21002, 59060} {
		msg := packet.Message{packet.NewMessageElemShort(skill), packet.NewMessageElemInt(520), packet.NewMessageElemInt(1)}
		if skill >= 59060 {
			msg = packet.Message{packet.NewMessageElemShort(skill), packet.NewMessageElemLong(200), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1), packet.NewMessageElemShort(0), packet.NewMessageElemShort(1), packet.NewMessageElemShort(3)}
		}
		if err := publisher.publishSkillExecutePacket(&packet.GamePacket{Id: 100, Op: packet.OpcodeSkillExecute, At: time.UnixMilli(1000 + int64(index)), Msg: msg}); err != nil {
			t.Fatal(err)
		}
	}
	if len(publisher.pendingEvents) != 6 {
		t.Fatalf("rapid repeated shots suppressed: %d events", len(publisher.pendingEvents))
	}
	for _, raw := range publisher.pendingEvents {
		action, ok := raw.(*event.EventSkillAction)
		if !ok || action.IsFallback || action.Id != "100" || action.CombatActionId != 0 {
			t.Fatalf("not a canonical execution: %#v", raw)
		}
	}
}

func hydroTestRelease(target uint64, angle float32) packet.Message {
	return packet.Message{packet.NewMessageElemInt(825), packet.NewMessageElemByte(0), packet.NewMessageElemByte(0), packet.NewMessageElemByte(0),
		packet.NewMessageElemFloat(8303), packet.NewMessageElemFloat(0), packet.NewMessageElemFloat(8234), packet.NewMessageElemFloat(3530),
		packet.NewMessageElemFloat(angle), packet.NewMessageElemFloat(7), packet.NewMessageElemLong(target),
		packet.NewMessageElemFloat(-0.52), packet.NewMessageElemFloat(0.85), packet.NewMessageElemFloat(7852), packet.NewMessageElemFloat(8964)}
}

func TestHydroChargeLifecycle(t *testing.T) {
	for _, scenario := range []string{"complete", "cancel", "foreign-owner", "other-target", "new-skill", "channel-reset", "truncated", "no-normal-end", "malformed", "non-finite"} {
		t.Run(scenario, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
			send := func(id uint64, op packet.OpCode, at int64, msg packet.Message) {
				publisher.publishStingerPacket(&packet.GamePacket{Id: id, Op: op, At: time.UnixMilli(at), Msg: msg})
			}
			execute := packet.Message{packet.NewMessageElemShort(59061), packet.NewMessageElemLong(200), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1), packet.NewMessageElemShort(1), packet.NewMessageElemShort(0), packet.NewMessageElemShort(3)}
			owner := uint64(100)
			if scenario == "foreign-owner" {
				owner = 101
			}
			if scenario != "truncated" {
				send(owner, packet.OpcodeSkillExecute, 1000, execute)
				send(owner, packet.OpcodeSkillExecute, 1000, execute) // repeated ACK cannot reset or count twice
			}
			end := packet.Message{packet.NewMessageElemByte(0), packet.NewMessageElemByte(1), packet.NewMessageElemByte(0), packet.NewMessageElemShort(59061)}
			if scenario == "cancel" {
				end[2] = packet.NewMessageElemByte(1)
			}
			send(owner, packet.OpcodeSkillPrepareEnd, 1001, end)
			if scenario == "new-skill" {
				send(owner, 27012, 1100, packet.Message{packet.NewMessageElemShort(21002)})
			}
			if scenario == "channel-reset" {
				publisher.beginConnectionEpoch(1, time.UnixMilli(1100))
				publisher.localEntityId = 100
			}
			release := hydroTestRelease(200, 29.470001)
			if scenario == "other-target" {
				release[10] = packet.NewMessageElemLong(201)
			}
			if scenario == "malformed" {
				release[8] = packet.NewMessageElemInt(30)
			}
			if scenario == "non-finite" {
				release[8] = packet.NewMessageElemFloat(float32(math.NaN()))
			}
			send(owner, 37011, 1155, release)
			send(owner, 37011, 1155, release)
			if scenario != "no-normal-end" {
				send(owner, 27017, 1155, packet.Message{packet.NewMessageElemShort(59061)})
				send(owner, 27017, 1155, packet.Message{packet.NewMessageElemShort(59061)})
			}
			var samples []*event.EventArcanaSignal
			for _, raw := range publisher.pendingEvents {
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "hydro-charge-sample" {
					samples = append(samples, signal)
				}
			}
			if scenario != "complete" {
				if len(samples) != 0 {
					t.Fatalf("invalid charge counted: %+v", samples)
				}
				return
			}
			if len(samples) != 1 || !samples[0].Complete || samples[0].CastAtMs != 1000 || samples[0].AtMs != 1155 || samples[0].TargetId != "200" || samples[0].Range != 3530 || math.Abs(float64(samples[0].Value)-29.470001) > 0.00001 {
				t.Fatalf("raw charge signal not retained: %+v", samples)
			}
		})
	}
}

func TestReplayHydroChargeCapture(t *testing.T) {
	file := os.Getenv("DILMETER_HYDRO_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_HYDRO_TEST_PCAP to replay the 00:20 capture")
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
	if destination := os.Getenv("DILMETER_HYDRO_TEST_EVENTS"); destination != "" {
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
	var samples []*event.EventArcanaSignal
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
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "hydro-charge-sample" {
					samples = append(samples, signal)
				}
			}
		case <-tick.C:
			if len(samples) > 0 && time.Since(last) > time.Second {
				_, _, decodeErrors := reader.GetStats()
				if decodeErrors != 0 || len(samples) != 9 {
					t.Fatalf("samples=%d decodeErrors=%d", len(samples), decodeErrors)
				}
				for index, sample := range samples {
					angle := float32(7.5)
					if index == 2 {
						angle = 29.470001
					}
					if !sample.Complete || sample.Id != "4503599631566088" || sample.TargetId != "4767482419838834" || math.Abs(float64(sample.Value-angle)) > 0.00001 || sample.AtMs <= sample.CastAtMs {
						t.Fatalf("charge %d: %+v", index, sample)
					}
				}
				t.Log("9 complete Hydro geometry signals; eight 7.5 and one 29.470001; no assumed charge percentage")
				return
			}
		case <-ctx.Done():
			t.Fatal("hydro capture replay timed out")
		}
	}
}

func TestReplayStingerKpiCapture(t *testing.T) {
	file := os.Getenv("DILMETER_STINGER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_STINGER_TEST_PCAP to replay the 23:33 capture")
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
	if destination := os.Getenv("DILMETER_STINGER_TEST_EVENTS"); destination != "" {
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
			}
		case <-tick.C:
			if casts[59060] > 0 && time.Since(last) > time.Second {
				_, _, decodeErrors := reader.GetStats()
				if decodeErrors != 0 {
					t.Fatalf("decode errors: %d", decodeErrors)
				}
				for skill, count := range map[uint16]int{21002: 77, 21014: 11, 59060: 14, 59061: 10, 59062: 6, 59063: 4, 59064: 6} {
					if casts[skill] != count {
						t.Fatalf("skill %d: %d casts, want %d; counts=%v", skill, casts[skill], count, casts)
					}
				}
				t.Logf("canonical executions match all raw archery packets: %v", casts)
				return
			}
		case <-ctx.Done():
			t.Fatal("stinger capture replay timed out")
		}
	}
}
