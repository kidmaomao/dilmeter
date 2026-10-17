package main

import (
	"math"
	"strings"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

const greenCurtainKey = "miel-green-curtain"
const greenCurtainIntervalMs int64 = 63_000
const greenCurtainBossRaceID uint32 = 7615 // 雷内恩的米耶尔：悔恨（四王）

// Kept outside settings-dependent state: saving a profile must not move the
// encounter's clock or repeat a warning that has already played.
type nativeGreenCurtainRuntime struct {
	BossID          string
	StartedAtMs     int64
	LastCurtainAtMs int64
	Occurrence      int64
	SeenCurtains    map[string]bool
	AnnouncedCycle  int64
}

func (runtime *nativeReminderRuntime) observeGreenCurtainBoss(entity *nativeReminderEntity, atMs int64) {
	if entity.RaceID != greenCurtainBossRaceID || entity.ID == "" {
		return
	}
	if runtime.greenCurtain.BossID == entity.ID {
		return // Repeated appearance snapshots do not restart the clock.
	}
	runtime.greenCurtain = nativeGreenCurtainRuntime{BossID: entity.ID, StartedAtMs: atMs, AnnouncedCycle: 1, SeenCurtains: make(map[string]bool)}
	delete(runtime.bossMechanics, greenCurtainKey)
	// The boss is initially invulnerable. Announce the opening immediately,
	// but do not use spawn or the first player's hit as the recurring anchor.
	if rule, ok := runtime.settings.BossMechanics.Rules[greenCurtainKey]; ok && rule.Enabled {
		runtime.playGreenCurtainSound(rule)
	}
}

func (runtime *nativeReminderRuntime) observeGreenCurtainCondition(value *event.EventCharacterConditionEnable) {
	clock := &runtime.greenCurtain
	entity := runtime.entities[value.Id]
	// In the captured encounter, the recurring curtain is race 7622 with
	// height 200. Opening decoys share the race but use height 250. Snapshots
	// are not evidence of a new curtain and must never reset the phase.
	if clock.BossID == "" || entity == nil || !entity.Active || entity.RaceID != 7622 ||
		value.Snapshot || value.CCId != 117 || clock.SeenCurtains[value.Id] ||
		!strings.Contains(";"+value.Metadata, ";HGHT:f:200;") {
		return
	}
	atMs := nativeEventAtMs(value.At, value.AtMs)
	if atMs < clock.StartedAtMs || (clock.LastCurtainAtMs > 0 && atMs-clock.LastCurtainAtMs < greenCurtainIntervalMs/2) {
		return
	}
	clock.SeenCurtains[value.Id] = true
	if clock.LastCurtainAtMs == 0 {
		clock.Occurrence = 1
	} else {
		clock.Occurrence += int64(math.Round(float64(atMs-clock.LastCurtainAtMs) / float64(greenCurtainIntervalMs)))
	}
	clock.LastCurtainAtMs = atMs
	if clock.AnnouncedCycle < clock.Occurrence {
		clock.AnnouncedCycle = clock.Occurrence // Never replay an expired warning.
	}
	delete(runtime.bossMechanics, greenCurtainKey)
}

func (runtime *nativeReminderRuntime) clearGreenCurtain() {
	runtime.greenCurtain = nativeGreenCurtainRuntime{}
	delete(runtime.bossMechanics, greenCurtainKey)
}

func greenCurtainWarningSeconds(rule nativeBossMechanicRule) float64 {
	if rule.WarningSeconds == nil {
		return 10
	}
	value := *rule.WarningSeconds
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 10
	}
	return math.Min(62, math.Max(1, value))
}

func (runtime *nativeReminderRuntime) evaluateGreenCurtain(now time.Time) {
	clock := &runtime.greenCurtain
	if clock.BossID == "" {
		return
	}
	boss := runtime.entities[clock.BossID]
	if boss == nil || !boss.Active || boss.Defeated || boss.RaceID != greenCurtainBossRaceID {
		runtime.clearGreenCurtain()
		return
	}
	rule, exists := runtime.settings.BossMechanics.Rules[greenCurtainKey]
	nowMs := now.UnixMilli()
	if !exists || !rule.Enabled || nowMs < clock.StartedAtMs {
		delete(runtime.bossMechanics, greenCurtainKey)
		return
	}
	if clock.LastCurtainAtMs == 0 || nowMs < clock.LastCurtainAtMs {
		return // Wait for the actual first curtain; opening audio already played.
	}
	// Predict from the latest observed curtain, not from polling ticks. Each
	// actual curtain corrects the small delay introduced by server scheduling.
	steps := (nowMs-clock.LastCurtainAtMs)/greenCurtainIntervalMs + 1
	cycle := clock.Occurrence + steps
	dueAtMs := clock.LastCurtainAtMs + steps*greenCurtainIntervalMs
	warningAtMs := dueAtMs - int64(greenCurtainWarningSeconds(rule)*1000)
	if nowMs < warningAtMs {
		delete(runtime.bossMechanics, greenCurtainKey)
		return
	}
	if runtime.bossMechanics == nil {
		runtime.bossMechanics = make(map[string]nativeBossMechanicRuntime)
	}
	runtime.bossMechanics[greenCurtainKey] = nativeBossMechanicRuntime{
		Key: greenCurtainKey, StartedAtMs: warningAtMs, EndsAtMs: dueAtMs,
		Generation: uint64(cycle), BossID: boss.ID, BossRaceID: boss.RaceID,
	}
	if clock.AnnouncedCycle >= cycle {
		return
	}
	clock.AnnouncedCycle = cycle
	runtime.playGreenCurtainSound(rule)
}

func (runtime *nativeReminderRuntime) playGreenCurtainSound(rule nativeBossMechanicRule) {
	if rule.SoundMode == "custom" && rule.CustomSoundID != "" {
		runtime.playSound(nativeReminderSoundRequest{Kind: "custom", Volume: runtime.settings.BossMechanics.Volume, SoundID: rule.CustomSoundID, Priority: true})
	} else if rule.SoundMode == "dedicated" {
		runtime.playSound(nativeReminderSoundRequest{Kind: "boss-" + greenCurtainKey, Volume: runtime.settings.BossMechanics.Volume, Priority: true})
	}
}
