package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func brontasTestFrame(now int64) brontasGuideFrame {
	zero, width := 0.0, .12
	return brontasGuideFrame{Session: "test", Sequence: 1, BossID: "boss", Active: true, Clock: 12, ObservedAtMs: now, GeometryComplete: true,
		Player: &brontasGuidePlayer{brontasPoint: brontasPoint{X: 0, Y: -.35}, Radius: .08, Speed: 1.2},
		Rays:   []brontasGuideRay{{ID: "ray", Axis: "vertical", WarningAtMs: now, LethalAtMs: now + 1600, EndsAtMs: now + 2500, Offset: &zero, HalfWidth: &width}}}
}

func TestBrontasGuideRoutes(t *testing.T) {
	const now int64 = 1700000000000
	for _, axis := range []string{"horizontal", "vertical"} {
		for _, entry := range []brontasPoint{{0, -.35}, {0, .35}, {-.35, 0}, {.35, 0}, {0, 0}} {
			f := brontasTestFrame(now)
			f.Player.brontasPoint = entry
			f.Rays[0].Axis = axis
			v := planBrontasGuide(f, now)
			if v.Status != "move" && v.Status != "hold" {
				t.Fatalf("%s at %v: %+v", axis, entry, v)
			}
			if v.Target != nil && (!brontasRouteClear(f, *v.Target, now) || math.Hypot(v.Target.X, v.Target.Y) > 1-f.Player.Radius) {
				t.Fatal("unsafe target", v.Target)
			}
		}
	}
}

func TestBrontasGuideCombinesOppositeAndRandomRays(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	f.Player.brontasPoint = brontasPoint{}
	r := f.Rays[0]
	r.ID = "opposite-or-random"
	r.Axis = "horizontal"
	f.Rays = append(f.Rays, r)
	v := planBrontasGuide(f, now)
	if v.Target == nil || math.Abs(v.Target.X) <= .23 || math.Abs(v.Target.Y) <= .23 {
		t.Fatalf("must escape both bands: %+v", v)
	}
	// An active band covering the entire circle cannot produce an arrow.
	width := 2.0
	f.Rays[0].HalfWidth = &width
	if got := planBrontasGuide(f, now); got.Status != "blocked" || got.Target != nil {
		t.Fatalf("%+v", got)
	}
}

// A wide offset cross leaves nine visual cells, but the two lower corner
// pockets cannot hold the player's entire disk plus the configured margin.
// These are schematic proportions, not measured game constants.
func TestBrontasGuideWideBandsRejectTinyCorners(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	f.Player.brontasPoint = brontasPoint{X: 0, Y: -.4}
	x, y, verticalWidth, horizontalWidth := 0.0, -.12, .40, .51
	f.Rays[0].Offset, f.Rays[0].HalfWidth = &x, &verticalWidth
	horizontal := f.Rays[0]
	horizontal.ID, horizontal.Axis = "wide-horizontal", "horizontal"
	horizontal.Offset, horizontal.HalfWidth = &y, &horizontalWidth
	f.Rays = append(f.Rays, horizontal)

	for _, side := range []float64{-1, 1} {
		q := brontasPoint{X: side * .45, Y: -.67}
		if math.Hypot(q.X, q.Y) >= 1 || math.Abs(q.X) <= verticalWidth || math.Abs(q.Y-y) <= horizontalWidth {
			t.Fatal("fixture should have a visibly clear point inside each small corner")
		}
		if brontasRouteClear(f, q, now) {
			t.Fatal("clear center point does not imply the player fits", q)
		}
	}
	// Even the closest potential center clearing both enlarged bands lies
	// beyond the inset circle; neither lower corner can fit this player.
	margin := f.Player.Radius + .03
	if math.Hypot(verticalWidth+margin, -y+horizontalWidth+margin) <= 1-margin {
		t.Fatal("fixture no longer excludes both lower pockets")
	}
	v := planBrontasGuide(f, now)
	if v.Target == nil || v.Target.Y <= y+horizontalWidth+margin || !brontasRouteClear(f, *v.Target, now) {
		t.Fatalf("must use a reachable upper pocket instead: %+v", v)
	}
}

func TestBrontasGuideChecksTravelAndDeadlines(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	f.Player.X = -.6
	f.Player.Y = 0
	f.Rays[0].WarningAtMs = now - 1000
	f.Rays[0].LethalAtMs = now - 500
	if brontasRouteClear(f, brontasPoint{X: .6}, now) {
		t.Fatal("safe endpoint across active ray is not a safe route")
	}
	f.Rays[0].LethalAtMs = now + 1600
	if !brontasRouteClear(f, brontasPoint{X: .6}, now) {
		t.Fatal("crossing warning before activation should be possible")
	}
	f.Player.X = 0
	f.Rays[0].LethalAtMs = now + 250
	if got := planBrontasGuide(f, now); got.Status != "blocked" {
		t.Fatalf("reaction budget must reject late route: %+v", got)
	}
	if got := planBrontasGuide(f, now+brontasGuideFreshMs); got.Target != nil || got.Status != "waiting" {
		t.Fatal("stale arrow", got)
	}
}

