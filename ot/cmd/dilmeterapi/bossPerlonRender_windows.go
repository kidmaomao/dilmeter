//go:build windows

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

func drawNativePerlonReminder(canvas *image.RGBA, item nativeBossMechanicOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	left, top := item.X+offset.X, item.Y+offset.Y
	size := max(59, int(math.Ceil(118*factor)))
	rect := image.Rect(left, top, left+size, top+size)
	opacity := clampNativeReminderInt(item.OpacityPercent, 20, 100, 100)
	if opacity == 100 {
		return drawNativePerlonCard(canvas, item, rect, factor, nowMs)
	}
	bounds := rect.Inset(-max(4, int(math.Ceil(35*factor)))).Intersect(canvas.Bounds())
	layer := image.NewRGBA(bounds)
	labels := drawNativePerlonCard(layer, item, rect, factor, nowMs)
	mask := image.NewUniform(color.Alpha{A: uint8(opacity * 255 / 100)})
	draw.DrawMask(canvas, bounds, layer, bounds.Min, mask, image.Point{}, draw.Over)
	for i := range labels {
		labels[i].opacityPercent = opacity
	}
	return labels
}

func drawNativePerlonCard(canvas *image.RGBA, item nativeBossMechanicOverlayItem, rect image.Rectangle, factor float64, nowMs int64) []nativeReminderText {
	if item.Key == perlonTornadoKey {
		return drawNativeTornadoWarning(canvas, item, rect, factor, nowMs)
	}
	left, top := rect.Min.X, rect.Min.Y
	background := color.RGBA{8, 10, 12, 36} // Same 14% base opacity as the WebView Boss reminders.
	border, glow, rim := color.RGBA{229, 213, 255, 250}, color.RGBA{160, 97, 255, 184}, color.RGBA{243, 232, 255, 199}
	glowRadius, rimRadius := 25.0, 10.0
	resonance, sun := item.Key == perlonResonanceKey, item.ResonanceMode == "release"
	warning := resonance && sun && item.AreaWarningUntilMs > nowMs
	if resonance {
		border, glow, rim = color.RGBA{183, 212, 243, 250}, color.RGBA{76, 163, 255, 184}, color.RGBA{235, 246, 255, 199}
		if sun {
			border, glow, rim = color.RGBA{244, 213, 138, 250}, color.RGBA{255, 177, 62, 184}, color.RGBA{255, 243, 196, 199}
			if warning && nowMs%700 >= 350 {
				// Flash the edge and glow without turning the card opaque.
				border, glow, rim = color.RGBA{255, 249, 223, 255}, color.RGBA{255, 177, 62, 255}, color.RGBA{255, 255, 255, 255}
				glowRadius, rimRadius = 34, 14
			}
		}
	}
	drawNativeReminderGlow(canvas, rect, glow, max(6, int(glowRadius*factor)))
	drawNativeReminderGlow(canvas, rect, rim, max(4, int(rimRadius*factor)))
	fillNativeOverlayRect(canvas, rect, background)
	drawNativeOverlayBorder(canvas, rect, border, max(1, int(2*factor)))
	if resonance {
		icon := image.Rect(left+int(33*factor), top+int(13*factor), left+int(121*factor), top+int(101*factor))
		drawNativePerlonModeIcon(canvas, icon, rect.Inset(max(1, int(2*factor))), sun)
	}
	seconds := math.Max(0, float64(item.EndsAtMs-nowMs)/1000)
	value := fmt.Sprintf("%.1f", math.Ceil(seconds*10)/10)
	if seconds >= 10 {
		value = fmt.Sprintf("%.0f", math.Ceil(seconds))
	}
	if item.TimingUnknown {
		value = "--"
	}
	caption := "预计切换 · 秒"
	if resonance {
		if item.TimingUnknown {
			caption = "等待切换信号"
		}
		if warning {
			caption = "仍在区域 · 快离开"
		}
	}
	text := func(label string, y1, y2, font int) nativeReminderText {
		return nativeReminderText{text: label, rect: imageRectToNative(image.Rect(left+int(4*factor), top+int(float64(y1)*factor), rect.Max.X-int(4*factor), top+int(float64(y2)*factor))), color: nativeColorRef(250, 247, 255), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(8, int(float64(font)*factor)), fontWeight: nativeFontBold}
	}
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	return []nativeReminderText{text(item.Name, 12, 30, 12), text(value, 35, 83, 42), text(caption, 88, 105, 11)}
}

