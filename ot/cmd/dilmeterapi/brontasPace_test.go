package main

import "testing"

func paceFixture() *nativeBrontasGuideView {
	v := rhythmFixture()
	v.Player.Speed = 1
	v.Rays[0].LethalAtMs, v.Rays[0].EndsAtMs = 2600, 3200
	return v
}

func TestBrontasPaceBaitsBeforeDepartureAndPreservesRoute(t *testing.T) {
	v := paceFixture()
	planBrontasStationGuide(v, 1000)
	if v.Guidance != "pace" || v.Target != nil || v.PaceUntilMs != 1483 || v.rhythm.To != 4 {
		t.Fatalf("early ray should allow bait pause: %+v", v)
	}
	// Opposite duplicate, faster speed estimate and UI refresh cannot push a
	// committed cue later or silently release it in the renderer.
	v.Rays = append(v.Rays, v.Rays[0])
	v.Rays[1].ID, v.Rays[1].LethalAtMs = "opposite", 3100
	v.Player.Speed = 1.1
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1200)
	if v.PaceUntilMs != 1483 || v.Target != nil {
		t.Fatal("countdown postponed", v)
	}
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1483)
	if v.Guidance != "move" || v.Target == nil || v.rhythm.To != 4 {
		t.Fatal("missed departure", v)
	}
	// Clear target cell is not an immediate order to run into the next one.
	v.Player.Y = .18 // Confirmed corner, not necessarily the exact cell center.
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 2100)
	if v.Guidance != "pace" || v.PaceUntilMs != 0 || v.Target != nil || v.rhythm.To != 3 || v.rhythm.Next != 0 {
		t.Fatal("arrival should queue 5 -> 4 -> 7 and wait for its cue", v)
	}
	zero, width := 0.0, 1.0/3
	v.Rays = append(v.Rays, brontasGuideRay{ID: "next-cell", Axis: "vertical", Offset: &zero, HalfWidth: &width, WarningAtMs: 2200, LethalAtMs: 3800, EndsAtMs: 4400})
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 2200)
	if v.Guidance != "pace" || v.PaceUntilMs <= 2200 || v.rhythm.To != 3 {
		t.Fatal("new bait did not retain turn", v)
	}
}

func TestBrontasPaceAcceleratingWarningsAndUrgency(t *testing.T) {
	v := paceFixture()
	planBrontasStationGuide(v, 1000)
	old := v.PaceUntilMs
	// A local earlier deadline cancels the remaining wait immediately. Global
	// unrelated rays do not drive a fixed 100ms movement timer.
	remote, width := -2.0/3, 1.0/3
	v.Rays = append(v.Rays, brontasGuideRay{ID: "remote", Axis: "horizontal", Offset: &remote, HalfWidth: &width, WarningAtMs: 1000, LethalAtMs: 1100, EndsAtMs: 1700})
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1100)
	if v.PaceUntilMs != old {
		t.Fatal("remote ray changed beat", v)
	}
	v.Rays[0].LethalAtMs = 2200
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1200)
	if v.Guidance != "move" || v.Target == nil {
		t.Fatal("earlier danger kept waiting", v)
	}
	// Once moving, retain the step even if its originating warning disappears.
	v.Rays = v.Rays[1:]
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1250)
	if v.Guidance == "pace" {
		t.Fatal("released leg was stopped", v)
	}
	for _, lethal := range []int64{900, 1100} {
		v = paceFixture()
		v.Rays[0].LethalAtMs = lethal
		planBrontasStationGuide(v, 1000)
		if v.Guidance == "pace" {
			t.Fatal("active/imminent danger must never wait", lethal)
		}
	}
}

func TestBrontasPaceHonorsMovementAndFreshness(t *testing.T) {
	v := paceFixture()
	planBrontasStationGuide(v, 1000)
	v.Player.Y -= .1
	v.GuidanceValidUntilMs = 20000
	planBrontasStationGuide(v, 1100)
	if v.Guidance == "pace" {
		t.Fatal("player already moving must finish the leg")
	}
	v = paceFixture()
	v.Motion = &brontasGuideMotion{From: v.Player.brontasPoint, To: brontasPoint{}, AtMs: 1000, UntilMs: 1700, Speed: 1}
	planBrontasStationGuide(v, 1000)
	if v.Guidance == "pace" {
		t.Fatal("confirmed aligned movement was stopped")
	}
	v = paceFixture()
	planBrontasStationGuide(v, 1000)
	v.GuidanceValidUntilMs = 1400
	planBrontasStationGuide(v, 1500)
	if v.Target != nil || v.Guidance == "pace" {
		t.Fatal("stale position produced pacing advice")
	}
}

func TestBrontasPaceStoppedPositionOutlivesMovingProjection(t *testing.T) {
	r := brontasLiveFixture()
	// Exercise the real runtime freshness selection, using its fixture clock.
	for _, moving := range []bool{false, true} {
		r.brontasPosition = nativeBrontasPosition{AtMs: 100000, RegionID: 0, X: 25160, Y: 40967, Moving: moving, Speed: 1400, SpeedSamples: 2, SpeedAtMs: 100000}
		v := r.brontasLiveRayView(101000)
		if v == nil || v.Stage != "station" {
			t.Fatalf("fixture not located: %+v", v)
		}
		if moving && v.Target != nil {
			t.Fatal("moving projection was extended")
		}
		if !moving && v.GuidanceValidUntilMs <= 101000 {
			t.Fatal("confirmed stop expired before bait cue", v)
		}
	}
}

func TestBrontasPaceLateEntryUsesExistingWarningClock(t *testing.T) {
	r := brontasLiveFixture()
	r.brontasPosition.Y = 35000 // Outside the mineral when the real ray arrives.
	e := brontasLiveEvent(101000)
	e.Lane = 11
	r.observeBrontasRay(e)
	if v := r.brontasLiveRayView(101200); v == nil || v.Stage != "map" {
		t.Fatal("entry fixture was already in a station", v)
	}
	r.brontasPosition = nativeBrontasPosition{AtMs: 102000, X: 25160, Y: 40967, Speed: 1400, SpeedSamples: 2, SpeedAtMs: 102000}
	v := r.brontasLiveRayView(102000)
	if v == nil || v.Stage != "station" || len(v.Rays) != 1 {
		t.Fatal("late entry lost the existing ray", v)
	}
	if v.Rays[0].WarningAtMs != 101090 || v.Rays[0].LethalAtMs != 102690 {
		t.Fatal("entry restarted the real warning deadline", v.Rays)
	}
	if v.Guidance == "pace" || v.Target == nil {
		t.Fatal("late entry must not start a fresh early-phase pause", v)
	}
}
