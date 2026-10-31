package main

import (
	"math"
	"testing"
)

func rhythmFixture() *nativeBrontasGuideView {
	offset, width := brontasCellWidth, 1.0/3
	return &nativeBrontasGuideView{Source: "packet", Stage: "station", Clock: 12, ValidUntilMs: 20000, PlayerValidUntilMs: 20000, GuidanceValidUntilMs: 20000,
		Player: &brontasGuidePlayer{brontasPoint: brontasPoint{0, brontasCellWidth}, Speed: 2, Radius: .055},
		Rays:   []brontasGuideRay{{ID: "first", Axis: "horizontal", Offset: &offset, HalfWidth: &width, WarningAtMs: 1000, LethalAtMs: 6000, EndsAtMs: 6600}}}
}

func TestBrontasRhythmTwoFiveFourSeven(t *testing.T) {
	v := rhythmFixture()
	planBrontasStationGuide(v, 1000)
	if v.rhythm.To != 4 || v.rhythm.Next != 3 || v.StepAxis != "vertical" {
		t.Fatal("expected 2 -> 5 -> 4", v.rhythm)
	}
	// Re-evaluate frequently, with sideways noise. Keep the same cell and do
	// not switch to the next leg while merely straddling its border.
	for i, y := range []float64{.6, .51, .44, .35, .30, .24} {
		v.Player.Y = y
		v.Player.X = float64(i%2) * .025
		v.GuidanceValidUntilMs = 20000
		planBrontasStationGuide(v, 1100+int64(i)*25)
		if v.rhythm.To != 4 || v.StepAxis != "vertical" {
			t.Fatal("reselected before arrival", i, v.rhythm)
		}
	}
	v.Player.Y = .18
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1300)
	if v.rhythm.To != 3 || v.rhythm.Next != 0 || v.StepAxis != "horizontal" {
		t.Fatal("expected 5 -> 4 -> 7", v.rhythm)
	}
	v.Player.X = -.60
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1700)
	if v.rhythm.To != 0 || v.rhythm.Next != 1 || v.StepAxis != "vertical" {
		t.Fatal("expected 4 -> 7 -> 8", v.rhythm)
	}
}

func TestBrontasRhythmEdgesAndAllStarts(t *testing.T) {
	for cell := 0; cell < 9; cell++ {
		for _, axis := range []string{"horizontal", "vertical"} {
			s := brontasRhythmPlan{Clock: 12}
			s = brontasRhythmContinue(s, cell, axis, 1000)
			for step := 0; step < 30; step++ {
				a, b, c := brontasCellCenter(s.From), brontasCellCenter(s.To), brontasCellCenter(s.Next)
				if brontasStepAxis(a, b) != s.Axis || brontasStepAxis(b, c) != brontasOtherAxis(s.Axis) {
					t.Fatal("skip, diagonal or non-alternating grid path", s)
				}
				s = brontasRhythmContinue(s, s.To, brontasOtherAxis(s.Axis), int64(1100+step*100))
			}
		}
	}
}

func TestBrontasRhythmAdaptsWhenPlayerChoosesOtherSide(t *testing.T) {
	v := rhythmFixture()
	planBrontasStationGuide(v, 1000)
	v.Player.Y = 0
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1400)
	v.Player.X = .6
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1900)
	if v.rhythm.From != 5 || v.rhythm.To != 2 || v.StepAxis != "vertical" {
		t.Fatal("did not follow 5 -> 6 -> 9", v.rhythm)
	}
}

func TestBrontasRhythmNewDangerCannotReverseCommittedStep(t *testing.T) {
	v := rhythmFixture()
	planBrontasStationGuide(v, 1000)
	to := v.rhythm.To
	v.Player.Y = .5
	zero, width := 0.0, 1.0/3
	v.Rays = append(v.Rays, brontasGuideRay{ID: "new-danger", Axis: "horizontal", Offset: &zero, HalfWidth: &width, LethalAtMs: 1100, EndsAtMs: 9000})
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1200)
	if v.Target != nil || v.NextTarget != nil || v.rhythm.To != to {
		t.Fatal("new active ray produced an unverified/reverse route", v)
	}
	v.Player.Speed = 0
	planBrontasStationGuide(v, 1300)
	if v.Target != nil {
		t.Fatal("stale speed produced route")
	}
}

func TestBrontasRhythmFacingAndTiming(t *testing.T) {
	for _, facing := range []string{"outward", "inward", "north", "south"} {
		v := rhythmFixture()
		v.Clock = 9
		v.Rays[0].LethalAtMs = 1700
		planBrontasStationGuide(v, 1000)
		worldTarget := *v.Target
		orientBrontasStation(v, facing)
		if brontasStepAxis(v.Player.brontasPoint, *v.Target) != v.StepAxis || math.Hypot(v.Target.X-v.Player.X, v.Target.Y-v.Player.Y) > brontasCellWidth+1e-7 {
			t.Fatal("rotated diagonal")
		}
		if *v.Target != rotateBrontasPoint(worldTarget, brontasFacingRotation(9, facing)) {
			t.Fatal("rotation changed route")
		}
	}
	// Irregular accelerating packet intervals do not act as an automatic move
	// timer. Only actual cell progress advances the next step.
	v := rhythmFixture()
	planBrontasStationGuide(v, 1000)
	for _, at := range []int64{1500, 1900, 2200, 2400, 2500} {
		v.GuidanceValidUntilMs = 20000
		planBrontasStationGuide(v, at)
		if v.rhythm.To != 4 {
			t.Fatal("timer skipped a step")
		}
	}
}

func TestBrontasRhythmPreviewKeepsCurrentDecisionWindow(t *testing.T) {
	v := rhythmFixture()
	planBrontasStationGuide(v, 1000)
	// Isolate second-leg source freshness from the new bait-pause state.
	v.rhythm.Released = true
	v.GuidanceValidUntilMs = 1350
	planBrontasStationGuide(v, 1000)
	if v.Target == nil || v.NextTarget == nil || v.GuidanceValidUntilMs != 1350 {
		t.Fatalf("first-leg travel incorrectly consumed snapshot freshness: %+v", v)
	}
	v = brontasStationPreview(100000, 102000, 9)
	if v.Guidance != "pace" || v.Target != nil {
		t.Fatal("preview skipped bait pause", v)
	}
	v = brontasStationPreview(100000, 103000, 9)
	if v.Target == nil || v.GuidanceValidUntilMs <= 103000 {
		t.Fatal("preview did not release after pause", v)
	}
	v = brontasStationPreview(100000, 103500, 9)
	if v.GuidanceValidUntilMs > 103850 {
		t.Fatal("expired source window was extended")
	}
}
