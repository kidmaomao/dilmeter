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

func brontasLiveFixture() *nativeReminderRuntime {
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true, GuidanceEnabled: true, ScalePercent: 100}}}, &sounds)
	r.localID = "self"
	r.entities["boss"] = &nativeReminderEntity{ID: "boss", Known: true, Active: true, RaceID: 7602, HealthKnown: true, MaximumHealth: 100, CurrentHealth: 50}
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "rock", At: 100}, AtMs: 100000, PropID: 49205, X: 25156, Y: 41145, Active: true, State: "landing"})
	r.brontasPosition = nativeBrontasPosition{AtMs: 100000, X: 25160, Y: 40150}
	return r
}

func brontasLiveEvent(at int64) *event.EventBrontasRay {
	return &event.EventBrontasRay{EventBase: event.EventBase{Id: "boss", At: at / 1000}, AtMs: at, Kind: "ray", Direction: 1, Lane: 10, DelayMs: 90, LeadMs: 1600, Live: true}
}

func TestBrontasLiveDeadlinesAndUnknownHistory(t *testing.T) {
	r := brontasLiveFixture()
	e := brontasLiveEvent(102000)
	r.observeBrontasRay(e)
	r.observeBrontasRay(e)
	v := r.brontasGuideView(102600)
	if v.Source != "packet" || v.ReceivedCount != 1 || v.LethalAtMs != 103690 || v.Player == nil || v.Target != nil || v.Rays[0].EndsAtMs != 104290 {
		t.Fatalf("packet facts or no-position boundary: %+v", v)
	}
	if v.ValidUntilMs != 104500 || v.Status != "axis" {
		t.Fatalf("normal wave gap incorrectly stale: %+v", v)
	}
	e2 := brontasLiveEvent(102600)
	e2.Direction = 2
	e2.Lane = 6
	r.observeBrontasRay(e2)
	if got := r.brontasGuideView(103700); got.LethalAtMs != 104290 || len(got.Rays) != 2 || got.PendingCount != 1 {
		t.Fatalf("old line masks next deadline: %+v", got)
	}
	v = r.brontasGuideView(104400)
	if v.Status != "watch" || v.LethalAtMs != 0 || len(v.Rays) != 2 || v.Target != nil {
		t.Fatalf("history became safe/ever-lethal: %+v", v)
	}
	if v = r.brontasGuideView(105100); v.Status != "waiting" || len(v.Rays) != 0 || v.ValidUntilMs > 105100 {
		t.Fatalf("stalled stream did not stop: %+v", v)
	}
	r.settings.BossMechanics.BrontasRocks.Facing = "inward"
	v = r.brontasGuideView(103000)
	offset := *v.Rays[0].Offset
	if v.Rays[0].Axis != "horizontal" || r.brontasGuideView(103000).Rays[0].Axis != "horizontal" || *r.brontasGuideView(103000).Rays[0].Offset != offset {
		t.Fatal("rotation mutated stored observations")
	}
}

func TestBrontasLiveLifecycleAndOfflineGuard(t *testing.T) {
	beforeFirst := brontasLiveFixture()
	beforeFirst.onEvent(&event.EventBrontasRay{EventBase: event.EventBase{Id: "self"}, AtMs: 101000, Kind: "death", Live: true})
	beforeFirst.onEvent(brontasLiveEvent(102000))
	if len(beforeFirst.brontasLiveRays.Rays) != 0 {
		t.Fatal("death before first ray was forgotten")
	}
	r := brontasLiveFixture()
	e := brontasLiveEvent(102000)
	e.Live = false
	r.onEvent(e)
	if len(r.brontasLiveRays.Rays) != 0 {
		t.Fatal("offline capture activated current guidance")
	}
	e.Live = true
	e.Id = "wrong"
	r.onEvent(e)
	if len(r.brontasLiveRays.Rays) != 0 {
		t.Fatal("wrong boss accepted")
	}
	e.Id = "boss"
	r.onEvent(e)
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "other", At: 100}, AtMs: 100001, PropID: 49205, X: 20756, Y: 36745, Active: true, State: "landing"})
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "rock"}})
	r.onEvent(brontasLiveEvent(102600))
	if len(r.brontasLiveRays.Rays) != 2 || r.brontasLiveRays.PhaseAtMs != 100000 {
		t.Fatal("removing one of two rocks reset the surviving phase")
	}
	r.onEvent(&event.EventBrontasRay{EventBase: event.EventBase{Id: "self"}, AtMs: 102700, Kind: "death", Live: true})
	r.onEvent(brontasLiveEvent(102800))
	if len(r.brontasLiveRays.Rays) != 0 {
		t.Fatal("late ray revived a dead player's phase")
	}
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "other"}})
	r.onEvent(brontasLiveEvent(103000))
	if len(r.brontasLiveRays.Rays) != 0 {
		t.Fatal("no-rock ray accepted")
	}
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "new", At: 110}, AtMs: 110000, PropID: 49205, X: 25156, Y: 41145, Active: true, State: "landing"})
	r.onEvent(brontasLiveEvent(109999))
	if len(r.brontasLiveRays.Rays) != 0 {
		t.Fatal("previous phase leaked into next one")
	}
	r.onEvent(brontasLiveEvent(112000))
	if len(r.brontasLiveRays.Rays) != 1 {
		t.Fatal("new phase failed to restart")
	}
	r.onEvent(&event.EventBrontasRay{EventBase: event.EventBase{Id: "boss"}, AtMs: 112100, Kind: "end", Live: true})
	r.onEvent(brontasLiveEvent(112200))
	if len(r.brontasLiveRays.Rays) != 0 {
		t.Fatal("whole-phase end ignored")
	}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "new-player"}})
	if len(r.brontasRocks) != 0 || r.brontasLiveRays.BossID != "" {
		t.Fatal("character switch retained arena state")
	}
}

