//go:build windows

package main

import (
	"image"
	"testing"
)

func TestSpiritNativeRendering(t *testing.T) {
	item := nativeSkillOverlayItem{SkillID: 35130, EnergyGate: true, AlwaysVisible: true, X: 20, Y: 20}
	percent, observed := 50.0, true
	item.ProgressPercent, item.ProgressObserved, item.EnergyActive = &percent, &observed, true
	draw := func() int {
		canvas := image.NewRGBA(image.Rect(0, 0, 120, 140))
		drawNativeSkillReminderItem(canvas, item, image.Point{}, 48, 100000)
		brightness := 0
		for y := 24; y < 64; y++ {
			for x := 24; x < 64; x++ {
				c := canvas.RGBAAt(x, y)
				brightness += int(c.R) + int(c.G) + int(c.B)
			}
		}
		return brightness
	}
	dim := draw()
	percent, item.EnergyReady = 100, true
	if draw() <= dim {
		t.Fatal("full charge did not light the reminder")
	}
}
