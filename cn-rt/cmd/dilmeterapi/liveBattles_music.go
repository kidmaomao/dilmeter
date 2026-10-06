package main

import (
	"fmt"
	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func musicPerformanceExpiresAt(v *event.EventCharacterConditionEnable) int64 {
	if v.DisableAtMs > v.At*1000 {
		return (v.DisableAtMs + 999) / 1000
	}
	if v.DisableAt > v.At {
		return v.DisableAt
	}
	if v.DurationMs > 0 {
		return v.At + (v.DurationMs+999)/1000
	}
	return v.At // Unknown duration cannot establish a pre-fight performance.
}

func (s *liveBattleIndex) retainMusicPerformance(v *event.EventCharacterConditionEnable) {
	if (v.CCId != 680 && v.CCId != 192 && v.CCId != 193) || v.AttackerId == "" || v.AttackerId == "0" {
		return
	}
	a := s.actor(v.AttackerId)
	if a.music == nil {
		a.music = map[string]*event.EventMusicPerformance{}
	}
	for key, previous := range a.music {
		if musicPerformanceExpiresAt(&previous.EventCharacterConditionEnable) <= v.At {
			delete(a.music, key)
		}
	}
	copy := *v
	copy.EventId, copy.Sequence = event.EventIdMusicPerformance, 0
	// Identical group applications are one performance. Keep different values
	// if a recipient's effect differs; KPI takes the maximum for each field.
	key := fmt.Sprintf("%d:%d:%s", v.At, v.CCId, v.Metadata)
	if previous := a.music[key]; previous == nil || musicPerformanceExpiresAt(&previous.EventCharacterConditionEnable) < musicPerformanceExpiresAt(v) {
		a.music[key] = &event.EventMusicPerformance{EventCharacterConditionEnable: copy}
	}
}
