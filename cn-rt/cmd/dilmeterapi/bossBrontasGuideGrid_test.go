package main

import (
	"fmt"
	"math"
	"testing"
)

func TestBrontasGridBandsTouchAndCoverCircle(t *testing.T) {
	g := brontasGuidePreviewGrid()
	for _, axis := range []string{"vertical", "horizontal"} {
		for lane := 0; lane < 2; lane++ {
			a, wa, err := g.rayGeometry(axis, lane)
			if err != nil {
				t.Fatal(err)
			}
			b, wb, err := g.rayGeometry(axis, lane+1)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(math.Abs(b-a)-wa-wb) > 1e-12 {
				t.Fatal("adjacent bands leave a gap", axis, lane)
			}
		}
		for i := -1000; i <= 1000; i++ {
			p := float64(i) / 1000
			covered := false
			for lane := 0; lane < 3; lane++ {
				offset, width, _ := g.rayGeometry(axis, lane)
				covered = covered || math.Abs(p-offset) <= width+1e-12
			}
			if !covered {
				t.Fatal("uncovered slit", axis, p)
			}
		}
		if _, _, err := g.rayGeometry(axis, 3); err == nil {
			t.Fatal("fourth lane accepted")
		}
	}
}

func TestBrontasGridPlannerRejectsFormerSlit(t *testing.T) {
	const now int64 = 1700000000000
	g := brontasGuidePreviewGrid()
	f := brontasTestFrame(now)
	f.Player.brontasPoint = brontasPoint{X: .5, Y: 0}
	// A tiny player would have fit the erroneous 0.4..0.6 slit. The corrected
	// geometry covers it even before expanding rays for the player's body.
	f.Player.Radius = .01
	f.Player.Speed = 4
	f.Rays = nil
	for lane := 1; lane <= 2; lane++ {
		offset, width, err := g.rayGeometry("vertical", lane)
		if err != nil {
			t.Fatal(err)
		}
		f.Rays = append(f.Rays, brontasGuideRay{ID: fmt.Sprint(lane), Axis: "vertical", WarningAtMs: now, LethalAtMs: now + 1600, EndsAtMs: now + 2500, Offset: &offset, HalfWidth: &width})
	}
	if brontasRouteClear(f, f.Player.brontasPoint, now) {
		t.Fatal("standing in the former slit was accepted")
	}
	v := planBrontasGuide(f, now)
	if v.Target == nil || v.Target.X >= -.4-.04 {
		t.Fatalf("must move to the uncovered left column: %+v", v)
	}
	for _, axis := range []string{"vertical", "horizontal"} {
		f.Rays = nil
		for lane := 0; lane < 3; lane++ {
			offset, width, _ := g.rayGeometry(axis, lane)
			f.Rays = append(f.Rays, brontasGuideRay{ID: fmt.Sprint(lane), Axis: axis, WarningAtMs: now - 1000, LethalAtMs: now - 500, EndsAtMs: now + 1000, Offset: &offset, HalfWidth: &width})
		}
		if v := planBrontasGuide(f, now); v.Target != nil || v.Status != "blocked" {
			t.Fatal("three active lanes must cover the whole circle", axis, v)
		}
	}
}

func TestBrontasGridLocksToCurrentPlayerLane(t *testing.T) {
	g := brontasGuideGrid{XHalfWidth: .5, YHalfWidth: .5}
	for _, tc := range []struct {
		p        brontasPoint
		col, row int
	}{
		{brontasPoint{X: -.7, Y: .7}, 0, 0}, {brontasPoint{X: 0, Y: 0}, 1, 1}, {brontasPoint{X: .7, Y: -.7}, 2, 2},
	} {
		col, err := g.laneAt("vertical", tc.p)
		if err != nil || col != tc.col {
			t.Fatalf("column lock: %+v %d %v", tc, col, err)
		}
		row, err := g.laneAt("horizontal", tc.p)
		if err != nil || row != tc.row {
			t.Fatalf("row lock: %+v %d %v", tc, row, err)
		}
	}
	p := brontasPoint{X: .7}
	lane, _ := g.laneAt("vertical", p)
	locked, _, _ := g.rayGeometry("vertical", lane)
	p.X = -.7
	nextLane, _ := g.laneAt("vertical", p)
	next, _, _ := g.rayGeometry("vertical", nextLane)
	if locked != 1 || next != -1 {
		t.Fatal("old ray moved with player, or next ray used old position")
	}
}

func TestBrontasDemoDetectsContactBetweenAnimationFrames(t *testing.T) {
	offset, width := 0.0, .1
	r := brontasGuideRay{Axis: "vertical", LethalAtMs: 40, EndsAtMs: 41, Offset: &offset, HalfWidth: &width}
	if !brontasDemoContact(brontasPoint{X: -1}, brontasPoint{X: 1}, 0, 80, .08, r) {
		t.Fatal("missed a lethal crossing between two rendered frames")
	}
	if brontasDemoContact(brontasPoint{X: -1, Y: .7}, brontasPoint{X: -.8, Y: .7}, 0, 80, .08, r) {
		t.Fatal("false collision")
	}
}
