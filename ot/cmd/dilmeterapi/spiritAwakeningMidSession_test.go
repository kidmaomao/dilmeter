package main

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func spiritTestDelta(atMs int64, charge uint32) *packet.GamePacket {
	return &packet.GamePacket{Id: 1, At: time.UnixMilli(atMs), Msg: packet.Message{
		packet.NewMessageElemInt(2), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1 << 14), packet.NewMessageElemInt(charge),
	}}
}

func spiritTestReady(atMs int64) *packet.GamePacket {
	return &packet.GamePacket{Id: 1, At: time.UnixMilli(atMs), Msg: packet.Message{packet.NewMessageElemShort(20)}}
}

func TestSpiritMidSessionFullAndExtraCast(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId = 1
	p.publishSpiritWeaponPacket(spiritTestDelta(100000, 10000))
	if len(p.spiritEnergySnapshot) != 0 {
		t.Fatal("partial charge guessed a weapon")
	}
	p.publishSpiritWeaponPacket(spiritTestDelta(110000, 60000))
	p.publishSpiritReadyPacket(spiritTestReady(110005))
	if state := p.spiritEnergySnapshot[35130]; !state.Active || state.Percent != 100 {
		t.Fatalf("mid-session full charge not published: %+v", state)
	}
	cast := &packet.GamePacket{Id: 1, At: time.UnixMilli(120000), Msg: packet.Message{packet.NewMessageElemShort(35130), packet.NewMessageElemLong(1)}}
	if err := p.publishSkillExecutePacket(cast); err != nil {
		t.Fatal(err)
	}
	if p.spiritEnergySnapshot[35130].Percent != 100 {
		t.Fatal("58015 retained charge was consumed by execution")
	}
	p.publishSpiritWeaponPacket(spiritTestDelta(130000, 0))
	cast.At = time.UnixMilli(130000)
	if err := p.publishSkillExecutePacket(cast); err != nil {
		t.Fatal(err)
	}
	p.publishSpiritWeaponPacket(spiritTestDelta(140000, 2000))
	state := p.spiritEnergySnapshot[35130]
	if !state.Active || state.Percent < 3 || state.Percent > 4 {
		t.Fatalf("recharge after extra use lost identity: %+v", state)
	}
}

func TestSpiritMidSessionNormalCastLearnsZeroDelta(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId = 1
	p.publishSpiritWeaponPacket(spiritTestDelta(100000, 0))
	cast := &packet.GamePacket{Id: 1, At: time.UnixMilli(100005), Msg: packet.Message{packet.NewMessageElemShort(35130), packet.NewMessageElemLong(1)}}
	if err := p.publishSkillExecutePacket(cast); err != nil {
		t.Fatal(err)
	}
	if state := p.spiritEnergySnapshot[35130]; !state.Active || state.Percent != 0 {
		t.Fatalf("normal cast did not resolve zero: %+v", state)
	}
	p.publishSpiritWeaponPacket(spiritTestDelta(110000, 30000))
	if p.spiritEnergySnapshot[35130].Percent != 50 {
		t.Fatal("subsequent deltas did not retain the learned slot")
	}
}

func TestSpiritMidSessionRefusesUncertainMapping(t *testing.T) {
	for _, scenario := range []string{"stale", "future", "ambiguous", "other-player", "conflicting-type", "reset", "proc-without-zero", "different-charge"} {
		t.Run(scenario, func(t *testing.T) {
			p := newSkillTestPublisher()
			p.localEntityId = 1
			delta := spiritTestDelta(100000, 60000)
			ready := spiritTestReady(100005)
			switch scenario {
			case "stale":
				ready.At = time.UnixMilli(101000)
			case "future":
				ready.At = time.UnixMilli(99000)
			case "ambiguous":
				delta.Msg = packet.Message{packet.NewMessageElemInt(2), packet.NewMessageElemInt(1 << 14), packet.NewMessageElemInt(60000), packet.NewMessageElemInt(1 << 14), packet.NewMessageElemInt(60000)}
			case "other-player":
				ready.Id = 2
			case "conflicting-type":
				delta.Msg = packet.Message{packet.NewMessageElemInt(1), packet.NewMessageElemInt(1 | 1<<14), packet.NewMessageElemShort(12), packet.NewMessageElemInt(60000)}
			case "different-charge":
				delta.Msg[3] = packet.NewMessageElemInt(2000)
			}
			p.publishSpiritWeaponPacket(delta)
			if scenario == "reset" {
				p.publish(&event.EventLocalEntity{EventBase: event.EventBase{Id: "1"}, Reset: true})
			}
			if scenario == "proc-without-zero" {
				p.observeSpiritAwakeningExecute(&packet.GamePacket{Id: 1, At: ready.At}, 35130)
			} else {
				p.publishSpiritReadyPacket(ready)
			}
			if p.spiritEnergySnapshot[35130].Active {
				t.Fatal("uncertain signal assigned an orb gauge")
			}
		})
	}
}

// This reported failure has no 44582 roster at all: only positional deltas.
// Its first orb cast procs 58015, followed by five ordinary consuming casts.
func TestReplaySpiritMidSessionCapture(t *testing.T) {
	file := os.Getenv("DILMETER_SPIRIT_MIDSESSION_PCAP")
	if file == "" {
		t.Skip("set DILMETER_SPIRIT_MIDSESSION_PCAP to the October 22 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	p := newEventPublisher(ctx, reader)
	ch := make(chan []event.IEvent, 1000)
	p.addClient(ctx, ch)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	localID, percent := "", -1.0
	casts := []float64{}
	full, partial, spent := 0, 0, 0
	var firstFullAt int64
	for {
		select {
		case batch := <-ch:
			for _, raw := range batch {
				switch v := raw.(type) {
				case *event.EventLocalEntity:
					localID = v.Id
					if v.Reset {
						percent = -1
					}
				case *event.EventSkillEnergy:
					if v.Id == localID && v.SkillId == 35130 && v.Active {
						percent = v.Percent
						switch {
						case percent == 100:
							full++
							if firstFullAt == 0 {
								firstFullAt = v.At
							}
						case percent == 0:
							spent++
						default:
							partial++
						}
					}
				case *event.EventSkillAction:
					if v.Id == localID && v.SkillId == 35130 && !v.IsFallback {
						casts = append(casts, percent)
					}
				}
			}
			if len(casts) >= 6 {
				if !reflect.DeepEqual(casts, []float64{100, 0, 0, 0, 0, 0}) || full != 5 || spent != 5 || partial < 100 || firstFullAt != 1792672547 {
					t.Fatalf("casts=%v full=%d spent=%d partial=%d firstFullAt=%d", casts, full, spent, partial, firstFullAt)
				}
				t.Logf("mid-session: casts=%v, full=%d, spent=%d, partial=%d; full charge recognized before first cast", casts, full, spent, partial)
				return
			}
		case <-ctx.Done():
			t.Fatalf("incomplete replay casts=%v full=%d spent=%d partial=%d", casts, full, spent, partial)
		}
	}
}
