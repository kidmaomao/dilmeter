package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestBurstCooldownSelectsEarliestKnownPlayerAndSwitches(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, false, true
	rule := r.settings.Burst.Rules[59005]
	rule.CooldownAlwaysVisible, rule.CooldownAlertEnabled = burstTestBool(true), burstTestBool(false)
	rule.Cast.Enabled, rule.Effect.Enabled = false, false
	r.settings.Burst.Rules[59005] = rule
	r.onEvent(liveTestAppear("second", "奶妈二", 10001, 100))
	r.healer.settings.Members = append(r.healer.settings.Members, healerMemberSelection{ID: "second", Name: "奶妈二", Included: true})
	check := func(at int64, actor, phase string, end int64) {
		t.Helper()
		items, _ := r.burstOverlay(time.UnixMilli(at))
		if len(items) != 1 || items[0].ActorID != actor || items[0].Phase != phase || !items[0].Compact || end > 0 && items[0].EndsAtMs != end {
			t.Fatalf("at %d: %+v", at, items)
		}
	}
	if items, _ := r.burstOverlay(time.UnixMilli(100000)); len(items) != 1 || !items[0].TimingUnknown || items[0].Phase == "ready" {
		t.Fatal("unknown cooldown invented readiness")
	}
	r.onEvent(partyUse("second", 59005, 100000, false, 1)) // shared 60 s -> 160 s
	r.onEvent(partyUse("ally", 59005, 105000, false, 2))   // personal 10 s -> 115 s
	check(106000, "ally", "cooldown", 115000)
	check(116000, "second", "cooldown", 160000)
	check(150000, "second", "cooldown", 160000) // completed ally must yield to a remaining cooldown
	r.onEvent(partyUse("ally", 59005, 151000, false, 3))
	check(151000, "second", "cooldown", 160000)
	r.onEvent(&event.EventSkillCooldown{EventBase: event.EventBase{Id: "ally", At: 152}, SkillId: 59005, AtMs: 152000, ReduceMs: 5000})
	check(152000, "ally", "cooldown", 156000)
	r.onEvent(&event.EventSkillCooldown{EventBase: event.EventBase{Id: "second", At: 153}, SkillId: 59005, AtMs: 153000, Reset: true})
	check(153000, "ally", "cooldown", 156000)
	r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "second", At: 154}})
	check(154000, "ally", "cooldown", 156000)
	r.healer.settings.Members[0].Included = false
	if items, _ := r.burstOverlay(time.UnixMilli(155000)); len(items) != 0 {
		t.Fatal("unselected player still displayed")
	}
}

func TestBurstReadyAllyYieldsToSelfAndRecast(t *testing.T) {
	for _, alert := range []bool{false, true} {
		r, _ := partyFixture(t)
		r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, true, true
		rule := r.settings.Burst.Rules[59005]
		rule.CooldownAlwaysVisible, rule.CooldownAlertEnabled = burstTestBool(true), burstTestBool(alert)
		rule.CooldownLeadSeconds = 5
		rule.Cast.Enabled, rule.Effect.Enabled = false, false
		r.settings.Burst.Rules[59005] = rule
		// Both release together: ally has a 10-second personal CD; self has 60 seconds.
		r.onEvent(partyUse("ally", 59005, 100000, false, 1))
		r.onEvent(partyUse("self", 59005, 100000, false, 2))
		check := func(at int64, actor, phase string, end int64) {
			t.Helper()
			items, _ := r.burstOverlay(time.UnixMilli(at))
			if len(items) != 1 || items[0].ActorID != actor || items[0].Phase != phase || end > 0 && items[0].EndsAtMs != end {
				t.Fatalf("alert=%v at=%d want=%s/%s/%d got=%+v", alert, at, actor, phase, end, items)
			}
		}
		check(100000, "ally", "cooldown", 110000)
		check(110000, "self", "cooldown", 160000)
		for _, at := range []int64{110000, 113000, 154000} {
			items, _ := r.burstOverlay(time.UnixMilli(at))
			if len(items) != 1 || len(items[0].ReadyActors) != 1 || items[0].ReadyActors[0].ActorID != "ally" || items[0].ActorID != "self" {
				t.Fatalf("ready ally lost while self still cooling: %+v", items)
			}
		}
		check(112999, "self", "cooldown", 160000)
		check(113000, "self", "cooldown", 160000)
		check(159000, "self", "cooldown", 160000)
		check(160000, "self", "ready", 0)
		check(163000, "self", "ready", 0) // all ready: retain the most recently completed one
		r.onEvent(partyUse("self", 59005, 170000, false, 3))
		check(170000, "self", "cooldown", 230000) // old ally readiness cannot starve the next self cast
		r.onEvent(partyUse("ally", 59005, 172000, false, 4))
		check(172000, "ally", "cooldown", 182000)
		items, _ := r.burstOverlay(time.UnixMilli(172000))
		if len(items[0].ReadyActors) != 0 {
			t.Fatal("recast ally remained ready")
		}
		check(182000, "self", "cooldown", 230000)
		check(185000, "self", "cooldown", 230000) // switch back again without resetting self's deadline
	}
}

