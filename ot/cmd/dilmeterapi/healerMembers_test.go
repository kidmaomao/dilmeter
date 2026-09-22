package main

import (
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHealerFavoritesRestoreOnlyAfterUniqueFreshAppearance(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	healerTick(runtime, 100_000)
	h.settings.Members[0].Favorite = true
	h.settings.Members = append(h.settings.Members, healerMemberSelection{ID: "temporary", Name: "临时队友", Included: true})
	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	})
	data, _ := json.Marshal(h.settings)
	request := httptest.NewRequest("PUT", "/api/healer_monitor", strings.NewReader(string(data)))
	request.RemoteAddr = "127.0.0.1:8000"
	result := httptest.NewRecorder()
	handleHealerMonitor(result, request)
	if result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	saved, err := os.ReadFile(h.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored healerSettings
	if json.Unmarshal(saved, &stored) != nil || len(stored.Members) != 1 || stored.Members[0].ID != "" || stored.Members[0].Name != "队友" {
		t.Fatalf("favorites leaked session identity or lost name: %s", saved)
	}
	runtime.healer = newHealerMonitor(h.settingsPath)
	h = runtime.healer
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "self", At: 101}, Reset: true})
	healerTick(runtime, 101_000)
	if len(h.settings.Members) != 1 || len(h.state.Members) != 1 || h.state.Members[0].Active || h.state.Members[0].WaitingReason == "" {
		t.Fatal("favorite must remain visible while waiting")
	}
	runtime.onEvent(liveTestAppear("ally", "另一个玩家", 10001, 102))
	runtime.onEvent(healerHP("ally", 102, 10, 100))
	healerTick(runtime, 102_000)
	healerTick(runtime, 102_600)
	if len(*sounds) != 0 || h.settings.Members[0].ID != "" {
		t.Fatal("reused entity ID incorrectly matched a favorite")
	}
	runtime.onEvent(liveTestAppear("new-id", "队友", 10001, 103))
	runtime.onEvent(healerHP("new-id", 103, 10, 100))
	healerTick(runtime, 103_000)
	healerTick(runtime, 103_600)
	if len(*sounds) != 1 || h.settings.Members[0].ID != "new-id" {
		t.Fatal("fresh named teammate did not restore monitoring")
	}
	runtime.onEvent(liveTestAppear("duplicate", "队友", 10001, 104))
	runtime.onEvent(healerHP("duplicate", 104, 10, 100))
	healerTick(runtime, 104_000)
	if h.settings.Members[0].ID != "" || len(h.state.Alerts) != 0 {
		t.Fatal("ambiguous duplicate names must suspend monitoring")
	}
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "duplicate", At: 105}})
	healerTick(runtime, 105_000)
	if h.settings.Members[0].ID != "new-id" {
		t.Fatal("unambiguous teammate failed to rebind")
	}
}

func TestHealerPerMemberThresholdSoundsRulesAndCoordinates(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	first := normalizeHealerMember(healerMemberSelection{ID: "ally", Name: "队友", Included: true, Health: true}, h.settings, 0)
	first.HealthSettings.Threshold = 25
	first.HealthSettings.SoundEnabled = false
	first.HealthSettings.Overlay = healerPosition{Enabled: true, X: -200, Y: 310}
	first.BuffSettings.Enabled = true
	first.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: 680, Name: "战争序曲", WarningSeconds: 10}, healerSound{Kind: "healer-music"})}
	first.BuffSettings.Overlay = healerPosition{Enabled: true, X: 20, Y: 110}
	second := normalizeHealerMember(healerMemberSelection{ID: "second", Name: "第二队友", Included: true, Health: true}, h.settings, 1)
	second.HealthSettings.Threshold = 60
	second.HealthSettings.Sound = healerSound{Kind: "skill-ready"}
	second.HealthSettings.Overlay.Enabled = false
	second.BuffSettings.Enabled = true
	second.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: 192, Name: "活跃进行曲", WarningSeconds: 5}, healerSound{Kind: "healer-buff"})}
	second.BuffSettings.Overlay = healerPosition{Enabled: true, X: 70, Y: 190}
	h.settings.Members = []healerMemberSelection{first, second}
	runtime.onEvent(liveTestAppear("second", "第二队友", 10001, 100))
	runtime.onEvent(healerHP("ally", 100, 40, 100))
	runtime.onEvent(healerHP("second", 100, 40, 100))
	for _, id := range []string{"ally", "second"} {
		for _, cc := range []uint32{680, 192} {
			runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: id, At: 100}, CCId: cc, DisableAt: 140})
		}
	}
	healerTick(runtime, 100_000)
	healerTick(runtime, 100_600)
	if len(h.state.Alerts) != 1 || h.state.Alerts[0].Name != "第二队友" || len(*sounds) != 1 || (*sounds)[0].Kind != "skill-ready" {
		t.Fatal("independent thresholds or audio were ignored")
	}
	if len(h.state.Cards) != 2 || h.state.Cards[0].CCID != 680 || h.state.Cards[1].CCID != 192 || h.state.Cards[0].X != 20 || h.state.Cards[1].X != 70 {
		t.Fatalf("per-member Buffs leaked: %+v", h.state.Cards)
	}
	runtime.onEvent(healerHP("ally", 101, 20, 100))
	healerTick(runtime, 101_000)
	healerTick(runtime, 101_600)
	if len(h.state.Cards) != 3 || h.state.Cards[0].Category != "health" || h.state.Cards[0].X != -200 || h.state.Cards[0].Y != 310 || len(*sounds) != 1 {
		t.Fatal("health overlay must work with its own coordinates and muted sound")
	}
	h.settings.Members[0].BuffSettings.Enabled = false
	healerTick(runtime, 102_000)
	for _, card := range h.state.Cards {
		if card.Name == "队友" && card.Category != "health" {
			t.Fatal("disabled per-member Buff monitor kept rendering")
		}
	}
}

