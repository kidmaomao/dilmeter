package main

import (
	"context"
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"math"
	"os"
	"testing"
	"time"
)

func TestBrontasStationOnlyNineCells(t *testing.T) {
	for _, clock := range []int{12, 3, 6, 9} {
		cx, cy, _ := brontasStationGrid(clock)
		for _, axis := range []string{"horizontal", "vertical"} {
			count := 0
			for lane := 0; lane <= 12; lane++ {
				ray, ok := brontasStationRay(nativeBrontasPacketRay{Lane: uint32(lane), brontasGuideRay: brontasGuideRay{Axis: axis}}, clock)
				if !ok {
					continue
				}
				count++
				center := cx
				if axis == "horizontal" {
					center = cy
				}
				if *ray.HalfWidth != 1.0/3 || math.Abs(*ray.Offset-float64(lane-center)*2/3) > 1e-8 {
					t.Fatal("bad crop", clock, axis, lane)
				}
			}
			if count != 3 {
				t.Fatal("not three contiguous channels", clock, axis, count)
			}
		}
	}
}

func TestBrontasMapThenEntryAndFootSwitch(t *testing.T) {
	r := brontasLiveFixture()
	r.brontasPosition = nativeBrontasPosition{}
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "left"}, AtMs: 100001, PropID: 49205, X: 20756, Y: 36745, State: "landing", Active: true})
	if v := r.brontasGuideView(101000); v.Stage != "map" || v.Player != nil || len(v.Rays) > 0 {
		t.Fatal("entry assumed", v)
	}
	position := func(x, y uint32, at int64) {
		r.onEvent(&event.EventBrontasRay{EventBase: event.EventBase{Id: "self"}, Kind: "position", AtMs: at, X: x, Y: y, Live: true, Moving: true})
	}
	// Still outside: movement destination is never involved in station choice.
	position(25160, 36740, 101100)
	if v := r.brontasGuideView(101100); v.Stage != "map" {
		t.Fatal("outside player entered", v)
	}
	position(25160, 40150, 102000)
	for i, spec := range []struct {
		direction uint16
		lane      uint32
		at        int64
	}{{1, 10, 102000}, {2, 6, 102300}, {0, 7, 102600}, {1, 1, 101900}} {
		e := brontasLiveEvent(spec.at)
		e.Direction, e.Lane = spec.direction, spec.lane
		r.onEvent(e)
		_ = i
	}
	v := r.brontasGuideView(102600)
	h, vert := brontasFootRayDeadlines(v, 102600)
	if v.Stage != "station" || v.Clock != 12 || len(v.Rays) != 3 || h != 103690 || vert != 103990 {
		t.Fatalf("wrong local rays or cross timers: %+v %d %d", v, h, vert)
	}
	// New coordinates, no new ray: both timers must immediately use the new cell.
	position(25900, 41000, 102650)
	v = r.brontasGuideView(102650)
	h, vert = brontasFootRayDeadlines(v, 102650)
	if h != 0 || vert != 104290 {
		t.Fatal("old cell countdown retained", h, vert)
	}
	// Rotation changes axes without changing which physical rays cover the feet.
	r.settings.BossMechanics.BrontasRocks.Facing = "inward"
	v = r.brontasGuideView(102650)
	h, vert = brontasFootRayDeadlines(v, 102650)
	if h != 0 || vert != 104290 || v.Clock != 12 {
		t.Fatal("rotation changes physical membership", h, vert, v.Clock)
	}
	if v = r.brontasGuideView(103350); v.Player != nil || v.LethalAtMs != 0 {
		t.Fatal("stale moving position kept personal countdown", v)
	}
	position(20800, 36700, 103360)
	if v = r.brontasGuideView(103360); v.Clock != 9 {
		t.Fatal("station switch did not follow actual position", v.Clock)
	} // Actual station label survives facing rotation.
	position(25160, 36740, 103400)
	if v = r.brontasGuideView(103400); v.Stage != "map" || len(v.Rays) != 0 {
		t.Fatal("exit retained local grid", v)
	}
}

