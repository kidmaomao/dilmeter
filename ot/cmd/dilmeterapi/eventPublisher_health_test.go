package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestPublisherRepublishesHealthAfterVisibilityOrLifeBoundary(t *testing.T) {
	for _, boundary := range []event.IEvent{
		&event.EventEntityDisappear{EventBase: event.EventBase{EventId: event.EventIdEntityDisappear, Id: "123", At: 100}},
		&event.EventFinish{EventBase: event.EventBase{EventId: event.EventIdFinish, Id: "123", At: 100}},
	} {
		publisher := &eventPublisher{lastSentEventAt: time.Now()}
		stats := []packet.StatUpdateEntry{{StatId: 28, Value: 500}, {StatId: 30, Value: 1000}}
		if len(publisher.filterChangedStats(123, stats)) != 2 {
			t.Fatal("initial health snapshot missing")
		}
		if len(publisher.filterChangedStats(123, stats)) != 0 {
			t.Fatal("duplicate-stat suppression stopped working")
		}
		publisher.filterChangedStats(456, stats)
		publisher.publish(boundary)
		if len(publisher.filterChangedStats(123, stats)) != 2 {
			t.Fatalf("new HP snapshot suppressed after %T", boundary)
		}
		if len(publisher.filterChangedStats(456, stats)) != 0 {
			t.Fatal("unrelated entity cache was cleared")
		}
	}
}
