package main

import (
	"math"
	"testing"
)

func TestBrontasSingleCellStepsAndCrossOrder(t *testing.T) {
	for _, point := range []brontasPoint{{0, 0}, {-.85, .8}, {.32, -.34}, {.6, .6}} {
		for _, target := range brontasStepCandidates(point, &brontasPoint{.67, -.67}) {
			if brontasStepAxis(point, target) == "" {
				t.Fatal("diagonal or skipped cell", point, target)
			}
		}
	}
	v := stationGuideFixture()
	if ok, _ := brontasGridLeg(v, brontasPoint{.48, .48}, 1000, true); ok {
		t.Fatal("diagonal route accepted")
	}
	v.Player.X = -.67
	if ok, _ := brontasGridLeg(v, brontasPoint{.67, 0}, 1000, true); ok {
		t.Fatal("two-cell jump accepted")
	}
	for _, axis := range []string{"horizontal", "vertical"} {
		v = stationGuideFixture()
		v.Rays = v.Rays[:1]
		v.Rays[0].Axis = axis
		v.GuidanceValidUntilMs = 5000
		planBrontasStationGuide(v, 1900)
		if v.Target == nil || v.StepAxis == axis {
			t.Fatal("single ray did not select perpendicular step", axis, v)
		}
	}
	for _, first := range []string{"horizontal", "vertical"} {
		v = stationGuideFixture()
		for i := range v.Rays {
			v.Rays[i].LethalAtMs = 3600
			if v.Rays[i].Axis == first {
				v.Rays[i].LethalAtMs = 3000
			}
		}
		v.GuidanceValidUntilMs = 5000
		planBrontasStationGuide(v, 1520)
		if v.Target == nil || v.NextTarget == nil || v.StepAxis == first {
			t.Fatal("wrong cross order", first, v)
		}
		corner, final := *v.Target, *v.NextTarget
		if brontasStepAxis(corner, final) == "" || brontasStepAxis(corner, final) == v.StepAxis {
			t.Fatal("no right-angle continuation")
		}
		arrive := 1520 + brontasGuideReactionMs + int64(math.Ceil(math.Hypot(corner.X, corner.Y)/v.Player.Speed*1000))
		v.Player.brontasPoint = corner
		v.GuidanceValidUntilMs = arrive + 700
		planBrontasStationGuide(v, arrive)
		if v.Guidance == "pace" {
			depart := v.PaceUntilMs
			v.GuidanceValidUntilMs = 5000
			planBrontasStationGuide(v, depart)
		}
		if v.Target == nil || v.StepAxis != first {
			t.Fatal("arrival did not advance to next leg", v)
		}
	}
}

func TestBrontasFirstMeasuredIntervalUnlocksGuidance(t *testing.T) {
	r := brontasLiveFixture()
	// Already in the mineral, one confirmed in-stride measurement, no guessed speed.
	r.brontasPosition = nativeBrontasPosition{AtMs: 102000, SpeedAtMs: 102000, X: 25160, Y: 40967, Speed: 1200, SpeedSamples: 1}
	p, _ := r.brontasStationPlayer(12, 102000)
	if p == nil || math.Abs(p.Speed-1200*.65/1269) > 1e-9 {
		t.Fatal("first measurement still waits for a third position packet", p)
	}
	r.brontasPosition.SpeedSamples = 0
	p, _ = r.brontasStationPlayer(12, 102000)
	if p == nil || p.Speed != 0 {
		t.Fatal("invented first speed", p)
	}
}

func TestBrontasDisplayedLegStaysOrthogonalWhileMoving(t *testing.T) {
	v := stationGuideFixture()
	v.StepAxis = "horizontal"
	v.Target = &brontasPoint{.48, 0}
	v.NextTarget = &brontasPoint{.48, -.48}
	v.Player.Y = .02
	first, next := brontasDisplayedStep(v)
	if first == nil || next == nil || first.Y != v.Player.Y || next.X != first.X {
		t.Fatal("rendered diagonal after motion")
	}
	v.Player.Y = .2
	if first, _ := brontasDisplayedStep(v); first != nil {
		t.Fatal("off-route movement retained old advice")
	}
}
