package main

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func newNativeReminderRuntimeForTest(settings nativeReminderSettings, sounds *[]nativeReminderSoundRequest) *nativeReminderRuntime {
	return &nativeReminderRuntime{
		ctx:                    context.Background(),
		settings:               normalizeNativeReminderSettings(settings),
		entities:               make(map[string]*nativeReminderEntity),
		announcedBuffSounds:    make(map[string]struct{}),
		lastBuffSoundExpiryMs:  make(map[uint32]int64),
		buffStackAbove:         make(map[uint32]bool),
		buffStackMissingSince:  make(map[uint32]int64),
		observedDebuffs:        make(map[string]bool),
		lastDebuffAppliedAt:    make(map[string]int64),
		debuffMissingSinceMs:   make(map[string]int64),
		announcedDebuffSounds:  make(map[string]struct{}),
		skillCooldowns:         make(map[uint16]nativeSkillCooldownRuntime),
		bossMechanics:          make(map[string]nativeBossMechanicRuntime),
		buffStackAlerts:        make(map[uint32]nativeBuffStackRuntime),
		announcedSkillSounds:   make(map[string]struct{}),
		recentBossMechanicAtMs: make(map[string]int64),
		playSound: func(request nativeReminderSoundRequest) bool {
			*sounds = append(*sounds, request)
			return true
		},
	}
}

