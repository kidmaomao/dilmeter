package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestBurstSelfPrivateIdentityWithoutAppearance(t *testing.T) {
	for _, source := range []string{"execute", "public-release", "condition"} {
		t.Run(source, func(t *testing.T) {
			sounds := []nativeReminderSoundRequest{}
			r := newNativeReminderRuntimeForTest(nativeReminderSettings{}, &sounds)
			r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, true, false
			rule := r.settings.Burst.Rules[59005]
			rule.CooldownAlwaysVisible, rule.CooldownAlertEnabled = burstTestBool(true), burstTestBool(false)
			rule.Cast.Enabled, rule.Effect.Enabled = false, false
			r.settings.Burst.Rules[59005] = rule
			r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "self", At: 100}})
			// Private stats often identify self before EntityAppear; max HP alone is not death.
			r.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{Id: "self", At: 100}, Private: true, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 1000}}})
			unknown, _ := r.burstOverlay(time.UnixMilli(100000))
			if len(unknown) != 1 || !unknown[0].TimingUnknown {
				t.Fatalf("always-visible unknown frame: %+v", unknown)
			}
			switch source {
			case "execute":
				r.onEvent(partyUse("self", 59005, 101000, false, 1))
			case "public-release":
				r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "self", At: 101}, AtMs: 101000, SkillId: 59005, Scope: "burst-release", Active: true})
			case "condition":
				r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 101}, CCId: 803, DurationMs: 10000, AttackerId: "self"})
			}
			items, _ := r.burstOverlay(time.UnixMilli(102000))
			if len(items) != 1 || items[0].ActorID != "self" || items[0].ActorName != "本人" || items[0].TimingUnknown || items[0].EndsAtMs != 161000 {
				t.Fatalf("local cast disappeared: %+v", items)
			}
			r.onEvent(liveTestAppear("self", "玩家昵称", 10001, 103))
			items, _ = r.burstOverlay(time.UnixMilli(103000))
			if len(items) != 1 || items[0].ActorName != "玩家昵称" || items[0].EndsAtMs != 161000 {
				t.Fatal("appearance lost the existing cooldown")
			}
			r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "self", At: 104}})
			if items, _ := r.burstOverlay(time.UnixMilli(104000)); len(items) != 0 {
				t.Fatal("dead self remained a candidate")
			}
			r.onEvent(healerHP("self", 105, 1000, 1000))
			items, _ = r.burstOverlay(time.UnixMilli(105000))
			if len(items) != 1 || items[0].EndsAtMs != 161000 {
				t.Fatal("revival reset or lost the cooldown")
			}
			r.settings.Burst.IncludeSelf = false
			if items, _ := r.burstOverlay(time.UnixMilli(106000)); len(items) != 0 {
				t.Fatal("self inclusion switch ignored")
			}
		})
	}
}

func TestBurstLocalFallbackDoesNotAdmitStrangersOrPets(t *testing.T) {
	r, _ := partyFixture(t)
	r.onEvent(partyUse("unknown-player", 59005, 101000, false, 1))
	if r.partySkills["unknown-player"] != nil {
		t.Fatal("unknown remote player accepted")
	}
	pet := liveTestAppear("self", "宠物", 10001, 101)
	pet.OwnerId = "actual-player"
	r.onEvent(pet)
	r.onEvent(partyUse("self", 59005, 102000, false, 2))
	if r.partySkills["self"] != nil {
		t.Fatal("known pet admitted through local identity")
	}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "0", At: 103}, Reset: true})
	if r.partyPlayerActive("0") {
		t.Fatal("reset identity treated as a player")
	}
}

func TestBurstContinuousCooldownExpandsAtLeadTime(t *testing.T) {
	for _, always := range []bool{false, true} {
		for _, alert := range []bool{false, true} {
			r, _ := partyFixture(t)
			r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, false, true
			rule := r.settings.Burst.Rules[59005]
			rule.CooldownAlwaysVisible, rule.CooldownAlertEnabled, rule.CooldownLeadSeconds = burstTestBool(always), burstTestBool(alert), 3
			rule.Cast.Enabled, rule.Effect.Enabled = false, false
			r.settings.Burst.Rules[59005] = rule
			r.onEvent(partyUse("ally", 59005, 100000, false, 1))
			for _, at := range []int64{100000, 106999, 107000, 109999, 110000, 112999, 113000, 150000} {
				items, _ := r.burstOverlay(time.UnixMilli(at))
				inAlert := alert && at >= 107000 && at < 113000
				if !(always || inAlert) {
					if len(items) != 0 {
						t.Fatalf("unexpected popup always=%v alert=%v at=%d: %+v", always, alert, at, items)
					}
					continue
				}
				if len(items) != 1 || items[0].Compact == inAlert {
					t.Fatalf("wrong popup size always=%v alert=%v at=%d: %+v", always, alert, at, items)
				}
				if at < 110000 && (items[0].Phase != "cooldown" || items[0].EndsAtMs != 110000) {
					t.Fatal("threshold transition restarted the countdown")
				}
				if at >= 110000 && items[0].Phase != "ready" {
					t.Fatal("readiness transition failed")
				}
			}
		}
	}
}