func TestBrontasLocalPositionOnlyAndOffline(t *testing.T) {
	r := brontasLiveFixture()
	before := r.brontasPosition
	for _, e := range []*event.EventBrontasRay{
		{EventBase: event.EventBase{Id: "teammate"}, AtMs: 102000, Kind: "position", X: 20900, Y: 36700, Live: true},
		{EventBase: event.EventBase{Id: "self"}, AtMs: 102000, Kind: "position", X: 20900, Y: 36700, Live: false},
		{EventBase: event.EventBase{Id: "self"}, AtMs: 99999, Kind: "position", X: 20900, Y: 36700, Live: true},
	} {
		r.onEvent(e)
		if r.brontasPosition != before {
			t.Fatal("untrusted position accepted")
		}
	}
	ch := make(chan []event.IEvent, 4)
	pub := &eventPublisher{ctx: context.Background(), localEntityId: 99, clientMap: map[uint32]*eventClient{1: {ctx: context.Background(), ch: ch}}}
	p := &packet.GamePacket{Id: 98, Op: 26004, At: time.UnixMilli(102000), Msg: packet.Message{packet.NewMessageElemByte(1), packet.NewMessageElemInt(1), packet.NewMessageElemInt(25160), packet.NewMessageElemInt(40150)}}
	pub.publishBrontasRayPacket(p)
	if len(ch) != 0 {
		t.Fatal("teammate published as local")
	}
	p.Id = 99
	pub.publishBrontasRayPacket(p)
	if len(ch) != 1 {
		t.Fatal("own position not immediately published")
	}
	batch := <-ch
	e := batch[0].(*event.EventBrontasRay)
	if e.Kind != "position" || e.X != 25160 || e.Live {
		t.Fatal("wrong local event", e)
	}
}

func TestBrontasTwoStagePreview(t *testing.T) {
	if v := brontasStationPreview(100000, 101000, 12); v.Stage != "map" || v.Player != nil {
		t.Fatal(v)
	}
	v := brontasStationPreview(100000, 102000, 12)
	h, vert := brontasFootRayDeadlines(v, 102000)
	if v.Stage != "station" || h != 105000 || vert != 104600 {
		t.Fatal(v, h, vert)
	}
	v = brontasStationPreview(100000, 104300, 12)
	h, vert = brontasFootRayDeadlines(v, 104300)
	if h != 105000 || vert != 106500 {
		t.Fatal("preview cell switch", h, vert)
	}
}