func TestNativeReminderSkillReadySoundDoesNotNeedWebViewTicks(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Volume: 70,
		Rules: map[uint16]nativeSkillCooldownRule{1234: {
			SkillID: 1234, Enabled: true, CooldownSeconds: 2, SoundMode: "default",
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.onEvent(&event.EventSkillAction{
		EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: 100, Id: "player"},
		SkillId:   1234, AtMs: 100_000, IsLocal: true,
	})

	runtime.evaluate(time.UnixMilli(101_999))
	if len(sounds) != 0 {
		t.Fatalf("skill sound fired before ready: %#v", sounds)
	}
	runtime.evaluate(time.UnixMilli(102_000))
	runtime.evaluate(time.UnixMilli(102_100))
	if len(sounds) != 1 || sounds[0].Kind != "skill-ready" || sounds[0].Volume != 70 {
		t.Fatalf("native skill sound = %#v, want one skill-ready request", sounds)
	}
}

func TestNativeReminderAppliesServerCooldownReductionAndReset(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Volume: 70,
		Rules: map[uint16]nativeSkillCooldownRule{27203: {
			SkillID: 27203, Enabled: true, CooldownSeconds: 10, SoundMode: "default",
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.localID = "player"
	runtime.observeSkillCooldown(27203, 100_000, false)
	runtime.onEvent(&event.EventSkillCooldown{
		EventBase: event.EventBase{EventId: event.EventIdSkillCooldown, At: 103, Id: "player"},
		AtMs:      103_000, SkillId: 27203, ReduceMs: 2000, Signal: "SLST",
	})
	if got := runtime.skillCooldowns[27203].ReadyAtMs; got != 108_000 {
		t.Fatalf("reduced readyAt = %d, want 108000", got)
	}
	runtime.onEvent(&event.EventSkillCooldown{
		EventBase: event.EventBase{EventId: event.EventIdSkillCooldown, At: 105, Id: "player"},
		AtMs:      105_000, SkillId: 27203, Reset: true, Signal: "27049",
	})
	if got := runtime.skillCooldowns[27203].ReadyAtMs; got != 105_000 {
		t.Fatalf("reset readyAt = %d, want 105000", got)
	}
	runtime.evaluate(time.UnixMilli(105_000))
	if len(sounds) != 1 || sounds[0].Kind != "skill-ready" {
		t.Fatalf("reset completion sound = %#v", sounds)
	}
}

func TestNativeReminderCooldownSignalDoesNotCreateUntrackedState(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Rules: map[uint16]nativeSkillCooldownRule{26002: {
			SkillID: 26002, Enabled: true, CooldownSeconds: 10,
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.onEvent(&event.EventSkillCooldown{
		EventBase: event.EventBase{EventId: event.EventIdSkillCooldown, At: 100},
		AtMs:      100_000, SkillId: 26002, Reset: true, Signal: "27049",
	})
	if _, exists := runtime.skillCooldowns[26002]; exists {
		t.Fatal("reset signal created a cooldown without a prior skill use")
	}
}

func TestNativeReminderCumulativeCooldownCountsDownAndAcceptsDamageFallback(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Volume: 70,
		Rules: map[uint16]nativeSkillCooldownRule{59145: {
			SkillID: 59145, Enabled: true, CooldownSeconds: 15,
			ShortCooldownSeconds: 3, CumulativeCooldownSeconds: 15,
			SoundMode: "default",
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)

	runtime.observeSkillCooldown(59145, 100_000, false)
	first := runtime.skillCooldowns[59145]
	if first.CooldownPhase != "accumulating" || first.AccumulatedCooldownSeconds != 3 ||
		first.AccumulatedReadyAtMs != 103_000 || first.ShortReadyAtMs != 0 {
		t.Fatalf("first cumulative use = %#v", first)
	}
	runtime.evaluate(time.UnixMilli(100_400))
	if len(sounds) != 0 {
		t.Fatalf("accumulating phase announced a false ready sound: %#v", sounds)
	}

	// The captured Spiral Burst stream reports repeat hits as fallback skill
	// actions. They are real re-uses for these two special skills and must count.
	runtime.observeSkillCooldown(59145, 101_000, true)
	second := runtime.skillCooldowns[59145]
	if second.CooldownPhase != "accumulating" || second.AccumulatedCooldownSeconds != 5 ||
		second.AccumulatedReadyAtMs != 106_000 || second.ShortReadyAtMs != 0 {
		t.Fatalf("second cumulative use = %#v", second)
	}
	runtime.evaluate(time.UnixMilli(102_000))
	if current := runtime.skillCooldowns[59145]; current.AccumulatedCooldownSeconds != 4 {
		t.Fatalf("cumulative pool after one second = %#v, want 4s", current)
	}
	runtime.observeSkillCooldown(59145, 102_000, true)
	third := runtime.skillCooldowns[59145]
	if third.CooldownPhase != "accumulating" || third.AccumulatedCooldownSeconds != 7 ||
		third.AccumulatedReadyAtMs != 109_000 {
		t.Fatalf("third cumulative use = %#v, want 4+3=7s", third)
	}
	runtime.evaluate(time.UnixMilli(108_999))
	if current := runtime.skillCooldowns[59145]; current.CooldownPhase != "accumulating" || current.AccumulatedCooldownSeconds != 0.1 {
		t.Fatalf("cumulative pool cleared too early: %#v", current)
	}
	runtime.evaluate(time.UnixMilli(109_000))
	reset := runtime.skillCooldowns[59145]
	if reset.CooldownPhase != "idle" || reset.AccumulatedCooldownSeconds != 0 ||
		reset.AccumulatedReadyAtMs != 0 || reset.ShortReadyAtMs != 0 || reset.ReadyAtMs != 0 {
		t.Fatalf("7s cumulative pool did not clear after 7s: %#v", reset)
	}
	if len(sounds) != 0 {
		t.Fatalf("counting-down cumulative phase announced a false ready sound: %#v", sounds)
	}
}

func TestNativeReminderCumulativeCooldownRepeatedUseEntersFullLockout(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Volume: 70,
		Rules: map[uint16]nativeSkillCooldownRule{59145: {
			SkillID: 59145, Enabled: true, CooldownSeconds: 15,
			ShortCooldownSeconds: 3, CumulativeCooldownSeconds: 15,
			SoundMode: "default",
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.observeSkillCooldown(59145, 100_000, false)
	for atMs := int64(101_000); atMs <= 106_000; atMs += 1_000 {
		runtime.observeSkillCooldown(59145, atMs, true)
	}
	full := runtime.skillCooldowns[59145]
	if full.CooldownPhase != "full" || full.AccumulatedCooldownSeconds != 15 ||
		full.ReadyAtMs != 121_000 || full.ShortReadyAtMs != 0 || full.AccumulatedReadyAtMs != 0 {
		t.Fatalf("full cumulative cooldown = %#v", full)
	}
	generation := full.Generation
	runtime.observeSkillCooldown(59145, 107_000, true)
	if runtime.skillCooldowns[59145].Generation != generation {
		t.Fatal("full cooldown accepted an impossible early use")
	}
	runtime.evaluate(time.UnixMilli(120_999))
	runtime.evaluate(time.UnixMilli(121_000))
	runtime.evaluate(time.UnixMilli(121_100))
	if len(sounds) != 1 || sounds[0].Kind != "skill-ready" {
		t.Fatalf("full cumulative cooldown sound = %#v, want one skill-ready", sounds)
	}

	runtime.observeSkillCooldown(59145, 122_000, false)
	nextCycle := runtime.skillCooldowns[59145]
	if nextCycle.CooldownPhase != "accumulating" || nextCycle.AccumulatedCooldownSeconds != 3 {
		t.Fatalf("next cumulative cycle = %#v", nextCycle)
	}
}

func TestNativeReminderSpiralBurstCapturedPacketSequenceUsesFallbackActions(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Rules: map[uint16]nativeSkillCooldownRule{59145: {
			SkillID: 59145, Enabled: true, CooldownSeconds: 15,
			ShortCooldownSeconds: 3, CumulativeCooldownSeconds: 15,
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)

	// packet_log_2026-08-29_01-02-42.ndjson: the first authoritative
	// activation is followed by damage-derived fallback actions.
	runtime.observeSkillCooldown(59145, 1_787_936_641_822, false)
	runtime.observeSkillCooldown(59145, 1_787_936_642_635, true)
	second := runtime.skillCooldowns[59145]
	if second.Generation != 2 || second.AccumulatedCooldownSeconds != 5.2 {
		t.Fatalf("captured first repeat was not counted: %#v", second)
	}
	for _, atMs := range []int64{1_787_936_644_173, 1_787_936_645_554, 1_787_936_646_881, 1_787_936_648_598} {
		runtime.observeSkillCooldown(59145, atMs, true)
	}
	state := runtime.skillCooldowns[59145]
	if state.Generation != 6 || state.CooldownPhase != "accumulating" || state.AccumulatedCooldownSeconds != 11.3 {
		t.Fatalf("captured Spiral Burst repeats were ignored: %#v", state)
	}
}

func TestNativeReminderCumulativeCooldownExpiresBackToInitialState(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Rules: map[uint16]nativeSkillCooldownRule{59104: {
			SkillID: 59104, Enabled: true, CooldownSeconds: 15,
			ShortCooldownSeconds: 3, CumulativeCooldownSeconds: 15,
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.observeSkillCooldown(59104, 200_000, false)
	runtime.evaluate(time.UnixMilli(202_999))
	if state := runtime.skillCooldowns[59104]; state.CooldownPhase != "accumulating" {
		t.Fatalf("first 3s time pool expired too early: %#v", state)
	}
	runtime.evaluate(time.UnixMilli(203_000))
	reset := runtime.skillCooldowns[59104]
	if reset.CooldownPhase != "idle" || reset.AccumulatedCooldownSeconds != 0 || reset.ReadyAtMs != 0 || reset.ShortReadyAtMs != 0 {
		t.Fatalf("expired cumulative cycle did not reset: %#v", reset)
	}
}

func TestNativeReminderBossMechanicSoundAndClusterDedup(t *testing.T) {
	settings := nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{
		Volume: 85,
		Rules: map[string]nativeBossMechanicRule{"miel-orb": {
			Key: "miel-orb", BossRaceIDs: []uint32{7603, 7615}, Trigger: "orb-spawn",
			SkillID: 52402, TriggerRaceIDs: []uint32{7604, 7616}, Enabled: true, SoundMode: "dedicated",
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.onEvent(&event.EventEntityAppear{
		EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "boss"},
		RaceId:    7603,
	})
	runtime.onEvent(&event.EventEntityAppear{
		EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "fragment"},
		RaceId:    7604, OwnerId: "boss",
	})
	for _, atMs := range []int64{100_000, 100_100} {
		runtime.onEvent(&event.EventSkillAction{
			EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: atMs / 1000, Id: "fragment"},
			SkillId:   52402, AtMs: atMs, SourceId: "fragment", IsFallback: true, IsLocal: false,
		})
	}
	if len(sounds) != 1 || sounds[0].Kind != "boss-miel-orb" || sounds[0].Volume != 85 {
		t.Fatalf("native Boss sounds = %#v, want one dedicated orb request", sounds)
	}
	state := runtime.bossMechanics["miel-orb"]
	if state.StartedAtMs != 100_000 || state.EndsAtMs != 120_000 || state.Generation != 1 {
		t.Fatalf("native orb timeline = %#v, want immediate 20-second generation", state)
	}
}

func TestNativeReminderToahThresholdAndFullChargeRefresh(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Volume: 65,
		Rules: map[uint16]nativeSkillCooldownRule{
			27012: {SkillID: 27012, Enabled: true, SoundMode: "default", ProgressThresholdPercent: 95},
			1234:  {SkillID: 1234, Enabled: true, CooldownSeconds: 30, SoundMode: "default"},
		},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.localID = "player"
	runtime.observeSkillCooldown(1234, 100_000, false)
	runtime.observeToahProgress(94, 101_000)
	runtime.observeToahProgress(95, 102_000)
	runtime.observeToahProgress(100, 103_000)
	if len(sounds) != 1 || sounds[0].Kind != "skill-ready" {
		t.Fatalf("Toah threshold sound = %#v, want one skill-ready request", sounds)
	}
	if cooldown := runtime.skillCooldowns[1234]; cooldown.ReadyAtMs != 103_000 {
		t.Fatalf("Toah refreshed readyAt = %d, want 103000", cooldown.ReadyAtMs)
	}
	runtime.evaluate(time.UnixMilli(103_100))
	if len(sounds) != 1 {
		t.Fatalf("Toah refresh emitted duplicate skill sound: %#v", sounds)
	}
}

func TestNativeReminderToahDoesNotRefreshPetSkill(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Rules: map[uint16]nativeSkillCooldownRule{
			27012: {SkillID: 27012, Enabled: true, ProgressThresholdPercent: 95},
			4321:  {SkillID: 4321, Enabled: true, CooldownSeconds: 30, OwnerMode: "auto"},
		},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.localID = "player"
	runtime.entities["pet"] = &nativeReminderEntity{ID: "pet", OwnerID: "player"}
	runtime.onEvent(&event.EventSkillAction{
		EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: 100, Id: "player"},
		SkillId:   4321, AtMs: 100_000, SourceId: "pet", IsLocal: true,
	})
	before := runtime.skillCooldowns[4321]
	if !before.PetSkill {
		t.Fatal("owned-pet action was not classified as a pet skill")
	}
	runtime.observeToahProgress(100, 103_000)
	after := runtime.skillCooldowns[4321]
	if after.ReadyAtMs != before.ReadyAtMs {
		t.Fatalf("Toah changed pet readyAt from %d to %d", before.ReadyAtMs, after.ReadyAtMs)
	}
}

func TestNativeReminderDebuffBossUsesResolvedRaceName(t *testing.T) {
	settings := nativeReminderSettings{BossRaceNames: map[uint32]string{7603: "雷内恩的米耶尔"}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	boss := &nativeReminderEntity{ID: "123456", Name: "123456", RaceID: 7603}
	if got := runtime.nativeBossDisplayName(boss); got != "雷内恩的米耶尔" {
		t.Fatalf("Boss display name = %q", got)
	}
}

func TestNativeReminderSelectedShardPublishesTrueHealth(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{
		Miel60HealthBarX: 640, Miel60HealthBarY: 180, Miel60HealthBarScale: 135,
	}}, &sounds)
	runtime.onEvent(&event.EventLocalEntity{
		EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 100, Id: "player"}, Reliable: true,
	})
	runtime.onEvent(&event.EventEntityAppear{
		EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 99, Id: "boss"},
		RaceId:    7603, Name: "Miel",
	})
	runtime.onEvent(&event.EventStatUpdate{
		EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 99, Id: "boss"},
		Stats:     []event.EventStatUpdateEntry{{StatId: 28, Value: 600_000_000}, {StatId: 30, Value: 1_000_000_000}},
	})
	runtime.onEvent(&event.EventEntityAppear{
		EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "4767482431088776"},
		RaceId:    7604, Name: "4767482431088776", OwnerId: "boss",
	})
	runtime.onEvent(&event.EventStatUpdate{
		EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 101, Id: "4767482431088776"},
		Stats:     []event.EventStatUpdateEntry{{StatId: 28, Value: 38_281_896}, {StatId: 30, Value: 63_803_160}},
	})
	runtime.onEvent(&event.EventCombatTarget{
		EventBase: event.EventBase{EventId: event.EventIdCombatTarget, At: 102, Id: "player"},
		AtMs:      102_000, TargetId: "4767482431088776",
	})

	target := runtime.selectedTargetHealth()
	if target == nil {
		t.Fatal("selected shard health is missing")
	}
	if target.Name != "安乐碎片" || target.CurrentHealth != 38_281_896 || target.MaximumHealth != 63_803_160 {
		t.Fatalf("selected shard health = %#v", target)
	}
	if target.X != 640 || target.Y != 180 || target.ScalePercent != 135 {
		t.Fatalf("selected shard presentation = %#v", target)
	}
	if target.PhaseLabel != "普通 60%" || target.OpacityPercent != 100 {
		t.Fatalf("selected shard phase presentation = %#v", target)
	}

	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: event.EventIdEntityDisappear, At: 103, Id: "4767482431088776"}})
	if runtime.selectedTargetID != "" || runtime.selectedTargetHealth() != nil {
		t.Fatal("disappeared selected shard remained visible")
	}
}

func TestNativeReminderSelectsConfiguredMielShardPhasesAndEndsAtZeroHealth(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{
		MielShardHealthPhases: &nativeMielShardHealthPhases{Normal80: true, Normal60: true, Normal40: true, Regret80: true},
	}}, &sounds)
	runtime.entities["boss"] = &nativeReminderEntity{ID: "boss", RaceID: 7603, Known: true, Active: true, CurrentHealth: 800, MaximumHealth: 1000}
	shard := &nativeReminderEntity{ID: "shard", OwnerID: "boss", RaceID: 7604, Known: true, Active: true, CurrentHealth: 38_281_896, MaximumHealth: 38_281_896}
	runtime.entities[shard.ID] = shard
	runtime.selectedTargetID = shard.ID

	if target := runtime.selectedTargetHealth(); target == nil || target.PhaseLabel != "普通 80%" {
		t.Fatalf("80%% shard health = %#v", target)
	}
	runtime.entities["boss"].CurrentHealth = 600
	if target := runtime.selectedTargetHealth(); target == nil || target.PhaseLabel != "普通 60%" {
		t.Fatalf("60%% shard health = %#v", target)
	}
	runtime.entities["boss"].CurrentHealth = 400
	if target := runtime.selectedTargetHealth(); target == nil || target.PhaseLabel != "普通 40%" {
		t.Fatalf("40%% shard health = %#v", target)
	}
	runtime.entities["boss"].RaceID = 7615
	runtime.entities["boss"].CurrentHealth = 800
	shard.RaceID = 7616
	if target := runtime.selectedTargetHealth(); target == nil || target.PhaseLabel != "悔恨 80%" {
		t.Fatalf("Regret 80%% shard health = %#v", target)
	}
	shard.CurrentHealth = 0
	if target := runtime.selectedTargetHealth(); target != nil {
		t.Fatalf("defeated shard health = %#v, want nil", target)
	}
}

