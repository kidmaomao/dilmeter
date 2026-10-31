package main

import "math"

// Persist cell destinations, not a per-frame nearest point. X/Y directions
// bounce at the grid edge; axes alternate at each confirmed corner. This
// produces the continuous 2->5->4->7->8->5... (or mirrored) walking pattern.
type brontasRhythmPlan struct {
	Clock, From, To, Next int
	DX, DY                int
	Axis                  string
	Ready                 bool
	ChangedAtMs           int64
	Origin                brontasPoint
	OriginSet, Released   bool
	ReleaseAtMs           int64
}

func brontasCell(p brontasPoint) int { return brontasGridCell(p.Y)*3 + brontasGridCell(p.X) }
func brontasCellCenter(cell int) brontasPoint {
	return brontasPoint{float64(cell%3-1) * brontasCellWidth, float64(cell/3-1) * brontasCellWidth}
}
func brontasOtherAxis(axis string) string {
	if axis == "horizontal" {
		return "vertical"
	}
	return "horizontal"
}
func brontasCellNeighbor(cell int, axis string, direction int) (int, int) {
	c := cell % 3
	if axis == "vertical" {
		c = cell / 3
	}
	if direction == 0 {
		if c == 0 {
			direction = 1
		} else {
			direction = -1
		}
	}
	if c+direction < 0 || c+direction > 2 {
		direction = -direction
	}
	if axis == "horizontal" {
		return cell + direction, direction
	}
	return cell + 3*direction, direction
}

func brontasRhythmTargets(v *nativeBrontasGuideView, s brontasRhythmPlan) (brontasPoint, brontasPoint) {
	target := v.Player.brontasPoint
	center := brontasCellCenter(s.To)
	clamp := func(a, b float64) float64 { return math.Max(a-brontasCellWidth, math.Min(a+brontasCellWidth, b)) }
	if s.Axis == "horizontal" {
		target.X = clamp(target.X, center.X)
	} else {
		target.Y = clamp(target.Y, center.Y)
	}
	next := target
	center = brontasCellCenter(s.Next)
	if s.Axis == "horizontal" {
		next.Y = clamp(next.Y, center.Y)
	} else {
		next.X = clamp(next.X, center.X)
	}
	return target, next
}

func brontasRhythmAxis(v *nativeBrontasGuideView, now int64) string {
	at := int64(math.MaxInt64)
	axis := ""
	for _, r := range v.Rays {
		if r.Offset == nil || r.HalfWidth == nil || r.EndsAtMs > 0 && r.EndsAtMs+brontasGuideGuardMs <= now {
			continue
		}
		c, move := v.Player.X, "horizontal"
		if r.Axis == "horizontal" {
			c, move = v.Player.Y, "vertical"
		}
		if c >= *r.Offset-*r.HalfWidth && c < *r.Offset+*r.HalfWidth && r.LethalAtMs < at {
			at, axis = r.LethalAtMs, move
		}
	}
	return axis
}

func brontasRhythmContinue(s brontasRhythmPlan, from int, axis string, now int64) brontasRhythmPlan {
	s.From, s.Axis, s.Ready, s.ChangedAtMs = from, axis, true, now
	s.OriginSet, s.Released, s.ReleaseAtMs = false, false, 0
	dir := s.DX
	if axis == "vertical" {
		dir = s.DY
	}
	s.To, dir = brontasCellNeighbor(from, axis, dir)
	if axis == "horizontal" {
		s.DX = dir
		s.Next, s.DY = brontasCellNeighbor(s.To, "vertical", s.DY)
	} else {
		s.DY = dir
		s.Next, s.DX = brontasCellNeighbor(s.To, "horizontal", s.DX)
	}
	return s
}

// Assess both complete legs at their respective arrival times. The orange
// fallback is explicitly risk reduction; it never asserts a collision-free run.
func brontasRhythmAssessment(v *nativeBrontasGuideView, s brontasRhythmPlan, now int64) (string, int64, float64, bool) {
	target, next := brontasRhythmTargets(v, s)
	if brontasStepAxis(v.Player.brontasPoint, target) == "" || brontasStepAxis(target, next) == "" {
		return "", 0, math.Inf(1), false
	}
	if clear, until := brontasGridLeg(v, target, now, false); clear {
		// A visible continuation is still checked independently below.
		_, nextOK := brontasRhythmNext(v, target, next, now)
		return "route", until, 0, nextOK
	}
	if clear, until := brontasGridLeg(v, target, now, true); clear {
		if secondUntil, ok := brontasRhythmNext(v, target, next, now); ok {
			return "route", min(until, secondUntil), 0, true
		}
	}
	base, _ := brontasExposure(v, v.Player.brontasPoint, now, false)
	cost, ok := brontasPathExposure(v, []brontasPoint{target, next}, now, true)
	if ok && base > 0 && cost < base {
		return "escape", min(v.GuidanceValidUntilMs, now+150), cost, true
	}
	cost, ok = brontasExposure(v, target, now, true)
	if ok && base > 0 && cost < base {
		return "escape", min(v.GuidanceValidUntilMs, now+150), cost, false
	}
	return "", 0, math.Inf(1), false
}

