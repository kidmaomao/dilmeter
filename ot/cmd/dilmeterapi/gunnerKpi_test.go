package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestGunnerCounterLifecycle(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId, p.lastSentEventAt = 100, time.Now()
	send := func(op packet.OpCode, ms int64, msg packet.Message) {
		p.publishGunnerPacket(&packet.GamePacket{Id: 100, Op: op, At: time.UnixMilli(ms), Msg: msg})
	}
	counter := func(phase, count uint32) packet.Message {
		return packet.Message{packet.NewMessageElemInt(phase), packet.NewMessageElemFloat(100), packet.NewMessageElemInt(count)}
	}
	send(27073, 999, counter(7, 6)) // unrelated packets cannot count
	send(27013, 1000, packet.Message{packet.NewMessageElemShort(59123), packet.NewMessageElemString(""), packet.NewMessageElemLong(200)})
	send(27073, 1010, counter(2, 0)) // zero arrives before execute ACK
	send(27073, 1011, counter(6, 1)) // positive count before execution is rejected
	send(27016, 1020, packet.Message{packet.NewMessageElemShort(59123), packet.NewMessageElemLong(200)})
	send(27028, 1020, packet.Message{packet.NewMessageElemByte(0), packet.NewMessageElemByte(1), packet.NewMessageElemByte(0), packet.NewMessageElemShort(59123)})
	send(27073, 1100, counter(6, 2))
	send(27073, 1101, counter(6, 1)) // reject regressing count
	send(27073, 1200, counter(7, 6)) // cumulative counter tolerates missing middle phases
	send(27073, 1201, counter(7, 6)) // end cannot count twice
	if len(p.pendingEvents) != 3 {
		t.Fatalf("events = %d, want zero, two and six", len(p.pendingEvents))
	}
	last := p.pendingEvents[2].(*event.EventArcanaSignal)
	if !last.Complete || last.Count != 6 || last.TargetId != "200" || last.CastAtMs != 1000 {
		t.Fatalf("wrong complete sniper: %#v", last)
	}
	send(27013, 2000, packet.Message{packet.NewMessageElemShort(59123)})
	send(27028, 2100, packet.Message{packet.NewMessageElemShort(59123)})
	send(27073, 2200, counter(2, 0))
	if p.gunnerSniper != nil || len(p.pendingEvents) != 3 {
		t.Fatal("cancelled sniper retained state")
	}
	send(27013, 3000, packet.Message{packet.NewMessageElemShort(59123)})
	send(27016, 3100, packet.Message{packet.NewMessageElemShort(59120)})
	send(27073, 3200, counter(2, 0))
	if p.gunnerSniper != nil || len(p.pendingEvents) != 3 {
		t.Fatal("another execution retained stale sniper")
	}
	if _, _, ok := parseGunnerSniperCounter(packet.Message{packet.NewMessageElemInt(7), packet.NewMessageElemFloat(float32(math.NaN())), packet.NewMessageElemInt(6)}); ok {
		t.Fatal("NaN counter accepted")
	}
}

