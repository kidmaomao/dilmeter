package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"math"
)

// CN private stats carry mana current (32), base maximum (34) and bonus (35).
// Public life snapshots do not imply that a teammate's mana was observed.
func (o *healerObservation) observeManaStat(stat event.EventStatUpdateEntry, atMs int64) {
	switch stat.StatId {
	case 32:
		if stat.Value >= 0 {
			o.manaCurrent, o.manaAtMs = stat.Value, atMs
		}
	case 34:
		o.manaBase, o.manaBaseKnown = stat.Value, true
	case 35:
		o.manaBonus, o.manaBonusKnown = stat.Value, true
	}
}

func normalizeHealerMana(settings *healerHealthSettings, index int) *healerHealthSettings {
	if settings == nil {
		settings = &healerHealthSettings{Threshold: 25, SoundEnabled: true, Sound: healerSound{Kind: "electronic"}, Overlay: healerPosition{Enabled: true, X: 850, Y: 180 + index*110}}
	}
	value := *settings
	value.Threshold = clampNativeReminderInt(value.Threshold, 5, 95, 25)
	value.RepeatCount = clampNativeReminderInt(value.RepeatCount, 1, 10, 1)
	value.RepeatIntervalSeconds = clampNativeReminderInt(value.RepeatIntervalSeconds, 2, 300, 5)
	value.Sound = normalizeHealerSound(value.Sound, "electronic")
	value.Overlay = normalizeHealerPosition(value.Overlay)
	return &value
}

func (h *healerMonitor) manaState(id string, o *healerObservation, settings *healerHealthSettings, nowMs int64) (string, *float64) {
	maximum := o.manaBase + o.manaBonus
	if o.manaAtMs <= 0 || !o.manaBaseKnown || !o.manaBonusKnown || maximum <= 0 || math.IsNaN(maximum) || math.IsInf(maximum, 0) {
		return "unknown", nil
	}
	if nowMs-o.manaAtMs > 30_000 {
		return "stale", nil
	}
	percent := math.Max(0, math.Min(100, o.manaCurrent/maximum*100))
	_, wasLow := h.alerts[id+":mana"]
	if percent <= float64(settings.Threshold) || wasLow && percent < float64(settings.Threshold+5) {
		return "low", &percent
	}
	return "normal", &percent
}
