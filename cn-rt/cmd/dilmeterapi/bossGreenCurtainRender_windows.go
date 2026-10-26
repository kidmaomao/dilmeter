package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

func drawNativeGreenCurtainPersistent(canvas *image.RGBA, item nativeBossMechanicOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	w, h := burstPopupDimensions(item)
	left, top := item.X+offset.X, item.Y+offset.Y
	rect := image.Rect(left, top, left+int(math.Ceil(float64(w)*factor)), top+int(math.Ceil(float64(h)*factor)))
	opacity := clampNativeReminderInt(item.OpacityPercent, 20, 100, 100)
	alpha := uint8(opacity * 255 / 100)
	label := fmt.Sprintf("%.1f", math.Max(0, float64(item.EndsAtMs-nowMs)/1000))
	if item.TimingUnknown {
		label = "--"
	}
	fillNativeOverlayRect(canvas, rect, color.RGBA{R: 16, G: 42, B: 29, A: alpha})
	if item.PassiveStyle == "bar" {
		fillNativeOverlayRect(canvas, rect, color.RGBA{R: 7, G: 27, B: 17, A: alpha})
		track := rect.Inset(max(1, int(factor)))
		track.Min.X = left + int(34*factor)
		ratio := math.Max(0, math.Min(1, float64(item.EndsAtMs-nowMs)/float64(max(int64(1), item.EndsAtMs-item.StartedAtMs))))
		if item.TimingUnknown {
			ratio = 0
		}
		track.Max.X = track.Min.X + int(float64(track.Dx())*ratio)
		fillNativeOverlayRect(canvas, track, color.RGBA{R: 40, G: 115, B: 68, A: alpha})
	}
	drawNativeOverlayBorder(canvas, rect, color.RGBA{R: 112, G: 184, B: 142, A: alpha}, 1)
	iconSize, fontSize := 22, 20
	if item.PassiveStyle == "bar" {
		iconSize, fontSize = 20, 18
	}
	icon := int(float64(iconSize) * factor)
	iconTop := top + (rect.Dy()-icon)/2
	drawNativeCurtainIcon(canvas, image.Rect(left+int(6*factor), iconTop, left+int(6*factor)+icon, iconTop+icon), alpha)
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	return []nativeReminderText{{opacityPercent: opacity, text: label, rect: imageRectToNative(rect), color: nativeColorRef(240, 255, 245), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(9, int(float64(fontSize)*factor)), fontWeight: nativeFontBold}}
}

// The same 24-unit curtain silhouette as the web overlay's inline SVG.
func drawNativeCurtainIcon(canvas *image.RGBA, rect image.Rectangle, alpha uint8) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		py := (float64(y-rect.Min.Y) + .5) * 24 / float64(rect.Dy())
		for x := rect.Min.X; x < rect.Max.X; x++ {
			px := (float64(x-rect.Min.X) + .5) * 24 / float64(rect.Dx())
			beam := py >= 2 && py < 4 && px >= 2 && px < 22
			edge := 11 - (py-5)*6/9
			if py >= 14 {
				edge = 5 + (py-14)*2/7
			}
			drape := py >= 5 && py < 21 && (px >= 3 && px < edge || px >= 24-edge && px < 21)
			if beam || drape {
				shade := color.RGBA{R: 169, G: 239, B: 192, A: alpha}
				if py >= 6 && py < 11 && (px >= 4 && px < 5 || px >= 19 && px < 20) {
					shade = color.RGBA{R: 18, G: 51, B: 34, A: alpha}
				}
				fillNativeOverlayRect(canvas, image.Rect(x, y, x+1, y+1), shade)
			}
		}
	}
}
