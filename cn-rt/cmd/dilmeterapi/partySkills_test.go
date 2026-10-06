package main

import (
	"encoding/json"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func partyUse(id string, skill uint16, atMs int64, fallback bool, action uint32) *event.EventSkillAction {
	return &event.EventSkillAction{EventBase: event.EventBase{EventId: event.EventIdSkillAction, Id: id, At: atMs / 1000}, SourceId: id, SkillId: skill, AtMs: atMs, IsLocal: id == "self", IsFallback: fallback, CombatActionId: action}
}
func partyFixture(t *testing.T) (*nativeReminderRuntime, *[]nativeReminderSoundRequest) {
	runtime, sounds := healerFixture(t)
	runtime.healer.settings.Members = []healerMemberSelection{{ID: "ally", Name: "队友", Included: true, SkillSettings: &healerMemberSkillSettings{Enabled: true, SoundEnabled: true, Overlay: healerPosition{Enabled: true, X: -40, Y: 240}, Rules: []healerSkillRule{{SkillID: 59005, Name: "崩坏波动", CooldownSeconds: 10, Sound: healerSound{Kind: "skill-ready"}}}}}}
	return runtime, sounds
}

func TestPartyCooldownUnknownActorsAndMultiHitIsolation(t *testing.T) {
	runtime, sounds := partyFixture(t)
	healerTick(runtime, 100000)
	if runtime.healer.state.Members[0].Skills[59005].State != "unknown" || len(*sounds) != 0 {
		t.Fatal("unobserved cooldown was reported ready")
	}
	runtime.onEvent(liveTestAppear("other", "未选择", 10001, 100))
	pet := liveTestAppear("pet", "宠物", 10001, 100)
	pet.OwnerId = "ally"
	runtime.onEvent(pet)
	runtime.onEvent(partyUse("other", 59005, 100000, false, 1))
	runtime.onEvent(partyUse("pet", 59005, 100000, false, 1))
	runtime.onEvent(partyUse("ally", 59005, 100000, false, 1))
	runtime.onEvent(partyUse("ally", 59005, 100500, false, 1))
	runtime.onEvent(partyUse("ally", 59005, 104000, true, 2))
	healerTick(runtime, 104000)
	state := runtime.healer.state.Members[0].Skills[59005]
	if state.State != "cooling" || *state.RemainingSeconds != 6 {
		t.Fatalf("multi-hit restarted cooldown: %+v", state)
	}
	if runtime.partySkills["pet"] != nil || len(runtime.skillCooldowns) != 0 {
		t.Fatal("pet/remote use polluted player cooldown state")
	}
	healerTick(runtime, 110000)
	healerTick(runtime, 112000)
	if len(*sounds) != 1 || runtime.healer.state.Members[0].Skills[59005].State != "ready" {
		t.Fatalf("readiness did not alert once: %+v", *sounds)
	}
	runtime.onEvent(partyUse("ally", 59005, 113000, false, 3))
	healerTick(runtime, 113000)
	healerTick(runtime, 123000)
	if len(*sounds) != 2 {
		t.Fatal("a new confirmed cast did not rearm readiness")
	}
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "ally", At: 124}})
	if runtime.partySkills["ally"] != nil {
		t.Fatal("disappeared actor retained a live cast")
	}
}

func TestPartyCooldownServerChangesNeverCreateUnobservedReadiness(t *testing.T) {
	runtime, _ := partyFixture(t)
	reset := &event.EventSkillCooldown{EventBase: event.EventBase{Id: "ally", At: 100}, SkillId: 59005, AtMs: 100000, Reset: true}
	runtime.onEvent(reset)
	healerTick(runtime, 100000)
	if runtime.healer.state.Members[0].Skills[59005].State != "unknown" {
		t.Fatal("reset fabricated a use")
	}
	runtime.onEvent(partyUse("ally", 59005, 101000, false, 1))
	runtime.onEvent(&event.EventSkillCooldown{EventBase: event.EventBase{Id: "ally", At: 102}, SkillId: 59005, AtMs: 102000, ReduceMs: 4000})
	healerTick(runtime, 103000)
	if remaining := runtime.healer.state.Members[0].Skills[59005].RemainingSeconds; remaining == nil || *remaining != 4 {
		t.Fatalf("wrong reduced cooldown: %v", remaining)
	}
	reset.At, reset.AtMs = 104, 104000
	runtime.onEvent(reset)
	healerTick(runtime, 104000)
	if runtime.healer.state.Members[0].Skills[59005].State != "ready" {
		t.Fatal("authoritative reset was ignored")
	}
	if len(runtime.skillCooldowns) != 0 {
		t.Fatal("remote reset started a personal cooldown")
	}
}

