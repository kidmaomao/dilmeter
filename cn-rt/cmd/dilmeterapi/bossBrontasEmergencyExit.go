package main

import (
	"math"
	"sort"
)

// A late player already in an active ray has no collision-free route from the
// present instant. Still show a distinct urgent exit, rather than silently
// hiding every direction. This is damage avoidance advice, not a safe-route
// assertion: do not enter another active ray or stop under an imminent one.
func planBrontasEmergencyExit(v *nativeBrontasGuideView, candidates []brontasPoint, nowMs int64) {
	p := v.Player
	if p == nil || p.Speed <= 0 {
		return
	}
	activeHere := false
	for _, ray := range v.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil {
			return
		}
		c := p.X
		if ray.Axis == "horizontal" {
			c = p.Y
		}
		if c >= *ray.Offset-*ray.HalfWidth && c < *ray.Offset+*ray.HalfWidth && ray.LethalAtMs-brontasGuideGuardMs <= nowMs && (ray.EndsAtMs == 0 || ray.EndsAtMs+brontasGuideGuardMs > nowMs) {
			activeHere = true
		}
	}
	if !activeHere {
		return
	}
	bestCost := math.Inf(1)
	var best brontasPoint
	var bestUntil int64
	for _, target := range candidates {
		distance := math.Hypot(target.X-p.X, target.Y-p.Y)
		if distance < .06 || brontasStepAxis(p.brontasPoint, target) == "" {
			continue
		}
		reaction := brontasGridReaction(v, target, nowMs)
		travel := distance / p.Speed * 1000
		depart := float64(nowMs + reaction)
		arrive := depart + travel
		until := min(v.GuidanceValidUntilMs, v.ValidUntilMs)
		allowed := true
		exposure := 0.0
		for _, ray := range v.Rays {
			a, b := p.X, target.X
			if ray.Axis == "horizontal" {
				a, b = p.Y, target.Y
			}
			low, high := *ray.Offset-*ray.HalfWidth-.12, *ray.Offset+*ray.HalfWidth+.12
			start, finish := float64(ray.LethalAtMs-brontasGuideGuardMs), math.Inf(1)
			if ray.EndsAtMs > 0 {
				finish = float64(ray.EndsAtMs + brontasGuideGuardMs)
			}
			if finish <= float64(nowMs) {
				continue
			}
			inside := func(x float64) bool { return x >= low && x <= high }
			if inside(b) && finish >= arrive && start <= arrive+float64(brontasGuideReactionMs) {
				allowed = false
				break
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
				// Only the unavoidable exposure at our initial position may be crossed.
				if !inside(a) || start > float64(nowMs) || inside(b) {
					allowed = false
					break
				}
				exposure = math.Max(exposure, leave-float64(nowMs))
			} else if leave < start {
				until = min(until, int64(math.Floor(start-float64(reaction)-w*travel)))
			}
		}
		cost := exposure + travel*.15
		if allowed && until > nowMs && cost < bestCost {
			best, bestCost, bestUntil = target, cost, until
		}
	}
	if !math.IsInf(bestCost, 1) {
		v.Target = &best
		v.Guidance = "move"
		v.GuidanceKind = "escape"
		v.GuidanceReason = "already-in-danger"
		v.GuidanceValidUntilMs = bestUntil
	}
}

// Last resort: compare unavoidable danger exposure over the next 2.4s. This
// never becomes a normal route. It may reduce a late/future hit, not promise to
// survive it. Refuse entering a new stripe during its dangerous interval,
// including a warning that activates while the player is crossing.
func planBrontasLeastExposure(v *nativeBrontasGuideView, candidates []brontasPoint, nowMs int64) {
	if v.Target != nil || v.Player == nil || v.Player.Speed <= 0 {
		return
	}
	baseline, ok := brontasExposure(v, v.Player.brontasPoint, nowMs, false)
	if !ok || baseline < 100 {
		return
	}
	bestCost := baseline * .85
	var best *brontasPoint
	var bestNext *brontasPoint
	for _, target := range candidates {
		if brontasStepAxis(v.Player.brontasPoint, target) == "" {
			continue
		}
		paths := [][]brontasPoint{{target}}
		for _, next := range brontasStepCandidates(target, nil) {
			if brontasStepAxis(target, next) != brontasStepAxis(v.Player.brontasPoint, target) {
				paths = append(paths, []brontasPoint{target, next})
			}
		}
		for _, path := range paths {
			cost, ok := brontasPathExposure(v, path, nowMs, true)
			if ok && cost < bestCost {
				copy := target
				best = &copy
				bestCost = cost
				bestNext = nil
				if len(path) > 1 {
					next := path[1]
					bestNext = &next
				}
			}
		}
	}
	if best != nil {
		v.Target = best
		v.NextTarget = bestNext
		v.Guidance = "move"
		v.GuidanceKind = "escape"
		v.GuidanceReason = "reduce-unavoidable-exposure"
		v.GuidanceValidUntilMs = min(v.GuidanceValidUntilMs, nowMs+100)
	}
}

