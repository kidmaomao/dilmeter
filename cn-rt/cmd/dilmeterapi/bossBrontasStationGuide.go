package main

import "math"

const brontasGuidePositionFreshMs int64 = brontasMovingPositionFreshMs

func normalizeBrontasFacing(value string) string {
	switch value {
	case "inward", "north", "south":
		return value
	default:
		return "outward"
	}
}

func brontasFacingRotation(clock int, facing string) int {
	switch normalizeBrontasFacing(facing) {
	case "north":
		return 0
	case "south":
		return 180
	case "inward":
		return (540 - clock*30) % 360
	default:
		return (360 - clock*30) % 360
	}
}

func orientBrontasStation(v *nativeBrontasGuideView, facing string) {
	clock := v.Clock
	if validBrontasClock(clock) {
		rotateBrontasGuide(v, brontasFacingRotation(clock, facing))
	}
	v.Clock = clock // Keep the actual mineral name even in a player-facing view.
	v.Facing = normalizeBrontasFacing(facing)
}

// A measured speed estimate from consecutive confirmed movement
// positions. No default race speed, teleport, destination or preview speed is
// used as a real character measurement.
func brontasPositionSpeed(previous, next nativeBrontasPosition) (float64, int, int64) {
	dt := next.AtMs - previous.AtMs
	if previous.RegionID != next.RegionID || previous.Mode != next.Mode || dt < 0 {
		return 0, 0, 0
	}
	retain := func() (float64, int, int64) {
		if next.AtMs-previous.SpeedAtMs < 3000 {
			return previous.Speed, previous.SpeedSamples, previous.SpeedAtMs
		}
		return 0, 0, 0
	}
	if !previous.Moving || !next.Moving || dt < 80 || dt > 700 {
		return retain()
	}
	dx, dy := next.X-previous.X, next.Y-previous.Y
	distance := math.Hypot(dx, dy)
	speed := distance * 1000 / float64(dt)
	if speed > 3000 {
		return 0, 0, 0
	} // discontinuity/teleport
	if distance < 40 || speed < 100 {
		return retain()
	}
	if previous.Mode != 0 {
		mx, my := previous.DestX-previous.X, previous.DestY-previous.Y
		travel := math.Hypot(mx, my)
		// An endpoint sample includes unknown standing time. Do not mistake
		// a pause at a destination for a reduction in running speed.
		if travel <= distance+35 || travel == 0 || (dx*mx+dy*my)/(distance*travel) < .97 {
			return retain()
		}
	}
	count := previous.SpeedSamples + 1
	if previous.SpeedSamples > 0 {
		if speed < previous.Speed*.70 || speed > previous.Speed*1.43 {
			count = 1
		} else {
			speed = previous.Speed*.65 + speed*.35
		}
	}
	return speed, min(count, 5), next.AtMs
}

// Check the entire straight route plus the destination. Unknown ray endings
// remain dangerous indefinitely; a zero countdown is never treated as clearance.
func brontasGridRoute(v *nativeBrontasGuideView, target brontasPoint, nowMs int64) (bool, int64) {
	return brontasGridLeg(v, target, nowMs, false)
}

func brontasGridReaction(v *nativeBrontasGuideView, target brontasPoint, nowMs int64) int64 {
	p := v.Player
	reaction := brontasGuideReactionMs
	if m := v.Motion; m != nil && nowMs < m.UntilMs {
		mx, my := m.To.X-p.X, m.To.Y-p.Y
		tx, ty := target.X-p.X, target.Y-p.Y
		lengths := math.Hypot(mx, my) * math.Hypot(tx, ty)
		if lengths > 1e-8 && (mx*tx+my*ty)/lengths > .95 {
			reaction = 0
		}
	}
	return reaction
}

func brontasGridLeg(v *nativeBrontasGuideView, target brontasPoint, nowMs int64, intermediate bool) (bool, int64) {
	p := v.Player
	if p == nil || p.Speed <= 0 || math.Abs(target.X) > .90 || math.Abs(target.Y) > .90 {
		return false, 0
	}
	distance := math.Hypot(target.X-p.X, target.Y-p.Y)
	if distance > 1e-7 && brontasStepAxis(p.brontasPoint, target) == "" {
		return false, 0
	}
	travel := distance / p.Speed * 1000
	reaction := brontasGridReaction(v, target, nowMs)
	depart := float64(nowMs + reaction)
	arrive := depart + travel
	until := min(v.ValidUntilMs, v.GuidanceValidUntilMs)
	// A generous planning margin is distinct from the decorative player dot.
	const margin = .12
	for _, ray := range v.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil {
			return false, 0
		}
		finish := math.Inf(1)
		if ray.EndsAtMs > 0 {
			finish = float64(ray.EndsAtMs + brontasGuideGuardMs)
		}
		if finish <= float64(nowMs) {
			continue
		}
		start := float64(ray.LethalAtMs - brontasGuideGuardMs)
		a, b := p.X, target.X
		if ray.Axis == "horizontal" {
			a, b = p.Y, target.Y
		}
		low, high := *ray.Offset-*ray.HalfWidth-margin, *ray.Offset+*ray.HalfWidth+margin
		inside := func(c float64) bool { return c >= low && c <= high }
		if inside(a) && start <= depart && finish >= float64(nowMs) {
			return false, 0
		}
		if inside(b) && finish >= arrive {
			if !intermediate || start <= arrive+float64(brontasGuideReactionMs) {
				return false, 0
			}
			until = min(until, int64(start-travel-float64(reaction+brontasGuideReactionMs)))
		}
		if math.Abs(b-a) < 1e-9 {
			continue
		}
		u, w := (low-a)/(b-a), (high-a)/(b-a)
		if u > w {
			u, w = w, u
		}
		u, w = math.Max(0, u), math.Min(1, w)
		if u > w {
			continue
		}
		enter, leave := depart+u*travel, depart+w*travel
		if enter <= finish && leave >= start {
			return false, 0
		}
		if leave < start {
			until = min(until, int64(math.Floor(start-float64(reaction)-w*travel)))
		}
	}
	return until > nowMs, until
}

