package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Axes and visible spawn intervals are sampled from the video's simulator.
// The public Canvas source verifies 1s to a single hit check and speed=5 radii/s.
// EndsAtMs uses a conservative 1ms proxy for that instantaneous check.
// Every fixed ray locks to THIS run's player position; recorded source lanes
// are evidence, never a script for where the next simulated ray will appear.
type brontasDemoRandom struct {
	WarningMs int64  `json:"warningMs"`
	Axis      string `json:"axis"`
	Lane      int    `json:"lane"`
}
type brontasCadenceProfile struct {
	RandomEvents []brontasDemoRandom `json:"randomEvents"`
	WarningMs    []int64             `json:"warningMs"`
	Axes         []string            `json:"axes"`
	Grid         brontasGuideGrid    `json:"grid"`
	LeadMs       []int64             `json:"illustrativeLeadMs"`
	ActiveMs     int64               `json:"illustrativeActiveMs"`
}
type brontasDemoLock struct {
	Kind   string       `json:"kind"`
	AtMs   int64        `json:"atMs"`
	Axis   string       `json:"axis"`
	Lane   int          `json:"lane"`
	Player brontasPoint `json:"player"`
}
type brontasCadenceFrame struct {
	RandomCount int                     `json:"randomCount"`
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
	Lock        *brontasDemoLock        `json:"lock,omitempty"`
	Motion      string                  `json:"motion"`
	DepartAtMs  int64                   `json:"departAtMs,omitempty"`
}
type brontasCadenceResult struct {
	FPS        float64               `json:"fps"`
	BaseAtMs   int64                 `json:"baseAtMs"`
	DurationMs int64                 `json:"durationMs"`
	Mode       string                `json:"mode"`
	Grid       brontasGuideGrid      `json:"grid"`
	Frames     []brontasCadenceFrame `json:"frames"`
	Locks      []brontasDemoLock     `json:"locks"`
	Validation string                `json:"validation"`
}

// Independent swept collision check: even a crossing BETWEEN output frames
// must fail the export. No freezing on contact, resurrection, or invulnerability.
func brontasDemoContact(a, b brontasPoint, t0, t1 int64, radius float64, r brontasGuideRay) bool {
	lo, hi := math.Max(float64(t0), float64(r.LethalAtMs)), math.Min(float64(t1), float64(r.EndsAtMs))
	if hi <= lo {
		return false
	}
	x, y := a.X, b.X
	if r.Axis == "horizontal" {
		x, y = a.Y, b.Y
	}
	at := func(t float64) float64 {
		if t1 == t0 {
			return x
		}
		return x + (y-x)*(t-float64(t0))/float64(t1-t0)
	}
	u, v := at(lo), at(hi)
	return math.Max(math.Min(u, v), *r.Offset-*r.HalfWidth-radius) <= math.Min(math.Max(u, v), *r.Offset+*r.HalfWidth+radius)
}

