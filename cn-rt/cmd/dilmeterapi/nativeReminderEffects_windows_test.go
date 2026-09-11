//go:build windows

package main

import "testing"

func TestNativeReadyBurstScaleHasVisiblePeaks(t *testing.T) {
	if got := nativeReadyBurstScale(0); got >= 1 {
		t.Fatalf("ready burst must enter below full size, got %.2f", got)
	}
	if got := nativeReadyBurstScale(.18); got <= 1.15 {
		t.Fatalf("ready burst first peak is too small: %.2f", got)
	}
	if got := nativeReadyBurstScale(.78); got <= 1.25 {
		t.Fatalf("ready burst second peak is too small: %.2f", got)
	}
	if got := nativeReadyBurstScale(1); got < .999 || got > 1.001 {
		t.Fatalf("ready burst must settle at 1, got %.2f", got)
	}
}