func TestNativeReminderMiel60HealthBarCanBeDisabled(t *testing.T) {
	enabled := false
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{
		BossMechanics: nativeBossMechanicSettings{Miel60HealthBarEnabled: &enabled},
	}, &sounds)
	runtime.entities["boss"] = &nativeReminderEntity{
		ID: "boss", RaceID: 7603, Known: true, Active: true, CurrentHealth: 600, MaximumHealth: 1000,
	}
	runtime.entities["shard"] = &nativeReminderEntity{
		ID: "shard", OwnerID: "boss", RaceID: 7604, Known: true, Active: true,
		CurrentHealth: nativeMiel60ShardMaximumHealth, MaximumHealth: nativeMiel60ShardMaximumHealth,
	}
	runtime.selectedTargetID = "shard"

	if target := runtime.selectedTargetHealth(); target != nil {
		t.Fatalf("disabled 60%% shard health = %#v, want nil", target)
	}
}

func TestNativeReminderEffectTimersTrackSkillAndConditionSources(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{EffectTimers: nativeEffectTimerSettings{Rules: map[string]nativeEffectTimerRule{
		"self-buff":  {Enabled: true, SourceType: "condition", SourceID: 123, DurationSeconds: 7.5, TargetMode: "self"},
		"boss-skill": {Enabled: true, SourceType: "skill", SourceID: 52401, DurationSeconds: 5, TargetMode: "monster"},
	}}}, &sounds)
	runtime.localID = "player"
	runtime.entities["player"] = &nativeReminderEntity{ID: "player", Known: true, Active: true, Conditions: map[uint32]nativeReminderCondition{}, RefreshGuard: map[uint32]int64{}}
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "player"}, CCId: 123,
	})
	selfTimer, exists := runtime.effectTimers["self-buff"]
	if !exists || selfTimer.StartedAtMs != 100_000 || selfTimer.EndsAtMs != 107_500 {
		t.Fatalf("self condition timer = %#v", selfTimer)
	}
	runtime.onEvent(&event.EventCharacterConditionDisable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionDisable, At: 101, Id: "player"}, CCId: 123,
	})
	if _, exists := runtime.effectTimers["self-buff"]; exists {
		t.Fatal("disabled condition kept its timer active")
	}
	runtime.onEvent(&event.EventSkillAction{
		EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: 110, Id: "boss"},
		SkillId:   52401, AtMs: 110_250, SourceId: "boss", IsLocal: false,
	})
	bossTimer, exists := runtime.effectTimers["boss-skill"]
	if !exists || bossTimer.StartedAtMs != 110_250 || bossTimer.EndsAtMs != 115_250 {
		t.Fatalf("monster skill timer = %#v", bossTimer)
	}
}