func TestBurstCastCancelReadyAndSelectedTeammates(t *testing.T) {
	runtime, sounds := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeSelf, runtime.settings.Burst.IncludeTeammates = true, true, true
	runtime.settings.Burst.Volume = 80
	rule := runtime.settings.Burst.Rules[59005]
	rule.CooldownSeconds = 2
	// Exercise the shared cooldown when no personal override is configured.
	runtime.healer.settings.Members[0].SkillSettings.Rules = nil
	runtime.settings.Burst.Rules[59005] = rule
	runtime.onEvent(liveTestAppear("other", "路人", 10001, 100))
	runtime.onEvent(partyUse("other", 59005, 100000, false, 1))
	prepare := &event.EventSkillState{EventBase: event.EventBase{Id: "ally", At: 100}, AtMs: 100000, SkillId: 59005, Scope: "prepare", Active: true}
	runtime.onEvent(prepare)
	popups, _ := runtime.burstOverlay(time.UnixMilli(100000))
	if len(popups) != 1 || popups[0].EndsAtMs != 102000 || len(*sounds) != 1 {
		t.Fatalf("bad preparation: %+v, %+v", popups, *sounds)
	}
	prepare.Active, prepare.AtMs = false, 100500
	runtime.onEvent(prepare)
	popups, _ = runtime.burstOverlay(time.UnixMilli(100500))
	if len(popups) != 0 {
		t.Fatal("cancelled preparation remained visible")
	}
	if runtime.partySkills["ally"][59005].UsedAtMs != 0 {
		t.Fatal("preparation/cancel started a cooldown")
	}
	runtime.onEvent(partyUse("ally", 59005, 101000, false, 2))
	popups, _ = runtime.burstOverlay(time.UnixMilli(103000))
	if len(popups) != 1 || !popups[0].HideCountdown || popups[0].Label != "队友 · 崩坏波动已就绪" {
		t.Fatalf("bad ready popup: %+v", popups)
	}
	runtime.burstOverlay(time.UnixMilli(103500))
	if len(*sounds) != 2 {
		t.Fatal("ready sound repeated every evaluation")
	}
	popups, _ = runtime.burstOverlay(time.UnixMilli(106000))
	if len(popups) != 0 {
		t.Fatal("expired ready popup remained visible")
	}
}

func TestBurstConditionsUseActualExpiryAndIndependentRecipients(t *testing.T) {
	runtime, _ := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeSelf, runtime.settings.Burst.IncludeTeammates = true, true, true
	runtime.settings.Burst.Volume = 80
	runtime.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	runtime.onEvent(liveTestAppear("other", "路人", 10001, 100))
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 100}, CCId: 803, DurationMs: 7500, AttackerId: "ally"})
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 100}, CCId: 516, DisableAtMs: 120250})
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "self", At: 100}, CCId: 516, DurationMs: 5000})
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "other", At: 100}, CCId: 516, DurationMs: 5000})
	bars, _ := runtime.burstOverlay(time.UnixMilli(101000))
	if len(bars) != 3 {
		t.Fatalf("lost/combined recipients or displayed stranger: %+v", bars)
	}
	ends := map[string]int64{}
	for _, bar := range bars {
		ends[bar.TargetID] = bar.EndsAtMs
	}
	if ends["boss"] != 107500 || ends["ally"] != 110000 || ends["self"] != 105000 {
		t.Fatalf("fabricated durations: %+v", ends)
	}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 102}, CCId: 516, DurationMs: 8000})
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 102}, CCId: 516})
	bars, _ = runtime.burstOverlay(time.UnixMilli(103000))
	ends = map[string]int64{}
	for _, bar := range bars {
		ends[bar.TargetID] = bar.EndsAtMs
	}
	if ends["ally"] != 110000 {
		t.Fatal("refresh replacement/remove destroyed the new condition")
	}
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 104}, CCId: 516})
	bars, _ = runtime.burstOverlay(time.UnixMilli(104000))
	for _, bar := range bars {
		if bar.TargetID == "ally" {
			t.Fatal("removed condition kept counting")
		}
	}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 105}, CCId: 516})
	bars, _ = runtime.burstOverlay(time.UnixMilli(106000))
	for _, bar := range bars {
		if bar.TargetID == "ally" && (bar.TimingUnknown || bar.Phase != "effect" || bar.EndsAtMs != 115000) {
			t.Fatal("missing live CC516 duration must use Awakening's ten-second limit")
		}
	}
}