func TestBrontasGuideArrowExpiresBeforeRouteBecomesLate(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	f.Rays[0].LethalAtMs = now + 800
	v := planBrontasGuide(f, now)
	if v.Target == nil || v.ValidUntilMs <= now || v.ValidUntilMs >= now+brontasGuideFreshMs {
		t.Fatalf("missing route deadline: %+v", v)
	}
	if !brontasRouteClear(f, *v.Target, v.ValidUntilMs-1) || brontasRouteClear(f, *v.Target, v.ValidUntilMs+1) {
		t.Fatal("route validity does not match collision deadline")
	}
}

func TestBrontasGuideRequiresCompleteGeometry(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	f.GeometryComplete = false
	f.Player = nil
	f.Rays[0].Offset = nil
	f.Rays[0].HalfWidth = nil
	if err := validateBrontasGuideFrame(f, now); err != nil {
		t.Fatal(err)
	}
	v := planBrontasGuide(f, now)
	if v.Status != "axis" || v.Target != nil {
		t.Fatal(v)
	}
	rotateBrontasGuide(v, 90)
	if v.Clock != 3 || v.Rays[0].Axis != "horizontal" || v.Instruction != "横向射线 · 沿地图纵向躲避" {
		t.Fatal(v)
	}
	f.GeometryComplete = true
	if validateBrontasGuideFrame(f, now) == nil {
		t.Fatal("missing geometry accepted")
	}
	f = brontasTestFrame(now)
	f.Player.X = 2
	if got := planBrontasGuide(f, now); got.Status != "enter" || got.Target != nil {
		t.Fatal(got)
	}
}

func TestBrontasGuideUnknownEndRemainsDangerous(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	f.Rays[0].EndsAtMs = 0
	if err := validateBrontasGuideFrame(f, now); err != nil {
		t.Fatal(err)
	}
	if got := planBrontasGuide(f, now); got.Status != "move" {
		t.Fatal("unknown end still permits leaving a warning", got)
	}
	f.Player.X = -.6
	f.Player.Y = 0
	f.Rays[0].WarningAtMs = now - 1000
	f.Rays[0].LethalAtMs = now - 500
	if brontasRouteClear(f, brontasPoint{X: .6}, now) {
		t.Fatal("unknown end must not declare active ray expired")
	}
}

func TestBrontasGuideGeometryRotatesTogether(t *testing.T) {
	const now int64 = 1700000000000
	f := brontasTestFrame(now)
	offset := .1
	f.Rays[0].Offset = &offset
	for _, rotation := range []int{0, 90, 180, 270} {
		v := planBrontasGuide(f, now)
		original := *v.Target
		rotateBrontasGuide(v, rotation)
		if *v.Target != rotateBrontasPoint(original, rotation) || v.Player.brontasPoint != rotateBrontasPoint(f.Player.brontasPoint, rotation) {
			t.Fatal("rotation mismatch")
		}
	}
	if *f.Rays[0].Offset != .1 || f.Rays[0].Axis != "vertical" {
		t.Fatal("rotation mutated input")
	}
}

func TestBrontasGuideRealMineralLifecycle(t *testing.T) {
	const now int64 = 1700000000000
	r := &nativeReminderRuntime{entities: map[string]*nativeReminderEntity{"boss": {ID: "boss", Known: true, Active: true, RaceID: 7602, HealthKnown: true, MaximumHealth: 100, CurrentHealth: 50}}, brontasGuide: brontasTestFrame(now)}
	r.settings.BossMechanics.BrontasRocks = nativeBrontasRockSettings{Enabled: true, GuidanceEnabled: true, ScalePercent: 100}
	if r.brontasRockOverlay(time.UnixMilli(now)) != nil {
		t.Fatal("feed must not invent minerals")
	}
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "rock", At: now / 1000}, AtMs: now, PropID: 49205, X: 25156, Y: 41145, Active: true, State: "landing"})
	v := r.brontasRockOverlay(time.UnixMilli(now))
	if v == nil || v.BrontasGuide == nil || v.BrontasGuide.Target == nil {
		t.Fatalf("live mineral did not enable guide: %+v", v)
	}
	r.brontasGuide.BossID = "other"
	if r.brontasRockOverlay(time.UnixMilli(now)).BrontasGuide.Target != nil {
		t.Fatal("wrong encounter used")
	}
	r.observeBrontasProp(&event.EventWorldProp{EventBase: event.EventBase{Id: "rock"}})
	if r.brontasRockOverlay(time.UnixMilli(now)) != nil {
		t.Fatal("removal did not hide overlay")
	}
}

func TestBrontasGuideAPI(t *testing.T) {
	now := time.Now().UnixMilli()
	r := &nativeReminderRuntime{brontasGuideCh: make(chan brontasGuideFrame, 4)}
	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = r
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	})
	for _, tc := range []struct {
		name, remote, origin string
		stale                bool
		want                 int
	}{
		{"valid", "127.0.0.1:1234", "", false, 202},
		{"remote", "192.168.1.5:1234", "", false, 403},
		{"foreign webpage", "127.0.0.1:1234", "https://example.com", false, 403},
		{"stale", "127.0.0.1:1234", "", true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := brontasTestFrame(now)
			if tc.stale {
				f.ObservedAtMs -= 1000
			}
			b, _ := json.Marshal(f)
			req := httptest.NewRequest("POST", "http://127.0.0.1:8030/api/brontas_guide", bytes.NewReader(b))
			req.RemoteAddr = tc.remote
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handleBrontasGuide(w, req)
			if w.Code != tc.want {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
		})
	}
	if len(r.brontasGuideCh) != 1 {
		t.Fatal("invalid input reached runtime")
	}
}
