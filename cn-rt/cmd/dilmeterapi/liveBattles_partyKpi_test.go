package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"testing"
)

func TestLiveBattlePartyKpiBeforeFirstDamage(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "observer", At: 100}, Reliable: true})
	for _, id := range []string{"player", "other"} {
		f.add(liveTestAppear(id, id, 9001, 100))
	}
	f.add(liveTestAppear("old", "old", 7603, 100))
	f.add(liveTestDamage("old", 101))
	f.add(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "old", At: 102}})
	f.add(&event.EventArcanaSignal{EventBase: event.EventBase{EventId: 22, Id: "player", At: 103}, AtMs: 103000, SkillId: 59041, Signal: "lightning-chain-state", Kind: 814, Count: 1, Complete: true})
	f.add(&event.EventArcanaSignal{EventBase: event.EventBase{EventId: 22, Id: "other", At: 104}, AtMs: 104000, SkillId: 59123, Signal: "sniper-counter", CastAtMs: 104000, TargetId: "new", Phase: 2})
	f.add(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "other", At: 104}, AtMs: 104000, SkillId: 59123, SourceId: "other"})
	f.add(liveTestAppear("new", "new", 7603, 104))
	f.add(liveTestDamage("new", 108))
	f.add(&event.EventArcanaSignal{EventBase: event.EventBase{EventId: 22, Id: "other", At: 109}, AtMs: 109000, SkillId: 59123, Signal: "sniper-counter", CastAtMs: 104000, TargetId: "new", Phase: 7, Count: 6, Complete: true})
	_, rows := f.read("session=" + f.index.current.key)
	chain, zero, terminal, release := 0, 0, 0, 0
	for _, r := range rows {
		if r["Signal"] == "lightning-chain-state" && r["Id"] == "player" {
			chain++
		}
		if r["Signal"] == "sniper-counter" && r["Id"] == "other" {
			if r["Count"] == float64(0) {
				zero++
			} else if r["Count"] == float64(6) {
				terminal++
			}
		}
		if r["EventId"] == float64(10) && r["Id"] == "other" {
			release++
		}
	}
	if chain != 1 || zero != 1 || terminal != 1 || release != 1 {
		t.Fatalf("lost teammate context: chain=%d zero=%d terminal=%d release=%d", chain, zero, terminal, release)
	}
	f.add(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "other", At: 110}})
	if f.index.actors["other"] != nil {
		t.Fatal("departed teammate state retained")
	}
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "observer2", At: 111}, Reliable: true})
	if a := f.index.actor("player"); a.chainState != nil || len(a.recentKpi) != 0 {
		t.Fatal("KPI crossed identity reset")
	}
}