// Opt-in user's 2026-10-30 capture. No raw capture, player IDs, or personal
// configuration is committed. The production reader remains offline; only a
// copy of its events is marked live inside this private test runtime.
func TestBrontasLiveOctober30Capture(t *testing.T) {
	path := os.Getenv("DILMETER_BRONTAS_LIVE_PCAP")
	if path == "" {
		t.Skip("set DILMETER_BRONTAS_LIVE_PCAP for the October 30 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	pub := newEventPublisher(ctx, reader)
	ch := make(chan []event.IEvent, 1000)
	pub.addClient(ctx, ch)
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true, GuidanceEnabled: true, ScalePercent: 100}}}, &sounds)
	if err = reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(3 * time.Second)
	defer idle.Stop()
	rays, visible, deaths, rocks := 0, 0, 0, 0
	for {
		select {
		case batch := <-ch:
			idle.Reset(3 * time.Second)
			for _, raw := range batch {
				if e, ok := raw.(*event.EventBrontasRay); ok {
					if e.Live {
						t.Fatal("offline parser produced a live observation")
					}
					if e.Kind == "ray" {
						rays++
					}
					if e.Kind == "death" && e.Id == r.localID {
						deaths++
					}
					copy := *e
					copy.Live = true
					r.onEvent(&copy)
					if e.Kind == "ray" {
						if view := r.brontasRockOverlay(time.UnixMilli(e.AtMs)); view != nil && view.BrontasGuide != nil && view.BrontasGuide.Source == "packet" && !r.brontasLiveRays.Ended && len(r.brontasLiveRays.Rays) > 0 {
							if view.BrontasGuide.Target != nil || (view.BrontasGuide.LethalAtMs != 0 && view.BrontasGuide.LethalAtMs <= e.AtMs) {
								t.Fatal("replay fabricated an arrow or reset a deadline")
							}
							visible++
							if visible == 5 && os.Getenv("DILMETER_BRONTAS_LIVE_VIEW_OUT") != "" {
								data, err := json.Marshal(struct {
									AtMs int64                          `json:"atMs"`
									Item *nativeBossMechanicOverlayItem `json:"item"`
								}{e.AtMs, view})
								if err != nil {
									t.Fatal(err)
								}
								if err := os.WriteFile(os.Getenv("DILMETER_BRONTAS_LIVE_VIEW_OUT"), data, 0600); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
				} else {
					r.onEvent(raw)
				}
				if e, ok := raw.(*event.EventWorldProp); ok && e.Active {
					if _, exists := r.brontasRocks[e.Id]; exists {
						rocks++
					}
				}
			}
		case <-idle.C:
			_, parsed, errors := reader.GetStats()
			if rays != 36 || visible != 17 || rocks != 2 || deaths != 3 || errors != 0 || len(r.brontasLiveRays.Rays) != 0 {
				t.Fatalf("capture: rays=%d visible=%d rocks=%d deaths=%d parsed=%d errors=%d remaining=%d", rays, visible, rocks, deaths, parsed, errors, len(r.brontasLiveRays.Rays))
			}
			t.Logf("%d parsed, %d ray starts, %d active-phase displays, %d rock phases, %d direct deaths, %d errors; late post-failure rays rejected", parsed, rays, visible, rocks, deaths, errors)
			return
		case <-ctx.Done():
			t.Fatal("capture replay timeout")
		}
	}
}