// Full new capture replay: verify both the arena crop and personal channel
// changes through the production packet reader/publisher, isolated from live UI.
func TestBrontasStationOctober30Capture(t *testing.T) {
	path := os.Getenv("DILMETER_BRONTAS_GRID_PCAP")
	if path == "" {
		t.Skip("optional local capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	pub := newEventPublisher(ctx, reader)
	ch := make(chan []event.IEvent, 2000)
	pub.addClient(ctx, ch)
	r := brontasLiveFixture()
	r.brontasRocks = nil
	r.brontasPosition = nativeBrontasPosition{}
	r.localID = ""
	if err = reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(3 * time.Second)
	defer idle.Stop()
	rays, positions, rocks, personal, filtered := 0, 0, 0, 0, 0
	guided, calibrated, projected := 0, 0, 0
	guideKinds := map[string]int{}
	var timeline []json.RawMessage
	clocks := map[int]bool{}
	exported := false
	for {
		select {
		case batch := <-ch:
			idle.Reset(3 * time.Second)
			for _, raw := range batch {
				at := int64(0)
				if e, ok := raw.(*event.EventBrontasRay); ok {
					if e.Live {
						t.Fatal("offline event live")
					}
					copy := *e
					copy.Live = true
					r.onEvent(&copy)
					at = e.AtMs
					if e.Kind == "ray" {
						rays++
					}
					if e.Kind == "position" {
						positions++
					}
				} else {
					r.onEvent(raw)
				}
				if e, ok := raw.(*event.EventWorldProp); ok && e.Active {
					if _, exists := r.brontasRocks[e.Id]; exists {
						rocks++
					}
				}
				if at == 0 {
					continue
				}
				item := r.brontasRockOverlay(time.UnixMilli(at))
				if item == nil || item.BrontasGuide == nil {
					continue
				}
				v := item.BrontasGuide
				if v.Stage != "station" {
					continue
				}
				clocks[v.Clock] = true
				if os.Getenv("DILMETER_BRONTAS_TIMELINE_OUT") != "" {
					b, _ := json.Marshal(struct {
						AtMs int64                   `json:"atMs"`
						View *nativeBrontasGuideView `json:"view"`
					}{at, v})
					timeline = append(timeline, b)
				}
				if v.Player != nil && v.Player.Speed > 0 {
					calibrated++
				}
				if v.Motion != nil {
					projected++
				}
				if v.Target != nil {
					axis := brontasStepAxis(v.Player.brontasPoint, *v.Target)
					if axis == "" || axis != v.StepAxis {
						t.Fatal("non-orthogonal live step", v.Player, v.Target)
					}
					if v.NextTarget != nil {
						nextAxis := brontasStepAxis(*v.Target, *v.NextTarget)
						if nextAxis == "" || nextAxis == axis {
							t.Fatal("live route is not a right angle")
						}
					}
					guided++
					guideKinds[v.GuidanceKind]++
					if guided == 1 && os.Getenv("DILMETER_BRONTAS_GUIDE_VIEW_OUT") != "" {
						data, _ := json.Marshal(struct {
							AtMs int64                          `json:"atMs"`
							Item *nativeBossMechanicOverlayItem `json:"item"`
						}{at, item})
						if err := os.WriteFile(os.Getenv("DILMETER_BRONTAS_GUIDE_VIEW_OUT"), data, 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				if len(v.Rays) < len(r.brontasLiveRays.Rays) {
					filtered++
				}
				for _, ray := range v.Rays {
					if math.Abs(*ray.Offset) > 2.0/3+1e-8 || *ray.HalfWidth != 1.0/3 {
						t.Fatal("global ray escaped crop")
					}
				}
				h, vert := brontasFootRayDeadlines(v, at)
				if h > 0 || vert > 0 {
					personal++
				}
				if !exported && h > 0 && vert > 0 && os.Getenv("DILMETER_BRONTAS_GRID_VIEW_OUT") != "" {
					data, err := json.Marshal(struct {
						AtMs int64                          `json:"atMs"`
						Item *nativeBossMechanicOverlayItem `json:"item"`
					}{at, item})
					if err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(os.Getenv("DILMETER_BRONTAS_GRID_VIEW_OUT"), data, 0600); err != nil {
						t.Fatal(err)
					}
					exported = true
				}
			}
		case <-idle.C:
			_, parsed, errors := reader.GetStats()
			if rays != 212 || rocks != 3 || parsed != 8555 || errors != 0 || positions < 30 || personal == 0 || filtered == 0 || !clocks[6] || !clocks[12] || !clocks[9] {
				t.Fatalf("rays=%d rocks=%d parsed=%d errors=%d positions=%d personal=%d filtered=%d clocks=%v", rays, rocks, parsed, errors, positions, personal, filtered, clocks)
			}
			t.Logf("guided=%d calibrated=%d projected=%d kinds=%v", guided, calibrated, projected, guideKinds)
			if out := os.Getenv("DILMETER_BRONTAS_TIMELINE_OUT"); out != "" {
				b, _ := json.Marshal(timeline)
				if err := os.WriteFile(out, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("parsed=%d errors=%d rays=%d rocks=%d positions=%d personalViews=%d filteredViews=%d clocks=%v", parsed, errors, rays, rocks, positions, personal, filtered, clocks)
			return
		case <-ctx.Done():
			t.Fatal("timeout")
		}
	}
}

func TestBrontasPreviewRefreshWithUnchangedCombat(t *testing.T) {
	preserveBossPreviewState(t)
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{}, &sounds)
	now := time.Now()
	r.publishNativeSkillState(now)
	setBossMechanicPreview(bossMechanicPreviewRequest{Key: brontasRockKey, GuidanceEnabled: true, DurationSeconds: 8}, now.UnixMilli()-2000)
	if readBurstPreviewState(t).Mechanics[0].BrontasGuide.Stage != "map" {
		t.Fatal("initial preview")
	}
	r.publishNativeSkillState(now)
	if readBurstPreviewState(t).Mechanics[0].BrontasGuide.Stage != "station" {
		t.Fatal("unchanged state froze preview")
	}
}
