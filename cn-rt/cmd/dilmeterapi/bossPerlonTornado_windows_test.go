package main

import (
	"bytes"
	"image"
	"testing"
)

func TestPerlonTornadoNativeCountdownAndTwoPulses(t *testing.T) {
	item := nativeBossMechanicOverlayItem{Key: perlonTornadoKey, Name: "尔格龙卷风", StartedAtMs: 1000, TornadoStartsAtMs: 2000, EndsAtMs: 4000, ScalePercent: 100, X: 40, Y: 40, TornadoTarget: "self"}
	draw := func(at int64) (*image.RGBA, []nativeReminderText) {
		canvas := image.NewRGBA(image.Rect(0, 0, 220, 220))
		return canvas, drawNativePerlonReminder(canvas, item, image.Point{}, 1, at)
	}
	original, before := draw(1000)
	first, _ := draw(1210)
	between, _ := draw(1420)
	second, _ := draw(1630)
	held, after := draw(2500)
	if len(before) != 2 || before[1].text != "1.0s" || after[1].text != "快躲" {
		t.Fatalf("changed startup or post-start timing: %+v / %+v", before, after)
	}
	if bytes.Equal(original.Pix, first.Pix) || !bytes.Equal(original.Pix, between.Pix) ||
		!bytes.Equal(first.Pix, second.Pix) || !bytes.Equal(original.Pix, held.Pix) {
		t.Fatal("triangle did not expand twice and then hold its original size")
	}
	item.TornadoTarget = "teammate"
	white, title := draw(1000)
	redPixel, whitePixel := original.RGBAAt(99, 79), white.RGBAAt(99, 79)
	if redPixel.R != 255 || redPixel.G >= 100 || whitePixel.R != 255 || whitePixel.G != 255 || title[0].text != "队友 · 龙卷风" {
		t.Fatalf("wrong tornado colors: self=%+v teammate=%+v", redPixel, whitePixel)
	}
	item.OpacityPercent = 40
	faded, labels := draw(1000)
	if pixel := faded.RGBAAt(99, 79); pixel.A != 102 || pixel.R != 102 || labels[0].opacityPercent != 40 {
		t.Fatalf("native opacity did not apply to icon and text: pixel=%+v text=%+v", pixel, labels)
	}
}

func TestSkillOverlayVisibilityWakesImmediately(t *testing.T) {
	oldWake, oldActive := skillOverlayWake, skillOverlayActive.Load()
	skillOverlayWake = make(chan struct{}, 1)
	defer func() { skillOverlayWake = oldWake; skillOverlayActive.Store(oldActive) }()
	setNativeSkillOverlayActive(true)
	setNativeSkillOverlayActive(true) // Multiple updates cannot block the event loop.
	select {
	case <-skillOverlayWake:
		if !skillOverlayActive.Load() {
			t.Fatal("window woke before the new active state was stored")
		}
	default:
		t.Fatal("new alert still waits for the visibility timer")
	}
}
