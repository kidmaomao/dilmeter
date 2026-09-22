package main

import (
	"fmt"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"testing"
	"time"
)

func healerRepeatMember(runtime *nativeReminderRuntime, id, name string, count, interval int) healerMemberSelection {
	member := normalizeHealerMember(healerMemberSelection{ID: id, Name: name, Included: true, Health: true}, runtime.healer.settings, 0)
	member.HealthSettings.RepeatCount, member.HealthSettings.RepeatIntervalSeconds = count, interval
	return member
}

func TestHealerMusicQuotaSurvivesIdleAndCapturePause(t *testing.T) {
	for _, ccID := range []uint32{680, 192, 511} {
		for _, count := range []int{1, 3} {
			t.Run(fmt.Sprintf("buff_%d_count_%d", ccID, count), func(t *testing.T) {
				runtime, sounds := healerFixture(t)
				member := healerRepeatMember(runtime, "ally", "队友", 1, 5)
				member.Health, member.BuffSettings.Enabled = false, true
				member.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: ccID, WarningSeconds: 10, RepeatCount: count, RepeatIntervalSeconds: 5}, healerSound{Kind: "healer-music"})}
				runtime.healer.settings.Members = []healerMemberSelection{member}
				runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: ccID, DisableAt: 105})
				healerTick(runtime, 100000)
				healerTick(runtime, 101250)
				// No game events for 30 seconds: suppress output but retain the first sound.
				healerTick(runtime, 131000)
				if runtime.healer.state.Capturing || len(runtime.healer.state.Alerts) != 0 || len(*sounds) != 1 {
					t.Fatal("idle capture did not pause output")
				}
				// Another player's appearance must not create a new music episode.
				runtime.onEvent(liveTestAppear("other", "路人", 10001, 140))
				healerTick(runtime, 140000)
				if len(*sounds) > min(count, 2) {
					t.Fatal("resume burst through repeat quota")
				}
				for at := int64(140250); at <= 150000; at += 250 {
					healerTick(runtime, at)
				}
				if len(*sounds) != count {
					t.Fatalf("idle gap produced %d sounds, want total %d", len(*sounds), count)
				}
				runtime.healer.evaluate(runtime, time.Unix(151, 0), false)
				healerTick(runtime, 152000)
				healerTick(runtime, 154000)
				if len(*sounds) != count {
					t.Fatal("capture pause reset exhausted count")
				}
				// Renewing the actual Buff must still start a new warning episode.
				runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 155}, CCId: ccID, DisableAt: 160})
				healerTick(runtime, 155000)
				healerTick(runtime, 156250)
				if len(*sounds) != count+1 {
					t.Fatal("actual renewal failed to rearm")
				}
			})
		}
	}
}

func TestHealerHealthQuotaSurvivesStaleData(t *testing.T) {
	runtime, sounds := healerFixture(t)
	runtime.healer.settings.Members = []healerMemberSelection{healerRepeatMember(runtime, "ally", "队友", 1, 5)}
	runtime.onEvent(healerHP("ally", 100, 200, 1000))
	healerTick(runtime, 100000)
	healerTick(runtime, 100500)
	runtime.onEvent(liveTestAppear("other", "路人", 13101, 131))
	healerTick(runtime, 131000)
	if len(runtime.healer.state.Alerts) != 0 || len(*sounds) != 1 {
		t.Fatal("stale HP did not pause alerts")
	}
	runtime.onEvent(healerHP("ally", 132, 200, 1000))
	healerTick(runtime, 132000)
	healerTick(runtime, 133000)
	if len(*sounds) != 1 {
		t.Fatal("fresh but still low HP reset exhausted count")
	}
	runtime.onEvent(healerHP("ally", 134, 800, 1000))
	healerTick(runtime, 134000)
	runtime.onEvent(healerHP("ally", 135, 200, 1000))
	healerTick(runtime, 135000)
	healerTick(runtime, 135500)
	if len(*sounds) != 2 {
		t.Fatal("recovery did not rearm low HP")
	}
	// Explicitly disabling monitoring should still discard the episode, even while idle.
	runtime.healer.settings.Members[0].Health = false
	healerTick(runtime, 166000)
	if len(runtime.healer.alerts) != 0 {
		t.Fatal("disabled monitoring kept paused quota")
	}
}

func TestHealerHealthRepeatLimitIntervalAndRecovery(t *testing.T) {
	runtime, sounds := healerFixture(t)
	runtime.healer.settings.Members = []healerMemberSelection{healerRepeatMember(runtime, "ally", "队友", 3, 5)}
	runtime.onEvent(healerHP("ally", 100, 200, 1000))
	for _, point := range []struct {
		at    int64
		count int
	}{{100000, 0}, {100500, 1}, {105250, 1}, {105500, 2}, {110250, 2}, {110500, 3}, {119000, 3}} {
		healerTick(runtime, point.at)
		if len(*sounds) != point.count {
			t.Fatalf("at %d: got %d sounds, want %d", point.at, len(*sounds), point.count)
		}
	}
	runtime.onEvent(healerHP("ally", 120, 800, 1000))
	healerTick(runtime, 120000)
	if len(runtime.healer.state.Cards) != 0 {
		t.Fatal("recovery left a low health reminder")
	}
	runtime.onEvent(healerHP("ally", 121, 200, 1000))
	healerTick(runtime, 121000)
	healerTick(runtime, 121500)
	if len(*sounds) != 4 {
		t.Fatal("new low health episode did not reset the count")
	}
	runtime.onEvent(healerHP("ally", 122, 800, 1000))
	healerTick(runtime, 122000)
	healerTick(runtime, 130000)
	if len(*sounds) != 4 {
		t.Fatal("recovered health kept repeating")
	}
}

