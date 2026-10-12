package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// The October 7 capture broadcasts CC938.MPD for the teammate without
// broadcasting that character's private Dorcha quantity (stat 196).
func TestReplayPartyManaPierceCapture(t *testing.T) {
	file := os.Getenv("DILMETER_PARTY_PIERCE_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PARTY_PIERCE_PCAP to the October 7 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	ch := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, ch)
	var encoder *json.Encoder
	if destination := os.Getenv("DILMETER_PARTY_PIERCE_EVENTS"); destination != "" {
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
	last, total, disabled, recovered, quantities := time.Now(), 0, 0, 0, 0
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case batch := <-ch:
			last = time.Now()
			for _, e := range batch {
				total++
				if encoder != nil {
					if err := encoder.Encode(e); err != nil {
						t.Fatal(err)
					}
				}
				if c, ok := e.(*event.EventCharacterConditionEnable); ok && c.Id == "4503599638023283" && c.CCId == 938 && !c.Snapshot {
					if strings.Contains(c.Metadata, "MPD:b:true;") {
						disabled++
					} else if strings.Contains(c.Metadata, "MPD:b:false;") {
						recovered++
					}
				}
				if s, ok := e.(*event.EventStatUpdate); ok && s.Id == "4503599638023283" {
					for _, value := range s.Stats {
						if value.StatId == 196 {
							quantities++
						}
					}
				}
			}
		case <-tick.C:
			if total > 0 && time.Since(last) > 3*time.Second {
				_, _, errors := reader.GetStats()
				if errors != 0 || disabled != 6 || recovered != 6 || quantities != 0 {
					t.Fatalf("decode errors=%d disabled=%d recovered=%d quantities=%d", errors, disabled, recovered, quantities)
				}
				t.Logf("events=%d; teammate: six MPD disable/recovery pairs, zero Dorcha quantity updates", total)
				return
			}
		case <-ctx.Done():
			t.Fatal("capture replay timed out")
		}
	}
}
