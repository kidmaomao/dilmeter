package main

// At an unissued corner, prefer a verified mirrored L over a blocked turn.
// Once an arrow has been issued or movement has started, never reverse it to
// chase a lower per-frame risk score. Both choices still move one cell per axis.
func brontasRhythmSafeTurn(v *nativeBrontasGuideView, s *brontasRhythmPlan, now int64) bool {
	if s.Released || brontasCell(v.Player.brontasPoint) != s.From {
		return false
	}
	target, _ := brontasRhythmTargets(v, *s)
	if brontasRhythmStarted(v, s, target, now) {
		return false
	}
	if kind, _, _, _ := brontasRhythmAssessment(v, *s, now); kind == "route" {
		return false
	}
	first, second := s.DX, s.DY
	if s.Axis == "vertical" {
		first, second = s.DY, s.DX
	}
	for _, a := range []int{first, -first} {
		for _, b := range []int{second, -second} {
			candidate := *s
			candidate.DX, candidate.DY = a, b
			if s.Axis == "vertical" {
				candidate.DX, candidate.DY = b, a
			}
			candidate = brontasRhythmContinue(candidate, s.From, s.Axis, now)
			if candidate.To == s.To && candidate.Next == s.Next {
				continue
			}
			if kind, _, _, _ := brontasRhythmAssessment(v, candidate, now); kind == "route" {
				candidate.Origin, candidate.OriginSet, candidate.ReleaseAtMs = s.Origin, s.OriginSet, s.ReleaseAtMs
				*s = candidate
				return true
			}
		}
	}
	return false
}