func TestPartySkillSettingsLegacyAndTemplatesAreIndependent(t *testing.T) {
	settings := normalizeHealerSettings(healerSettings{Members: []healerMemberSelection{{Name: "队友", Included: true}}})
	member := settings.Members[0]
	if member.SkillSettings == nil || member.SkillSettings.Enabled || len(member.SkillSettings.Rules) != 0 {
		t.Fatal("legacy preferences enabled synthetic skill reminders")
	}
	member.SkillSettings.Enabled = true
	member.SkillSettings.Rules = []healerSkillRule{{SkillID: 59005, Name: "崩坏", CooldownSeconds: 12.5, Sound: healerSound{Kind: "none"}}}
	templates := normalizeHealerTemplates([]healerMemberTemplate{{ID: "one", Name: "输出", SkillSettings: member.SkillSettings}}, settings)
	member.SkillSettings.Rules[0].CooldownSeconds = 99
	if templates[0].SkillSettings.Rules[0].CooldownSeconds != 12.5 {
		t.Fatal("template shared skill rules")
	}
	data, _ := json.Marshal(templates)
	var restored []healerMemberTemplate
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored[0].SkillSettings.Rules[0].CooldownSeconds != 12.5 {
		t.Fatal("skill template did not roundtrip")
	}
}

func TestPublisherPartyEventsKeepActualPlayerIdentity(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	for _, id := range []uint64{200, 300} {
		publisher.entityCache[id] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: id, RaceId: 10001}}
	}
	at := time.Unix(100, 0)
	for _, id := range []uint64{200, 300} {
		publisher.publishTechniqueConditionSkillAction(at, &packet.CharacterConditionPacket{Id: id, IsEnable: true, EntityCharacterCondition: packet.EntityCharacterCondition{CCId: 520}})
		publisher.publishPartyPreparation(&packet.GamePacket{Id: id, At: at, Msg: packet.Message{packet.NewMessageElemShort(59005)}}, true, false)
	}
	if len(publisher.pendingEvents) != 4 {
		t.Fatalf("remote players were filtered/deduplicated together: %d", len(publisher.pendingEvents))
	}
	for _, raw := range publisher.pendingEvents {
		if action, ok := raw.(*event.EventSkillAction); ok && (action.Id == "100" || action.IsLocal || action.SourceId != action.Id) {
			t.Fatal("remote technique was attributed to local player")
		}
	}
	publisher.pendingEvents = nil
	publisher.publishPartyCombatAction(&packet.GamePacket{At: at}, &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{{EntityId: 200, SkillId: 59005, Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 400}}}})
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("remote attack fallback was lost")
	}
	publisher.pendingEvents = nil
	for _, id := range []uint64{200, 300} {
		if err := publisher.publishSkillExecutePacket(&packet.GamePacket{Id: id, At: at, Msg: packet.Message{packet.NewMessageElemShort(59005), packet.NewMessageElemInt(9)}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(publisher.pendingEvents) != 2 {
		t.Fatal("same action token from different players was merged")
	}
	if err := publisher.publishSkillExecutePacket(&packet.GamePacket{Id: 200, At: at, Msg: packet.Message{packet.NewMessageElemShort(59005), packet.NewMessageElemInt(9)}}); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 2 {
		t.Fatal("duplicate remote execution was published twice")
	}
}

func TestBurstSuccessMuteAndConnectionReset(t *testing.T) {
	runtime, sounds := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeTeammates, runtime.settings.Burst.Volume = true, true, 80
	rule := runtime.settings.Burst.Rules[59005]
	rule.Cast.SoundEnabled = false
	runtime.settings.Burst.Rules[59005] = rule
	runtime.onEvent(partyUse("ally", 59005, 100000, false, 1))
	popups, _ := runtime.burstOverlay(time.UnixMilli(100500))
	if len(popups) != 1 || len(*sounds) != 0 {
		t.Fatal("cast sound toggle also suppressed the visual or played sound")
	}
	runtime.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 101}, CCId: 803, DurationMs: 5000, AttackerId: "ally"})
	popups, bars := runtime.burstOverlay(time.UnixMilli(101000))
	if len(popups) != 1 || popups[0].Phase != "effect" || len(bars) != 0 {
		t.Fatal("resulting condition did not replace the cast popup")
	}
	runtime.onEvent(partyUse("ally", 58014, 101000, false, 2))
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 102}, CCId: 516, DurationMs: 8000})
	popups, bars = runtime.burstOverlay(time.UnixMilli(102000))
	if len(popups) != 2 || popups[0].Phase != "effect" || popups[1].Phase != "effect" || len(bars) != 0 {
		t.Fatal("awakening did not replace the power cast popup")
	}
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "self", At: 103}, Reset: true})
	popups, bars = runtime.burstOverlay(time.UnixMilli(103000))
	if len(runtime.partySkills) != 0 || len(popups)+len(bars) != 0 {
		t.Fatal("reconnected session retained previous timers")
	}
}

