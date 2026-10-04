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

func newPartyStingerTestPublisher() *eventPublisher {
	p := newSkillTestPublisher()
	p.localEntityId, p.lastSentEventAt = 100, time.Now()
	for _, id := range []uint64{101, 102} {
		p.entityCache[id] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: id, RaceId: 9001}}
	}
	p.entityCache[103] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 103, RaceId: 9001, OwnerId: 101}}
	p.entityCache[104] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 104, RaceId: 20001}}
	return p
}

func partyHydroStart() packet.Message {
	return packet.Message{packet.NewMessageElemInt(825), packet.NewMessageElemByte(1), packet.NewMessageElemByte(0)}
}

func partyBlazingRelease() packet.Message {
	return packet.Message{packet.NewMessageElemInt(600), packet.NewMessageElemInt(821), packet.NewMessageElemLong(200), packet.NewMessageElemInt(795)}
}

func TestPartyStingerReleaseRecords(t *testing.T) {
	p := newPartyStingerTestPublisher()
	send := func(id uint64, at int64, msg packet.Message) {
		p.publishStingerPacket(&packet.GamePacket{Id: id, Op: packet.OpcodeEffectDelayed, At: time.UnixMilli(at), Msg: msg})
	}
	send(101, 1000, partyBlazingRelease())
	send(101, 1000, partyBlazingRelease())
	send(102, 1000, partyBlazingRelease())
	send(101, 1001, partyBlazingRelease()) // no arbitrary cooldown/time clustering
	configured := partyBlazingRelease()
	configured[0], configured[3] = packet.NewMessageElemInt(300), packet.NewMessageElemInt(900)
	send(101, 1002, configured) // delay/effect values do not identify the skill
	for _, id := range []uint64{100, 103, 104, 999} {
		send(id, 1000, partyBlazingRelease())
	}
	for index, replacement := range []packet.IMessageElem{packet.NewMessageElemShort(600), packet.NewMessageElemInt(824), packet.NewMessageElemLong(0), packet.NewMessageElemShort(795)} {
		msg := partyBlazingRelease()
		msg[index] = replacement
		send(101, 2000, msg)
	}
	for index, skill := range []uint16{59060, 59060, 59060, 21002, 21002, 21002} {
		action := &packet.CombatActionPacket{EntityId: 101, SkillId: skill, CombatActionId: uint32(10 + index), Type: 0x42,
			Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 200}}
		if index == 3 || index == 4 {
			action.Type = 0x72
			action.CombatActionId = 20 // duplicate public success cannot count twice
		}
		p.publishPartyCombatAction(&packet.GamePacket{At: time.UnixMilli(3000 + int64(index))}, &packet.CombatActionPackPacket{CombatActionId: action.CombatActionId, SubPackets: []*packet.CombatActionPacket{action}})
	}
	counts := map[uint16]int{}
	fallbacks := 0
	for _, raw := range p.pendingEvents {
		if action, ok := raw.(*event.EventSkillAction); ok {
			if action.IsLocal || action.SourceId != action.Id {
				t.Fatalf("public release lost original party caster: %+v", action)
			}
			if action.IsFallback {
				fallbacks++
			} else {
				counts[action.SkillId]++
			}
		}
	}
	if counts[59060] != 4 || counts[21002] != 1 || fallbacks != 4 {
		t.Fatalf("release counts=%v fallback phases=%d", counts, fallbacks)
	}
}

