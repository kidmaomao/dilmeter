package main

import "math"

// All four giants have independent life counters and no owner field. Selection
// is authoritative: collateral damage must never choose a different giant.
func (runtime *nativeReminderRuntime) selectedAncientGiantHealth(target *nativeReminderEntity) *nativeTargetHealthOverlayItem {
	s := runtime.settings.BossMechanics
	if !s.AncientGiantHealthEnabled || target == nil || !target.Known || !target.Active || target.Defeated || !target.HealthKnown || target.RaceID < 7611 || target.RaceID > 7614 {
		return nil
	}
	current, maximum := target.CurrentHealth, target.MaximumHealth
	if current <= 0 || maximum <= 0 || math.IsNaN(current) || math.IsNaN(maximum) || math.IsInf(current, 0) || math.IsInf(maximum, 0) {
		return nil
	}
	phase := "一王机制"
	if boss := runtime.entities[runtime.activeBossID]; boss != nil && boss.Active && !boss.Defeated {
		if boss.RaceID == 7600 {
			phase = "一王 70%"
		}
		if boss.RaceID == 7601 {
			phase = "一王 25%"
		}
	}
	return &nativeTargetHealthOverlayItem{Kind: "ancient-giant", EntityID: target.ID, Name: "古代巨人", PhaseLabel: phase, CurrentHealth: math.Min(current, maximum), MaximumHealth: maximum, X: s.AncientGiantHealthX, Y: s.AncientGiantHealthY, ScalePercent: s.AncientGiantHealthScale, OpacityPercent: s.AncientGiantHealthOpacity}
}
