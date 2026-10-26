package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

func drawNativeBrontasRocks(canvas *image.RGBA, item nativeBossMechanicOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	x, y := item.X+offset.X, item.Y+offset.Y
	rect := image.Rect(x, y, x+int(math.Ceil(260*factor)), y+int(math.Ceil(180*factor)))
	fillNativeOverlayRect(canvas, rect, color.RGBA{R: 33, G: 29, B: 24, A: 240})
	drawNativeOverlayBorder(canvas, rect, color.RGBA{R: 214, G: 171, B: 104, A: 255}, 1)
	text := func(label string, left, top, w, h int, fontSize int, active bool) nativeReminderText {
		shade := nativeColorRef(138, 131, 118)
		if active {
			shade = nativeColorRef(255, 207, 107)
		}
		return nativeReminderText{text: label, rect: imageRectToNative(image.Rect(x+int(float64(left)*factor), y+int(float64(top)*factor), x+int(float64(left+w)*factor), y+int(float64(top+h)*factor))), color: shade, flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(9, int(float64(fontSize)*factor)), fontWeight: nativeFontBold}
	}
	texts := []nativeReminderText{text(fmt.Sprintf("二王落石 · %d 处", len(item.Rocks)), 5, 6, 250, 28, 17, true), text("地图方位", 90, 90, 80, 24, 12, false)}
	for _, p := range []struct{ clock, x, y int }{{12, 90, 37}, {3, 170, 83}, {6, 90, 131}, {9, 10, 83}} {
		active := false
		for _, rock := range item.Rocks {
			if rock.Clock == p.clock {
				active = true
			}
		}
		label := fmt.Sprintf("%d 点", p.clock)
		if active {
			label += " ◆"
			// Alternate the marker highlight every 500 ms, keeping the location readable.
			r := image.Rect(x+int(float64(p.x)*factor), y+int(float64(p.y)*factor), x+int(float64(p.x+80)*factor), y+int(float64(p.y+32)*factor))
			shade := color.RGBA{R: 72, G: 48, B: 16, A: 255}
			if nowMs%1000 < 500 {
				shade = color.RGBA{R: 163, G: 91, B: 10, A: 255}
			}
			fillNativeOverlayRect(canvas, r, shade)
			drawNativeOverlayBorder(canvas, r, color.RGBA{R: 255, G: 207, B: 107, A: 255}, 1)
		}
		texts = append(texts, text(label, p.x, p.y, 80, 32, 22, active))
	}
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	return texts
}
