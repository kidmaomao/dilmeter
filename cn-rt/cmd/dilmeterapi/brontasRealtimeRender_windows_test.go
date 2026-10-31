package main

import (
	"image"
	"image/color"
	"testing"
)

func TestBrontasNativeActiveBandWinsFutureWarning(t *testing.T) {
	zero, width := 0.0, 1.0/3
	active := brontasGuideRay{Axis: "vertical", Offset: &zero, HalfWidth: &width, LethalAtMs: 900, EndsAtMs: 1600}
	next := active
	next.LethalAtMs = 5000
	next.EndsAtMs = 5600
	for _, rays := range [][]brontasGuideRay{{active, next}, {next, active}} {
		canvas := image.NewRGBA(image.Rect(0, 0, 320, 224))
		item := nativeBossMechanicOverlayItem{ScalePercent: 100, BrontasGuide: &nativeBrontasGuideView{Stage: "station", Clock: 12, ValidUntilMs: 6000, Rays: rays}}
		drawNativeBrontasGuide(canvas, item, image.Point{}, 1, 1000)
		if got := canvas.RGBAAt(104, 74); got != (color.RGBA{128, 55, 70, 255}) {
			t.Fatal("active band hidden by later warning", got)
		}
	}
}

func TestBrontasNativeActionStripBoundsAndLabels(t *testing.T) {
	v := stationGuideFixture()
	v.Guidance, v.PaceUntilMs = "pace", 1200
	for _, scale := range []int{50, 100, 150, 200} {
		item := nativeBossMechanicOverlayItem{ScalePercent: scale, BrontasGuide: v}
		w, h := burstPopupDimensions(item)
		if w != 320 || h != 278 {
			t.Fatal("action strip outside native window", w, h)
		}
		canvas := image.NewRGBA(image.Rect(0, 0, w*scale/100, h*scale/100))
		texts := drawNativeBrontasGuide(canvas, item, image.Point{}, 1, 1000)
		foundTitle, foundCountdown := false, false
		for _, text := range texts {
			if text.text == "等拍" {
				foundTitle = true
			}
			if text.text == "0.2 s" {
				foundCountdown = true
			}
			if int(text.rect.Bottom) > canvas.Bounds().Max.Y || int(text.rect.Right) > canvas.Bounds().Max.X {
				t.Fatal("clipped native cue", scale, text)
			}
		}
		if !foundTitle || !foundCountdown {
			t.Fatal("native wait cue missing")
		}
	}
	v.Stage = "map"
	if _, h := burstPopupDimensions(nativeBossMechanicOverlayItem{BrontasGuide: v}); h != 224 {
		t.Fatal("map unnecessarily resized")
	}
}
