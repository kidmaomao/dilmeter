//go:build windows

package main

import (
	"reflect"
	"testing"
)

func TestCalculateBuffOverlayMetricsScalesWindowAndRegions(t *testing.T) {
	tests := []struct {
		name         string
		scalePercent int
		wantWidth    int
		wantHeight   int
		wantFirst    overlayRegionRect
		wantSecond   overlayRegionRect
	}{
		{
			name: "100 percent", scalePercent: 100, wantWidth: 88, wantHeight: 54,
			wantFirst:  overlayRegionRect{left: 8, top: 8, right: 28, bottom: 46},
			wantSecond: overlayRegionRect{left: 34, top: 8, right: 54, bottom: 46},
		},
		{
			name: "125 percent", scalePercent: 125, wantWidth: 110, wantHeight: 68,
			wantFirst:  overlayRegionRect{left: 10, top: 10, right: 35, bottom: 58},
			wantSecond: overlayRegionRect{left: 42, top: 10, right: 68, bottom: 58},
		},
		{
			name: "150 percent", scalePercent: 150, wantWidth: 132, wantHeight: 81,
			wantFirst:  overlayRegionRect{left: 12, top: 12, right: 42, bottom: 69},
			wantSecond: overlayRegionRect{left: 51, top: 12, right: 81, bottom: 69},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics := calculateBuffOverlayMetrics(3, 20, test.scalePercent, 0)
			if metrics.width != test.wantWidth || metrics.height != test.wantHeight {
				t.Fatalf("size = %dx%d, want %dx%d", metrics.width, metrics.height, test.wantWidth, test.wantHeight)
			}
			if len(metrics.regions) != 3 {
				t.Fatalf("region count = %d, want 3", len(metrics.regions))
			}
			if !reflect.DeepEqual(metrics.regions[0], test.wantFirst) {
				t.Errorf("first region = %+v, want %+v", metrics.regions[0], test.wantFirst)
			}
			if !reflect.DeepEqual(metrics.regions[1], test.wantSecond) {
				t.Errorf("second region = %+v, want %+v", metrics.regions[1], test.wantSecond)
			}
		})
	}
}

func TestCalculateBuffOverlayMetricsClampsScale(t *testing.T) {
	low := calculateBuffOverlayMetrics(1, 20, 1, 0)
	high := calculateBuffOverlayMetrics(1, 20, 999, 0)
	if low.width != scaleOverlayCeil(36, 50) {
		t.Fatalf("low scale width = %d, want 50%% clamp", low.width)
	}
	if high.width != scaleOverlayCeil(36, 500) {
		t.Fatalf("high scale width = %d, want 500%% clamp", high.width)
	}
}
