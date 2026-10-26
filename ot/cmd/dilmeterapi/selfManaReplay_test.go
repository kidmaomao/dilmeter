package main

import (
	"bufio"
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"os"
	"path/filepath"
	"testing"
)

func TestSelfManaOctoberLog(t *testing.T) {
	path := os.Getenv("DILMETER_SELF_MANA_LOG")
	if path == "" {
		t.Skip("set DILMETER_SELF_MANA_LOG to replay self mana")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sounds := []nativeReminderSoundRequest{}
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{}, &sounds)
	r.healer = newHealerMonitor(filepath.Join(t.TempDir(), "healer.json"))
	r.healer.settings = normalizeHealerSettings(healerSettings{Enabled: true, Members: []healerMemberSelection{{Self: true, Mana: true}}})
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	samples, low := 0, 0
	minimum, maximum := 100.0, 0.0
	for scanner.Scan() {
		var base event.EventBase
		if err := json.Unmarshal(scanner.Bytes(), &base); err != nil {
			t.Fatal(err)
		}
		var e event.IEvent
		switch base.EventId {
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
		default:
			continue
		}
		if err := json.Unmarshal(scanner.Bytes(), e); err != nil {
			t.Fatal(err)
		}
		r.onEvent(e)
		if stats, ok := e.(*event.EventStatUpdate); ok && stats.Id == r.localID {
			hasMana := false
			for _, stat := range stats.Stats {
				hasMana = hasMana || stat.StatId == 32
			}
			if !hasMana {
				continue
			}
			healerTick(r, base.At*1000)
			for _, m := range r.healer.state.Members {
				if m.Key == "self" && m.ManaPercent != nil {
					if m.ID != r.localID {
						t.Fatal("self bound to a teammate")
					}
					samples++
					minimum = min(minimum, *m.ManaPercent)
					maximum = max(maximum, *m.ManaPercent)
					if m.ManaState == "low" {
						low++
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if samples == 0 {
		t.Fatal("no self mana samples recognized")
	}
	t.Logf("self mana: %d observed samples, %.2f%%..%.2f%%, %d below threshold", samples, minimum, maximum, low)
}