func TestPartySkillDoesNotTriggerMonsterEffectTimers(t *testing.T) {
	runtime, _ := partyFixture(t)
	runtime.settings.EffectTimers.Rules = map[string]nativeEffectTimerRule{"monster": {Enabled: true, SourceType: "skill", SourceID: 59005, TargetMode: "monster", DurationSeconds: 5}}
	runtime.onEvent(partyUse("ally", 59005, 100000, false, 1))
	if len(runtime.effectTimers) != 0 {
		t.Fatal("remote player was treated as a monster skill source")
	}
	runtime.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	runtime.onEvent(partyUse("boss", 59005, 101000, false, 2))
	if len(runtime.effectTimers) != 1 {
		t.Fatal("real monster skill was filtered with teammates")
	}
}

func TestPartySkillsResumeAfterRevivalWithoutReappearance(t *testing.T) {
	runtime, _ := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeTeammates = true, true
	runtime.onEvent(healerHP("ally", 100, 100, 100))
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 101}})
	runtime.onEvent(partyUse("ally", 59005, 102000, false, 1))
	if runtime.partySkills["ally"][59005] != nil {
		t.Fatal("fallen player used a skill from old HP")
	}
	runtime.onEvent(healerHP("ally", 103, 100, 100))
	runtime.onEvent(partyUse("ally", 59005, 104000, false, 2))
	popups, _ := runtime.burstOverlay(time.UnixMilli(104000))
	if len(popups) != 1 || runtime.partySkills["ally"][59005].UsedAtMs != 104000 {
		t.Fatal("in-place revival did not resume skill monitoring")
	}
}

func TestBurstConditionBeforeDerivedUseDoesNotRestartCompletedCast(t *testing.T) {
	runtime, _ := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeTeammates = true, true
	runtime.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 101}, CCId: 803, DurationMs: 5000, AttackerId: "ally"})
	runtime.onEvent(partyUse("ally", 59005, 101300, true, 0))
	popups, bars := runtime.burstOverlay(time.UnixMilli(101500))
	if len(popups) != 1 || popups[0].Phase != "effect" || len(bars) != 0 || runtime.partySkills["ally"][59005].UsedAtMs != 101000 {
		t.Fatal("completed cast was replayed after its resulting condition")
	}
	if runtime.partySkills["ally"][58014] != nil {
		t.Fatal("collapse confirmation created unrelated skills")
	}
}

func TestPartySkillReadyContinuesThroughIdleHealthTraffic(t *testing.T) {
	runtime, sounds := partyFixture(t)
	runtime.healer.settings.Members[0].SkillSettings.Rules[0].CooldownSeconds = 60
	runtime.onEvent(partyUse("ally", 59005, 100000, false, 1))
	healerTick(runtime, 159000)
	if state := runtime.healer.state.Members[0].Skills[59005]; state.State != "cooling" || *state.RemainingSeconds != 1 {
		t.Fatalf("idle traffic paused known cooldown: %+v", state)
	}
	healerTick(runtime, 160000)
	if len(*sounds) != 1 || runtime.healer.state.Members[0].Skills[59005].State != "ready" {
		t.Fatal("idle teammate did not announce cooldown readiness")
	}
	runtime.onEvent(partyUse("ally", 59005, 161000, false, 2))
	runtime.healer.evaluate(runtime, time.UnixMilli(221000), false)
	if len(*sounds) != 1 {
		t.Fatal("stopped capture announced a new readiness")
	}
}

func TestCollapseAreaHitFollowsOneBoss(t *testing.T) {
	runtime, _ := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeSelf = true, true
	for _, id := range []string{"boss", "trash"} {
		runtime.onEvent(liveTestAppear(id, id, 7603, 100))
		runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: id, At: 100}, CCId: 803, DurationMs: 10000, AttackerId: "self"})
	}
	runtime.entities["boss"].MaximumHealth = 1_000_000_000
	runtime.entities["trash"].MaximumHealth = 100
	bars, _ := runtime.burstOverlay(time.UnixMilli(101000))
	if len(bars) != 1 || bars[0].TargetID != "boss" {
		t.Fatalf("ordinary area targets crowded the Boss bar: %+v", bars)
	}
}
