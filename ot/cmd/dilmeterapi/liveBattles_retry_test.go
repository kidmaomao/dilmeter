package main

import (
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestLiveBattleTaiwanRetryWithoutLeavingDungeon(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("player", "player", 10001, 100))
	f.add(liveTestAppear("attempt-1", "same-boss", 193810, 100))
	f.add(liveTestHP("attempt-1", 100))
	f.add(liveTestDamage("attempt-1", 101))
	f.add(liveTestDamage("attempt-1", 110))
	oldKey := f.index.current.key
	// Retry replaces the Boss without a kill, connection reset or room clear.
	f.add(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "attempt-1", At: 111}})
	f.add(liveTestAppear("attempt-2", "same-boss", 193810, 165))
	f.add(liveTestHP("attempt-2", 165))
	f.add(liveTestDamage("attempt-2", 166))
	if f.index.current.key == oldKey {
		t.Fatal("retry reused the failed attempt's live window")
	}
	for _, check := range []struct {
		key, target string
		total       float64
	}{{oldKey, "attempt-1", 200}, {"latest", "attempt-2", 100}} {
		response, rows := f.read("session=" + check.key)
		if response.Code != 200 {
			t.Fatalf("retry window returned HTTP %d", response.Code)
		}
		total := float64(0)
		for _, row := range rows {
			if row["EventId"] == float64(3) {
				if row["TargetId"] != check.target {
					t.Fatalf("another attempt leaked into %s", check.key)
				}
				total += row["Damage"].(float64)
			}
		}
		if total != check.total {
			t.Fatalf("%s damage=%v want=%v", check.key, total, check.total)
		}
	}
}
