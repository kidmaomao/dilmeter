package main

// Explicitly labeled demonstration, never fed to the live packet runtime.
func brontasStationPreview(start, now int64, clock int, facing ...string) *nativeBrontasGuideView {
	mode := "outward"
	if len(facing) > 0 {
		mode = normalizeBrontasFacing(facing[0])
	}
	elapsed := (now - start) % 8000
	v := &nativeBrontasGuideView{Facing: mode, Source: "packet", Stage: "map", Clock: clock, Status: "watch", ValidUntilMs: now + 1000}
	if elapsed < 1800 {
		return v
	}
	v.Stage = "station"
	zero, right, width := 0.0, 2.0/3, 1.0/3
	cycle := start + (now-start)/8000*8000
	v.Player = &brontasGuidePlayer{brontasPoint: brontasPoint{}, Radius: .055, Speed: 1.0}
	if elapsed >= 4200 {
		v.Player.X = right
	}
	v.Rays = []brontasGuideRay{
		{ID: "preview-h", Axis: "horizontal", Offset: &zero, HalfWidth: &width, WarningAtMs: cycle + 1800, LethalAtMs: cycle + 5000, EndsAtMs: cycle + 5600},
		{ID: "preview-v", Axis: "vertical", Offset: &zero, HalfWidth: &width, WarningAtMs: cycle + 1800, LethalAtMs: cycle + 4600, EndsAtMs: cycle + 5200},
		{ID: "preview-next", Axis: "vertical", Offset: &right, HalfWidth: &width, WarningAtMs: cycle + 1800, LethalAtMs: cycle + 6500, EndsAtMs: cycle + 7100},
	}
	v.GuidanceValidUntilMs = now + 350
	planBrontasStationGuide(v, now)
	orientBrontasStation(v, mode)
	return v
}
