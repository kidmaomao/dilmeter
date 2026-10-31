package main

import "math"

// Pace the already chosen cell route from real warning deadlines. Global ray
// frequency includes opposite shots and remote/random lanes; it is not a step
// timer. Keep a clear cell until it is baited, then leave with a measured-speed
// travel budget, reaction time and the existing guard. Never delay an active
// danger, or stop a leg that the player has already started.
func brontasRhythmPace(v *nativeBrontasGuideView, s *brontasRhythmPlan, now int64) bool {
	if s.Released {
		return false
	}
	target, next := brontasRhythmTargets(v, *s)
	p := v.Player.brontasPoint
	if !s.OriginSet {
		s.Origin, s.OriginSet = p, true
	}
	if brontasRhythmStarted(v, s, target, now) {
		return false
	}
	travel := func(a, b brontasPoint) int64 {
		return int64(math.Ceil(math.Hypot(b.X-a.X, b.Y-a.Y) / v.Player.Speed * 1000))
	}
	first := travel(p, target) + brontasGuideReactionMs
	second := first + travel(target, next) + brontasGuideReactionMs
	depart := int64(math.MaxInt64)
	for _, ray := range v.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil {
			return false // Unknown geometry cannot justify a pause.
		}
		if ray.EndsAtMs > 0 && ray.EndsAtMs+brontasGuideGuardMs <= now {
			continue
		}
		c := p.X
		if ray.Axis == "horizontal" {
			c = p.Y
		}
		// Use the same .12 spatial allowance as brontasGridLeg. Adding the
		// decorative dot radius again would treat a confirmed corner as still
		// inside the old stripe and immediately release the next step.
		if math.Abs(c-*ray.Offset) > *ray.HalfWidth+.12 {
			continue
		}
		budget := first
		if ray.Axis == s.Axis {
			// A parallel ray needs the subsequent turn to leave its channel.
			budget = second
		}
		depart = min(depart, ray.LethalAtMs-budget-brontasGuideGuardMs)
	}
	// A pause must not consume the remaining crossing window. Remove only
	// snapshot expiry for this geometry query; actual advice is still gated by
	// the original source deadline in planBrontasRhythmGuide.
	geometry := *v
	geometry.ValidUntilMs, geometry.GuidanceValidUntilMs = math.MaxInt64, math.MaxInt64
	kind, until, _, _ := brontasRhythmAssessment(&geometry, *s, now)
	// A blocked route is not a verified reason to wait. In the Oct 31 capture
	// the next leg crossed an older warning, yet an empty assessment still
	// displayed a bait countdown before switching to an urgent, unsafe return.
	if kind != "route" {
		return false
	}
	if until < math.MaxInt64 {
		depart = min(depart, until-200)
	}
	if depart != math.MaxInt64 && (s.ReleaseAtMs == 0 || depart < s.ReleaseAtMs) {
		s.ReleaseAtMs = depart
	}
	// New warnings can bring the cue forward; speed noise cannot postpone it.
	if s.ReleaseAtMs != 0 && s.ReleaseAtMs <= now {
		return false
	}
	v.Guidance = "pace"
	v.GuidanceReason = "wait-foot-warning"
	v.PaceUntilMs = s.ReleaseAtMs
	if s.ReleaseAtMs > 0 {
		v.GuidanceReason = "wait-departure"
		// Only a fresh backend assessment can release the movement arrow.
		v.GuidanceValidUntilMs = min(v.GuidanceValidUntilMs, s.ReleaseAtMs)
	}
	return true
}

func brontasRhythmStarted(v *nativeBrontasGuideView, s *brontasRhythmPlan, target brontasPoint, now int64) bool {
	if brontasGridReaction(v, target, now) == 0 {
		return true
	}
	if !s.OriginSet {
		return false
	}
	progress, direction := v.Player.X-s.Origin.X, target.X-s.Origin.X
	if s.Axis == "vertical" {
		progress, direction = v.Player.Y-s.Origin.Y, target.Y-s.Origin.Y
	}
	return progress*direction > 0 && math.Abs(progress) > .08
}