func TestNativeReminderSelectedPlayerDoesNotPublishHealth(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{}, &sounds)
	runtime.entities["player"] = &nativeReminderEntity{ID: "player", RaceID: 10001, Known: true, Active: true, CurrentHealth: 100, MaximumHealth: 100}
	runtime.selectedTargetID = "player"
	if target := runtime.selectedTargetHealth(); target != nil {
		t.Fatalf("player target health = %#v, want nil", target)
	}
}

func TestNativeReminderBuffSoundDoesNotNeedWebViewTicks(t *testing.T) {
	settings := nativeReminderSettings{Buff: nativeBuffSettings{
		Volume: 80,
		Rules: map[uint32]nativeBuffRule{511: {
			CCID: 511, SoundMode: "voice", SoundThresholdSeconds: 5,
			DurationMode: "auto", FlashThresholdSeconds: 10,
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.preferredBossID = settings.PreferredBossID
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 100, Id: "player"}, Reliable: true})
	runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "player"}, RaceId: 10001, Name: "Player"})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "player"},
		CCId:      511, DisableAtMs: 110_000, DurationMs: 10_000,
	})

	runtime.evaluate(time.UnixMilli(104_900))
	if len(sounds) != 0 {
		t.Fatalf("sound fired before threshold: %#v", sounds)
	}
	runtime.evaluate(time.UnixMilli(105_100))
	runtime.evaluate(time.UnixMilli(106_000))
	if len(sounds) != 1 || sounds[0].Kind != "voice" || sounds[0].Volume != 80 {
		t.Fatalf("native Buff sound = %#v, want one voice request", sounds)
	}
}

