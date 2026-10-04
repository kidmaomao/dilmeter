package main

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func liveFighterSignal(signal string, atMs int64, value float32) *event.EventArcanaSignal {
	return &event.EventArcanaSignal{EventBase: event.EventBase{EventId: 22, Id: "player", At: atMs / 1000},
		Signal: signal, AtMs: atMs, Value: value, Rate: 3, Maximum: 400, Kind: 3}
}

func TestLiveBattleFighterCheckpoint(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 100}, Reliable: true})
	f.add(liveTestAppear("player", "fighter", 10001, 100))
	f.add(liveTestAppear("old", "old", 7603, 100))
	f.add(liveTestDamage("old", 101))
	f.add(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "old", At: 102}})
	f.add(liveFighterSignal("fighter-energy-baseline", 102250, 390))
	// Growth saturates at 400 before the 200-point spend; never add it
	// after consumption or overwrite this state with a later observation.
	f.add(liveFighterSignal("fighter-energy-delta", 107000, -200))
	start := liveFighterSignal("fighter-spend-start", 107000, 0)
	start.SkillId, start.CastAtMs = 59187, 107000
	f.add(start)
	f.add(liveTestAppear("new", "new", 7603, 107))
	f.add(liveTestDamage("new", 108))
	key := f.index.current.key
	end := liveFighterSignal("fighter-spend-end", 109000, 0)
	end.SkillId, end.CastAtMs, end.Complete = 59187, 107000, true
	f.add(end)
	f.add(liveFighterSignal("fighter-energy-delta", 110000, 12))
	_, rows := f.read("session=" + key)
	baseline, spend, finishes := 0, 0, 0
	for _, row := range rows {
		switch row["Signal"] {
		case "fighter-energy-baseline":
			baseline++
			if row["Value"] != float64(200) || row["AtMs"] != float64(107000) || row["Rate"] != float64(3) || row["Maximum"] != float64(400) {
				t.Fatalf("bad checkpoint: %v", row)
			}
		case "fighter-spend-start":
			spend++
		case "fighter-spend-end":
			finishes++
		}
	}
	if baseline != 1 || spend != 1 || finishes != 1 {
		t.Fatalf("context missing: %d/%d/%d", baseline, spend, finishes)
	}
	if f.index.actor("player").fighterSpend != nil {
		t.Fatal("finished spend retained")
	}
	// Subsequent windows carry the evolved value, without mutating archives.
	g := f.index.actor("player").fighterEnergy
	if g.value != 221 || g.atMs != 110000 {
		t.Fatalf("delta/natural growth=%+v", g)
	}
}

func TestLiveBattleFighterInvalidState(t *testing.T) {
	for _, scenario := range []string{"no-baseline", "resource-reset", "connection-reset", "identity-change", "overdraft", "nonfinite", "backwards"} {
		t.Run(scenario, func(t *testing.T) {
			f := newLiveBattleFixture(t)
			f.index.observe(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 100}})
			if scenario != "no-baseline" {
				f.index.observe(liveFighterSignal("fighter-energy-baseline", 100000, 100))
			}
			switch scenario {
			case "resource-reset":
				f.index.observe(liveFighterSignal("fighter-energy-reset", 101000, 0))
			case "connection-reset":
				f.index.observe(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 101}, Reset: true})
			case "identity-change":
				f.index.observe(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "other", At: 101}})
			case "overdraft":
				f.index.observe(liveFighterSignal("fighter-energy-delta", 101000, -200))
			case "nonfinite":
				f.index.observe(liveFighterSignal("fighter-energy-delta", 101000, float32(math.NaN())))
			case "backwards":
				f.index.observe(liveFighterSignal("fighter-energy-delta", 99000, 6))
			}
			f.index.observe(liveFighterSignal("fighter-energy-delta", 102000, 6))
			if g := f.index.actor("player").fighterEnergy; g != nil && (!g.inferred || g.baseline("player").Signal != "fighter-energy-bounds") {
				t.Fatal("missing energy was replaced by a fabricated exact baseline")
			}
			// Only an authoritative new baseline restores a valid gauge.
			f.index.observe(liveFighterSignal("fighter-energy-baseline", 103000, 0))
			if f.index.actor("player").fighterEnergy == nil {
				t.Fatal("fresh baseline not recovered")
			}
		})
	}
}