func TestBurstCooldownLeadBoundarySoundAndDisable(t *testing.T) {
	r, sounds := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates, r.settings.Burst.Volume = true, true, 80
	rule := r.settings.Burst.Rules[59005]
	rule.CooldownAlwaysVisible, rule.CooldownLeadSeconds = burstTestBool(false), 2.5
	rule.Cast.Enabled, rule.Effect.Enabled = false, false
	r.settings.Burst.Rules[59005] = rule
	r.onEvent(partyUse("ally", 59005, 100000, false, 1))
	if items, _ := r.burstOverlay(time.UnixMilli(107499)); len(items) != 0 {
		t.Fatal("lead reminder started early")
	}
	items, _ := r.burstOverlay(time.UnixMilli(107500))
	if len(items) != 1 || items[0].Phase != "cooldown" || items[0].EndsAtMs != 110000 || len(*sounds) != 0 {
		t.Fatalf("lead boundary: %+v, sounds %+v", items, *sounds)
	}
	items, _ = r.burstOverlay(time.UnixMilli(110000))
	r.burstOverlay(time.UnixMilli(110500))
	if len(items) != 1 || items[0].Phase != "ready" || len(*sounds) != 1 {
		t.Fatal("ready transition did not sound exactly once")
	}
	if items, _ := r.burstOverlay(time.UnixMilli(113000)); len(items) != 0 {
		t.Fatal("lead popup failed to expire")
	}
	rule.CooldownAlwaysVisible = burstTestBool(true)
	r.settings.Burst.Rules[59005] = rule
	r.burstOverlay(time.UnixMilli(120000))
	if len(*sounds) != 1 {
		t.Fatal("switching display mode repeated readiness sound")
	}
	rule.Ready.Enabled = false
	r.settings.Burst.Rules[59005] = rule
	if items, _ := r.burstOverlay(time.UnixMilli(121000)); len(items) != 0 {
		t.Fatal("disabled cooldown still visible")
	}
}

func TestBurstCooldownDoesNotHideCastEffectOrOverlap(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	rule := r.settings.Burst.Rules[59005]
	rule.CooldownAlwaysVisible = burstTestBool(true)
	r.settings.Burst.Rules[59005] = rule
	r.onEvent(partyUse("ally", 59005, 100000, false, 1))
	items, _ := r.burstOverlay(time.UnixMilli(100100))
	if len(items) != 2 || items[0].Phase != "cooldown" || items[1].Phase != "cast" || items[1].Y < items[0].Y+burstCooldownHeight {
		t.Fatalf("cast/countdown overlap: %+v", items)
	}
	r.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 101}, CCId: 803, DurationMs: 8000, AttackerId: "ally"})
	items, _ = r.burstOverlay(time.UnixMilli(101000))
	if len(items) != 2 || items[1].Phase != "effect" || items[1].EndsAtMs != 109000 {
		t.Fatalf("cooldown hid effect: %+v", items)
	}
	// Respect the scaled portrait card, including its newly taller footprint.
	first := nativeBossMechanicOverlayItem{Compact: true, X: 0, Y: 0, ScalePercent: 150}
	second := separateBurstPopup([]nativeBossMechanicOverlayItem{first}, nativeBossMechanicOverlayItem{X: 80, Y: 100, ScalePercent: 100})
	if second.Y != 128 {
		t.Fatalf("compact bounds not respected: %+v", second)
	}
	beside := separateBurstPopup([]nativeBossMechanicOverlayItem{first}, nativeBossMechanicOverlayItem{X: 180, Y: 10, ScalePercent: 100})
	if beside.Y != 10 {
		t.Fatalf("portrait card should leave adjacent space available: %+v", beside)
	}
}

func TestBurstCooldownNormalizationAndPersistence(t *testing.T) {
	for _, value := range []float64{-1, math.NaN(), math.Inf(1)} {
		s := normalizeNativeBurstSettings(nativeBurstSettings{Rules: map[uint16]nativeBurstRule{59005: {CooldownMode: "invalid", CooldownLeadSeconds: value}}})
		if burstCooldownAlwaysVisible(s.Rules[59005]) || !burstCooldownAlertEnabled(s.Rules[59005]) || s.Rules[59005].CooldownLeadSeconds != 0 {
			t.Fatal("invalid settings did not preserve legacy defaults")
		}
	}
	s := normalizeNativeBurstSettings(nativeBurstSettings{Rules: map[uint16]nativeBurstRule{59005: {CooldownMode: "always", CooldownLeadSeconds: 2.5}}})
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored nativeBurstSettings
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !burstCooldownAlwaysVisible(restored.Rules[59005]) || !burstCooldownAlertEnabled(restored.Rules[59005]) || restored.Rules[59005].CooldownLeadSeconds != 2.5 {
		t.Fatal("cooldown preferences lost on roundtrip")
	}
}

func TestBurstPersonalCooldownRearmsPublicCollapseRelease(t *testing.T) {
	r, _ := partyFixture(t)
	r.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	for _, at := range []int64{100, 110} {
		r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: at}, CCId: 803, DurationMs: 8000, AttackerId: "ally"})
		if r.partySkills["ally"][59005].UsedAtMs != at*1000 {
			t.Fatal("shared 60 second estimate suppressed the teammate's next 10 second cast")
		}
	}
}

func burstTestBool(value bool) *bool { return &value }
