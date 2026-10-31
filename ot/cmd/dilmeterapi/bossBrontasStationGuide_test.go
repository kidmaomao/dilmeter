package main

import (
	"math"
	"testing"
)

func TestBrontasFacingAllStations(t *testing.T) {
	cases := []struct {
		clock   int
		out, in int
	}{{12, 0, 180}, {3, 270, 90}, {6, 180, 0}, {9, 90, 270}}
	for _, tc := range cases {
		for _, f := range []struct {
			name     string
			rotation int
		}{{"outward", tc.out}, {"", tc.out}, {"inward", tc.in}, {"north", 0}, {"south", 180}} {
			if got := brontasFacingRotation(tc.clock, f.name); got != f.rotation {
				t.Fatal(tc, f, got)
			}
			offset, width := .6, 1.0/3
			v := &nativeBrontasGuideView{Clock: tc.clock, Player: &brontasGuidePlayer{brontasPoint: brontasPoint{X: .2, Y: .3}}, Target: &brontasPoint{X: -.6, Y: .6}, Rays: []brontasGuideRay{{Axis: "vertical", Offset: &offset, HalfWidth: &width}}}
			wantPlayer := rotateBrontasPoint(v.Player.brontasPoint, f.rotation)
			wantTarget := rotateBrontasPoint(*v.Target, f.rotation)
			orientBrontasStation(v, f.name)
			if v.Clock != tc.clock || v.Player.brontasPoint != wantPlayer || *v.Target != wantTarget {
				t.Fatal("inconsistent facing", v)
			}
		}
	}
}

func TestBrontasMeasuredSpeed(t *testing.T) {
	a := nativeBrontasPosition{AtMs: 1000, RegionID: 1, X: 10000, Y: 20000, Moving: true}
	b := nativeBrontasPosition{AtMs: 1200, RegionID: 1, X: 10280, Y: 20000, Moving: true}
	b.Speed, b.SpeedSamples, b.SpeedAtMs = brontasPositionSpeed(a, b)
	c := nativeBrontasPosition{AtMs: 1400, RegionID: 1, X: 10560, Y: 20000, Moving: true}
	c.Speed, c.SpeedSamples, c.SpeedAtMs = brontasPositionSpeed(b, c)
	if math.Abs(c.Speed-1400) > .01 || c.SpeedSamples != 2 {
		t.Fatal(c)
	}
	for _, bad := range []nativeBrontasPosition{{AtMs: 1500, RegionID: 2, X: 10700, Y: 20000, Moving: true}, {AtMs: 1500, RegionID: 1, X: 40000, Y: 20000, Moving: true}, {AtMs: 4500, RegionID: 1, X: 10700, Y: 20000, Moving: true}} {
		if speed, n, _ := brontasPositionSpeed(c, bad); speed != 0 || n != 0 {
			t.Fatal("unverified speed", bad, speed, n)
		}
	}
}

func stationGuideFixture() *nativeBrontasGuideView {
	zero, width := 0.0, 1.0/3
	return &nativeBrontasGuideView{Source: "packet", Stage: "station", Clock: 12, ValidUntilMs: 5000, PlayerValidUntilMs: 1700, GuidanceValidUntilMs: 1350, Player: &brontasGuidePlayer{brontasPoint: brontasPoint{}, Speed: 1.0, Radius: .055}, Rays: []brontasGuideRay{
		{ID: "h", Axis: "horizontal", Offset: &zero, HalfWidth: &width, WarningAtMs: 1000, LethalAtMs: 3000},
		{ID: "v", Axis: "vertical", Offset: &zero, HalfWidth: &width, WarningAtMs: 1000, LethalAtMs: 3200},
	}}
}

func TestBrontasGuideRouteAndUnknownEnd(t *testing.T) {
	v := stationGuideFixture()
	v.GuidanceValidUntilMs, v.PlayerValidUntilMs = 5000, 5000
	planBrontasStationGuide(v, 1120)
	if v.Target == nil || v.NextTarget == nil || v.Guidance != "move" || v.StepAxis != "vertical" || v.Target.X != 0 || math.Abs(v.Target.Y) < .453 {
		t.Fatal("no cross exit", v)
	}
	if ok, _ := brontasGridLeg(v, *v.Target, 1120, true); !ok {
		t.Fatal("recommended route unsafe")
	}
	// Arrival at the corner promotes the perpendicular second step.
	final := *v.NextTarget
	v.Player.brontasPoint = *v.Target
	v.GuidanceValidUntilMs = 5000
	planBrontasStationGuide(v, 2083)
	if v.Target == nil || v.StepAxis != "horizontal" {
		t.Fatal("no second step", v)
	}
	v.Player.brontasPoint = final
	v.GuidanceValidUntilMs = 5000
	planBrontasStationGuide(v, 2800)
	if v.Guidance != "hold" || v.Target != nil {
		t.Fatal("arrival not reflected", v)
	}
	v = stationGuideFixture()
	v.Rays[0].LethalAtMs = 1100
	planBrontasStationGuide(v, 1000)
	if v.Target == nil || v.GuidanceKind != "escape" {
		t.Fatal("late departure needs an explicitly urgent exit")
	}
	v = stationGuideFixture()
	v.Rays[0].LethalAtMs = 900
	planBrontasStationGuide(v, 1000)
	if v.Target == nil || v.GuidanceKind != "escape" {
		t.Fatal("unknown active ray must not be advertised as safe", v)
	}
	v = stationGuideFixture()
	v.Player.Speed = 0
	planBrontasStationGuide(v, 1000)
	if v.Target != nil || v.Guidance != "waiting" {
		t.Fatal("invented move speed")
	}
	v = stationGuideFixture()
	planBrontasStationGuide(v, 1350)
	if v.Target != nil {
		t.Fatal("expired position guided")
	}
}

func TestBrontasGuideRejectsDangerDuringCrossing(t *testing.T) {
	v := stationGuideFixture()
	v.Player.brontasPoint = brontasPoint{X: -.8, Y: -.8}
	v.Rays = v.Rays[:1]
	v.Rays[0].Axis = "vertical"
	v.Rays[0].LethalAtMs = 1500
	// Endpoint is outside the line, but the route enters it after activation.
	if ok, _ := brontasGridRoute(v, brontasPoint{X: .8, Y: -.8}, 1000); ok {
		t.Fatal("endpoint-only route accepted")
	}
	v.Player.X = -.5
	v.Rays[0].EndsAtMs = 700
	v.Rays[0].LethalAtMs = 500
	if ok, _ := brontasGridRoute(v, brontasPoint{X: 0, Y: -.8}, 1000); !ok {
		t.Fatal("explicitly ended ray never clears")
	}
}

func TestBrontasFacingDoesNotChangePhysicalRoute(t *testing.T) {
	base := brontasStationPreview(100000, 103000, 9, "north")
	if base.Target == nil {
		t.Fatal("preview has no route")
	}
	for _, facing := range []string{"outward", "inward", "north", "south"} {
		v := brontasStationPreview(100000, 103000, 9, facing)
		want := rotateBrontasPoint(*base.Target, brontasFacingRotation(9, facing))
		if v.Target == nil || *v.Target != want {
			t.Fatal("view preference changed route choice", facing, v.Target, want)
		}
		if v.NextTarget == nil || *v.NextTarget != rotateBrontasPoint(*base.NextTarget, brontasFacingRotation(9, facing)) || brontasStepAxis(v.Player.brontasPoint, *v.Target) != v.StepAxis || brontasStepAxis(*v.Target, *v.NextTarget) == v.StepAxis {
			t.Fatal("second leg did not rotate with the grid", facing)
		}
	}
}
