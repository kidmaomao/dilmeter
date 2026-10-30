package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Same guide state and physical size as BrontasGuideOverlay.vue, for the
// native GDI fallback. The normal Windows host uses its transparent WebView.
func drawNativeBrontasGuide(canvas *image.RGBA, item nativeBossMechanicOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	v := item.BrontasGuide
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	x, y := item.X+offset.X, item.Y+offset.Y
	rect := func(l, t, w, h float64) image.Rectangle {
		return image.Rect(x+int(l*factor), y+int(t*factor), x+int((l+w)*factor), y+int((t+h)*factor))
	}
	bounds := rect(0, 0, 320, 340)
	fillNativeOverlayRect(canvas, bounds, color.RGBA{R: 9, G: 18, B: 27, A: 240})
	drawNativeOverlayBorder(canvas, bounds, color.RGBA{R: 86, G: 120, B: 141, A: 255}, 1)
	white := color.RGBA{R: 236, G: 245, B: 250, A: 255}
	cyan := color.RGBA{R: 104, G: 234, B: 219, A: 255}
	text := func(s string, l, t, w, h, size float64, c color.RGBA) nativeReminderText {
		return nativeReminderText{text: s, rect: imageRectToNative(rect(l, t, w, h)), color: nativeColorRef(c.R, c.G, c.B), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(9, int(size*factor)), fontWeight: nativeFontBold}
	}
	fresh := nowMs < v.ValidUntilMs
	label := "未连接"
	if fresh && v.Status != "waiting" {
		label = "实时"
	}
	if item.PreviewExpiresAtMs > 0 {
		label = "预览"
	}
	texts := []nativeReminderText{text("二王 50% · 走位提示", 12, 12, 220, 25, 16, white), text(label, 250, 12, 58, 25, 12, white)}
	for i, c := range []int{12, 3, 6, 9} {
		shade := color.RGBA{R: 97, G: 117, B: 132, A: 255}
		suffix := "—"
		for _, rock := range item.Rocks {
			if rock.Clock == c {
				suffix = "◆"
				shade = color.RGBA{R: 229, G: 185, B: 95, A: 255}
			}
		}
		if fresh && v.Clock == c {
			shade = cyan
		}
		texts = append(texts, text(fmt.Sprintf("%d点 %s", c, suffix), float64(10+i*76), 43, 76, 22, 12, shade))
	}
	cx, cy, radius := 93.0, 143.0, 67.0
	for py := -67; py <= 67; py++ {
		for px := -67; px <= 67; px++ {
			d := math.Hypot(float64(px), float64(py))
			if d > radius {
				continue
			}
			shade := color.RGBA{R: 17, G: 31, B: 42, A: 255}
			if d > radius-1.5 {
				shade = color.RGBA{R: 101, G: 132, B: 155, A: 255}
			}
			if fresh && v.Player != nil {
				for _, ray := range v.Rays {
					if ray.Offset == nil || ray.HalfWidth == nil || (ray.EndsAtMs > 0 && nowMs >= ray.EndsAtMs) {
						continue
					}
					u := float64(px) / radius
					if ray.Axis == "horizontal" {
						u = -float64(py) / radius
					}
					if math.Abs(u-*ray.Offset) <= *ray.HalfWidth {
						shade = color.RGBA{R: 128, G: 97, B: 41, A: 255}
						if nowMs >= ray.LethalAtMs {
							shade = color.RGBA{R: 142, G: 48, B: 60, A: 255}
						}
					}
				}
			}
			fillNativeOverlayRect(canvas, rect(cx+float64(px), cy+float64(py), 1.5, 1.5), shade)
		}
	}
	point := func(p brontasPoint) (float64, float64) { return cx + p.X*radius, cy - p.Y*radius }
	dot := func(p brontasPoint, r float64, c color.RGBA) {
		xx, yy := point(p)
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r {
					fillNativeOverlayRect(canvas, rect(xx+dx, yy+dy, 1.5, 1.5), c)
				}
			}
		}
	}
	if fresh && v.Player != nil {
		if v.Target != nil && v.Status == "move" {
			a, b := v.Player.brontasPoint, *v.Target
			for u := 0.0; u <= 1; u += .01 {
				p := brontasPoint{X: a.X + (b.X-a.X)*u, Y: a.Y + (b.Y-a.Y)*u}
				dot(p, 1, cyan)
			}
			dot(b, 5, cyan)
		}
		if math.Hypot(v.Player.X, v.Player.Y) <= 1 {
			dot(v.Player.brontasPoint, max(3, v.Player.Radius*radius), white)
		}
	} else {
		texts = append(texts, text("待定位", 30, 125, 126, 30, 15, white))
	}
	deadline := "--"
	if fresh && v.LethalAtMs > 0 {
		deadline = fmt.Sprintf("%.1f s", math.Max(0, float64(v.LethalAtMs-nowMs)/1000))
		if nowMs >= v.LethalAtMs {
			deadline = "已致命"
		}
	}
	texts = append(texts, text("最早射线致命", 178, 96, 128, 22, 12, white), text(deadline, 178, 124, 128, 40, 28, color.RGBA{R: 255, G: 191, B: 71, A: 255}))
	action, detail := v.Instruction, v.Detail
	if !fresh {
		action, detail = "数据过期 · 停止走位引导", "等待新的识别结果"
	}
	texts = append(texts, text(action, 10, 222, 300, 28, 16, white), text(detail, 10, 253, 300, 20, 11, white), text("地图方向 · 12↔6 / 3↔9 注意对侧", 10, 287, 300, 18, 11, white), text("箭头仅针对当前已识别射线", 10, 308, 300, 18, 11, white))
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, bounds)
	}
	return texts
}
