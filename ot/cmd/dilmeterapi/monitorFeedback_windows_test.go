package main

import (
	"bytes"
	"image"
	"testing"
)

func TestRockMarkersFlashOnlyAtObservedSockets(t *testing.T) {
	item := nativeBossMechanicOverlayItem{ScalePercent: 100, Rocks: []nativeBrontasRockMarker{{Clock: 12}, {Clock: 9}}}
	a, b := image.NewRGBA(image.Rect(0, 0, 260, 180)), image.NewRGBA(image.Rect(0, 0, 260, 180))
	drawNativeBrontasRocks(a, item, image.Point{}, 1, 100)
	drawNativeBrontasRocks(b, item, image.Point{}, 1, 600)
	if bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("rock highlight did not flash")
	}
	if a.RGBAAt(95, 48) == b.RGBAAt(95, 48) {
		t.Fatal("active 12 o'clock did not flash")
	}
	if a.RGBAAt(185, 95) != b.RGBAAt(185, 95) {
		t.Fatal("inactive 3 o'clock flashed")
	}
}

func TestGreenCurtainCompactRender(t *testing.T) {
	for _, style := range []string{"timer", "bar"} {
		item := nativeBossMechanicOverlayItem{PassiveStyle: style, ScalePercent: 100, StartedAtMs: 1000, EndsAtMs: 64000, OpacityPercent: 75}
		w, h := burstPopupDimensions(item)
		canvas := image.NewRGBA(image.Rect(0, 0, w, h))
		texts := drawNativeGreenCurtainPersistent(canvas, item, image.Point{}, 1, 14000)
		if len(texts) != 1 || texts[0].text != "50.0" {
			t.Fatalf("timer has extra text: %+v", texts)
		}
		if style == "timer" && (w != 104 || h != 34) || style == "bar" && (w != 180 || h != 30) {
			t.Fatal("unexpected display bounds")
		}
		if style == "bar" && canvas.RGBAAt(50, 15) == canvas.RGBAAt(170, 15) {
			t.Fatal("remaining time bar did not shrink")
		}
	}
}
