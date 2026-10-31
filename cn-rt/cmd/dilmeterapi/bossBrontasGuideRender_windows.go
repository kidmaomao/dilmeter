package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Same compact two-stage view as the WebView, also used by native fallback.
func drawNativeBrontasGuide(canvas *image.RGBA, item nativeBossMechanicOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	snapshot := *item.BrontasGuide
	v := &snapshot
	if v.Player != nil && v.Motion != nil {
		p, m := *v.Player, v.Motion
		dx, dy := m.To.X-m.From.X, m.To.Y-m.From.Y
		distance := math.Hypot(dx, dy)
		travel := math.Min(distance, m.Speed*float64(max(0, min(nowMs, m.UntilMs)-m.AtMs))/1000)
		p.brontasPoint = m.From
		if distance > 0 {
			p.X += dx / distance * travel
			p.Y += dy / distance * travel
		}
		v.Player = &p
		if math.Abs(p.X) >= 1 || math.Abs(p.Y) >= 1 {
			v.Player = nil
		}
	}
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	x, y := item.X+offset.X, item.Y+offset.Y
	rect := func(l, t, w, h float64) image.Rectangle {
		return image.Rect(x+int(l*factor), y+int(t*factor), x+int((l+w)*factor), y+int((t+h)*factor))
	}
	width, height := burstPopupDimensions(item)
	bounds := rect(0, 0, float64(width), float64(height))
	fillNativeOverlayRect(canvas, bounds, color.RGBA{9, 18, 27, 240})
	drawNativeOverlayBorder(canvas, bounds, color.RGBA{86, 120, 141, 255}, 1)
	white, muted, gold := color.RGBA{236, 245, 250, 255}, color.RGBA{155, 182, 195, 255}, color.RGBA{255, 191, 87, 255}
	text := func(s string, l, t, w, h, size float64, c color.RGBA) nativeReminderText {
		return nativeReminderText{text: s, rect: imageRectToNative(rect(l, t, w, h)), color: nativeColorRef(c.R, c.G, c.B), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(9, int(size*factor)), fontWeight: nativeFontBold}
	}
	texts := []nativeReminderText{text("二王 50%", 12, 12, 110, 24, 15, white)}
	station := v.Stage == "station" || (v.Stage == "" && v.Clock != 0 && v.Player != nil)
	if item.PreviewExpiresAtMs > 0 {
		texts = append(texts, text("预览", 120, 12, 42, 24, 11, muted))
	}
	if station {
		texts = append(texts, text(fmt.Sprintf("%d点", v.Clock), 256, 12, 48, 24, 13, muted))
	}
	cx, cy, scale := 104.0, 124.0, 74.58
	if !station {
		cx = 160
	}
	radius := 1.05
	if station {
		scale = 55.15
		radius = 1.42
	}
	fresh := nowMs < v.ValidUntilMs
	for py := -79; py <= 79; py++ {
		for px := -79; px <= 79; px++ {
			d := math.Hypot(float64(px), float64(py))
			if d > scale*radius {
				continue
			}
			shade := color.RGBA{16, 30, 40, 255}
			if d > scale*radius-1.5 {
				shade = color.RGBA{100, 134, 152, 255}
			}
			gx, gy := float64(px)/(scale), -float64(py)/(scale)
			if station {
				if fresh {
					for _, ray := range v.Rays {
						if ray.Offset == nil || ray.HalfWidth == nil || (ray.EndsAtMs > 0 && ray.EndsAtMs <= nowMs) {
							continue
						}
						c := gx
						if ray.Axis == "horizontal" {
							c = gy
						}
						low, high := *ray.Offset-*ray.HalfWidth, *ray.Offset+*ray.HalfWidth
						if *ray.Offset < -.5 {
							low = -1.42
						}
						if *ray.Offset > .5 {
							high = 1.42
						}
						if c >= low && c < high {
							if ray.LethalAtMs <= nowMs {
								shade = color.RGBA{128, 55, 70, 255}
							} else if shade != (color.RGBA{128, 55, 70, 255}) {
								shade = color.RGBA{128, 97, 41, 255}
								if ray.LethalAtMs-nowMs > 1100 {
									shade = color.RGBA{44, 95, 123, 255}
								}
							}
						}
					}
				}
				for _, edge := range []float64{-1.0 / 3, 1.0 / 3} {
					if math.Abs(gx-edge) < .018 || math.Abs(gy-edge) < .018 {
						shade = color.RGBA{103, 132, 145, 255}
					}
				}
			}
			fillNativeOverlayRect(canvas, rect(cx+float64(px), cy+float64(py), 1.5, 1.5), shade)
		}
	}
	dot := func(xx, yy, radius float64, c color.RGBA) {
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy <= radius*radius {
					fillNativeOverlayRect(canvas, rect(xx+dx, yy+dy, 1.5, 1.5), c)
				}
			}
		}
	}
	if !station {
		for _, clock := range []int{12, 3, 6, 9} {
			dx, dy := 0.0, 0.0
			switch clock {
			case 12:
				dy = -.76
			case 3:
				dx = .76
			case 6:
				dy = .76
			case 9:
				dx = -.76
			}
			shade, ink := color.RGBA{25, 43, 53, 255}, muted
			for _, rock := range item.Rocks {
				if rock.Clock == clock {
					shade = gold
					ink = color.RGBA{23, 34, 42, 255}
				}
			}
			xx, yy := cx+dx*scale, cy+dy*scale
			dot(xx, yy, .21*scale, shade)
			texts = append(texts, text(fmt.Sprint(clock), xx-18, yy-12, 36, 24, 13, ink))
		}
		texts = append(texts, text("落石", cx-30, cy-12, 60, 24, 11, muted))
	} else {
		located := fresh && v.Player != nil && (v.PlayerValidUntilMs == 0 || nowMs < v.PlayerValidUntilMs)
		cue := brontasGuideCue(v, nowMs)
		guideColor, cueBackground := muted, color.RGBA{38, 50, 60, 255}
		switch cue.Mode {
		case "wait":
			guideColor, cueBackground = color.RGBA{142, 203, 255, 255}, color.RGBA{25, 51, 75, 255}
		case "go":
			guideColor, cueBackground = color.RGBA{140, 244, 154, 255}, color.RGBA{22, 58, 38, 255}
		case "urgent":
			guideColor, cueBackground = color.RGBA{255, 113, 133, 255}, color.RGBA{69, 30, 42, 255}
		}
		if located && v.Target != nil && (cue.Mode == "go" || cue.Mode == "urgent") {
			targetPoint, nextPoint := brontasDisplayedStep(v)
			if targetPoint != nil {
				target := *targetPoint
				dot(cx+target.X*scale, cy-target.Y*scale, 4.5, guideColor)
				ax, ay := cx+v.Player.X*scale, cy-v.Player.Y*scale
				bx, by := cx+target.X*scale, cy-target.Y*scale
				line := func(x1, y1, x2, y2 float64) {
					for u := 0.0; u <= 1; u += .015 {
						dot(x1+(x2-x1)*u, y1+(y2-y1)*u, 1.3, guideColor)
					}
				}
				line(ax, ay, bx, by)
				if nextPoint != nil {
					cx2, cy2 := cx+nextPoint.X*scale, cy-nextPoint.Y*scale
					for u := 0.0; u < 1; u += .16 {
						end := math.Min(1, u+.08)
						line(bx+(cx2-bx)*u, by+(cy2-by)*u, bx+(cx2-bx)*end, by+(cy2-by)*end)
					}
					dot(cx2, cy2, 2.5, guideColor)
				}
				d := math.Hypot(bx-ax, by-ay)
				if d > .01 {
					dx, dy := (bx-ax)/d, (by-ay)/d
					line(bx, by, bx-dx*9-dy*5, by-dy*9+dx*5)
					line(bx, by, bx-dx*9+dy*5, by-dy*9-dx*5)
				}
			}
		}
		ring := func(x, y, radius float64) {
			for angle := 0.0; angle < 2*math.Pi; angle += .015 {
				dot(x+math.Cos(angle)*radius, y+math.Sin(angle)*radius, .8, guideColor)
			}
		}
		ring(cx, cy, 1.46*scale)
		if located && cue.Mode == "wait" {
			ring(cx+v.Player.X*scale, cy-v.Player.Y*scale, .17*scale)
		}
		// The action strip has its own allocated height in burstPopupDimensions,
		// so the native window and drag area include it at every display scale.
		fillNativeOverlayRect(canvas, rect(12, 216, 296, 50), cueBackground)
		drawNativeOverlayBorder(canvas, rect(12, 216, 296, 50), guideColor, 1)
		if cue.Mode == "wait" {
			fillNativeOverlayRect(canvas, rect(30, 231, 4, 20), guideColor)
			fillNativeOverlayRect(canvas, rect(40, 231, 4, 20), guideColor)
		} else if cue.Mode == "go" || cue.Mode == "urgent" {
			line := func(a, b, c, d float64) {
				for u := 0.0; u <= 1; u += .025 {
					x, y := a+(c-a)*u, b+(d-b)*u
					dot(37+x*math.Cos(cue.Angle)-y*math.Sin(cue.Angle), 241+x*math.Sin(cue.Angle)+y*math.Cos(cue.Angle), 1.4, guideColor)
				}
			}
			line(-11, 0, 11, 0)
			line(3, -8, 11, 0)
			line(3, 8, 11, 0)
		} else {
			texts = append(texts, text("!", 21, 224, 32, 32, 24, guideColor))
		}
		texts = append(texts, text(cue.Title, 58, 218, 140, 27, 20, guideColor), text(cue.Detail, 58, 245, 140, 17, 11, guideColor))
		if cue.Seconds != "" {
			texts = append(texts, text(cue.Seconds+" s", 212, 226, 84, 30, 24, guideColor))
		} else if cue.Mode == "go" || cue.Mode == "urgent" {
			texts = append(texts, text("一格", 216, 230, 78, 24, 16, guideColor))
		}
		texts = append(texts, text("距下一次判定", 202, 40, 100, 18, 11, muted))
		if located {
			dot(cx+v.Player.X*scale, cy-v.Player.Y*scale, 4, white)
		}
		horizontal, vertical := brontasFootRayDeadlines(v, nowMs)
		line := 0
		for _, timer := range []struct {
			label string
			at    int64
		}{{"横线", horizontal}, {"竖线", vertical}} {
			if timer.at == 0 {
				continue
			}
			top := 60.0 + float64(line)*70
			line++
			shade := gold
			if timer.at-nowMs <= 700 {
				shade = color.RGBA{255, 117, 128, 255}
			}
			texts = append(texts, text(timer.label, 206, top, 90, 20, 11, muted), text(fmt.Sprintf("%.1f s", float64(timer.at-nowMs)/1000), 206, top+22, 90, 36, 27, shade))
		}
		if line == 0 {
			label := "脚下未预警"
			if !fresh {
				label = "信号中断"
			} else if !located {
				label = "定位中"
			}
			texts = append(texts, text("--", 206, 95, 90, 36, 28, muted), text(label, 206, 136, 90, 20, 11, muted))
		}
	}
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, bounds)
	}
	return texts
}
