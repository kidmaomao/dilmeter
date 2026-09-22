package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const healerReplayAlly = "4503599631374038"

// Opt-in regression using the user's 2026-09-22 capture, kept outside the repo.
func TestHealerReplayIdleMusicLog(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_REPLAY_LOG")
	if path == "" {
		t.Skip("set DILMETER_HEALER_REPLAY_LOG to replay the 2026-09-22 event log")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []event.IEvent
	name := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var base event.EventBase
		if err := json.Unmarshal(scanner.Bytes(), &base); err != nil {
			t.Fatal(err)
		}
		var current event.IEvent = &base
		switch base.EventId {
		case 1:
			current = &event.EventEntityAppear{}
		case 2:
			current = &event.EventEntityDisappear{}
		case 4:
			current = &event.EventCharacterConditionEnable{}
		case 5:
			current = &event.EventCharacterConditionDisable{}
		case 6:
			current = &event.EventFinish{}
		case 11:
			current = &event.EventLocalEntity{}
		case 17:
			current = &event.EventStatUpdate{}
		}
		if err := json.Unmarshal(scanner.Bytes(), current); err != nil {
			t.Fatal(err)
		}
		if appear, ok := current.(*event.EventEntityAppear); ok && appear.Id == healerReplayAlly {
			name = appear.Name
		}
		events = append(events, current)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || name == "" {
		t.Fatal("capture does not contain the expected teammate")
	}
	runtime, _ := healerFixture(t)
	member := healerRepeatMember(runtime, healerReplayAlly, name, 1, 5)
	member.Favorite, member.Health, member.BuffSettings.Enabled = true, false, true
	member.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: 680, Name: "战争序曲", WarningSeconds: 10, RepeatCount: 1}, healerSound{Kind: "healer-music"})}
	runtime.healer.settings.Members = []healerMemberSelection{member}
	at := func(current event.IEvent) int64 {
		return current.(interface{ GetEventBase() *event.EventBase }).GetEventBase().At * 1000
	}
	clock := at(events[0])
	var played []string
	runtime.playSound = func(request nativeReminderSoundRequest) bool {
		played = append(played, time.UnixMilli(clock).In(time.FixedZone("HKT", 8*3600)).Format("15:04:05.000"))
		return true
	}
	for index := 0; index < len(events); {
		for clock < at(events[index]) {
			healerTick(runtime, clock)
			clock += 250
		}
		for index < len(events) && at(events[index]) <= clock {
			runtime.onEvent(events[index])
			index++
		}
	}
	for end := clock + 5000; clock <= end; clock += 250 {
		healerTick(runtime, clock)
	}
	t.Logf("replayed %d events; once-only music sound requests at %v", len(events), played)
	if len(played) != 1 {
		t.Fatalf("one music cycle produced %d sound requests, want 1", len(played))
	}
}

func TestHealerReplayIdleMusicPackets(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_REPLAY_PCAP")
	if path == "" {
		t.Skip("set DILMETER_HEALER_REPLAY_PCAP to replay the 2026-09-22 packet capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	if err := reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	grants, removals, appearances := 0, 0, 0
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	for {
		select {
		case batch := <-eventCh:
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(4 * time.Second)
			for _, raw := range batch {
				switch current := raw.(type) {
				case *event.EventCharacterConditionEnable:
					if current.Id == healerReplayAlly && current.CCId == 680 {
						grants++
					}
				case *event.EventCharacterConditionDisable:
					if current.Id == healerReplayAlly && current.CCId == 680 {
						removals++
					}
				case *event.EventEntityAppear:
					if current.Id == healerReplayAlly {
						appearances++
					}
				}
			}
		case <-idle.C:
			t.Logf("packet replay: teammate appearances=%d, music grants=%d, removals=%d", appearances, grants, removals)
			if appearances != 1 || grants != 1 || removals != 1 {
				t.Fatal("unexpected teammate/music lifecycle in capture")
			}
			return
		case <-ctx.Done():
			t.Fatal("packet replay timed out")
		}
	}
}
