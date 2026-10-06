package main

import (
	"image"
	"testing"
)

func TestNativeCompactBurstFallbackKeepsReadyAndCoolingPlayers(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 500, 400))
	item := nativeBossMechanicOverlayItem{Compact: true, Phase: "cooldown", SkillID: 59005, ActorID: "4503599627370490", ActorName: "队友B", ReadyActors: []nativeBurstReadyActor{{ActorID: "ally", ActorName: "队友A"}}, ScalePercent: 100, EndsAtMs: 106000, NextReadySoon: true}
	texts := drawNativeMechanicReminder(canvas, item, image.Point{}, 1, 100000)
	if len(texts) != 2 || texts[0].text != "队友A 已就绪" || texts[1].text != "队友B 6.0s" {
		t.Fatalf("lost ready/cooling rows: %+v", texts)
	}
	if canvas.RGBAAt(111, 79).A == 0 || canvas.RGBAAt(112, 80).A != 0 {
		t.Fatal("compact bounds mismatch")
	}
	if texts[0].rect.Top < 40 || texts[1].rect.Top < texts[0].rect.Bottom {
		t.Fatal("rows overlap the icon or one another")
	}
	border := canvas.RGBAAt(0, 0)
	item.Compact = false
	texts = drawNativeMechanicReminder(canvas, item, image.Point{}, 1, 100000)
	if len(texts) != 3 || texts[0].text != "队友A 已就绪" || texts[1].text != "队友B 即将就绪" || texts[2].text != "6.0s" {
		t.Fatalf("large rows: %+v", texts)
	}
	if canvas.RGBAAt(0, 0) != border {
		t.Fatal("compact and large border differ")
	}
	item.Phase = "ready"
	item.ReadyActors = append(item.ReadyActors, nativeBurstReadyActor{ActorName: "队友B"})
	texts = drawNativeMechanicReminder(canvas, item, image.Point{}, 1, 106000)
	if len(texts) != 2 || texts[1].text != "队友B 已就绪" {
		t.Fatal("all-ready list lost a player")
	}
}
