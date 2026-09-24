package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestHealerReplaySeparateDeathVoice(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_DEATH_VOICE_LOG")
	if path == "" {
		t.Skip("set DILMETER_HEALER_DEATH_VOICE_LOG to the 2026-09-23 event log")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	runtime, _ := healerFixture(t)
	member := normalizeHealerMember(healerMemberSelection{Name: "栗艾", Favorite: true, Overture: true}, runtime.healer.settings, 0)
	member.BuffSettings.Rules[0].WarningSeconds = 25
	runtime.healer.settings.Members = []healerMemberSelection{member}
	const start, end = int64(1790172415_000), int64(1790174160_000)
	clock := start
	var played []int64
	runtime.playSound = func(request nativeReminderSoundRequest) bool {
		if request.Kind == "healer-death" {
			played = append(played, clock)
		}
		return true
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var base event.EventBase
		if err := json.Unmarshal(scanner.Bytes(), &base); err != nil {
			t.Fatal(err)
		}
		at := base.At * 1000
		if at > end {
			break
		}
		for clock < at {
			healerTick(runtime, clock)
			clock += 250
		}
		var current event.IEvent = &base
		switch base.EventId {
		case 1:
			current = &event.EventEntityAppear{}
		case 2:
			current = &event.EventEntityDisappear{}
		case 4:
			current = &event.EventCharacterConditionEnable{}
		case 5:
			current = &event.EventCharacterConditionDisable{}
		case 6:
			current = &event.EventFinish{}
		case 11:
			current = &event.EventLocalEntity{}
		case 17:
			current = &event.EventStatUpdate{}
		}
		if err := json.Unmarshal(scanner.Bytes(), current); err != nil {
			t.Fatal(err)
		}
		runtime.onEvent(current)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for clock <= end {
		healerTick(runtime, clock)
		clock += 250
	}
	wanted := []int64{1790172420_000, 1790172466_000, 1790173399_000, 1790174147_000}
	if len(played) != len(wanted) {
		t.Fatalf("want one death voice per early loss, got %v", played)
	}
	for i, at := range played {
		if at < wanted[i] || at > wanted[i]+4000 {
			t.Fatalf("late/wrong death voice at %d", at)
		}
		t.Logf("Xiaoxiao death voice at %s", time.UnixMilli(at).In(time.FixedZone("HKT", 8*3600)).Format("15:04:05.000"))
	}
}

func deathReminderFixture(t *testing.T) (*nativeReminderRuntime, *[]nativeReminderSoundRequest) {
	runtime, sounds := healerFixture(t)
	member := normalizeHealerMember(healerMemberSelection{ID: "ally", Name: "队友", Overture: true}, runtime.healer.settings, 0)
	member.BuffSettings.Rules[0].WarningSeconds = 25
	runtime.healer.settings.Members = []healerMemberSelection{member}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 100}, CCId: 680, DisableAt: 700})
	return runtime, sounds
}

func TestHealerDeathReminderWaitsForDelayedDeathAndDoesNotRepeatOnRevival(t *testing.T) {
	runtime, sounds := deathReminderFixture(t)
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 110}, CCId: 680})
	healerTick(runtime, 110000)
	healerTick(runtime, 111500)
	if len(*sounds) != 0 {
		t.Fatal("normal sound played before death could be correlated")
	}
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 112}})
	healerTick(runtime, 112000)
	healerTick(runtime, 113500)
	if len(*sounds) != 1 || (*sounds)[0].Kind != "healer-death" {
		t.Fatalf("wrong death sound: %v", *sounds)
	}
	if len(runtime.healer.state.Alerts) != 1 || !strings.Contains(runtime.healer.state.Alerts[0].Message, "因死亡消失") || runtime.healer.state.Cards[0].Value != "死亡丢失" {
		t.Fatal("death reason absent from overlay")
	}
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 114}})
	runtime.onEvent(healerHP("ally", 115, 800, 1000))
	healerTick(runtime, 115000)
	healerTick(runtime, 120000)
	if len(*sounds) != 1 {
		t.Fatal("duplicate death/revival replayed sound")
	}
}

func TestHealerDeathReminderIndependentSoundCountAndInterval(t *testing.T) {
	runtime, sounds := deathReminderFixture(t)
	rule := &runtime.healer.settings.Members[0].BuffSettings.Rules[0]
	rule.Sound = &healerSound{Kind: "none"}
	rule.DeathLoss.Sound = healerSound{Kind: "custom", SoundID: "death-choice"}
	rule.DeathLoss.RepeatCount, rule.DeathLoss.RepeatIntervalSeconds = 2, 7
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 110}})
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 111}, CCId: 680})
	healerTick(runtime, 111000)
	healerTick(runtime, 112500)
	healerTick(runtime, 119000)
	if len(*sounds) != 1 {
		t.Fatal("repeat interval ignored")
	}
	healerTick(runtime, 119500)
	healerTick(runtime, 127000)
	if len(*sounds) != 2 || (*sounds)[0].SoundID != "death-choice" {
		t.Fatalf("independent sound/count ignored: %v", *sounds)
	}
	// Idle gaps and revival preserve this separate quota too.
	healerTick(runtime, 145000)
	runtime.onEvent(healerHP("ally", 146, 800, 1000))
	healerTick(runtime, 146000)
	healerTick(runtime, 149000)
	if len(*sounds) != 2 {
		t.Fatal("idle/revival reset death quota")
	}
}