func TestNativeReminderBossDebuffStackCrossingTriggersScreenAndSound(t *testing.T) {
	settings := nativeReminderSettings{Buff: nativeBuffSettings{Volume: 72}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	rule := nativeBuffRule{
		CCID: 1098, StackAlertEnabled: true, StackScreenEnabled: true,
		StackThreshold: 3, StackSoundMode: "voice",
	}

	runtime.evaluateBuffStack(rule, nativeReminderCondition{Metadata: "MCSTCT:2:2;"}, 100_000)
	if len(sounds) != 0 || len(runtime.buffStackAlerts) != 0 {
		t.Fatalf("stack below threshold triggered an alert: sounds=%#v alerts=%#v", sounds, runtime.buffStackAlerts)
	}
	runtime.evaluateBuffStack(rule, nativeReminderCondition{Metadata: "MCSTCT:2:3;"}, 101_000)
	alert := runtime.buffStackAlerts[1098]
	if len(sounds) != 1 || sounds[0].Kind != "voice" || sounds[0].Volume != 72 || alert.Stack != 3 || alert.Generation != 1 {
		t.Fatalf("threshold crossing did not trigger one screen-and-sound alert: sounds=%#v alert=%#v", sounds, alert)
	}
	runtime.evaluateBuffStack(rule, nativeReminderCondition{Metadata: "MCSTCT:2:4;"}, 102_000)
	if len(sounds) != 1 || runtime.buffStackAlerts[1098].Generation != 1 {
		t.Fatal("remaining above the selected stack threshold repeated the alert")
	}
	runtime.evaluateBuffStack(rule, nativeReminderCondition{Metadata: "MCSTCT:2:1;"}, 103_000)
	runtime.evaluateBuffStack(rule, nativeReminderCondition{Metadata: "MCSTCT:2:3;"}, 104_000)
	if len(sounds) != 2 || runtime.buffStackAlerts[1098].Generation != 2 {
		t.Fatal("dropping below and crossing the stack threshold again did not re-arm the alert")
	}
}

func TestNativeReminderStackPromptCoordinatesAreNormalized(t *testing.T) {
	settings := normalizeNativeReminderSettings(nativeReminderSettings{Buff: nativeBuffSettings{
		Rules: map[uint32]nativeBuffRule{
			1080: {StackX: 1260, StackY: 740},
			1098: {StackX: 40000, StackY: -40000},
		},
	}})
	feather := settings.Buff.Rules[1080]
	if feather.StackX != 1260 || feather.StackY != 740 {
		t.Fatalf("custom stack prompt coordinates = (%d, %d), want (1260, 740)", feather.StackX, feather.StackY)
	}
	debuff := settings.Buff.Rules[1098]
	if debuff.StackX != 850 || debuff.StackY != 280 {
		t.Fatalf("invalid stack prompt coordinates = (%d, %d), want defaults (850, 280)", debuff.StackX, debuff.StackY)
	}
}

func TestNativeReminderBuffSnapshotDoesNotReplaySoundAfterMapTransition(t *testing.T) {
	settings := nativeReminderSettings{Buff: nativeBuffSettings{
		Volume: 80,
		Rules: map[uint32]nativeBuffRule{192: {
			CCID: 192, SoundMode: "voice", SoundThresholdSeconds: 120,
			DurationMode: "auto", FlashThresholdSeconds: 120,
		}},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 100, Id: "player"}, Reliable: true})
	runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "player"}, RaceId: 10001, Name: "Player"})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "player"},
		CCId:      192, DisableAtMs: 200_000, DurationMs: 630_000,
	})
	runtime.evaluate(time.UnixMilli(100_100))
	if len(sounds) != 1 {
		t.Fatalf("initial Buff sound count = %d, want 1", len(sounds))
	}

	// A channel/map rebuild clears active entities and then republishes the
	// same server Buff snapshot with a new packet timestamp and tiny expiry drift.
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 150, Id: "0"}, Reset: true})
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 150, Id: "player"}, Reliable: true})
	runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 150, Id: "player"}, RaceId: 10001, Name: "Player"})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 150, Id: "player"},
		CCId:      192, DisableAtMs: 200_017, DurationMs: 630_000,
	})
	runtime.evaluate(time.UnixMilli(150_100))
	if len(sounds) != 1 {
		t.Fatalf("map snapshot replayed Buff sound: %#v", sounds)
	}

	runtime.onEvent(&event.EventCharacterConditionDisable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionDisable, At: 210, Id: "player"}, CCId: 192,
	})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 210, Id: "player"},
		CCId:      192, DisableAtMs: 310_000, DurationMs: 100_000,
	})
	runtime.evaluate(time.UnixMilli(210_100))
	if len(sounds) != 2 {
		t.Fatalf("a genuinely new Buff did not re-arm sound: %#v", sounds)
	}
}