func brontasExposure(v *nativeBrontasGuideView, target brontasPoint, nowMs int64, checkEntry bool) (float64, bool) {
	return brontasPathExposure(v, []brontasPoint{target}, nowMs, checkEntry)
}

func brontasPathExposure(v *nativeBrontasGuideView, path []brontasPoint, nowMs int64, checkEntry bool) (float64, bool) {
	p := v.Player
	if p == nil || p.Speed <= 0 {
		return 0, false
	}
	horizon := float64(nowMs + 2400)
	type leg struct {
		from, to           brontasPoint
		at, depart, arrive float64
	}
	legs := []leg{}
	from, at := p.brontasPoint, float64(nowMs)
	for i, target := range path {
		if checkEntry && brontasStepAxis(from, target) == "" {
			return 0, false
		}
		reaction := brontasGuideReactionMs
		if i == 0 {
			reaction = brontasGridReaction(v, target, nowMs)
		}
		depart := at + float64(reaction)
		arrive := depart + math.Hypot(target.X-from.X, target.Y-from.Y)/p.Speed*1000
		legs = append(legs, leg{from, target, at, depart, arrive})
		from, at = target, arrive
	}
	if checkEntry && at > horizon-300 {
		return 0, false
	}
	type span struct{ a, b float64 }
	spans := []span{}
	add := func(a, b, start, finish float64) {
		a = math.Max(a, math.Max(float64(nowMs), start))
		b = math.Min(b, math.Min(horizon, finish))
		if b > a {
			spans = append(spans, span{a, b})
		}
	}
	for _, ray := range v.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil {
			return 0, false
		}
		low, high := *ray.Offset-*ray.HalfWidth-.12, *ray.Offset+*ray.HalfWidth+.12
		start, finish := float64(ray.LethalAtMs-brontasGuideGuardMs), math.Inf(1)
		if ray.EndsAtMs > 0 {
			finish = float64(ray.EndsAtMs + brontasGuideGuardMs)
		}
		inside := func(c float64) bool { return c >= low && c <= high }
		for _, leg := range legs {
			a, b := leg.from.X, leg.to.X
			if ray.Axis == "horizontal" {
				a, b = leg.from.Y, leg.to.Y
			}
			if inside(a) {
				add(leg.at, leg.depart, start, finish)
			}
			if math.Abs(b-a) < 1e-9 {
				if inside(a) {
					add(leg.depart, leg.arrive, start, finish)
				}
			} else {
				u, w := (low-a)/(b-a), (high-a)/(b-a)
				if u > w {
					u, w = w, u
				}
				u, w = math.Max(0, u), math.Min(1, w)
				if u <= w {
					enter, leave := leg.depart+u*(leg.arrive-leg.depart), leg.depart+w*(leg.arrive-leg.depart)
					// Do not trade existing exposure for entering another damaging
					// stripe. Its activation can be in the future: crossing before it
					// lights up is still unsafe if we remain inside at the hit time.
					if checkEntry && !inside(a) && enter <= finish && leave >= start {
						return 0, false
					}
					add(enter, leave, start, finish)
				}
			}
		}
		end := from.X
		if ray.Axis == "horizontal" {
			end = from.Y
		}
		if inside(end) {
			add(at, horizon, start, finish)
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].a < spans[j].a })
	total := 0.0
	last := float64(nowMs)
	for _, s := range spans {
		a := math.Max(last, s.a)
		if s.b > a {
			total += s.b - a
			last = s.b
		}
	}
	return total, true
}
