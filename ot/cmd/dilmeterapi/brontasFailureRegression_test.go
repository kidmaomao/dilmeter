package main

import "testing"

// Normalized geometry and relative times from the Oct 31 return-to-center
// failure. No private identities or raw captures are embedded in this fixture.
func brontasReturnToCenterFixture() *nativeBrontasGuideView {
	zero, edge, bottom, width := 0.0, 2.0/3, -2.0/3, 1.0/3
	s := brontasRhythmContinue(brontasRhythmPlan{Clock: 12, DX: -1, DY: 1}, 1, "vertical", 3263)
	return &nativeBrontasGuideView{Source: "packet", Stage: "station", Clock: 12, ValidUntilMs: 6000, PlayerValidUntilMs: 6000, GuidanceValidUntilMs: 6000, rhythm: &s,
		Player: &brontasGuidePlayer{brontasPoint: brontasPoint{.174, -.461}, Speed: .733, Radius: .055},
		Rays: []brontasGuideRay{
			{ID: "right", Axis: "vertical", Offset: &edge, HalfWidth: &width, WarningAtMs: 2011, LethalAtMs: 3611, EndsAtMs: 4211},
			{ID: "old-center", Axis: "horizontal", Offset: &zero, HalfWidth: &width, WarningAtMs: 2664, LethalAtMs: 4264, EndsAtMs: 4864},
			{ID: "current-bottom", Axis: "horizontal", Offset: &bottom, HalfWidth: &width, WarningAtMs: 3165, LethalAtMs: 4765, EndsAtMs: 5365},
		}}
}

func TestBrontasBlockedReturnDoesNotOfferBaitPause(t *testing.T) {
	v := brontasReturnToCenterFixture()
	planBrontasStationGuide(v, 3263)
	if v.Guidance != "blocked" || v.Target != nil || v.PaceUntilMs != 0 {
		t.Fatal("blocked center crossing disguised as a timed pause", v)
	}
}

func TestBrontasEmergencyCannotEnterAnUpcomingHit(t *testing.T) {
	v := brontasReturnToCenterFixture()
	v.Player.X, v.Player.Y = .035, -.478
	v.Motion = &brontasGuideMotion{From: v.Player.brontasPoint, To: brontasPoint{.035, -.228}, AtMs: 4002, UntilMs: 4702, Speed: .977}
	target, next := brontasRhythmTargets(v, *v.rhythm)
	if _, ok := brontasPathExposure(v, []brontasPoint{target, next}, 4002, true); ok {
		t.Fatal("new center line starts after now but before leaving: unsafe return accepted")
	}
	planBrontasStationGuide(v, 4002)
	if v.Target != nil {
		t.Fatal("urgent arrow sent player into upcoming center hit", v)
	}
	// Crossing a warning is permitted if the entire leg leaves it before the
	// hit interval. The restriction is timed contact, not all warning colors.
	v = stationGuideFixture()
	v.Player.brontasPoint = brontasPoint{-.6, 0}
	v.Player.Speed = 2
	v.Rays = v.Rays[:1]
	v.Rays[0].Axis = "vertical"
	v.Rays[0].LethalAtMs = 5000
	if _, ok := brontasPathExposure(v, []brontasPoint{{0, 0}}, 1000, true); !ok {
		t.Fatal("harmless future warning rejected")
	}
}

func TestBrontasUnissuedCornerCanChooseVerifiedMirror(t *testing.T) {
	zero, left, width := 0.0, -2.0/3, 1.0/3
	s := brontasRhythmContinue(brontasRhythmPlan{Clock: 12, DX: -1, DY: 1}, 4, "horizontal", 1000)
	v := &nativeBrontasGuideView{Source: "packet", Stage: "station", Clock: 12, ValidUntilMs: 8000, PlayerValidUntilMs: 8000, GuidanceValidUntilMs: 8000, rhythm: &s,
		Player: &brontasGuidePlayer{brontasPoint: brontasPoint{}, Speed: 2, Radius: .055},
		Rays: []brontasGuideRay{{ID: "foot", Axis: "vertical", Offset: &zero, HalfWidth: &width, WarningAtMs: 1000, LethalAtMs: 2600, EndsAtMs: 3200},
			{ID: "blocked-left", Axis: "vertical", Offset: &left, HalfWidth: &width, WarningAtMs: 100, LethalAtMs: 900, EndsAtMs: 6000}}}
	planBrontasStationGuide(v, 1000)
	if v.rhythm.To != 5 || v.rhythm.Axis != "horizontal" {
		t.Fatal("did not choose clear right-hand L before issuing arrow", v.rhythm)
	}
	v.GuidanceValidUntilMs = 8000
	planBrontasStationGuide(v, 1900)
	if v.Target == nil || !v.rhythm.Released {
		t.Fatal("verified mirror did not release", v)
	}
	right := 2.0 / 3
	v.Rays[1].Offset = &right
	v.GuidanceValidUntilMs = 8000
	planBrontasStationGuide(v, 1925)
	if v.rhythm.To != 5 {
		t.Fatal("new hazard reversed an already issued step", v.rhythm)
	}
}