func TestHealerPerMemberPreviewDoesNotMutateLiveConfiguration(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	})
	member := normalizeHealerMember(healerMemberSelection{ID: "example", Name: "预览队友", Included: true, Overture: true}, h.settings, 0)
	member.HealthSettings.Overlay = healerPosition{Enabled: true, X: -600, Y: 250}
	before, _ := json.Marshal(h.settings)
	data, _ := json.Marshal(map[string]any{"member": member, "kind": "health", "iconSize": 30, "opacityPercent": 35, "text": healerTextSettings{FontSize: 18}})
	request := httptest.NewRequest("POST", "/api/healer_text_preview", strings.NewReader(string(data)))
	request.RemoteAddr = "127.0.0.1:8000"
	result := httptest.NewRecorder()
	handleHealerTextPreview(result, request)
	if result.Code != 204 {
		t.Fatal(result.Body.String())
	}
	frame := h.overlaySnapshot(h.previewUntilMs - 1000)
	if !frame.Preview || len(frame.Groups) != 1 || frame.Groups[0].Name != "预览队友" || frame.Groups[0].X != -600 || frame.FontSize != 18 || frame.OpacityPercent != 35 {
		t.Fatalf("draft preview incorrect: %+v", frame)
	}
	after, _ := json.Marshal(h.settings)
	if string(before) != string(after) || len(*sounds) != 0 {
		t.Fatal("preview changed live configuration or made a sound")
	}
	if frame := h.overlaySnapshot(h.previewUntilMs + 1); len(frame.Groups) != 0 || frame.OpacityPercent != 100 {
		t.Fatal("preview did not expire")
	}
}

func TestHealerTemplatesPersistWithoutSelectingAPlayer(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	member := normalizeHealerMember(healerMemberSelection{Name: "模板来源", Health: true, Overture: true}, h.settings, 0)
	member.HealthSettings.Threshold = 35
	member.HealthSettings.RepeatCount, member.HealthSettings.RepeatIntervalSeconds = 3, 7
	member.HealthSettings.Overlay = healerPosition{Enabled: true, X: -200, Y: 320}
	member.BuffSettings.Rules[0].WarningSeconds = 25
	member.BuffSettings.Rules[0].RepeatCount, member.BuffSettings.Rules[0].RepeatIntervalSeconds = 4, 12
	h.settings.Members = nil
	h.settings.OpacityPercent = 55
	h.settings.Templates = []healerMemberTemplate{{ID: "template-1", Name: "队友1", Health: member.Health, HealthSettings: member.HealthSettings, BuffSettings: member.BuffSettings}}
	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	})
	data, _ := json.Marshal(h.settings)
	request := httptest.NewRequest("PUT", "/api/healer_monitor", strings.NewReader(string(data)))
	request.RemoteAddr = "127.0.0.1:8000"
	result := httptest.NewRecorder()
	handleHealerMonitor(result, request)
	if result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	runtime.healer = newHealerMonitor(h.settingsPath)
	restored := runtime.healer.settings
	if len(restored.Templates) != 1 || len(restored.Members) != 0 || restored.OpacityPercent != 55 {
		t.Fatalf("templates or opacity lost after restart: %+v", restored)
	}
	template := restored.Templates[0]
	if template.Name != "队友1" || template.HealthSettings.Threshold != 35 || template.HealthSettings.RepeatCount != 3 || template.HealthSettings.RepeatIntervalSeconds != 7 || template.BuffSettings.Rules[0].RepeatCount != 4 || template.BuffSettings.Rules[0].RepeatIntervalSeconds != 12 || template.HealthSettings.Overlay.X != -200 || template.BuffSettings.Rules[0].WarningSeconds != 25 {
		t.Fatalf("template preferences lost: %+v", template)
	}
	runtime.onEvent(liveTestAppear("template-named-player", "队友1", 10001, 103))
	runtime.onEvent(healerHP("template-named-player", 103, 1, 100))
	healerTick(runtime, 103_000)
	healerTick(runtime, 103_600)
	if len(runtime.healer.state.Alerts) != 0 || len(*sounds) != 0 {
		t.Fatal("a template alone must never select or monitor a character")
	}
	if frame := runtime.healer.overlaySnapshot(103_600); frame.OpacityPercent != 55 {
		t.Fatal("saved opacity did not reach the overlay")
	}
	if normalizeHealerSettings(healerSettings{}).OpacityPercent != 100 {
		t.Fatal("legacy settings must keep full opacity")
	}
}