func runBrontasCadenceDemo(t *testing.T) brontasCadenceResult {
	t.Helper()
	var profile brontasCadenceProfile
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "demos", "brontas-guide", "cadence-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if len(profile.WarningMs) == 0 || len(profile.WarningMs) != len(profile.Axes) || len(profile.LeadMs) == 0 {
		t.Fatal("invalid cadence profile")
	}
	const base int64 = 1700000000000
	const start int64 = 2000
	const duration int64 = 20000
	const radius = 28.8 / 330
	const speed = 5.0
	result := brontasCadenceResult{FPS: 12.5, BaseAtMs: base, DurationMs: duration, Mode: "cadence", Grid: profile.Grid}
	// Integrate at <=10 ms AND exactly at each spawn, avoiding an 80 ms
	// rendering tick changing the player position used to lock a ray.
	timeline := map[int64]bool{}
	for ms := int64(0); ms <= duration; ms += 10 {
		timeline[ms] = true
	}
	for _, at := range profile.WarningMs {
		timeline[start+at] = true
	}
	for _, r := range profile.RandomEvents {
		timeline[start+r.WarningMs] = true
	}
	times := []int64{}
	for ms := range timeline {
		times = append(times, ms)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	player := brontasPoint{X: .6, Y: -.3}
	rays := []brontasGuideRay{}
	var target *brontasPoint
	var lock *brontasDemoLock
	depart, previous := int64(0), int64(0)
	wave, gap, lead := 0, int64(0), int64(0)
	moves, finishedAt := 0, int64(0)
	randomCount := 0
	for _, ms := range times {
		before := player
		if ms < start {
			u := math.Min(1, float64(ms)/1400)
			player = brontasPoint{X: .6 * u, Y: -3.6 + 3.3*u}
		} else if target != nil && ms > depart {
			dt := ms - max(previous, depart)
			if dt > 0 {
				d := math.Hypot(target.X-player.X, target.Y-player.Y)
				if d < 1e-9 {
					target = nil
				} else {
					u := math.Min(1, speed*float64(dt)/1000/d)
					player = brontasPoint{X: player.X + (target.X-player.X)*u, Y: player.Y + (target.Y-player.Y)*u}
					if u >= 1 {
						target = nil
					}
				}
			}
		}
		if ms >= start && previous >= start {
			if math.Hypot(player.X, player.Y) > 1-radius-.03+1e-8 {
				t.Fatalf("left circle at %d", ms)
			}
			for _, r := range rays {
				if brontasDemoContact(before, player, base+previous, base+ms, radius, r) {
					t.Fatalf("lethal contact at %d: %s; %+v -> %+v", ms, r.ID, before, player)
				}
			}
		}
		previous = ms
		for wave < len(profile.WarningMs) && start+profile.WarningMs[wave] == ms {
			if wave > 0 {
				gap = profile.WarningMs[wave] - profile.WarningMs[wave-1]
			}
			lead = profile.LeadMs[min(wave, len(profile.LeadMs)-1)]
			axis := profile.Axes[wave]
			lane, err := profile.Grid.laneAt(axis, player)
			if err != nil {
				t.Fatal(err)
			}
			offset, width, err := profile.Grid.rayGeometry(axis, lane)
			if err != nil {
				t.Fatal(err)
			}
			r := brontasGuideRay{ID: fmt.Sprintf("player-lock-%02d", wave+1), Axis: axis, WarningAtMs: base + ms, LethalAtMs: base + ms + lead, EndsAtMs: base + ms + lead + profile.ActiveMs, Offset: &offset, HalfWidth: &width}
			p := player.X
			if axis == "horizontal" {
				p = player.Y
			}
			if math.Abs(p-offset) > width+1e-9 {
				t.Fatal("fixed ray missed its locked player")
			}
			rays = append(rays, r)
			locked := brontasDemoLock{"fixed", base + ms, axis, lane, player}
			lock = &locked
			result.Locks = append(result.Locks, locked)
			wave++
		}
		for randomCount < len(profile.RandomEvents) && start+profile.RandomEvents[randomCount].WarningMs == ms {
			spec := profile.RandomEvents[randomCount]
			lane := spec.Lane
			parent := result.Locks[len(result.Locks)-1]
			if spec.Axis == parent.Axis && lane == parent.Lane {
				lane = (lane + 1) % 3
			}
			offset, width, err := profile.Grid.rayGeometry(spec.Axis, lane)
			if err != nil {
				t.Fatal(err)
			}
			rays = append(rays, brontasGuideRay{ID: fmt.Sprintf("random-%02d", randomCount+1), Axis: spec.Axis, WarningAtMs: base + ms, LethalAtMs: base + ms + 1000, EndsAtMs: base + ms + 1001, Offset: &offset, HalfWidth: &width})
			randomLock := brontasDemoLock{"random", base + ms, spec.Axis, lane, brontasPoint{}}
			lock = &randomLock
			randomCount++
		}
		active := []brontasGuideRay{}
		for _, r := range rays {
			if base+ms < r.EndsAtMs+brontasGuideGuardMs {
				active = append(active, r)
			}
		}
		rays = active
		f := brontasGuideFrame{Session: "position-locked-demo", Sequence: uint64(ms + 1), BossID: "demo-boss", Active: true, Clock: 12, ObservedAtMs: base + ms, GeometryComplete: true, Player: &brontasGuidePlayer{brontasPoint: player, Radius: radius, Speed: speed}, Rays: rays}
		v := planBrontasGuide(f, base+ms)
		if ms >= start {
			travelFrame := f
			if target != nil {
				arrival := base + max(ms, depart) + int64(math.Ceil(math.Hypot(target.X-player.X, target.Y-player.Y)/speed*1000))
				travelFrame.Rays = nil
				for _, r := range f.Rays {
					if r.LethalAtMs-brontasGuideGuardMs <= arrival+brontasGuideReactionMs {
						travelFrame.Rays = append(travelFrame.Rays, r)
					}
				}
			}
			if target != nil && !brontasRouteClearAfter(travelFrame, *target, base+ms, max(0, depart-ms)) {
				target = nil // A NEW observation invalidated the accepted route.
			}
			if target == nil && v.Target != nil {
				q := *v.Target
				target = &q
				depart = ms + brontasGuideReactionMs
				// Keep the locked lane until a guarded, latest departure. Moving
				// immediately would bait the next ray into another lane.
				for delay := brontasGuideReactionMs; delay <= 2000; delay += 10 {
					if brontasRouteClearAfter(f, q, base+ms, delay) {
						depart = ms + delay
					}
				}
				depart = max(ms+brontasGuideReactionMs, depart-60)
				moves++
			}
		}
		motion := "hold"
		if target != nil {
			motion = "reacting"
			if ms >= depart {
				motion = "moving"
			}
			// Display the accepted, revalidated route the demo is actually following.
			// Do not add a new 300 ms reaction on every animation frame.
			q := *target
			v.Target = &q
			v.Status = "move"
			v.Player = f.Player
			v.ValidUntilMs = base + ms + brontasGuideFreshMs
			v.Detail = "按已锁定射线校验路径；新线出现后重新检查"
		}
		if ms >= start && wave == len(profile.WarningMs) && len(rays) == 0 && target == nil && finishedAt == 0 {
			finishedAt = ms
		}
		step, caption := 0, "从中心方向进入落石区（示意）"
		switch {
		case finishedAt > 0:
			step, caption = 5, "本段全部射线结束 · 路径检测无命中"
		case ms >= start+15000:
			step, caption = 4, "倒计时归零后，仍等待射线实际结束"
		case ms >= start+5000:
			step, caption = 3, "新线锁定当前站位；旧线结束后才可回走"
		case ms >= start+2500:
			step, caption = 2, "出线加速：每次锁定后重新判断落点"
		case ms >= start:
			step, caption = 1, "固定线按当前所在列／行生成，锁定后不追踪"
		}
		if ms < duration {
			if ms%80 == 0 {
				result.Frames = append(result.Frames, brontasCadenceFrame{RandomCount: randomCount, AtMs: base + ms, Step: step, Caption: caption, Player: player, Rays: append([]brontasGuideRay(nil), rays...), Guide: v, RemainingMs: max(0, start+15000-max(start, ms)), Wave: wave, GapMs: gap, LeadMs: lead, Lock: lock, Motion: motion, DepartAtMs: base + depart})
			}
		}
	}
	if wave != len(profile.WarningMs) || randomCount != len(profile.RandomEvents) || finishedAt == 0 || moves < 10 {
		t.Fatalf("incomplete avoidance: waves %d, moves %d, finish %d", wave, moves, finishedAt)
	}
	result.Validation = "All fixed rays cover their locked player; continuous swept-path checks found zero lethal contacts; no circle exits after entry; no future rays supplied to the planner."
	t.Logf("%d position-locked rays; %d random rays; %d moves; all rays cleared at %.3fs; zero contacts", wave, randomCount, moves, float64(finishedAt)/1000)
	return result
}

func TestBrontasDemoPositionLockedContinuous(t *testing.T) { runBrontasCadenceDemo(t) }
func TestExportBrontasCadenceDemo(t *testing.T) {
	out := os.Getenv("DILMETER_BRONTAS_CADENCE_OUT")
	if out == "" {
		t.Skip("set DILMETER_BRONTAS_CADENCE_OUT to export")
	}
	result := runBrontasCadenceDemo(t)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, data, 0644); err != nil {
		t.Fatal(err)
	}
}