func TestLiveBattleFighterBoundsCheckpoint(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("player", "fighter", 10001, 98))
	f.add(liveTestAppear("old", "old", 7603, 98))
	f.add(liveTestDamage("old", 99))
	f.add(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "old", At: 100}})
	f.add(liveFighterSignal("fighter-energy-delta", 100000, -100))
	f.add(liveFighterSignal("fighter-energy-delta", 101000, 6))
	f.add(liveTestAppear("new", "new", 7603, 101))
	f.add(liveTestDamage("new", 102))
	_, rows := f.read("session=latest")
	found := false
	for _, row := range rows {
		if row["Signal"] == "fighter-energy-baseline" {
			t.Fatal("inferred state serialized as an observed baseline")
		}
		if row["Signal"] == "fighter-energy-bounds" {
			found = true
			if row["Value"] != float64(9) || row["UpperValue"] != float64(309) || row["AtMs"] != float64(101000) {
				t.Fatalf("bad bounds checkpoint: %v", row)
			}
		}
	}
	if !found {
		t.Fatal("bounds lost on encounter rotation")
	}
}

// Replay the same disk-window API used by the live UI, not just the complete
// log: a baseline before the first hit must survive an encounter rotation.
func TestLiveBattleFighterCapture(t *testing.T) {
	path := os.Getenv("DILMETER_FIGHTER_WINDOW_LOG")
	if path == "" {
		t.Skip("set DILMETER_FIGHTER_WINDOW_LOG to the 09:52:50 capture")
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	f := newLiveBattleFixture(t)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 65536), 4*1024*1024)
	for scanner.Scan() {
		var base event.EventBase
		if err := json.Unmarshal(scanner.Bytes(), &base); err != nil {
			t.Fatal(err)
		}
		var e event.IEvent = &base
		switch base.EventId {
		case 1:
			e = &event.EventEntityAppear{}
		case 2:
			e = &event.EventEntityDisappear{}
		case 3:
			e = &event.EventDamage{}
		case 4:
			e = &event.EventCharacterConditionEnable{}
		case 5:
			e = &event.EventCharacterConditionDisable{}
		case 6:
			e = &event.EventFinish{}
		case 7:
			e = &event.EventEntityEquipItem{}
		case 8:
			e = &event.EventEntityUnequipItem{}
		case 9:
			e = &event.EventEntityUpdateBody{}
		case 11:
			e = &event.EventLocalEntity{}
		case 17:
			e = &event.EventStatUpdate{}
		case 19:
			e = &event.EventCombatTarget{}
		case 22:
			e = &event.EventArcanaSignal{}
		}
		if err := json.Unmarshal(scanner.Bytes(), e); err != nil {
			t.Fatal(err)
		}
		if err := f.index.append(f.file, e, append(append([]byte(nil), scanner.Bytes()...), '\n')); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	var key string
	for _, window := range f.index.windows {
		if window.targets["4767482419336734"] != nil {
			key = window.key
		}
	}
	if key == "" {
		t.Fatal("fighter encounter missing")
	}
	response, rows := f.read("session=" + key)
	if path := os.Getenv("DILMETER_FIGHTER_WINDOW_OUTPUT"); path != "" {
		if err := os.WriteFile(path, response.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	baselines, deltas := 0, 0
	for _, row := range rows {
		switch row["Signal"] {
		case "fighter-energy-baseline":
			baselines++
		case "fighter-energy-delta":
			deltas++
		}
	}
	t.Logf("live encounter: rows=%d baselines=%d deltas=%d", len(rows), baselines, deltas)
	if baselines != 1 || deltas != 78 {
		t.Fatalf("pre-fight energy context lost: baselines=%d deltas=%d", baselines, deltas)
	}
}