func brontasRhythmNext(v *nativeBrontasGuideView, target, next brontasPoint, now int64) (int64, bool) {
	copy := *v
	p := *v.Player
	p.brontasPoint = target
	copy.Player = &p
	copy.Motion = nil
	// Snapshot freshness gates the current decision, not hypothetical arrival
	// at the second leg. Subtracting first-leg travel from that freshness limit
	// made an otherwise valid arrow expire after only a few milliseconds.
	copy.ValidUntilMs = math.MaxInt64
	copy.GuidanceValidUntilMs = copy.ValidUntilMs
	travel := int64(math.Ceil(math.Hypot(target.X-v.Player.X, target.Y-v.Player.Y)/p.Speed*1000)) + brontasGridReaction(v, target, now)
	ok, until := brontasGridLeg(&copy, next, now+travel, true)
	until = min(v.ValidUntilMs, v.GuidanceValidUntilMs, until-travel)
	return until, ok && until > now
}

func planBrontasRhythmGuide(v *nativeBrontasGuideView, now int64) {
	v.Target, v.NextTarget = nil, nil
	v.StepAxis = ""
	v.Guidance = "waiting"
	v.GuidanceKind = ""
	v.PaceUntilMs = 0
	v.GuidanceReason = "position-or-speed"
	if v.Player == nil || v.Player.Speed <= 0 || v.Stage != "station" || now >= v.ValidUntilMs || now >= v.GuidanceValidUntilMs {
		return
	}
	if v.rhythm == nil {
		v.rhythm = &brontasRhythmPlan{Clock: v.Clock}
	}
	s := v.rhythm
	cell := brontasCell(v.Player.brontasPoint)
	if cell < 0 || cell > 8 {
		return
	}
	if s.Clock != v.Clock {
		*s = brontasRhythmPlan{Clock: v.Clock}
	}
	reason := "continue-cell"
	if s.Ready {
		if cell == s.To {
			// Wait until safely inside the new row/column; boundary noise must not
			// promote a corner and then immediately send the player back.
			center := brontasCellCenter(s.To)
			distance := math.Abs(v.Player.X - center.X)
			if s.Axis == "vertical" {
				distance = math.Abs(v.Player.Y - center.Y)
			}
			if distance <= brontasCellWidth/2-.14 {
				*s = brontasRhythmContinue(*s, s.To, brontasOtherAxis(s.Axis), now)
				reason = "next-cell"
			}
		} else if cell != s.From {
			if now-s.ChangedAtMs < 180 {
				v.GuidanceReason = "confirm-cell"
				return
			}
			// Following a different neighbor is a new starting cell, not an order
			// to return to the old waypoint. Keep alternating from the observed move.
			axis := brontasOtherAxis(s.Axis)
			dx, dy := cell%3-s.From%3, cell/3-s.From/3
			if dx != 0 && dy == 0 {
				axis = "vertical"
				s.DX = 1
				if dx < 0 {
					s.DX = -1
				}
			}
			if dy != 0 && dx == 0 {
				axis = "horizontal"
				s.DY = 1
				if dy < 0 {
					s.DY = -1
				}
			}
			*s = brontasRhythmContinue(*s, cell, axis, now)
			reason = "follow-player-cell"
		}
	}
	if !s.Ready {
		axis := brontasRhythmAxis(v, now)
		if axis == "" {
			v.Guidance = "hold"
			v.GuidanceReason = "clear"
			return
		}
		initial := brontasRhythmContinue(*s, cell, axis, now)
		// Decide between mirrored paths once at entry. Do not repeat this nearest-
		// risk comparison on each position packet once a direction is selected.
		best := initial
		bestCost := math.Inf(1)
		for _, dx := range []int{-1, 1} {
			for _, dy := range []int{-1, 1} {
				candidate := initial
				candidate.DX, candidate.DY = dx, dy
				candidate = brontasRhythmContinue(candidate, cell, axis, now)
				kind, _, cost, _ := brontasRhythmAssessment(v, candidate, now)
				if kind != "" && cost < bestCost {
					best, bestCost = candidate, cost
				}
			}
		}
		*s = best
		reason = "start-cell"
	}
	if brontasRhythmSafeTurn(v, s, now) {
		reason = "verified-mirrored-turn"
	}
	v.StepAxis = s.Axis
	if brontasRhythmPace(v, s, now) {
		return
	}
	kind, until, _, showNext := brontasRhythmAssessment(v, *s, now)
	if kind == "" {
		// Preserve the chosen turn while it is temporarily blocked. A new live
		// danger may suppress advice; it cannot trigger arbitrary reverse arrows.
		v.Guidance = "blocked"
		v.GuidanceReason = "cell-route-blocked"
		if clear, until := brontasGridRoute(v, v.Player.brontasPoint, now); clear {
			v.Guidance = "hold"
			v.GuidanceReason = "clear"
			v.GuidanceValidUntilMs = until
		}
		return
	}
	target, next := brontasRhythmTargets(v, *s)
	v.Target = &target
	if showNext {
		v.NextTarget = &next
	}
	v.StepAxis = s.Axis
	v.Guidance = "move"
	v.GuidanceKind = kind
	v.GuidanceReason = reason
	v.GuidanceValidUntilMs = until
	s.Released = true
}
