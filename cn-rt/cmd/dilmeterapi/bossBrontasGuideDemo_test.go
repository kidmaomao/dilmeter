package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// This opt-in exporter drives the production planner with deterministic demo
// observations. Normal tests do not write files or generate synthetic events.
// DILMETER_BRONTAS_DEMO_OUT is a JSON output path, consumed by
// docs/demos/brontas-guide/render.mjs. All displayed timing is illustrative.
func TestExportBrontasGuideDemo(t *testing.T) {
	out := os.Getenv("DILMETER_BRONTAS_DEMO_OUT")
	if out == "" {
		t.Skip("set DILMETER_BRONTAS_DEMO_OUT to export the animated demonstration")
	}
	const base int64 = 1700000000000
	type demoFrame struct {
		AtMs    int64                   `json:"atMs"`
		Step    int                     `json:"step"`
		Caption string                  `json:"caption"`
		Player  brontasPoint            `json:"player"`
		Rays    []brontasGuideRay       `json:"rays"`
		Guide   *nativeBrontasGuideView `json:"guide"`
	}
	frames := []demoFrame{}
	player := brontasPoint{X: 0, Y: -.35}
	var target *brontasPoint
	var origin brontasPoint
	var lastWave string
	var frozen brontasGuideFrame
	for ms := int64(0); ms < 16000; ms += 80 {
		f := brontasGuideFrame{Session: "demo", Sequence: uint64(ms + 1), BossID: "demo-boss", Active: true, Clock: 12, ObservedAtMs: base + ms, GeometryComplete: true}
		step, caption := 0, "进入已分配的落石点，等待射线"
		wave, start, lethal, end := "", int64(0), int64(0), int64(0)
		axis := "vertical"
		switch {
		case ms < 1600:
			player = brontasPoint{X: 0, Y: -2.5 + 2.15*math.Min(1, float64(ms)/1200)}
		case ms < 4240:
			wave, start, lethal, end = "vertical", 1600, 3200, 4240
			step, caption = 1, "纵线出现 → 横向移出；以实际时间为准"
		case ms < 4480:
			step, caption = 1, "本条射线结束，继续观察"
		case ms < 6960:
			wave, start, lethal, end, axis = "horizontal", 4480, 5730, 6960, "horizontal"
			step, caption = 2, "横线出现 → 纵向移位，保持在落石区内"
		case ms < 7200:
			step, caption = 2, "等待新的观察，不预判下一条射线"
		case ms < 9280:
			wave, start, lethal, end = "cross", 7200, 8200, 9280
			step, caption = 3, "宽射线分出九宫格 → 下方两个角很小"
		case ms < 9600:
			step, caption = 3, "当前已识别射线结束"
		case ms < 11520:
			wave, start, lethal, end = "axis-only", 9600, 11200, 11520
			step, caption = 4, "只有方向和时间 → 不显示具体落点"
		case ms < 14400:
			wave, start, lethal, end = "stale", 11520, 13120, 14400
			step, caption = 5, "识别中断 → 500 ms 内撤回旧箭头"
		default:
			step, caption = 5, "等待真实识别数据，旧路线不再显示"
		}
		if wave != "" {
			if wave != lastWave {
				origin = player
				target = nil
			}
			offset := origin.X
			if axis == "horizontal" {
				offset = origin.Y
			}
			// Illustrative proportions from the user's sketch, not game measurements.
			// Half-width / zone radius also equals full width / zone diameter.
			width := .40
			if axis == "horizontal" {
				width = .51
			}
			if wave == "cross" {
				offset = 0
			}
			f.Rays = []brontasGuideRay{{ID: wave, Axis: axis, WarningAtMs: base + start, LethalAtMs: base + lethal, EndsAtMs: base + end, Offset: &offset, HalfWidth: &width}}
			if wave == "cross" {
				y, horizontalWidth := -.12, .51
				f.Rays = append(f.Rays, brontasGuideRay{ID: "additional-ray", Axis: "horizontal", WarningAtMs: base + start, LethalAtMs: base + lethal, EndsAtMs: base + end, Offset: &y, HalfWidth: &horizontalWidth})
			}
			f.Player = &brontasGuidePlayer{brontasPoint: player, Radius: .08, Speed: 1.2}
			if wave != lastWave && wave != "axis-only" {
				v := planBrontasGuide(f, base+ms)
				if v.Target != nil {
					q := *v.Target
					target = &q
				}
				if wave != "stale" && target == nil {
					t.Fatalf("demo %s has no initial route: %+v", wave, v)
				}
			}
			if target != nil && wave != "stale" && wave != "axis-only" {
				d := math.Hypot(target.X-origin.X, target.Y-origin.Y)
				u := math.Max(0, math.Min(1, float64(ms-start-brontasGuideReactionMs)/1000*1.2/d))
				player = brontasPoint{X: origin.X + (target.X-origin.X)*u, Y: origin.Y + (target.Y-origin.Y)*u}
				f.Player.brontasPoint = player
			}
			if wave == "axis-only" {
				f.GeometryComplete = false
				f.Player = nil
				f.Rays[0].Offset = nil
				f.Rays[0].HalfWidth = nil
			}
			if wave == "stale" {
				if wave != lastWave {
					frozen = f
				}
				f = frozen
			}
		} else {
			f.Player = &brontasGuidePlayer{brontasPoint: player, Radius: .08, Speed: 1.2}
			if ms >= 14400 {
				f.Active = false
			}
		}
		lastWave = wave
		v := planBrontasGuide(f, base+ms)
		if ms < 1600 {
			v.Instruction = "进入已分配的落石区"
			v.Detail = "区外路线尚未校准"
		}
		if step == 5 && base+ms >= v.ValidUntilMs {
			v.Instruction = "数据过期 · 停止走位引导"
			v.Detail = "等待新的识别结果"
		}
		frames = append(frames, demoFrame{AtMs: base + ms, Step: step, Caption: caption, Player: player, Rays: f.Rays, Guide: v})
	}
	data, err := json.MarshalIndent(struct {
		FPS      float64     `json:"fps"`
		BaseAtMs int64       `json:"baseAtMs"`
		Frames   []demoFrame `json:"frames"`
	}{12.5, base, frames}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d deterministic frames to %s", len(frames), out)
}
