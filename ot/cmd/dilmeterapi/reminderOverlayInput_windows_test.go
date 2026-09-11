//go:build windows

package main

import "testing"

func TestNativeReminderWorkerGestureIsStale(t *testing.T) {
	tests := []struct {
		name         string
		active       bool
		activeEpoch  uint64
		currentEpoch uint64
		want         bool
	}{
		{name: "old wake cannot cancel inactive worker", activeEpoch: 4, currentEpoch: 5},
		{name: "old wake cannot cancel new epoch gesture", active: true, activeEpoch: 5, currentEpoch: 5},
		{name: "current cancellation cancels old gesture", active: true, activeEpoch: 4, currentEpoch: 5, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeReminderWorkerGestureIsStale(test.active, test.activeEpoch, test.currentEpoch); got != test.want {
				t.Fatalf("nativeReminderWorkerGestureIsStale() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNativeReminderHookDownInvalidated(t *testing.T) {
	tests := []struct {
		name         string
		locked       bool
		downEpoch    uint64
		currentEpoch uint64
		want         bool
	}{
		{name: "unchanged down remains valid", downEpoch: 7, currentEpoch: 7},
		{name: "lock after ownership fails closed", locked: true, downEpoch: 7, currentEpoch: 7, want: true},
		{name: "epoch change after ownership fails closed", downEpoch: 7, currentEpoch: 8, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeReminderHookDownInvalidated(test.locked, test.downEpoch, test.currentEpoch); got != test.want {
				t.Fatalf("nativeReminderHookDownInvalidated() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNativeReminderHookTargetAtUsesPublishedPriority(t *testing.T) {
	snapshot := &nativeReminderHitSnapshot{Regions: []nativeReminderHitRegion{
		{Kind: "web", X: 10, Y: 20, Rect: nativeRect{Left: 0, Top: 0, Right: 50, Bottom: 50}},
		{Kind: "buff", X: 30, Y: 40, Rect: nativeRect{Left: 0, Top: 0, Right: 50, Bottom: 50}},
	}}
	target, ok := nativeReminderHookTargetAt(nativePoint{X: 25, Y: 25}, snapshot)
	if !ok {
		t.Fatal("expected point to hit immutable snapshot")
	}
	if target.Kind != "buff" || target.X != 30 || target.Y != 40 {
		t.Fatalf("unexpected topmost target: %+v", target)
	}
	if _, ok := nativeReminderHookTargetAt(nativePoint{X: 50, Y: 25}, snapshot); ok {
		t.Fatal("right edge must remain exclusive")
	}
}
