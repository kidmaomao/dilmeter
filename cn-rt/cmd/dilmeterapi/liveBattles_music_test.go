package main

import (
	"bufio"
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"os"
	"path/filepath"
	"testing"
)

func TestOpeningPerformancesSurviveWindowAndSongReplacement(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 100}, Reliable: true})
	f.add(liveTestAppear("player", "player", 9001, 100))
	f.add(liveTestAppear("old", "old", 7603, 100))
	f.add(liveTestDamage("old", 101))
	f.add(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "old", At: 102}})
	song := func(id string, at, expiry int64, cc uint32, metadata string) {
		f.add(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: id, At: at}, CCId: cc, DisableAt: expiry, AttackerId: "player", Metadata: metadata})
	}
	song("player", 103, 403, 680, "MCMBAMAX:f:90;")
	song("non-attacking-recipient", 103, 403, 680, "MCMBAMAX:f:90;")
	f.add(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "player", At: 104}, CCId: 680})
	f.add(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "non-attacking-recipient", At: 104}, CCId: 680})
	song("player", 104, 404, 192, "MFCP:f:80;LSMA:f:85;")
	song("player", 105, 106, 193, "SPDPC:f:1.9;") // expired before fight
	f.add(liveTestAppear("new", "new", 7603, 108))
	f.add(liveTestDamage("new", 110))
	_, rows := f.read("session=" + f.index.current.key)
	war, active, expired := 0, 0, 0
	for _, row := range rows {
		if row["EventId"] == float64(23) {
			switch row["CCId"] {
			case float64(680):
				war++
			case float64(192):
				active++
			case float64(193):
				expired++
			}
		}
		if row["EventId"] == float64(4) && row["CCId"] == float64(680) {
			t.Fatal("historical performance resurrected an overwritten active Buff")
		}
	}
	if war != 1 || active != 1 || expired != 0 {
		t.Fatalf("opening music: war=%d active=%d expired=%d", war, active, expired)
	}
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 120}, Reliable: true, Reset: true})
	if len(f.index.actor("player").music) != 0 {
		t.Fatal("opening performances crossed connection reset")
	}
}

func TestLiveBattleOpeningMusicCapture(t *testing.T) {
	path := os.Getenv("DILMETER_MUSIC_WINDOW_LOG")
	if path == "" {
		t.Skip("set DILMETER_MUSIC_WINDOW_LOG to the October 4 capture")
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
		case 10:
			e = &event.EventSkillAction{}
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

	found := 0
	for _, window := range f.index.windows {
		if window.targets["4767482427018234"] == nil {
			continue
		}
		response, rows := f.read("session=" + window.key)
		performances, completed := 0, 0
		for _, row := range rows {
			if row["EventId"] == float64(23) {
				performances++
			}
			if row["Signal"] == "sniper-counter" && row["Complete"] == true {
				completed++
			}
		}
		if performances == 0 || completed == 0 {
			t.Fatalf("lost live context: performances=%d completed=%d", performances, completed)
		}
		if destination := os.Getenv("DILMETER_MUSIC_WINDOW_OUTPUT"); destination != "" {
			if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(destination, response.Body.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		found++
		t.Logf("live window: records=%d opening music=%d completed sniper=%d", len(rows), performances, completed)
	}
	if found != 1 {
		t.Fatalf("expected one window for the selected boss, got %d", found)
	}
}
