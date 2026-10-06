package main

import (
	"context"
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"os"
	"testing"
	"time"
)

func TestReplayPartySniperOctober4Capture(t *testing.T) {
	file := os.Getenv("DILMETER_PARTY_SNIPER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PARTY_SNIPER_TEST_PCAP to replay the 2026-10-04 party sniper capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	if destination := os.Getenv("DILMETER_PARTY_SNIPER_TEST_EVENTS"); destination != "" {
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
	starts, completed, shots := 0, 0, 0
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
				if sample, ok := raw.(*event.EventArcanaSignal); ok && sample.Id == "4503599631431147" && sample.Signal == "sniper-counter" {
					if sample.Phase == 2 {
						starts++
					}
					if sample.Complete {
						completed++
						shots += int(sample.Count)
					}
				}
			}
		case <-tick.C:
			_, parsed, errors := reader.GetStats()
			if parsed > 0 && time.Since(last) > time.Second {
				t.Logf("observer sniper: starts=%d complete=%d completed shots=%d parsed=%d errors=%d", starts, completed, shots, parsed, errors)
				if starts != 63 || completed != 57 || errors != 2 {
					t.Fatalf("sniper capture mismatch: starts=%d completed=%d errors=%d", starts, completed, errors)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("party stinger capture timed out")
		}
	}
}
