package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"testing"
	"time"
)

func TestBurstSquareKeepsCasterAndAnchorThroughPublicPowerState(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	send := func(scope string, active bool, at int64) {
		r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "ally", At: at / 1000}, AtMs: at, SkillId: 58014, Scope: scope, Active: active})
	}
	send("prepare", true, 100000)
	cast, _ := r.burstOverlay(time.UnixMilli(100000))
	send("burst-effect", true, 101105)
	effect, bars := r.burstOverlay(time.UnixMilli(101105))
	if len(cast) != 1 || len(effect) != 1 || len(bars) != 0 {
		t.Fatalf("cast/effect = %+v / %+v", cast, effect)
	}
	if effect[0].Phase != "effect" || effect[0].TimingUnknown || effect[0].EndsAtMs != 111105 || effect[0].ActorID != "ally" || effect[0].Key != cast[0].Key || effect[0].Generation != cast[0].Generation || effect[0].X != cast[0].X || effect[0].Y != cast[0].Y {
		t.Fatalf("card jumped or fabricated timing: %+v", effect[0])
	}
	if r.partySkills["ally"][58014].UsedAtMs != 0 {
		t.Fatal("power activation invented cooldown")
	}
	send("burst-effect", false, 107939)
	effect, _ = r.burstOverlay(time.UnixMilli(107939))
	if len(effect) != 1 || effect[0].EndsAtMs != 111105 {
		t.Fatal("putting Power down changed the Awakening deadline")
	}
	send("prepare", true, 120000)
	send("burst-effect", true, 120100)
	r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 120}})
	r.onEvent(healerHP("ally", 121, 100, 100))
	effect, _ = r.burstOverlay(time.UnixMilli(121000))
	if len(effect) != 0 {
		t.Fatal("revival restored an old public Power effect")
	}
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 121}, CCId: 516, DurationMs: 10000})
	effect, _ = r.burstOverlay(time.UnixMilli(121000))
	if len(effect) != 1 || effect[0].TimingUnknown || effect[0].EndsAtMs != 131000 {
		t.Fatalf("valid CC516 lost its countdown: %+v", effect)
	}
}

func TestHealerBuffAndSkillsShareMemberFrame(t *testing.T) {
	cards := []healerCard{
		{Key: "b", MemberKey: "a", Name: "A", Category: "buff", CCID: 516, X: 40, Y: 160},
		{Key: "s", MemberKey: "a", Name: "A", Category: "skill", SkillID: 59005, X: 40, Y: 160},
		{Key: "other", MemberKey: "b", Name: "B", Category: "skill", SkillID: 59005, X: 40, Y: 232},
		{Key: "hp", MemberKey: "a", Name: "A", Category: "health", X: 600, Y: 180},
	}
	frame := buildHealerOverlayFrame(16, 30, cards, false, 100000)
	if len(frame.Groups) != 3 || len(frame.Groups[0].Cards) != 2 || frame.Groups[0].Cards[1].SkillID != 59005 {
		t.Fatalf("mixed or split teammate strip: %+v", frame.Groups)
	}
	r, _ := partyFixture(t)
	m := r.healer.settings.Members[0]
	m.SkillSettings.Overlay.X = 900
	m.SkillSettings.Overlay.Y = 999
	m = normalizeHealerMember(m, r.healer.settings, 0)
	if m.SkillSettings.Overlay.X != m.BuffSettings.Overlay.X || m.SkillSettings.Overlay.Y != m.BuffSettings.Overlay.Y {
		t.Fatal("legacy skill coordinates did not migrate")
	}
}

func TestBurstCollapseRequiresActualSource(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	r.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 100}, CCId: 803, DurationMs: 10000})
	items, _ := r.burstOverlay(time.UnixMilli(101000))
	if len(items) != 0 {
		t.Fatal("unknown source attributed to selected teammate")
	}
}
