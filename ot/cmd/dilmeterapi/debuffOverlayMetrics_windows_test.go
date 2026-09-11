//go:build windows

package main

import (
	"reflect"
	"testing"
)

func TestCalculateDebuffOverlayMetricsIncludesBossHeaderAndIconRow(t *testing.T) {
	metrics := calculateDebuffOverlayMetrics(3, 30, 100, true, false)
	if metrics.width != 436 || metrics.height != 90 {
		t.Fatalf("size = %dx%d, want 436x90", metrics.width, metrics.height)
	}
	wantRegion := overlayRegionRect{left: 8, top: 8, right: 428, bottom: 82}
	if !reflect.DeepEqual(metrics.regions, []overlayRegionRect{wantRegion}) {
		t.Fatalf("regions = %+v, want %+v", metrics.regions, []overlayRegionRect{wantRegion})
	}
}

func TestCalculateDebuffOverlayMetricsKeepsBossHeaderWithoutMissingIcons(t *testing.T) {
	metrics := calculateDebuffOverlayMetrics(0, 30, 125, true, false)
	if metrics.width != 545 || metrics.height != 48 {
		t.Fatalf("size = %dx%d, want 545x48", metrics.width, metrics.height)
	}
	if len(metrics.regions) != 1 {
		t.Fatalf("region count = %d, want 1", len(metrics.regions))
	}
}

func TestCalculateDebuffOverlayMetricsFallsBackWithoutBoss(t *testing.T) {
	got := calculateDebuffOverlayMetrics(2, 30, 100, false, false)
	want := calculateBuffOverlayMetrics(2, 30, 100, 0)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metrics = %+v, want %+v", got, want)
	}
}

func TestCalculateDebuffOverlayMetricsIncludesSelectedTargetHealth(t *testing.T) {
	metrics := calculateDebuffOverlayMetrics(0, 30, 100, false, true)
	if metrics.width != 436 || metrics.height != 54 {
		t.Fatalf("size = %dx%d, want 436x54", metrics.width, metrics.height)
	}
}

func TestCalculateDebuffOverlayMetricsStacksHealthAndDebuffs(t *testing.T) {
	metrics := calculateDebuffOverlayMetrics(3, 30, 100, true, true)
	if metrics.width != 436 || metrics.height != 132 {
		t.Fatalf("size = %dx%d, want 436x132", metrics.width, metrics.height)
	}
}
