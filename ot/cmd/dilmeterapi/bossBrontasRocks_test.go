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

func TestBrontasEarlyMapLifecycle(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true, ScalePercent: 100}}}, &sounds)
	r.onEvent(liveTestAppear("boss", "brontas", 7602, 100))
	r.onEvent(healerHP("boss", 100, 501, 1000))
	if r.brontasRockOverlay(time.Unix(100, 0)) != nil {
		t.Fatal("map appeared before the HP gate")
	}
	r.onEvent(healerHP("boss", 101, 500.0000437, 1000))
	before := r.brontasRockOverlay(time.Unix(101, 0))
	if before == nil || len(before.Rocks) != 0 || !before.HideCountdown {
		t.Fatalf("50%% must show a static compass without guessed directions: %+v", before)
	}
	r.onEvent(&event.EventWorldProp{EventBase: event.EventBase{EventId: 24, Id: "random", At: 102}, PropID: 49205, X: 25000, Y: 36000, State: "landing", Active: true})
	if got := r.brontasRockOverlay(time.Unix(102, 0)); got == nil || len(got.Rocks) != 0 {
		t.Fatal("ordinary mineral must not light a pending direction")
	}
	r.onEvent(&event.EventWorldProp{EventBase: event.EventBase{EventId: 24, Id: "north", At: 105}, AtMs: 105200, PropID: 49205, X: 25156, Y: 41145, State: "landing", Active: true})
	after := r.brontasRockOverlay(time.Unix(106, 0))
	if after == nil || !reflect.DeepEqual(after.Rocks, []nativeBrontasRockMarker{{Clock: 12}}) || after.Generation != before.Generation || after.EndsAtMs != 150200 {
		t.Fatalf("first actual rock must update the same card and extend its lifetime: %+v", after)
	}
	r.onEvent(&event.EventWorldProp{EventBase: event.EventBase{EventId: 24, Id: "north", At: 107}})
	r.onEvent(healerHP("boss", 108, 500, 1000))
	if r.brontasRockOverlay(time.Unix(108, 0)) != nil {
		t.Fatal("removed rocks reopened the pending map")
	}
	if len(sounds) != 0 {
		t.Fatal("static compass unexpectedly played an alert")
	}
}

func TestBrontasEarlyMapGuardsAndTimeout(t *testing.T) {
	for _, test := range []struct {
		name          string
		race          uint32
		first, second float64
		want          bool
	}{
		{"initial gate", 7602, 500, 500, true},
		{"late join", 7602, 490, 480, false},
		{"other boss", 7600, 510, 500, false},
		{"above gate", 7602, 510, 500.01, false},
		{"cross gate", 7602, 510, 499, true},
		{"death", 7602, 510, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var sounds []nativeReminderSoundRequest
			r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true}}}, &sounds)
			r.onEvent(liveTestAppear("boss", "boss", test.race, 100))
			r.onEvent(healerHP("boss", 100, test.first, 1000))
			r.onEvent(healerHP("boss", 101, test.second, 1000))
			if (r.brontasRockOverlay(time.Unix(102, 0)) != nil) != test.want {
				t.Fatal("unexpected pending map")
			}
			r.onEvent(healerHP("boss", 129, test.second, 1000))
			if r.brontasRockOverlay(time.Unix(131, 0)) != nil {
				t.Fatal("refresh extended a missing-rock timeout")
			}
		})
	}
}

func TestBrontasEarlyMapClearsOnExit(t *testing.T) {
	for _, end := range []event.IEvent{
		&event.EventEntityDisappear{EventBase: event.EventBase{Id: "boss", At: 102}},
		&event.EventFinish{EventBase: event.EventBase{Id: "boss", At: 102}},
		&event.EventEntityDisappear{EventBase: event.EventBase{Id: "self", At: 102}},
		&event.EventLocalEntity{EventBase: event.EventBase{Id: "self", At: 102}, Reset: true},
	} {
		var sounds []nativeReminderSoundRequest
		r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true}}}, &sounds)
		r.localID = "self"
		r.onEvent(liveTestAppear("boss", "brontas", 7602, 100))
		r.onEvent(healerHP("boss", 101, 500, 1000))
		r.onEvent(end)
		if r.brontasRockOverlay(time.Unix(102, 0)) != nil {
			t.Fatalf("map survived exit %T", end)
		}
	}
}

// Optional full capture replay checks the parser -> publisher -> runtime path,
// including every ordinary mineral and the entire encounter, not just windows.
func TestBrontasEarlyMapCapture(t *testing.T) {
	path := os.Getenv("DILMETER_BRONTAS_PCAP")
	if path == "" {
		t.Skip("set DILMETER_BRONTAS_PCAP to the October 26 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true}}}, &sounds)
	if err = reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	pending, confirmed := map[string]bool{}, map[string]bool{}
	clocks := []int{}
	for {
		select {
		case batch := <-events:
			idle.Reset(4 * time.Second)
			for _, e := range batch {
				r.onEvent(e)
				for id, phase := range r.brontasRockPhases {
					if !phase.Confirmed {
						pending[id] = true
					}
				}
				if prop, ok := e.(*event.EventWorldProp); ok && prop.Active {
					if rock, exists := r.brontasRocks[prop.Id]; exists {
						phase := r.brontasRockPhases[rock.BossID]
						if !pending[rock.BossID] || phase.AtMs >= prop.AtMs {
							t.Fatal("rock arrived without an earlier static map")
						}
						confirmed[rock.BossID] = true
						clocks = append(clocks, rock.Clock)
					}
				}
			}
		case <-idle.C:
			want := []int{12, 3, 9, 3, 12, 9, 3, 12, 9, 12}
			if os.Getenv("DILMETER_BRONTAS_CAPTURE_DATE") == "2026-10-20" {
				want = []int{9, 12, 3, 6, 12, 6, 3, 6}
			}
			if !reflect.DeepEqual(clocks, want) || len(pending) != len(want)/2 || len(confirmed) != len(pending) {
				t.Fatalf("pending=%d confirmed=%d clocks=%v", len(pending), len(confirmed), clocks)
			}
			t.Logf("%d early static maps, %d confirmed phases, actual clocks %v", len(pending), len(confirmed), clocks)
			return
		case <-ctx.Done():
			t.Fatal("capture replay timeout")
		}
	}
}