func drawNativeTornadoWarning(canvas *image.RGBA, item nativeBossMechanicOverlayItem, rect image.Rectangle, factor float64, nowMs int64) []nativeReminderText {
	elapsed := max(int64(0), nowMs-item.StartedAtMs)
	pulse := 1.0
	if elapsed < 840 {
		pulse += .14 * (1 - math.Cos(2*math.Pi*float64(elapsed%420)/420))
	}
	centerX := float64(rect.Min.X) + 59*factor
	centerY := float64(rect.Min.Y) + 39*factor
	scale := factor * pulse
	shade := color.RGBA{255, 44, 67, 255}
	title := "尔格龙卷风"
	if item.TornadoTarget == "teammate" {
		shade = color.RGBA{255, 255, 255, 255}
		title = "队友 · 龙卷风"
	} else if item.TornadoTarget == "unknown" {
		title = "龙卷风 · 目标未知"
	}
	// Rasterize the rounded triangle stroke and the exclamation mark within
	// the same footprint as the WebView icon, at the active display scale.
	bounds := image.Rect(
		int(math.Floor(centerX-40*scale)), int(math.Floor(centerY-39*scale)),
		int(math.Ceil(centerX+40*scale)), int(math.Ceil(centerY+31*scale)),
	).Intersect(canvas.Bounds())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		py := (float64(y) + .5 - centerY) / scale
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			px := (float64(x) + .5 - centerX) / scale
			distance := min(
				tornadoSegmentDistanceSquared(px, py, 0, -27, -32, 26),
				tornadoSegmentDistanceSquared(px, py, 0, -27, 32, 26),
				tornadoSegmentDistanceSquared(px, py, -32, 26, 32, 26),
				tornadoSegmentDistanceSquared(px, py, 0, -9, 0, 6),
				px*px+(py-16)*(py-16),
			)
			pixel := image.Rect(x, y, x+1, y+1)
			switch {
			case distance <= 3.2*3.2:
				fillNativeOverlayRect(canvas, pixel, shade)
			case distance <= 5.5*5.5:
				fillNativeOverlayRect(canvas, pixel, color.RGBA{27, 7, 10, 165})
			}
		}
	}
	value := "快躲"
	startsAt := item.TornadoStartsAtMs
	if startsAt == 0 {
		startsAt = item.EndsAtMs
	}
	if nowMs < startsAt {
		value = fmt.Sprintf("%.1fs", math.Ceil(float64(startsAt-nowMs)/100)/10)
	}
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	text := func(label string, y1, y2, font int) nativeReminderText {
		return nativeReminderText{
			text: label, rect: imageRectToNative(image.Rect(rect.Min.X, rect.Min.Y+int(float64(y1)*factor), rect.Max.X, rect.Min.Y+int(float64(y2)*factor))),
			color: nativeColorRef(255, 247, 249), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis,
			fontHeight: max(8, int(float64(font)*factor)), fontWeight: nativeFontBold,
		}
	}
	return []nativeReminderText{text(title, 76, 94, 12), text(value, 91, 118, 22)}
}

func tornadoSegmentDistanceSquared(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	along := min(1, max(0, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
	x, y := px-ax-along*dx, py-ay-along*dy
	return x*x + y*y
}

func drawNativePerlonModeIcon(canvas *image.RGBA, bounds, clip image.Rectangle, sun bool) {
	shade := color.RGBA{200, 227, 255, 61}
	if sun {
		shade = color.RGBA{255, 217, 117, 61}
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		py := (float64(y-bounds.Min.Y)+.5)*24/float64(bounds.Dy()) - 12
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !image.Pt(x, y).In(clip) {
				continue
			}
			px := (float64(x-bounds.Min.X)+.5)*24/float64(bounds.Dx()) - 12
			inside := px*px+py*py <= 9.2*9.2 && (px-4)*(px-4)+(py+4)*(py+4) > 8.8*8.8
			if sun {
				inside = px*px+py*py <= 25
				for ray := 0; ray < 8 && !inside; ray++ {
					a := float64(ray) * math.Pi / 4
					along, across := px*math.Cos(a)+py*math.Sin(a), px*math.Sin(a)-py*math.Cos(a)
					inside = along >= 8 && along <= 11 && math.Abs(across) <= .9
				}
			}
			if inside {
				fillNativeOverlayRect(canvas, image.Rect(x, y, x+1, y+1), shade)
			}
		}
	}
}
