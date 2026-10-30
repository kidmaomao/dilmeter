package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Only the visible spawn cadence comes from the video. Damage deadlines,
// geometry and movement below are explicit demo parameters, not game facts.
func TestExportBrontasCadenceDemo(t *testing.T) {
	out := os.Getenv("DILMETER_BRONTAS_CADENCE_OUT")
	if out == "" {
		t.Skip("set DILMETER_BRONTAS_CADENCE_OUT to export")
	}
	var profile struct {
		WarningMs []int64          `json:"warningMs"`
		Axes      []string         `json:"axes"`
		Lanes     []int            `json:"lanes"`
		Grid      brontasGuideGrid `json:"grid"`
		LeadMs    []int64          `json:"illustrativeLeadMs"`
		ActiveMs  int64            `json:"illustrativeActiveMs"`
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "demos", "brontas-guide", "cadence-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if len(profile.WarningMs) == 0 || len(profile.WarningMs) != len(profile.Axes) || len(profile.Axes) != len(profile.Lanes) || len(profile.LeadMs) == 0 {
		t.Fatal("invalid cadence profile")
	}
	const base int64 = 1700000000000
	const duration int64 = 20000
	const start int64 = 2000
	const zeroAt int64 = start + 15000
	const stopAt int64 = 19000
	const tick int64 = 80
	const speed = 2.5
	type frame struct {
		AtMs        int64                   `json:"atMs"`
		Step        int                     `json:"step"`
		Caption     string                  `json:"caption"`
		Player      brontasPoint            `json:"player"`
		Rays        []brontasGuideRay       `json:"rays"`
		Guide       *nativeBrontasGuideView `json:"guide"`
		RemainingMs int64                   `json:"remainingMs"`
		Wave        int                     `json:"wave"`
		GapMs       int64                   `json:"gapMs"`
		LeadMs      int64                   `json:"leadMs"`
		Contact     bool                    `json:"contact"`
	}
	frames := []frame{}
	rays := []brontasGuideRay{}
	player := brontasPoint{X: 0, Y: -.35}
	var target *brontasPoint
	depart := int64(0)
	wave, gap, lead := 0, int64(0), int64(0)
	for ms := int64(0); ms < duration; ms += tick {
		if ms < start {
			player = brontasPoint{X: 0, Y: -2.5 + 2.15*math.Min(1, float64(ms)/1400)}
		}
		for wave < len(profile.WarningMs) && start+profile.WarningMs[wave] <= ms && ms < stopAt {
			at := start + profile.WarningMs[wave]
			if wave > 0 {
				gap = profile.WarningMs[wave] - profile.WarningMs[wave-1]
			}
			lead = profile.LeadMs[min(wave, len(profile.LeadMs)-1)]
			offset, width, err := profile.Grid.rayGeometry(profile.Axes[wave], profile.Lanes[wave])
			if err != nil {
				t.Fatal(err)
			}
			rays = append(rays, brontasGuideRay{ID: fmt.Sprintf("video-cadence-%02d", wave+1), Axis: profile.Axes[wave], WarningAtMs: base + at, LethalAtMs: base + at + lead, EndsAtMs: base + at + lead + profile.ActiveMs, Offset: &offset, HalfWidth: &width})
			wave++
		}
		active := []brontasGuideRay{}
		for _, ray := range rays {
			if base+ms < ray.EndsAtMs+brontasGuideGuardMs {
				active = append(active, ray)
			}
		}
		rays = active
		f := brontasGuideFrame{Session: "cadence-demo", Sequence: uint64(ms + 1), BossID: "demo-boss", Active: ms < stopAt, Clock: 12, ObservedAtMs: base + ms, GeometryComplete: true, Player: &brontasGuidePlayer{brontasPoint: player, Radius: .08, Speed: speed}, Rays: rays}
		v := planBrontasGuide(f, base+ms)
		// Scripted movement exists only in this opt-in test, never in the plugin.
		if ms >= start && ms < stopAt {
			if target == nil && v.Target != nil {
				q := *v.Target
				target = &q
				depart = ms + brontasGuideReactionMs
			}
			if target != nil && ms >= depart {
				d := math.Hypot(target.X-player.X, target.Y-player.Y)
				if d < .001 {
					target = nil
				} else {
					u := math.Min(1, speed*float64(tick)/1000/d)
					q := brontasPoint{X: player.X + (target.X-player.X)*u, Y: player.Y + (target.Y-player.Y)*u}
					// Stop rather than teleport through a currently active band.
					blocked := false
					for _, r := range rays {
						if base+ms >= r.LethalAtMs-brontasGuideGuardMs && base+ms <= r.EndsAtMs+brontasGuideGuardMs {
							a, b := player.X, q.X
							if r.Axis == "horizontal" {
								a, b = player.Y, q.Y
							}
							low, high := *r.Offset-*r.HalfWidth-.11, *r.Offset+*r.HalfWidth+.11
							if math.Max(math.Min(a, b), low) <= math.Min(math.Max(a, b), high) {
								blocked = true
							}
						}
					}
					if blocked {
						target = nil
					} else {
						player = q
						if u == 1 {
							target = nil
						}
					}
				}
			}
			f.Player.brontasPoint = player
			v = planBrontasGuide(f, base+ms)
		}
		contact := false
		for _, r := range rays {
			if base+ms >= r.LethalAtMs && base+ms < r.EndsAtMs {
				p := player.X
				if r.Axis == "horizontal" {
					p = player.Y
				}
				if math.Abs(p-*r.Offset) <= *r.HalfWidth+.08 {
					contact = true
				}
			}
		}
		step, caption := 0, "入场阶段：等待全员到位（示意）"
		switch {
		case ms >= stopAt:
			step, caption = 5, "演示结束；实战以成功判定为准"
			v.Target = nil
		case ms >= zeroAt:
			step, caption = 4, "倒计时归零：不提前假定已经无敌"
		case ms >= start+5000:
			step, caption = 3, "约 0.76 秒一条：新旧射线持续重叠"
		case ms >= start+2500:
			step, caption = 2, "出线加密：横纵线可能同时存在"
		case ms >= start:
			step, caption = 1, "开始完整 15 秒倒计时（示意）"
		}
		frames = append(frames, frame{base + ms, step, caption, player, rays, v, max(0, zeroAt-max(ms, start)), wave, gap, lead, contact})
	}
	result := struct {
		FPS        float64          `json:"fps"`
		BaseAtMs   int64            `json:"baseAtMs"`
		DurationMs int64            `json:"durationMs"`
		Mode       string           `json:"mode"`
		Grid       brontasGuideGrid `json:"grid"`
		Frames     []frame          `json:"frames"`
	}{12.5, base, duration, "cadence", profile.Grid, frames}
	data, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d cadence frames", len(frames))
}
