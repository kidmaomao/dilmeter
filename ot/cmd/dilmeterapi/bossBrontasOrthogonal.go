package main

import (
	"math"
	"sort"
)

const brontasCellWidth = 2.0 / 3

func brontasGridCell(c float64) int { return int(math.Floor((c+1)/brontasCellWidth + 1e-7)) }

// A step changes one coordinate only, spans at most one grid width, and cannot
// skip a cell. Same-cell continuation is allowed while approaching a waypoint.
func brontasStepAxis(from, to brontasPoint) string {
	dx, dy := math.Abs(to.X-from.X), math.Abs(to.Y-from.Y)
	if dx > 1e-7 && dy > 1e-7 || math.Max(dx, dy) > brontasCellWidth+1e-7 || math.Abs(to.X) > .9 || math.Abs(to.Y) > .9 {
		return ""
	}
	if math.Abs(float64(brontasGridCell(from.X)-brontasGridCell(to.X)))+math.Abs(float64(brontasGridCell(from.Y)-brontasGridCell(to.Y))) > 1 {
		return ""
	}
	if dx > 1e-7 {
		return "horizontal"
	}
	if dy > 1e-7 {
		return "vertical"
	}
	return ""
}

func brontasStepCandidates(p brontasPoint, previous *brontasPoint) []brontasPoint {
	result := []brontasPoint{}
	add := func(q brontasPoint, continuation bool) {
		if brontasStepAxis(p, q) == "" || math.Hypot(q.X-p.X, q.Y-p.Y) < .06 {
			return
		}
		if !continuation && brontasGridCell(p.X) == brontasGridCell(q.X) && brontasGridCell(p.Y) == brontasGridCell(q.Y) {
			return
		}
		for _, old := range result {
			if math.Hypot(old.X-q.X, old.Y-q.Y) < 1e-7 {
				return
			}
		}
		result = append(result, q)
	}
	if previous != nil {
		add(*previous, true)
	}
	for _, c := range []float64{-.67, -.48, -.18, 0, .18, .48, .67} {
		add(brontasPoint{c, p.Y}, false)
		add(brontasPoint{p.X, c}, false)
	}
	for _, d := range []float64{-brontasCellWidth, brontasCellWidth} {
		add(brontasPoint{math.Max(-.85, math.Min(.85, p.X+d)), p.Y}, false)
		add(brontasPoint{p.X, math.Max(-.85, math.Min(.85, p.Y+d))}, false)
	}
	return result
}

// The earliest ray underfoot decides which arm of a cross to leave first.
func prioritizeBrontasSteps(v *nativeBrontasGuideView, candidates []brontasPoint, nowMs int64) {
	first, axis := int64(math.MaxInt64), ""
	for _, ray := range v.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil || ray.EndsAtMs > 0 && ray.EndsAtMs+brontasGuideGuardMs <= nowMs {
			continue
		}
		c, move := v.Player.X, "horizontal"
		if ray.Axis == "horizontal" {
			c, move = v.Player.Y, "vertical"
		}
		if c >= *ray.Offset-*ray.HalfWidth && c < *ray.Offset+*ray.HalfWidth && ray.LethalAtMs < first {
			first, axis = ray.LethalAtMs, move
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return brontasStepAxis(v.Player.brontasPoint, candidates[i]) == axis && brontasStepAxis(v.Player.brontasPoint, candidates[j]) != axis
	})
}

// Keep an already checked axis aligned during between-packet animation. A
// substantial sideways departure invalidates it instead of drawing a diagonal.
func brontasDisplayedStep(v *nativeBrontasGuideView) (*brontasPoint, *brontasPoint) {
	if v.Player == nil || v.Target == nil {
		return nil, nil
	}
	target := *v.Target
	switch v.StepAxis {
	case "horizontal":
		if math.Abs(target.Y-v.Player.Y) > .08 {
			return nil, nil
		}
		target.Y = v.Player.Y
	case "vertical":
		if math.Abs(target.X-v.Player.X) > .08 {
			return nil, nil
		}
		target.X = v.Player.X
	default:
		return v.Target, nil
	}
	if math.Hypot(target.X-v.Player.X, target.Y-v.Player.Y) > brontasCellWidth+1e-7 {
		return nil, nil
	}
	if v.NextTarget == nil {
		return &target, nil
	}
	next := *v.NextTarget
	if v.StepAxis == "horizontal" {
		next.X = target.X
	} else {
		next.Y = target.Y
	}
	if axis := brontasStepAxis(target, next); axis == "" || axis == v.StepAxis {
		return &target, nil
	}
	return &target, &next
}
