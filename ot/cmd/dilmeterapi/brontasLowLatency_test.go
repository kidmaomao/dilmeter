package main

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestBrontasPauseDoesNotReduceMeasuredRunSpeed(t *testing.T) {
	a := nativeBrontasPosition{AtMs: 1000, RegionID: 1, Mode: 1, X: 20000, Y: 36000, DestX: 21000, DestY: 36000, Moving: true, Speed: 1000, SpeedSamples: 3, SpeedAtMs: 1000}
	b := a
	b.AtMs = 1500
	b.X = 20300
	b.DestX = 22000
	// The prior destination was already reached before the next click.
	a.DestX = 20300
	b.Speed, b.SpeedSamples, b.SpeedAtMs = brontasPositionSpeed(a, b)
	if b.Speed != 1000 || b.SpeedSamples != 3 || b.SpeedAtMs != 1000 {
		t.Fatal("standing time reduced run speed", b)
	}
	c := b
	c.AtMs = 1700
	c.X = 20510
	c.Speed, c.SpeedSamples, c.SpeedAtMs = brontasPositionSpeed(b, c)
	if c.Speed <= 1000 || c.Speed >= 1050 {
		t.Fatal("new movement did not update estimate", c)
	}
	stopped := c
	stopped.AtMs = 1800
	stopped.Moving = false
	stopped.Speed, stopped.SpeedSamples, stopped.SpeedAtMs = brontasPositionSpeed(c, stopped)
	if stopped.SpeedSamples < 2 || brontasPositionAt(stopped, 2000).X != stopped.X {
		t.Fatal("stop lost calibration or kept moving")
	}
}

func TestBrontasLateExitNeverClaimsSafeOrCrossesNewActiveRay(t *testing.T) {
	v := stationGuideFixture()
	v.Rays = v.Rays[:1]
	v.Rays[0].Axis = "vertical"
	v.Rays[0].LethalAtMs = 900
	v.Rays[0].EndsAtMs = 2200
	planBrontasStationGuide(v, 1000)
	if v.Target == nil || v.GuidanceKind != "escape" || math.Abs(v.Target.X) < .45 {
		t.Fatal("no urgent exit", v)
	}
	original := v.Player.brontasPoint
	candidate := brontasPoint{X: .67, Y: 0}
	offset, width := .67, 1.0/3
	v.Rays = append(v.Rays, brontasGuideRay{Axis: "vertical", Offset: &offset, HalfWidth: &width, LethalAtMs: 900, EndsAtMs: 3000})
	if _, ok := brontasExposure(v, candidate, 1000, true); ok {
		t.Fatal("emergency crossed a newly entered active ray")
	}
	v.Player.brontasPoint = original
	v.GuidanceValidUntilMs = 999
	planBrontasStationGuide(v, 1000)
	if v.Target != nil {
		t.Fatal("emergency used stale location")
	}
}

func TestBrontasRealPacketProjectionComparison(t *testing.T) {
	path := os.Getenv("DILMETER_BRONTAS_LOCAL_SAMPLES")
	if path == "" {
		t.Skip("optional private local movement samples")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var old, nextState nativeBrontasPosition
	totalOld, totalNew, n := 0.0, 0.0, 0
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 4096), 1024*1024)
	for scan.Scan() {
		var raw struct {
			At  int64             `json:"at"`
			Op  uint64            `json:"op"`
			Msg []json.RawMessage `json:"msg"`
		}
		if err := json.Unmarshal(scan.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		move := raw.Op == 256162723 || raw.Op == 265367585
		fix := raw.Op == 26004 || raw.Op == 26009
		if !move && !fix {
			continue
		}
		var m []float64
		data, _ := json.Marshal(raw.Msg)
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		p := nativeBrontasPosition{AtMs: raw.At, RegionID: nextState.RegionID, Mode: nextState.Mode}
		if move && len(m) == 6 {
			p.X, p.Y, p.DestX, p.DestY = m[0], m[1], m[2], m[3]
			p.Moving = p.X != p.DestX || p.Y != p.DestY
			p.Mode = raw.Op
		} else if fix && len(m) == 4 && m[0] == 1 {
			p.RegionID = uint32(m[1])
			p.X, p.Y = m[2], m[3]
		} else {
			continue
		}
		dt := p.AtMs - nextState.AtMs
		inArena := p.X >= 19661 && p.X <= 30661 && p.Y >= 31238 && p.Y <= 42238
		if inArena && dt >= 80 && dt <= 700 && nextState.Moving && p.RegionID == nextState.RegionID && p.Mode == nextState.Mode {
			predictedOld := old
			if old.SpeedSamples >= 2 && p.AtMs-old.SpeedAtMs < 1000 {
				predictedOld = brontasPositionAt(old, p.AtMs)
			}
			predictedNew := brontasPositionAt(nextState, p.AtMs)
			totalOld += math.Hypot(predictedOld.X-p.X, predictedOld.Y-p.Y)
			totalNew += math.Hypot(predictedNew.X-p.X, predictedNew.Y-p.Y)
			n++
		}
		fresh := p
		fresh.Speed, fresh.SpeedSamples, fresh.SpeedAtMs = brontasPositionSpeed(nextState, p)
		// The pr1.5 estimator, retained only here as a fixed comparison baseline.
		baseline := p
		distance := math.Hypot(p.X-old.X, p.Y-old.Y)
		speed := distance * 1000 / float64(p.AtMs-old.AtMs)
		if old.Moving && p.Moving && p.RegionID == old.RegionID && p.Mode == old.Mode && p.AtMs-old.AtMs >= 80 && p.AtMs-old.AtMs <= 700 && distance >= 40 && speed >= 100 && speed <= 3000 {
			baseline.SpeedSamples = old.SpeedSamples + 1
			if old.SpeedSamples > 0 {
				if speed < old.Speed*.70 || speed > old.Speed*1.43 {
					baseline.SpeedSamples = 1
				} else {
					speed = math.Min(old.Speed, speed)
				}
			}
			baseline.Speed = speed
			baseline.SpeedAtMs = p.AtMs
		}
		old, nextState = baseline, fresh
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if n < 10 {
		t.Fatal("too few matched movement pairs", n)
	}
	t.Logf("matchedPairs=%d priorMeanError=%.2f newMeanError=%.2f reduction=%.1f%% (world-coordinate units, not measured end-to-end latency)", n, totalOld/float64(n), totalNew/float64(n), (1-totalNew/totalOld)*100)
	if totalNew >= totalOld {
		t.Fatal("new projection did not improve this capture")
	}
}

func BenchmarkBrontasDenseGuidance(b *testing.B) {
	offset, width := 0.0, 1.0/3
	v := stationGuideFixture()
	for i := 0; i < 9; i++ {
		o := offset + float64(i%3-1)*2/3
		axis := "vertical"
		if i%2 == 0 {
			axis = "horizontal"
		}
		v.Rays = append(v.Rays, brontasGuideRay{Axis: axis, Offset: &o, HalfWidth: &width, LethalAtMs: 900 + int64(i)*200, EndsAtMs: 1500 + int64(i)*200})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy := *v
		planBrontasStationGuide(&copy, 1000)
	}
}