func TestNativeReminderDebuffExpiryAndMissingSounds(t *testing.T) {
	settings := nativeReminderSettings{
		PreferredBossID: "boss",
		Debuff: nativeDebuffSettings{
			Volume: 75, OverlayEnabled: true, IconSize: 30,
			Rules: map[uint32]nativeDebuffRule{392: {
				CCID: 392, Enabled: true, WarningSeconds: 5, FlashEnabled: true,
				SoundEnabled: true, SoundMode: "electronic",
			}},
		},
	}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.preferredBossID = settings.PreferredBossID
	runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "boss"}, RaceId: 7603, Name: "Boss"})
	runtime.onEvent(&event.EventStatUpdate{
		EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 100, Id: "boss"},
		Stats:     []event.EventStatUpdateEntry{{StatId: 30, Value: 200_000_000}},
	})
	// CC 504 is equivalent to configured CC 392.
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "boss"},
		CCId:      504, DisableAtMs: 120_000,
	})
	runtime.evaluate(time.UnixMilli(114_900))
	if len(sounds) != 0 {
		t.Fatalf("Debuff sound fired before threshold: %#v", sounds)
	}
	runtime.evaluate(time.UnixMilli(115_100))
	if len(sounds) != 1 {
		t.Fatalf("expiry sound count = %d, want 1", len(sounds))
	}

	// A new application clears the previous announced generation. Its disable
	// must sound after the short packet-replacement debounce without a renderer.
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 130, Id: "boss"},
		CCId:      504,
	})
	runtime.evaluate(time.UnixMilli(130_100))
	runtime.debuffSoundBlockedUntil = time.Time{}
	runtime.onEvent(&event.EventCharacterConditionDisable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionDisable, At: 132, Id: "boss"},
		CCId:      504,
	})
	runtime.evaluate(time.UnixMilli(132_100))
	if len(sounds) != 1 {
		t.Fatalf("missing Debuff sounded before debounce: %#v", sounds)
	}
	runtime.evaluate(time.UnixMilli(133_400))
	if len(sounds) != 2 {
		t.Fatalf("missing Debuff sound count = %d, want 2", len(sounds))
	}
}

func TestNativeReminderEquivalentDebuffReplacementDoesNotSoundMissing(t *testing.T) {
	settings := nativeReminderSettings{
		PreferredBossID: "boss",
		Debuff: nativeDebuffSettings{
			Volume: 75, OverlayEnabled: true,
			Rules: map[uint32]nativeDebuffRule{912: {
				CCID: 912, Enabled: true, WarningSeconds: 20, FlashEnabled: true,
				SoundEnabled: true, SoundMode: "electronic",
			}},
		},
	}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.preferredBossID = settings.PreferredBossID
	runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "boss"}, RaceId: 7603, Name: "Boss"})
	runtime.onEvent(&event.EventStatUpdate{
		EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 100, Id: "boss"},
		Stats:     []event.EventStatUpdateEntry{{StatId: 30, Value: 200_000_000}},
	})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "boss"},
		CCId:      912, DisableAtMs: 300_000,
	})
	runtime.evaluate(time.UnixMilli(100_100))
	runtime.onEvent(&event.EventCharacterConditionDisable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionDisable, At: 150, Id: "boss"}, CCId: 912,
	})
	runtime.evaluate(time.UnixMilli(150_100))
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 150, Id: "boss"},
		CCId:      913, DisableAtMs: 750_000,
	})
	runtime.evaluate(time.UnixMilli(150_400))
	if len(sounds) != 0 {
		t.Fatalf("912-to-913 replacement sounded missing: %#v", sounds)
	}
}

