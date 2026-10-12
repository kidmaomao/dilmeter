package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestBurstPowerRecipientsShareOneFrameAndExpireIndependently(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, true, true
	for _, entry := range []struct {
		id string
		at int64
	}{{"ally", 100}, {"self", 102}} {
		r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: entry.id, At: entry.at}, CCId: 516, DurationMs: 10000})
	}
	items, _ := r.burstOverlay(time.UnixMilli(103000))
	if len(items) != 1 || len(items[0].EffectActors) != 2 || items[0].EndsAtMs != 112000 {
		t.Fatalf("recipients were split or group used the first expiry: %+v", items)
	}
	group := items[0]
	deadlines := map[string]int64{}
	for _, actor := range group.EffectActors {
		deadlines[actor.ActorID] = actor.EndsAtMs
	}
	if deadlines["ally"] != 110000 || deadlines["self"] != 112000 {
		t.Fatalf("lost per-player timing: %+v", deadlines)
	}
	items, _ = r.burstOverlay(time.UnixMilli(110000))
	if len(items) != 1 || len(items[0].EffectActors) != 1 || items[0].EffectActors[0].ActorID != "self" || items[0].Key != group.Key || items[0].X != group.X || items[0].Y != group.Y {
		t.Fatalf("first expiry removed or moved the remaining recipient: %+v", items)
	}
	items, _ = r.burstOverlay(time.UnixMilli(112000))
	if len(items) != 0 {
		t.Fatalf("empty frame remained: %+v", items)
	}
}

func TestBurstPowerGroupRemovesOnlyAffectedRecipient(t *testing.T) {
	for _, removal := range []string{"condition", "death", "disappear", "unselect"} {
		t.Run(removal, func(t *testing.T) {
			r, _ := partyFixture(t)
			r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, true, true
			for _, id := range []string{"ally", "self"} {
				r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: id, At: 100}, CCId: 516, DurationMs: 10000})
			}
			switch removal {
			case "condition":
				r.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 102}, CCId: 516})
			case "death":
				r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 102}})
			case "disappear":
				r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "ally", At: 102}})
			case "unselect":
				r.healer.settings.Members[0].Included = false
			}
			items, _ := r.burstOverlay(time.UnixMilli(102000))
			if len(items) != 1 || len(items[0].EffectActors) != 1 || items[0].EffectActors[0].ActorID != "self" || items[0].EndsAtMs != 110000 {
				t.Fatalf("removed another player's effect: %+v", items)
			}
		})
	}
}

func TestBurstPowerGroupCoexistsWithCastAndUsesFullHeight(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeSelf, r.settings.Burst.IncludeTeammates = true, true, true
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "self", At: 100}, CCId: 516, DurationMs: 10000})
	r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "ally", At: 101}, AtMs: 101000, SkillId: 58014, Scope: "prepare", Active: true})
	items, _ := r.burstOverlay(time.UnixMilli(102000))
	if len(items) != 2 || items[0].Phase != "cast" || len(items[0].EffectActors) != 0 || len(items[1].EffectActors) != 1 || items[1].EffectActors[0].ActorID != "self" {
		t.Fatalf("casting actor was treated as an effect recipient: %+v", items)
	}
	group := items[1]
	group.EffectActors = make([]nativeBurstEffectActor, 8)
	w, h := burstPopupDimensions(group)
	if w != 128 || h != 224 {
		t.Fatalf("roster dimensions = %d x %d", w, h)
	}
	for _, scale := range []int{50, 100, 200} {
		group.X, group.Y, group.ScalePercent = 20, 30, scale
		next := separateBurstPopup([]nativeBossMechanicOverlayItem{group}, group)
		if next.Y != group.Y+h*scale/100+8 {
			t.Fatalf("group overlaps next popup at scale %d: %+v", scale, next)
		}
	}
}