func planBrontasStationGuide(v *nativeBrontasGuideView, nowMs int64) {
	planBrontasRhythmGuide(v, nowMs)
}

// Retained as a comparison planner for recorded regression checks.
func planBrontasFreeStationGuide(v *nativeBrontasGuideView, nowMs int64) {
	previousTarget := v.Target
	if previousTarget != nil && v.NextTarget != nil && v.Player != nil && math.Hypot(previousTarget.X-v.Player.X, previousTarget.Y-v.Player.Y) < .08 {
		previousTarget = v.NextTarget
	}
	v.Target = nil
	v.NextTarget = nil
	v.StepAxis = ""
	defer func() {
		if v.Target != nil && v.Player != nil {
			v.StepAxis = brontasStepAxis(v.Player.brontasPoint, *v.Target)
		}
	}()
	v.Guidance = "waiting"
	v.GuidanceKind = ""
	v.GuidanceReason = "position-or-speed"
	if len(v.Rays) == 0 || v.Stage != "station" || v.Player == nil || nowMs >= v.ValidUntilMs || nowMs >= v.GuidanceValidUntilMs || v.Player.Speed <= 0 {
		return
	}
	// Only classify holding as clear after checking every known ray.
	if clear, until := brontasGridRoute(v, v.Player.brontasPoint, nowMs); clear {
		v.Guidance = "hold"
		v.GuidanceReason = "clear"
		v.GuidanceValidUntilMs = until
		return
	}
	// Prefer short perpendicular exits just beyond a channel edge. A temporary
	// cell is permitted only when a second complete route is also checked.
	candidates := brontasStepCandidates(v.Player.brontasPoint, previousTarget)
	prioritizeBrontasSteps(v, candidates, nowMs)
	best, bestCost, bestUntil := brontasPoint{}, math.Inf(1), int64(0)
	var bestNext *brontasPoint
	for pass := 0; pass < 2; pass++ {
		for _, target := range candidates {
			cost := math.Hypot(target.X-v.Player.X, target.Y-v.Player.Y)
			if cost < .06 || cost >= bestCost {
				continue
			}
			clear, until := brontasGridRoute(v, target, nowMs)
			var nextTarget *brontasPoint
			if !clear && pass == 1 {
				clear, until = brontasGridLeg(v, target, nowMs, true)
				if clear {
					next := *v
					p := *v.Player
					p.brontasPoint = target
					next.Player = &p
					next.Motion = nil
					arrive := nowMs + brontasGridReaction(v, target, nowMs) + int64(math.Ceil(cost/p.Speed*1000))
					next.GuidanceValidUntilMs = next.ValidUntilMs
					clear = false
					for _, exit := range brontasStepCandidates(target, nil) {
						if brontasStepAxis(target, exit) == brontasStepAxis(v.Player.brontasPoint, target) {
							continue
						}
						if ok, exitUntil := brontasGridLeg(&next, exit, arrive, true); ok && exitUntil-(arrive-nowMs) > nowMs {
							clear = true
							point := exit
							nextTarget = &point
							until = min(until, exitUntil-(arrive-nowMs))
							break
						}
					}
				}
			}
			if clear {
				best, bestCost, bestUntil = target, cost, until
				bestNext = nextTarget
			}
		}
		if !math.IsInf(bestCost, 1) {
			break
		}
	}
	if math.IsInf(bestCost, 1) {
		v.Guidance = "blocked"
		v.GuidanceReason = "no-route"
		planBrontasEmergencyExit(v, candidates, nowMs)
		planBrontasLeastExposure(v, candidates, nowMs)
		return
	}
	v.Target = &best
	v.NextTarget = bestNext
	v.Guidance = "move"
	v.GuidanceKind = "route"
	v.GuidanceReason = "checked-steps"
	v.GuidanceValidUntilMs = bestUntil
}