func TestGunnerDomainAssociation(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId, p.lastSentEventAt = 100, time.Now()
	links := packet.Message{packet.NewMessageElemInt(918), packet.NewMessageElemByte(1), packet.NewMessageElemInt(2), packet.NewMessageElemLong(45468001264271361), packet.NewMessageElemLong(45468001264271362)}
	send := func(op packet.OpCode, ms int64, msg packet.Message) {
		p.publishGunnerPacket(&packet.GamePacket{Id: 100, Op: op, At: time.UnixMilli(ms), Msg: msg})
	}
	send(37011, 1000, links)
	send(27012, 1004, packet.Message{packet.NewMessageElemShort(59121), packet.NewMessageElemString("")})
	send(27013, 1124, packet.Message{packet.NewMessageElemShort(59121), packet.NewMessageElemString("")})
	send(27016, 1130, packet.Message{packet.NewMessageElemShort(59121), packet.NewMessageElemLong(200), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1)})
	send(27017, 1430, packet.Message{packet.NewMessageElemShort(59121)})
	send(27017, 1431, packet.Message{packet.NewMessageElemShort(59121)})
	if len(p.pendingEvents) != 2 {
		t.Fatalf("domain event count %d, want raw list and one sample", len(p.pendingEvents))
	}
	sample := p.pendingEvents[1].(*event.EventArcanaSignal)
	if sample.Signal != "domain-sample" || !sample.Complete || sample.TargetId != "200" || sample.AtMs != 1130 || sample.Count != 2 || sample.ObjectIds[0] != "45468001264271361" {
		t.Fatalf("wrong sample: %#v", sample)
	}
	// A list interrupted by another preparation must not be reused by heavy.
	send(37011, 2000, links)
	send(27012, 2004, packet.Message{packet.NewMessageElemShort(59120)})
	send(27012, 2100, packet.Message{packet.NewMessageElemShort(59121)})
	send(27016, 2200, packet.Message{packet.NewMessageElemShort(59121), packet.NewMessageElemLong(200)})
	send(27017, 2500, packet.Message{packet.NewMessageElemShort(59121)})
	if len(p.pendingEvents) != 3 {
		t.Fatal("reused stale list or inferred zero from missing list")
	}
	send(37011, 3000, links)
	send(27012, 3004, packet.Message{packet.NewMessageElemShort(59121)})
	send(27028, 3100, packet.Message{packet.NewMessageElemShort(59121)})
	send(27017, 3200, packet.Message{packet.NewMessageElemShort(59121)})
	if len(p.pendingEvents) != 4 {
		t.Fatal("cancelled heavy became sample")
	}
	duplicate := append(packet.Message{}, links...)
	duplicate[4] = duplicate[3]
	if _, ok := parseGunnerDomainLinks(duplicate); ok {
		t.Fatal("duplicate zone IDs accepted")
	}
	if _, ok := parseGunnerDomainLinks(links[:4]); ok {
		t.Fatal("truncated zone list accepted")
	}
	p.publishGunnerPacket(&packet.GamePacket{Id: 999, Op: 37011, At: time.UnixMilli(4000), Msg: links})
	if len(p.pendingEvents) != 4 {
		t.Fatal("other player counted")
	}
	p.beginConnectionEpoch(1, time.UnixMilli(5000))
	if p.gunnerHeavy != nil || p.gunnerPendingDomains != nil || p.gunnerSniper != nil {
		t.Fatal("connection reset retained gunner state")
	}
}

// Optional replay writes the software's actual event stream, permitting the
// frontend KPI/export regression to run against the user's raw capture.
func TestReplayGunnerKpiCapture(t *testing.T) {
	file := os.Getenv("DILMETER_GUNNER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_GUNNER_TEST_PCAP to replay a controlled Gunner capture")
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
	var output *os.File
	var encoder *json.Encoder
	if destination := os.Getenv("DILMETER_GUNNER_TEST_EVENTS"); destination != "" {
		output, err = os.Create(destination)
		if err != nil {
			t.Fatal(err)
		}
		defer output.Close()
		encoder = json.NewEncoder(output)
	}
	if err = reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	var signals []*event.EventArcanaSignal
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
				if signal, ok := raw.(*event.EventArcanaSignal); ok {
					signals = append(signals, signal)
				}
			}
		case <-timer.C:
			if len(signals) > 0 && time.Since(last) > time.Second {
				_, _, errors := reader.GetStats()
				if errors != 0 {
					t.Fatalf("decode errors %d", errors)
				}
				counts := map[string]int{}
				var domains []uint32
				var shots []uint32
				for _, signal := range signals {
					counts[signal.Signal]++
					if signal.Signal == "domain-sample" {
						domains = append(domains, signal.Count)
					}
					if signal.Signal == "sniper-counter" && signal.Complete {
						shots = append(shots, signal.Count)
					}
				}
				switch os.Getenv("DILMETER_GUNNER_TEST_CASE") {
				case "sniper":
					if !reflect.DeepEqual(shots, []uint32{6}) || counts["domain-created"] != 3 {
						t.Fatalf("sniper replay: shots=%v signals=%v", shots, counts)
					}
				case "three":
					if !reflect.DeepEqual(domains, []uint32{3}) || counts["gunner-hit-extra"] != 15 {
						t.Fatalf("three-zone replay: domains=%v signals=%v", domains, counts)
					}
				case "repeat":
					if !reflect.DeepEqual(domains, []uint32{1, 2}) || counts["domain-remove-signal"] != 1 {
						t.Fatalf("repeat replay: domains=%v signals=%v", domains, counts)
					}
				default:
					t.Fatal("set DILMETER_GUNNER_TEST_CASE to sniper, three, or repeat")
				}
				t.Logf("confirmed shots=%v domains=%v signals=%v", shots, domains, counts)
				return
			}
		case <-ctx.Done():
			t.Fatal("capture replay timed out")
		}
	}
}
