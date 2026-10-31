package main

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOverlayStreamImmediateLatestAndCancellation(t *testing.T) {
	preserveBossPreviewState(t)
	setSkillOverlayState([]byte(`{"type":"skill-cooldown-state","atMs":100}`))
	server := httptest.NewServer(http.HandlerFunc(handleSkillOverlayStream))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	lines := make(chan nativeSkillOverlayMessage, 8)
	go func() {
		scanner := bufio.NewScanner(response.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				var m nativeSkillOverlayMessage
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m)
				lines <- m
			}
		}
	}()
	first := <-lines
	start := time.Now()
	setSkillOverlayState([]byte(`{"type":"skill-cooldown-state","atMs":101}`))
	select {
	case second := <-lines:
		if second.AtMs != 101 || second.Revision <= first.Revision {
			t.Fatal(second)
		}
		t.Logf("local server push: %s", time.Since(start))
	case <-time.After(time.Second):
		t.Fatal("snapshot waited for poll")
	}
	cancel()
	// Exercise backpressure independently of OS network buffering.
	ch := make(chan []byte, 1)
	skillOverlayState.Lock()
	skillOverlayState.listeners[ch] = true
	for i := 0; i < 100; i++ {
		publishSkillOverlaySnapshotLocked([]byte{byte(i)})
	}
	delete(skillOverlayState.listeners, ch)
	skillOverlayState.Unlock()
	if len(ch) != 1 || (<-ch)[0] != 99 {
		t.Fatal("slow client received obsolete backlog")
	}
	deadline := time.Now().Add(time.Second)
	for {
		skillOverlayState.RLock()
		n := len(skillOverlayState.listeners)
		skillOverlayState.RUnlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stream listener leaked")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestBrontasBoundedProjectionAndCorrection(t *testing.T) {
	p := nativeBrontasPosition{AtMs: 1000, SpeedAtMs: 1000, RegionID: 1, X: 25000, Y: 40500, DestX: 26000, DestY: 40500, Moving: true, Mode: 1, Speed: 1000, SpeedSamples: 2}
	for _, sample := range []struct {
		at int64
		x  float64
	}{{1000, 25000}, {1100, 25100}, {1300, 25300}, {1600, 25600}, {1750, 25700}} {
		if got := brontasPositionAt(p, sample.at); math.Abs(got.X-sample.x) > 1e-9 || got.Y != p.Y {
			t.Fatal(sample, got)
		}
	}
	p.DestX = 25120
	if got := brontasPositionAt(p, 1300); got.X != 25120 {
		t.Fatal("overshot destination", got)
	}
	p.SpeedSamples = 0
	if got := brontasPositionAt(p, 1300); got.X != p.X {
		t.Fatal("uncalibrated prediction", got)
	}
	p.SpeedSamples = 2
	p.Moving = false
	if got := brontasPositionAt(p, 1300); got.X != p.X {
		t.Fatal("stationary sample extrapolated", got)
	}
	r := brontasLiveFixture()
	r.localID = "self"
	r.brontasPosition = p
	// A correction must replace the projected point, not ease it through old cells.
	p.X = 25200
	p.Y = 40800
	p.AtMs = 1250
	p.Moving = false
	r.brontasPosition = p
	got, _ := r.brontasStationPlayer(12, 1300)
	if got == nil || math.Abs(got.X-(25200.0-(19661+6*846+423))/1269) > 1e-9 {
		t.Fatal("correction was delayed", got)
	}
	m := brontasStationMotion(nativeBrontasPosition{AtMs: 1000, SpeedAtMs: 1000, X: 25000, Y: 40500, DestX: 26000, DestY: 40500, Moving: true, Mode: 1, Speed: 1000, SpeedSamples: 2}, 12, 1100)
	v := &nativeBrontasGuideView{Clock: 12, Motion: m}
	orientBrontasStation(v, "south")
	if v.Motion.From.X != -m.From.X || v.Motion.To.Y != -m.To.Y {
		t.Fatal("motion did not rotate with grid")
	}
}

func TestBrontasExperimentalEndDoesNotResurrect(t *testing.T) {
	r := brontasLiveFixture()
	e := brontasLiveEvent(102000)
	r.observeBrontasRay(e)
	if r.brontasLiveRays.Rays[0].EndsAtMs != 104290 {
		t.Fatal("wrong experimental tail")
	}
	newer := brontasLiveEvent(104500)
	r.observeBrontasRay(newer)
	if len(r.brontasLiveRays.Rays) != 1 {
		t.Fatal("old ray not pruned")
	}
	r.observeBrontasRay(e)
	if len(r.brontasLiveRays.Rays) != 1 {
		t.Fatal("duplicate resurrected an ended line")
	}
}

func TestBrontasAlreadyMovingDoesNotRestartReaction(t *testing.T) {
	v := stationGuideFixture()
	v.Player.X = .4
	v.Rays = v.Rays[1:]
	offset := 2.0 / 3
	v.Rays[0].Offset = &offset
	v.Rays[0].LethalAtMs = 1450
	target := brontasPoint{X: 0, Y: 0}
	if ok, _ := brontasGridRoute(v, target, 1000); ok {
		t.Fatal("standing departure should be too late")
	}
	v.Motion = &brontasGuideMotion{From: v.Player.brontasPoint, To: target, AtMs: 1000, UntilMs: 1350, Speed: 1}
	if ok, _ := brontasGridRoute(v, target, 1000); !ok {
		t.Fatal("continued movement restarted reaction delay")
	}
	v.Motion.To = brontasPoint{X: .8}
	if ok, _ := brontasGridRoute(v, target, 1000); ok {
		t.Fatal("turning also skipped reaction")
	}
}