func TestNativeReminderBossDebuffWinsOverShortOwnedMechanicCondition(t *testing.T) {
	settings := nativeReminderSettings{
		PreferredBossID: "boss",
		Debuff: nativeDebuffSettings{
			Volume: 75, OverlayEnabled: true,
			Rules: map[uint32]nativeDebuffRule{1165: {
				CCID: 1165, Enabled: true, WarningSeconds: 10, FlashEnabled: true,
				SoundEnabled: true, SoundMode: "electronic",
			}},
		},
	}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.preferredBossID = settings.PreferredBossID
	runtime.onEvent(&event.EventEntityAppear{
		EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 100, Id: "boss"},
		RaceId:    7603, Name: "Boss",
	})
	runtime.onEvent(&event.EventStatUpdate{
		EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 100, Id: "boss"},
		Stats:     []event.EventStatUpdateEntry{{StatId: 30, Value: 200_000_000}},
	})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "boss"},
		CCId:      1165, DisableAtMs: 300_000,
	})
	runtime.onEvent(&event.EventEntityAppear{
		EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 200, Id: "orb"},
		RaceId:    7604, Name: "Orb", OwnerId: "boss",
	})
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 200, Id: "orb"},
		CCId:      1165, DisableAtMs: 210_000,
	})

	condition, active := runtime.findBossDebuffCondition(runtime.entities["boss"], 1165)
	if !active || condition.DisableAtMs != 300_000 {
		t.Fatalf("selected condition = %#v, active=%v; want direct Boss expiry 300000", condition, active)
	}
	runtime.evaluate(time.UnixMilli(200_100))
	if len(sounds) != 0 {
		t.Fatalf("owned mechanic condition produced false 10-second warning: %#v", sounds)
	}
}

func TestNativeReminderPreferredBossWins(t *testing.T) {
	settings := nativeReminderSettings{
		PreferredBossID: "preferred",
		Debuff:          nativeDebuffSettings{Rules: map[uint32]nativeDebuffRule{}},
	}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.preferredBossID = settings.PreferredBossID
	for _, id := range []string{"preferred", "larger"} {
		runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 1, Id: id}, RaceId: 7603})
	}
	runtime.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 1, Id: "preferred"}, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 150_000_000}}})
	runtime.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 1, Id: "larger"}, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 500_000_000}}})
	if selected := runtime.selectDebuffBoss(); selected == nil || selected.ID != "preferred" {
		t.Fatalf("selected boss = %#v, want preferred", selected)
	}
}

func TestNativeReminderAutomaticFallbackStaysSelected(t *testing.T) {
	settings := nativeReminderSettings{
		PreferredBossID: "original",
		Debuff:          nativeDebuffSettings{Rules: map[uint32]nativeDebuffRule{}},
	}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.preferredBossID = settings.PreferredBossID
	for _, id := range []string{"original", "fallback"} {
		runtime.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{EventId: event.EventIdEntityAppear, At: 1, Id: id}, RaceId: 7603})
	}
	runtime.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 1, Id: "original"}, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 600_000_000}}})
	runtime.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: 1, Id: "fallback"}, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 200_000_000}}})

	runtime.activeBossID = "original"
	runtime.entities["original"].Active = false
	selected := runtime.selectDebuffBoss()
	if selected == nil || selected.ID != "fallback" {
		t.Fatalf("fallback selection = %#v, want fallback", selected)
	}
	runtime.activeBossID = selected.ID
	runtime.entities["original"].Active = true
	if selected = runtime.selectDebuffBoss(); selected == nil || selected.ID != "fallback" {
		t.Fatalf("returning old target stole selection: %#v", selected)
	}
}

