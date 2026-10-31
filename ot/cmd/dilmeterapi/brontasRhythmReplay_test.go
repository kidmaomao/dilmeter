package main

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

// Local opt-in replay keeps private captures outside the source tree/package.
// Include the same 25ms between-packet evaluations as the desktop runtime.
func TestBrontasRhythmEventReplay(t *testing.T) {
	path := os.Getenv("DILMETER_BRONTAS_RHYTHM_LOG")
	if path == "" {
		t.Skip("optional local event recording")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r := brontasLiveFixture()
	r.brontasRocks = nil
	r.brontasPosition = nativeBrontasPosition{}
	r.localID = ""
	var out *os.File
	if name := os.Getenv("DILMETER_BRONTAS_RHYTHM_OUT"); name != "" {
		out, err = os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
	}
	var enc *json.Encoder
	if out != nil {
		enc = json.NewEncoder(out)
	}
	frames, arrows, turns, lastCell, lastTarget := 0, 0, 0, -1, -1
	firstAt := int64(0)
	lastAt := int64(0)
	lastRay := int64(0)
	rays := 0
	emit := func(at int64) {
		v := r.brontasLiveRayView(at)
		if v == nil || v.Stage != "station" {
			return
		}
		frames++
		if v.Target != nil && v.Player != nil {
			arrows++
			cell := brontasGridCell(v.Player.X) + 3*brontasGridCell(v.Player.Y)
			target := brontasGridCell(v.Target.X) + 3*brontasGridCell(v.Target.Y)
			if cell == lastCell && target != lastTarget {
				turns++
			}
			lastCell, lastTarget = cell, target
			if brontasStepAxis(v.Player.brontasPoint, *v.Target) == "" {
				t.Fatal("non-grid current step")
			}
		}
		if enc != nil {
			if err := enc.Encode(struct {
				AtMs int64                   `json:"atMs"`
				View *nativeBrontasGuideView `json:"view"`
			}{at, v}); err != nil {
				t.Fatal(err)
			}
		}
	}
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 4096), 4*1024*1024)
	for scan.Scan() {
		var header struct {
			EventId  int
			At, AtMs int64
		}
		if json.Unmarshal(scan.Bytes(), &header) != nil {
			t.Fatal("invalid event JSON")
		}
		var e event.IEvent
		switch header.EventId {
		case 1:
			e = &event.EventEntityAppear{}
		case 2:
			e = &event.EventEntityDisappear{}
		case 6:
			e = &event.EventFinish{}
		case 11:
			e = &event.EventLocalEntity{}
		case 17:
			e = &event.EventStatUpdate{}
		case 24:
			e = &event.EventWorldProp{}
		case 25:
			e = &event.EventBrontasRay{}
		default:
			continue
		}
		if err := json.Unmarshal(scan.Bytes(), e); err != nil {
			t.Fatal(err)
		}
		at := header.AtMs
		if at == 0 {
			at = header.At * 1000
		}
		at = max(at, lastAt)
		if firstAt == 0 {
			firstAt = at
		}
		for tick := lastAt + 25; lastAt > 0 && tick < at && tick < lastRay+4000; tick += 25 {
			emit(tick)
		}
		if ray, ok := e.(*event.EventBrontasRay); ok {
			ray.Live = true
			if ray.Kind == "ray" {
				lastRay = at
				rays++
			}
		}
		r.onEvent(e)
		emit(at)
		lastAt = at
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if rays == 0 || frames == 0 {
		t.Fatal("no real rays or station frames")
	}
	t.Logf("durationMs=%d rays=%d views=%d arrowViews=%d targetChangesWhileSameCell=%d", lastAt-firstAt, rays, frames, arrows, turns)
}
