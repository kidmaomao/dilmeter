package main

import (
	"math"
	"strconv"
)

const dorchaMasterySkillID = uint16(27000)
const dorchaStatID = uint32(196)

type dorchaQuantityState struct {
	Observed    bool
	Quantity    float64
	Below       bool
	StartedAtMs int64
	Generation  uint64
}

func nativeDorchaThreshold(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 1 || value > 15 {
		return 3
	}
	return math.Round(value)
}

func nativeDorchaQuantityText(value float64) string {
	return strconv.FormatFloat(math.Floor(value*100)/100, 'f', -1, 64)
}

func (runtime *nativeReminderRuntime) observeDorchaQuantity(quantity float64, atMs int64) {
	if math.IsNaN(quantity) || math.IsInf(quantity, 0) || quantity < 0 || quantity > 15 {
		return
	}
	rule, exists := runtime.settings.SkillCooldowns.Rules[dorchaMasterySkillID]
	below := quantity < nativeDorchaThreshold(rule.QuantityThreshold)
	crossed := below && (!runtime.dorcha.Observed || !runtime.dorcha.Below)
	runtime.dorcha.Observed, runtime.dorcha.Quantity, runtime.dorcha.Below = true, quantity, below
	if crossed {
		runtime.dorcha.StartedAtMs = atMs
		runtime.dorcha.Generation++
		if exists && rule.Enabled && !rule.BarOnly && rule.SoundMode != "none" {
			runtime.playNativeSkillSound(rule)
		}
	}
}

func (runtime *nativeReminderRuntime) dorchaQuantityOverlay() *nativeBuffStackOverlayItem {
	rule, exists := runtime.settings.SkillCooldowns.Rules[dorchaMasterySkillID]
	if !exists || !rule.Enabled || rule.BarOnly {
		return nil
	}
	below := runtime.dorcha.Observed && runtime.dorcha.Quantity < nativeDorchaThreshold(rule.QuantityThreshold)
	if !rule.AlwaysVisible && !below {
		return nil
	}
	name, quantity := "多尔卡数量", "--"
	if below {
		name = "多尔卡不足"
	}
	if runtime.dorcha.Observed {
		quantity = nativeDorchaQuantityText(runtime.dorcha.Quantity)
	}
	return &nativeBuffStackOverlayItem{
		SkillID: dorchaMasterySkillID, Name: name, QuantityText: quantity, QuantityUnit: "/ 15",
		Persistent: true, StartedAtMs: runtime.dorcha.StartedAtMs, Generation: runtime.dorcha.Generation,
		X: rule.X, Y: rule.Y, ScalePercent: 100,
	}
}

func nativeStackReminderIdentity(item nativeBuffStackOverlayItem) (string, string) {
	if item.SkillID != 0 {
		return "skill", strconv.FormatUint(uint64(item.SkillID), 10)
	}
	return "stack", strconv.FormatUint(uint64(item.CCID), 10)
}

func nativeStackReminderValue(item nativeBuffStackOverlayItem) string {
	if item.QuantityText != "" {
		return item.QuantityText + " " + item.QuantityUnit
	}
	return strconv.Itoa(item.Stack) + " 层"
}