func TestNativeMagnumAimAppliesBuffPriorityAndMultiplication(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		AimReminder: nativeAimReminderSettings{Enabled: true, WeaponRange: 2200, CalibrationPercent: 40, ErgSpeedPercent: 200, ScalePercent: 100, X: 600, Y: 180},
		Rules:       map[uint16]nativeSkillCooldownRule{},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.onEvent(&event.EventLocalEntity{
		EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 100, Id: "player"}, Reliable: true,
	})
	for _, ccID := range []uint32{latikaSecretCCID, rapidAimCCID} {
		runtime.onEvent(&event.EventCharacterConditionEnable{
			EventBase: event.EventBase{EventId: event.EventIdCharacterConditionEnable, At: 100, Id: "player"},
			CCId:      ccID, DisableAtMs: 1_000_000,
		})
	}
	runtime.onEvent(&event.EventSkillState{
		EventBase: event.EventBase{EventId: event.EventIdSkillState, At: 100, Id: "player"},
		AtMs:      100_000, SkillId: finalShotSkillID, Scope: "active", Active: true,
	})
	runtime.onEvent(&event.EventSkillState{
		EventBase: event.EventBase{EventId: event.EventIdSkillState, At: 100, Id: "player"},
		AtMs:      100_000, SkillId: magnumShotSkillID, Scope: "aim", Active: true, TargetId: "enemy",
	})
	aim := runtime.magnumAim
	if !aim.Active || aim.SpeedMultiplier != 19.2 || aim.ReadyAtMs-aim.StartedAtMs != 20 {
		t.Fatalf("combined aim = %#v, want active Erg × temporary 19.2x and 20ms", aim)
	}
	if len(aim.BuffNames) != 3 || aim.BuffNames[0] != "弓尔格 200%" || aim.BuffNames[1] != "无影箭" || aim.BuffNames[2] != "拉蒂卡秘术" {
		t.Fatalf("effective buffs = %#v", aim.BuffNames)
	}
	if aim.CalibrationPercent != 40 {
		t.Fatalf("calibration = %v, want 40", aim.CalibrationPercent)
	}
	runtime.onEvent(&event.EventSkillState{
		EventBase: event.EventBase{EventId: event.EventIdSkillState, At: 100, Id: "player"},
		AtMs:      aim.ReadyAtMs, SkillId: magnumShotSkillID, Scope: "aim", Active: false,
	})
	if runtime.magnumAim.Active {
		t.Fatal("shooting at the 85% best-shot point did not hide the progress runtime")
	}
}

func TestNativeMagnumAimUsesCalibrationErgAndFineTune(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		AimReminder: nativeAimReminderSettings{
			Enabled: true, WeaponRange: 1600, CalibrationPercent: 20, ErgSpeedPercent: 100, FineTuneSeconds: .5,
			ScalePercent: 100, X: 600, Y: 180,
		},
		Rules: map[uint16]nativeSkillCooldownRule{},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.onEvent(&event.EventLocalEntity{
		EventBase: event.EventBase{EventId: event.EventIdLocalEntity, At: 100, Id: "player"}, Reliable: true,
	})
	runtime.onEvent(&event.EventSkillState{
		EventBase: event.EventBase{EventId: event.EventIdSkillState, At: 100, Id: "player"},
		AtMs:      100_000, SkillId: magnumShotSkillID, Scope: "aim", Active: true, TargetId: "enemy",
	})
	aim := runtime.magnumAim
	if aim.ReadyAtMs-aim.StartedAtMs != 1844 {
		t.Fatalf("custom aim duration = %dms, want 1844ms", aim.ReadyAtMs-aim.StartedAtMs)
	}
	if aim.SpeedMultiplier != 1 || len(aim.BuffNames) != 1 || aim.BuffNames[0] != "弓尔格 100%" {
		t.Fatalf("custom effective aim state = %#v", aim)
	}
}

func TestNativeMagnumAimDisplayProgressUsesCalibratedSquare(t *testing.T) {
	const startedAtMs int64 = 1000
	const bestAtMs int64 = 1188
	if got := nativeMagnumAimDisplayProgress(startedAtMs, bestAtMs, startedAtMs, 40); got != .4 {
		t.Fatalf("starting progress = %v, want 0.4", got)
	}
	if got := nativeMagnumAimDisplayProgress(startedAtMs, bestAtMs, bestAtMs, 40); math.Abs(got-.85) > 1e-9 {
		t.Fatalf("best progress = %v, want 0.85", got)
	}
	if got := nativeMagnumAimDisplayProgress(startedAtMs, bestAtMs, startedAtMs+188*4, 40); got != 1 {
		t.Fatalf("full progress = %v, want 1", got)
	}
}

func TestNativeMagnumAimSpeedMatrix(t *testing.T) {
	tests := []struct {
		name                 string
		final, latika, rapid bool
		want                 float64
	}{
		{"base", false, false, false, 1},
		{"rapid", false, false, true, 2},
		{"final", true, false, false, 2.4},
		{"latika", false, true, false, 4},
		{"final beats rapid", true, false, true, 2.4},
		{"latika beats rapid", false, true, true, 4},
		{"final multiplies latika", true, true, true, 9.6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := nativeMagnumAimSpeed(test.final, test.latika, test.rapid)
			if got != test.want {
				t.Fatalf("multiplier = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNativeMagnumAimCanStayVisibleWhileIdle(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		AimReminder: nativeAimReminderSettings{
			Enabled: true, AlwaysVisible: true, CalibrationPercent: 40, ErgSpeedPercent: 200, ScalePercent: 100, X: 600, Y: 180,
		},
		Rules: map[uint16]nativeSkillCooldownRule{},
	}}
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(settings, &sounds)
	runtime.publishNativeSkillState(time.UnixMilli(100_000))

	skillOverlayState.RLock()
	data := append([]byte(nil), skillOverlayState.data...)
	skillOverlayState.RUnlock()
	var message nativeSkillOverlayMessage
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("decode skill overlay: %v", err)
	}
	if message.AimReminder == nil || message.AimReminder.Active || !message.AimReminder.AlwaysVisible {
		t.Fatalf("idle aim reminder = %#v, want inactive always-visible item", message.AimReminder)
	}
	if message.AimReminder.SpeedMultiplier != 1 || len(message.AimReminder.BuffNames) != 0 {
		t.Fatalf("idle aim state = %#v, want zero-progress 1x state", message.AimReminder)
	}
	if message.AimReminder.CalibrationPercent != 40 {
		t.Fatalf("idle aim calibration = %v, want 40", message.AimReminder.CalibrationPercent)
	}
}