func TestHealerDeathReminderDisabledDoesNotUseNormalVoice(t *testing.T) {
	runtime, sounds := deathReminderFixture(t)
	runtime.healer.settings.Members[0].BuffSettings.Rules[0].DeathLoss.Enabled = false
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 110}})
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 110}, CCId: 680})
	healerTick(runtime, 110000)
	healerTick(runtime, 112000)
	healerTick(runtime, 120000)
	if len(*sounds) != 0 || len(runtime.healer.state.Cards) != 1 {
		t.Fatal("disabled death audio fell back to normal sound or hid missing icon")
	}
}

func TestHealerNaturalExpiryAndUnrelatedRemovalAreNotDeathLoss(t *testing.T) {
	for _, test := range []struct {
		name                   string
		expiry, death, removal int64
	}{
		{"natural-at-death", 110, 110, 110}, {"unrelated-later", 700, 110, 120},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, sounds := deathReminderFixture(t)
			runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 101}, CCId: 680, DisableAt: test.expiry})
			runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: test.death}})
			runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: test.removal}, CCId: 680})
			healerTick(runtime, test.removal*1000)
			healerTick(runtime, (test.removal+4)*1000)
			if len(*sounds) != 1 || (*sounds)[0].Kind != "healer-music" || runtime.healer.state.Members[0].Overture.LossReason != "" {
				t.Fatal("ordinary expiry/removal incorrectly classified as death")
			}
		})
	}
}

func TestHealerDeathReasonUsesServerExpiryInsteadOfManualCountdown(t *testing.T) {
	runtime, sounds := deathReminderFixture(t)
	rule := &runtime.healer.settings.Members[0].BuffSettings.Rules[0]
	rule.DurationMode, rule.ManualDurationSeconds = "manual", 5
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 110}})
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 110}, CCId: 680})
	healerTick(runtime, 110000)
	healerTick(runtime, 111250)
	if len(*sounds) != 1 || (*sounds)[0].Kind != "healer-death" {
		t.Fatal("manual estimate hid a confirmed early server Buff loss")
	}
}

func TestHealerDeathReminderFreshGrantRearmsAndStaleRemovalDoesNot(t *testing.T) {
	runtime, sounds := deathReminderFixture(t)
	for _, at := range []int64{110, 130} {
		if at == 130 {
			runtime.onEvent(healerHP("ally", 125, 800, 1000))
			runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 125}, CCId: 680, DisableAt: 800})
		}
		runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: at}})
		runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: at}, CCId: 680})
		healerTick(runtime, at*1000)
		healerTick(runtime, at*1000+1250)
	}
	if len(*sounds) != 2 {
		t.Fatal("new grant/death did not rearm")
	}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 140}, CCId: 680, DisableAt: 900})
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 141}, CCId: 680, DisableAt: 901})
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 141}})
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 141}, CCId: 680})
	healerTick(runtime, 141000)
	healerTick(runtime, 143000)
	if len(*sounds) != 2 || runtime.healer.state.Members[0].Overture.State != "active" {
		t.Fatal("guarded stale removal invented death loss")
	}
}

func TestHealerDeathReminderMigrationAndTemplatePersistence(t *testing.T) {
	runtime, _ := deathReminderFixture(t)
	rule := runtime.healer.settings.Members[0].BuffSettings.Rules[0]
	if !rule.DeathLoss.Enabled || rule.DeathLoss.Sound.Kind != "healer-death" || rule.DeathLoss.RepeatCount != 1 {
		t.Fatal("old rule did not get independent defaults")
	}
	rule.DeathLoss = &healerDeathLossSettings{Sound: healerSound{Kind: "custom", SoundID: "saved-death"}, RepeatCount: 3, RepeatIntervalSeconds: 9}
	member := runtime.healer.settings.Members[0]
	member.BuffSettings.Rules = []healerBuffRule{rule}
	s := runtime.healer.settings
	s.Members = []healerMemberSelection{member}
	s.Templates = []healerMemberTemplate{{ID: "template", Name: "队友1", BuffSettings: member.BuffSettings}}
	data, _ := json.Marshal(s)
	var loaded healerSettings
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded = normalizeHealerSettings(loaded)
	for _, buffs := range []*healerMemberBuffSettings{loaded.Members[0].BuffSettings, loaded.Templates[0].BuffSettings} {
		death := buffs.Rules[0].DeathLoss
		if death.Enabled || death.Sound.SoundID != "saved-death" || death.RepeatCount != 3 || death.RepeatIntervalSeconds != 9 {
			t.Fatalf("settings lost: %+v", death)
		}
	}
}