func TestPartyHydroLifecycle(t *testing.T) {
	for _, scenario := range []string{"complete", "cancel", "end", "truncated", "malformed", "backwards", "channel-reset", "self", "pet", "monster", "unknown"} {
		t.Run(scenario, func(t *testing.T) {
			p := newPartyStingerTestPublisher()
			send := func(id uint64, op packet.OpCode, at int64, msg packet.Message) {
				p.publishStingerPacket(&packet.GamePacket{Id: id, Op: op, At: time.UnixMilli(at), Msg: msg})
			}
			owner := uint64(101)
			switch scenario {
			case "self":
				owner = 100
			case "pet":
				owner = 103
			case "monster":
				owner = 104
			case "unknown":
				owner = 999
			}
			if scenario != "truncated" {
				send(owner, 37011, 1000, partyHydroStart())
				send(owner, 37011, 1000, partyHydroStart())
			}
			// A different teammate's charge must survive the first caster's cancel.
			send(102, 37011, 1100, partyHydroStart())
			switch scenario {
			case "cancel":
				send(owner, 37011, 1200, packet.Message{packet.NewMessageElemInt(825), packet.NewMessageElemByte(0), packet.NewMessageElemByte(0), packet.NewMessageElemByte(1)})
			case "end":
				send(owner, 28006, 1200, packet.Message{packet.NewMessageElemByte(0)})
			case "channel-reset":
				p.beginConnectionEpoch(1, time.UnixMilli(1200))
				p.localEntityId = 100
			}
			msg := hydroTestRelease(200, 7.5)
			at := int64(1600)
			if scenario == "malformed" {
				msg[8] = packet.NewMessageElemInt(7)
			}
			if scenario == "backwards" {
				at = 999
			}
			send(owner, 37011, at, msg)
			send(owner, 37011, at, msg)
			send(102, 37011, 1700, hydroTestRelease(201, 24.01))
			var samples []*event.EventArcanaSignal
			for _, raw := range p.pendingEvents {
				if signal, ok := raw.(*event.EventArcanaSignal); ok && signal.Signal == "hydro-charge-sample" {
					samples = append(samples, signal)
				}
			}
			want := 1
			if scenario == "complete" {
				want = 2
			}
			if scenario == "channel-reset" {
				want = 0
			}
			if len(samples) != want {
				t.Fatalf("samples=%+v wanted %d", samples, want)
			}
			if scenario == "complete" && (!samples[0].Complete || samples[0].Id != "101" || samples[0].TargetId != "200" || samples[0].CastAtMs != 1000 || samples[0].AtMs != 1600 || samples[0].Value != 7.5) {
				t.Fatalf("party full-charge sample: %+v", samples[0])
			}
			if want > 0 {
				last := samples[len(samples)-1]
				if !last.Complete || last.Id != "102" || last.TargetId != "201" || last.CastAtMs != 1100 || last.Value != 24.01 {
					t.Fatalf("other caster affected: %+v", last)
				}
			}
		})
	}
}

// Opt-in integration regression against the supplied 14:34 capture. The source
// begins mid-stream (two initial short packets); all subsequent frames decode.
func TestReplayPartyStingerCapture(t *testing.T) {
	file := os.Getenv("DILMETER_PARTY_STINGER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PARTY_STINGER_TEST_PCAP to replay the 14:34 party archer capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
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
	if destination := os.Getenv("DILMETER_PARTY_STINGER_TEST_EVENTS"); destination != "" {
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
	counts := map[uint16]int{}
	full, partial := 0, 0
	last := time.Now()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case batch := <-ch:
			last = time.Now()
			for _, raw := range batch {
				if encoder != nil {
					if err := encoder.Encode(raw); err != nil {
						t.Fatal(err)
					}
				}
				if action, ok := raw.(*event.EventSkillAction); ok && action.Id == "4503599631671241" && !action.IsFallback {
					if action.IsLocal || action.SourceId != action.Id {
						t.Fatalf("party caster changed: %+v", action)
					}
					counts[action.SkillId]++
				}
				if sample, ok := raw.(*event.EventArcanaSignal); ok && sample.Id == "4503599631671241" && sample.Signal == "hydro-charge-sample" {
					if !sample.Complete || sample.CastAtMs <= 0 || sample.AtMs < sample.CastAtMs || sample.TargetId == "" {
						t.Fatalf("incomplete party sample: %+v", sample)
					}
					switch sample.Value {
					case 7.5:
						full++
					case 24.01:
						partial++
					default:
						t.Fatalf("unexpected range: %+v", sample)
					}
				}
			}
		case <-tick.C:
			_, parsed, errors := reader.GetStats()
			if parsed > 0 && time.Since(last) > time.Second {
				if counts[59060] != 262 || counts[21002] != 1119 || full != 144 || partial != 1 || errors != 2 {
					t.Fatalf("releases=%v Hydro full=%d partial=%d decoded=%d errors=%d", counts, full, partial, parsed, errors)
				}
				t.Logf("public party releases: Blazing=%d Magnum=%d; Hydro 144 full / 1 partial; decode errors=2 initial short packets", counts[59060], counts[21002])
				return
			}
		case <-ctx.Done():
			t.Fatal("party stinger capture timed out")
		}
	}
}
