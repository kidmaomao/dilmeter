package main

import (
	"math"
	"strconv"
	"strings"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

// Mana uses current (32), base maximum (33), and equipment/status modifier
// (34). Stat 35 is stamina.
func (o *healerObservation) observeManaStat(stat event.EventStatUpdateEntry, atMs int64) {
	switch stat.StatId {
	case 32:
		if stat.Value >= 0 {
			o.manaCurrent, o.manaAtMs = stat.Value, atMs
		}
	case 33:
		o.manaBase, o.manaBaseKnown = stat.Value, stat.Value >= 0
		o.manaReportedMaximum = 0
	case 34:
		o.manaBonus, o.manaBonusKnown = stat.Value, true
		o.manaReportedMaximum = 0
	}
}

func (o *healerObservation) observeManaMaximum(maximum float64) {
	if maximum <= 0 || math.IsNaN(maximum) || math.IsInf(maximum, 0) {
		return
	}
	o.manaReportedMaximum = maximum
	if o.manaBonusKnown && maximum-o.manaBonus >= 0 {
		// One actual maximum paired with its modifier establishes the base.
		// A later modifier-only update can then follow the equipment change.
		o.manaBase, o.manaBaseKnown = maximum-o.manaBonus, true
	}
}

func (o *healerObservation) observedManaMaximum() float64 {
	if o.manaReportedMaximum > 0 {
		return o.manaReportedMaximum
	}
	maximum := o.manaBase + o.manaBonus
	if o.manaBaseKnown && o.manaBonusKnown && maximum > 0 && !math.IsInf(maximum, 0) {
		return maximum
	}
	return 0
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
	maximum := o.observedManaMaximum()
	if o.manaAtMs <= 0 || maximum == 0 {
		return "unknown", nil
	}
	percent := math.Max(0, math.Min(100, o.manaCurrent/maximum*100))
	if nowMs-o.manaAtMs > 30_000 {
		return "stale", &percent
	}
	_, wasLow := h.alerts[id+":mana"]
	if percent <= float64(settings.Threshold) || wasLow && percent < float64(settings.Threshold+5) {
		return "low", &percent
	}
	return "normal", &percent
}

func healerManaPercentText(percent float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(percent, 'f', 1, 64), ".0") + "%"
}
