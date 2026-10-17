package main

import (
	"context"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"os"
	"reflect"
	"testing"
	"time"
)

// Local opt-in regression: do not commit or upload the user's capture.
func TestGreenCurtainCaptureReplay(t *testing.T) {
	path := os.Getenv("DILMETER_GREEN_CURTAIN_PCAP")
	if path == "" {
		t.Skip("set DILMETER_GREEN_CURTAIN_PCAP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	<-events
	if err := reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	var sounds []nativeReminderSoundRequest
	r := newGreenCurtainTestRuntime(&sounds)
	var observed []int64
	var clock int64
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	for {
		select {
		case batch := <-events:
			idle.Reset(4 * time.Second)
			for _, e := range batch {
				base, ok := e.(interface{ GetEventBase() *event.EventBase })
				if !ok {
					continue
				}
				at := base.GetEventBase().At * 1000
				if c, ok := e.(*event.EventCharacterConditionEnable); ok && c.AtMs > 0 {
					at = c.AtMs
				}
				if clock == 0 {
					clock = at
				}
				for clock < at {
					r.evaluateBossMechanics(time.UnixMilli(clock))
					clock += 100
				}
				before := r.greenCurtain.LastCurtainAtMs
				r.onEvent(e)
				if after := r.greenCurtain.LastCurtainAtMs; after > 0 && after != before {
					observed = append(observed, after)
				}
			}
		case <-idle.C:
			want := []int64{1792249943640, 1792250006433, 1792250069674}
			if !reflect.DeepEqual(observed, want) {
				t.Fatalf("actual curtain timestamps: %v, want %v", observed, want)
			}
			if len(sounds) != 3 {
				t.Fatalf("want opening + two advance warnings, got %d", len(sounds))
			}
			t.Logf("Captured curtains %v; opening and two advance warnings passed", observed)
			return
		case <-ctx.Done():
			t.Fatal("capture replay timed out")
		}
	}
}
