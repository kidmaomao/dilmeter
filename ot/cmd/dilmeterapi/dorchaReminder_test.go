package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func newDorchaReminderTestRuntime(soundMode string) (*nativeReminderRuntime, *[]nativeReminderSoundRequest) {
	sounds := []nativeReminderSoundRequest{}
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{
		SkillCooldowns: nativeSkillCooldownSettings{Rules: map[uint16]nativeSkillCooldownRule{
			dorchaMasterySkillID: {Enabled: true, QuantityThreshold: 3, SoundMode: soundMode, CustomSoundID: "dorcha-low", X: 100, Y: 120},
		}},
	}, &sounds)
	runtime.localID = "player"
	return runtime, &sounds
}

func dorchaStat(id string, private bool, value float64) *event.EventStatUpdate {
	return &event.EventStatUpdate{
		EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 100, Id: id},
		Private:   private, Stats: []event.EventStatUpdateEntry{{StatId: dorchaStatID, Value: value}},
	}
}

func TestDorchaLowQuantityStrictThresholdAndRearm(t *testing.T) {
	runtime, sounds := newDorchaReminderTestRuntime("default")
	for _, quantity := range []float64{15, 3, 3} {
		runtime.onEvent(dorchaStat("player", true, quantity))
	}
	if runtime.dorchaQuantityOverlay() != nil || len(*sounds) != 0 {
		t.Fatal("3 points must not trigger a below-3 alert")
	}
	for _, quantity := range []float64{2.75, 2.75, 2, 0} {
		runtime.onEvent(dorchaStat("player", true, quantity))
	}
	if len(*sounds) != 1 || (*sounds)[0].Kind != "skill-ready" {
		t.Fatalf("low updates must sound once: %+v", *sounds)
	}
	item := runtime.dorchaQuantityOverlay()
	if item == nil || item.QuantityText != "0" || item.QuantityUnit != "/ 15" || !item.Persistent || item.ScalePercent != 100 {
		t.Fatalf("bad live quantity card: %+v", item)
	}
	runtime.onEvent(dorchaStat("player", true, 3))
	if runtime.dorchaQuantityOverlay() != nil {
		t.Fatal("recovery must dismiss the alert")
	}
	runtime.onEvent(dorchaStat("player", true, 2.5))
	if len(*sounds) != 2 {
		t.Fatal("a subsequent real spend must re-arm the alert")
	}
	if nativeStackReminderValue(*runtime.dorchaQuantityOverlay()) != "2.5 / 15" {
		t.Fatal("fractional quantity was lost")
	}
}

func TestDorchaQuantityScopeResetAndSoundPreferences(t *testing.T) {
	runtime, sounds := newDorchaReminderTestRuntime("none")
	for _, stat := range []*event.EventStatUpdate{
		dorchaStat("pet", true, 0), dorchaStat("teammate", true, 0), dorchaStat("player", false, 0),
		dorchaStat("player", true, math.NaN()), dorchaStat("player", true, math.Inf(1)),
		dorchaStat("player", true, -1), dorchaStat("player", true, 16),
	} {
		runtime.onEvent(stat)
	}
	if runtime.dorcha.Observed || runtime.dorchaQuantityOverlay() != nil {
		t.Fatal("foreign/invalid data became local energy")
	}
	runtime.onEvent(dorchaStat("player", true, 2))
	if runtime.dorchaQuantityOverlay() == nil || len(*sounds) != 0 {
		t.Fatal("muting must retain the visual alert")
	}
	runtime.resetSettingsDependentState()
	if !runtime.dorcha.Observed || runtime.dorcha.Quantity != 2 {
		t.Fatal("saving settings erased current quantity")
	}
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: event.EventIdLocalEntity, Id: "player"}, Reset: true})
	if runtime.dorcha.Observed || runtime.dorchaQuantityOverlay() != nil {
		t.Fatal("connection reset retained stale quantity")
	}
	rule := runtime.settings.SkillCooldowns.Rules[dorchaMasterySkillID]
	rule.AlwaysVisible, rule.SoundMode = true, "custom"
	runtime.settings.SkillCooldowns.Rules[dorchaMasterySkillID] = rule
	if got := runtime.dorchaQuantityOverlay(); got == nil || got.QuantityText != "--" {
		t.Fatal("unknown quantity must not be rendered as zero")
	}
	runtime.onEvent(dorchaStat("player", true, 1.5))
	if len(*sounds) != 1 || (*sounds)[0].Kind != "custom" || (*sounds)[0].SoundID != "dorcha-low" {
		t.Fatal("custom sound preference was ignored")
	}
}

func TestDorchaQuantityUsesCardAndNeverStartsCooldown(t *testing.T) {
	runtime, _ := newDorchaReminderTestRuntime("none")
	// Exercise the same settings JSON used by the frontend and saved native profile.
	var settings nativeReminderSettings
	if err := json.Unmarshal([]byte(`{"skillCooldowns":{"rules":{"27000":{"enabled":true,"quantityThreshold":3,"soundMode":"none","scalePercent":150,"x":744,"y":268}}}}`), &settings); err != nil {
		t.Fatal(err)
	}
	runtime.settings = normalizeNativeReminderSettings(settings)
	runtime.onEvent(dorchaStat("player", true, 2.75))
	runtime.onEvent(&event.EventSkillAction{EventBase: event.EventBase{At: 100, Id: "player"}, SkillId: dorchaMasterySkillID, IsLocal: true})
	runtime.observeSkillCooldown(dorchaMasterySkillID, 100000, true)
	runtime.observeSkillCooldownAdjustment(&event.EventSkillCooldown{EventBase: event.EventBase{At: 100, Id: "player"}, SkillId: dorchaMasterySkillID, Reset: true})
	if _, exists := runtime.skillCooldowns[dorchaMasterySkillID]; exists {
		t.Fatal("passive mastery incorrectly acquired a cooldown")
	}
	runtime.publishNativeSkillState(time.Unix(100, 0))
	skillOverlayState.RLock()
	data := append([]byte(nil), skillOverlayState.data...)
	skillOverlayState.RUnlock()
	var message nativeSkillOverlayMessage
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatal(err)
	}
	if len(message.Items) != 0 || len(message.StackAlerts) != 1 {
		t.Fatalf("expected one quantity card instead of a CD icon: %+v", message)
	}
	item := message.StackAlerts[0]
	if item.QuantityText != "2.75" || item.SkillID != dorchaMasterySkillID || !nativeSkillOverlayMessageVisible(message, 999999) {
		t.Fatal("quantity card was lost/expired in the overlay bridge")
	}
	if item.ScalePercent != 150 || item.X != 744 || item.Y != 268 {
		t.Fatalf("saved popup size/position did not reach the overlay: %+v", item)
	}
	kind, id := nativeStackReminderIdentity(item)
	if kind != "skill" || id != "27000" {
		t.Fatal("quantity card must drag with its skill setting")
	}
}