func TestHealerRepeatsCoalesceOnlyDueRulesAndRetryRejectedSound(t *testing.T) {
	runtime, sounds := healerFixture(t)
	first := healerRepeatMember(runtime, "ally", "队友", 3, 2)
	second := healerRepeatMember(runtime, "second", "第二位队友", 2, 10)
	runtime.healer.settings.Members = []healerMemberSelection{first, second}
	runtime.onEvent(liveTestAppear("second", "第二位队友", 10001, 100))
	for _, id := range []string{"ally", "second"} {
		runtime.onEvent(healerHP(id, 100, 100, 1000))
	}
	original := runtime.playSound
	attempts := 0
	runtime.playSound = func(request nativeReminderSoundRequest) bool {
		attempts++
		if attempts == 1 {
			return false
		}
		return original(request)
	}
	healerTick(runtime, 100000)
	healerTick(runtime, 100500)
	if len(*sounds) != 1 {
		t.Fatal("retry did not play and coalesce both due first warnings")
	}
	healerTick(runtime, 102500)
	healerTick(runtime, 104500)
	if len(*sounds) != 3 || runtime.healer.alerts["second:health"].announcedCount != 1 {
		t.Fatal("another rule's short interval consumed the slower rule's quota")
	}
	healerTick(runtime, 110500)
	healerTick(runtime, 120500)
	if len(*sounds) != 4 || runtime.healer.alerts["second:health"].announcedCount != 2 {
		t.Fatal("independent repeat interval/count was not respected")
	}
}

func TestHealerBuffRepeatExpiryIsOneEpisodeAndRefreshRearms(t *testing.T) {
	runtime, sounds := healerFixture(t)
	member := healerRepeatMember(runtime, "ally", "队友", 1, 5)
	member.Health = false
	member.BuffSettings.Enabled = true
	rule := normalizeHealerBuffRule(healerBuffRule{CCID: 680, Name: "战争序曲", WarningSeconds: 10, RepeatCount: 3, RepeatIntervalSeconds: 3}, healerSound{Kind: "healer-music"})
	member.BuffSettings.Rules = []healerBuffRule{rule}
	runtime.healer.settings.Members = []healerMemberSelection{member}
	enable := func(at, end int64) {
		runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: at}, CCId: 680, DisableAt: end})
	}
	enable(100, 105)
	for _, point := range []struct {
		at    int64
		count int
	}{{100000, 0}, {101200, 1}, {103900, 1}, {104200, 2}, {105200, 2}, {107200, 3}, {110200, 3}} {
		healerTick(runtime, point.at)
		if len(*sounds) != point.count {
			t.Fatalf("at %d got %d sounds want %d", point.at, len(*sounds), point.count)
		}
	}
	// A short refresh stays inside the warning window, so it must explicitly reset the episode.
	enable(111, 116)
	healerTick(runtime, 111000)
	healerTick(runtime, 112200)
	if len(*sounds) != 4 {
		t.Fatal("short renewal did not rearm the reminder count")
	}
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "ally", At: 113}})
	healerTick(runtime, 113000)
	healerTick(runtime, 120000)
	if len(*sounds) != 4 {
		t.Fatal("out-of-view player kept repeating")
	}
}

func TestHealerFirstWarningPrecedesRepeatsAndStaleHealthStops(t *testing.T) {
	runtime, sounds := healerFixture(t)
	member := healerRepeatMember(runtime, "ally", "队友", 10, 2)
	member.BuffSettings.Enabled = true
	member.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: 680, Name: "战争序曲", WarningSeconds: 10}, healerSound{Kind: "healer-music"})}
	runtime.healer.settings.Members = []healerMemberSelection{member}
	runtime.onEvent(healerHP("ally", 100, 100, 1000))
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: 680, DisableAt: 110})
	healerTick(runtime, 100000)
	healerTick(runtime, 100500)
	healerTick(runtime, 102500)
	if len(*sounds) != 2 || (*sounds)[1].Kind != "healer-music" {
		t.Fatal("health repeat starved the first song warning")
	}
	// Keep the connection alive with unrelated traffic, while HP becomes stale.
	runtime.onEvent(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "self", At: 131}})
	healerTick(runtime, 131000)
	healerTick(runtime, 140000)
	if len(*sounds) != 2 {
		t.Fatal("stale health kept repeating")
	}
}
