package main

import (
	"image"
	"testing"
)

func TestNativePowerEffectRosterKeepsSeparateCountdowns(t *testing.T) {
	item := nativeBossMechanicOverlayItem{SkillID: 58014, Phase: "effect", ScalePercent: 100, EffectActors: []nativeBurstEffectActor{
		{ActorID: "a", ActorName: "队友 A", StartedAtMs: 100000, EndsAtMs: 110000},
		{ActorID: "b", ActorName: "队友 B", StartedAtMs: 102000, EndsAtMs: 112000},
	}}
	canvas := image.NewRGBA(image.Rect(0, 0, 500, 400))
	texts := drawNativeMechanicReminder(canvas, item, image.Point{}, 1, 104000)
	if len(texts) != 5 || texts[1].text != "队友 A" || texts[2].text != "6.0s" || texts[3].text != "队友 B" || texts[4].text != "8.0s" {
		t.Fatalf("lost recipient rows or countdowns: %+v", texts)
	}
	_, height := burstPopupDimensions(item)
	if texts[1].rect.Top < texts[0].rect.Bottom || texts[3].rect.Top < texts[1].rect.Bottom || int(texts[4].rect.Bottom) >= height || canvas.RGBAAt(127, height-1).A == 0 || canvas.RGBAAt(128, height).A != 0 {
		t.Fatal("roster overlaps or exceeds its frame")
	}
	texts = drawNativeMechanicReminder(canvas, item, image.Point{}, 1, 110000)
	if len(texts) != 3 || texts[1].text != "队友 B" || texts[2].text != "2.0s" {
		t.Fatalf("expired row retained: %+v", texts)
	}
}
