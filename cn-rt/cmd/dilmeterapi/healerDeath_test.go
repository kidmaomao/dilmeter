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

func TestHealerDeathBuffRemovalKeepsReminderHistory(t *testing.T) {
	for _, order := range []string{"remove-first", "finish-first"} {
		t.Run(order, func(t *testing.T) {
			runtime, sounds := healerFixture(t)
			member := healerRepeatMember(runtime, "ally", "队友", 1, 5)
			member.BuffSettings.Enabled = true
			member.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: 680, Name: "战争序曲", WarningSeconds: 25, RepeatCount: 1}, healerSound{Kind: "healer-music"})}
			runtime.healer.settings.Members = []healerMemberSelection{member}
			runtime.onEvent(healerHP("ally", 100, 800, 1000))
			runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: 680, DisableAt: 700})
			healerTick(runtime, 100000)
			finish := &event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "ally", At: 101}}
			remove := &event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "ally", At: 101}, CCId: 680}
			if order == "remove-first" {
				runtime.onEvent(remove)
				runtime.onEvent(finish)
			} else {
				runtime.onEvent(finish)
				runtime.onEvent(remove)
			}
			runtime.onEvent(finish) // The real capture repeats the death notification.
			healerTick(runtime, 101000)
			healerTick(runtime, 102250)
			state := runtime.healer.state
			if len(*sounds) != 1 || (*sounds)[0].Kind != "healer-death" || state.Members[0].Active || state.Members[0].Overture.LossReason != "death" {
				t.Fatalf("death/removal should warn once about music and suppress stale HP: sounds=%v member=%+v", *sounds, state.Members[0])
			}
			runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "ally", At: 103}})
			runtime.onEvent(healerHP("ally", 103, 800, 1000))
			healerTick(runtime, 103000)
			healerTick(runtime, 105000)
			if len(*sounds) != 1 || !runtime.healer.state.Members[0].Active || runtime.healer.state.Members[0].Overture.State != "missing" {
				t.Fatal("revival forgot missing music or replenished its quota")
			}
			runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "ally", At: 106}})
			healerTick(runtime, 106000)
			if len(runtime.healer.state.Alerts) != 0 || len(runtime.healer.state.Cards) != 0 {
				t.Fatal("out-of-view player still has warnings")
			}
		})
	}
}

func TestHealerReplayDeathMusicPackets(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_DEATH_PCAP")
	if path == "" {
		t.Skip("set DILMETER_HEALER_DEATH_PCAP to replay the 21:09 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	wanted := map[string]int64{"4503599635344486": 1790087965, "4503599639418618": 1790088127}
	deaths, removals := map[string]int{}, map[string]int{}
	expiry := map[string]int64{}
	remaining := map[string]int64{}
	idle := time.NewTimer(8 * time.Second)
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
			idle.Reset(8 * time.Second)
			for _, raw := range batch {
				switch current := raw.(type) {
				case *event.EventCharacterConditionEnable:
					if current.CCId == 680 && wanted[current.Id] > 0 && current.At <= wanted[current.Id] {
						expiry[current.Id] = current.DisableAt
					}
				case *event.EventCharacterConditionDisable:
					if current.CCId == 680 && current.At == wanted[current.Id] {
						removals[current.Id]++
						remaining[current.Id] = expiry[current.Id] - current.At
					}
				case *event.EventFinish:
					if current.At == wanted[current.Id] {
						deaths[current.Id]++
					}
				}
			}
		case <-idle.C:
			for id := range wanted {
				if deaths[id] == 0 || removals[id] != 1 || remaining[id] <= 25 {
					t.Fatalf("missing early-death removal for %s: deaths=%d removals=%d remaining=%d", id, deaths[id], removals[id], remaining[id])
				}
				t.Logf("teammate %s: death notifications=%d music removals=%d, %d seconds before expected expiry", id, deaths[id], removals[id], remaining[id])
			}
			return
		case <-ctx.Done():
			t.Fatal("packet replay timed out")
		}
	}
}

func TestHealerDeathDoesNotInventBuffLossOrResetQuota(t *testing.T) {
	runtime, sounds := healerFixture(t)
	member := healerRepeatMember(runtime, "ally", "队友", 1, 5)
	member.Health, member.BuffSettings.Enabled = false, true
	member.BuffSettings.Rules = []healerBuffRule{
		normalizeHealerBuffRule(healerBuffRule{CCID: 680, WarningSeconds: 25}, healerSound{Kind: "healer-music"}),
		normalizeHealerBuffRule(healerBuffRule{CCID: 192, WarningSeconds: 25}, healerSound{Kind: "healer-music"}),
	}
	runtime.healer.settings.Members = []healerMemberSelection{member}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: 680, DisableAt: 700})
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "ally", At: 101}})
	healerTick(runtime, 101000)
	healerTick(runtime, 103000)
	if len(*sounds) != 0 || runtime.healer.state.Members[0].Overture.State != "active" || runtime.healer.state.Members[0].Vivace.State != "unknown" {
		t.Fatal("death discarded known Buff state or invented a removal")
	}
	// Normal expiry and early death loss have independent reminder quotas.
	runtime.onEvent(healerHP("ally", 104, 800, 1000))
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 104}, CCId: 680, DisableAt: 110})
	healerTick(runtime, 104000)
	healerTick(runtime, 105250)
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "ally", At: 106}, CCId: 680})
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "ally", At: 106}})
	healerTick(runtime, 106000)
	healerTick(runtime, 108000)
	if len(*sounds) != 2 || (*sounds)[0].Kind != "healer-music" || (*sounds)[1].Kind != "healer-death" {
		t.Fatal("death loss did not use its separate voice and quota")
	}
}

// Opt-in replay of the final encounter in the user's 2026-09-22 21:09 capture.
func TestHealerReplayDeathMusicLog(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_DEATH_LOG")
	if path == "" {
		t.Skip("set DILMETER_HEALER_DEATH_LOG to replay the 21:09 event log")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	runtime, _ := healerFixture(t)
	runtime.healer.settings.Members = nil
	for id, name := range map[string]string{"4503599635344486": "乙羽希娅里", "4503599639418618": "霖芽"} {
		member := healerRepeatMember(runtime, id, name, 1, 5)
		member.Favorite, member.Health, member.BuffSettings.Enabled = true, false, true
		member.BuffSettings.Rules = []healerBuffRule{normalizeHealerBuffRule(healerBuffRule{CCID: 680, Name: "战争序曲", WarningSeconds: 25, RepeatCount: 1}, healerSound{Kind: "healer-music"})}
		runtime.healer.settings.Members = append(runtime.healer.settings.Members, member)
	}
	const start, end = int64(1790087928_000), int64(1790088190_000)
	clock := start
	var played []int64
	runtime.playSound = func(request nativeReminderSoundRequest) bool { played = append(played, clock); return true }
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count := 0
	for scanner.Scan() {
		var base event.EventBase
		if err := json.Unmarshal(scanner.Bytes(), &base); err != nil {
			t.Fatal(err)
		}
		at := base.At * 1000
		if at > end {
			break
		}
		for clock < at {
			healerTick(runtime, clock)
			clock += 250
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
		runtime.onEvent(current)
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for clock <= end {
		healerTick(runtime, clock)
		clock += 250
	}
	for _, at := range played {
		t.Logf("music sound request at %s", time.UnixMilli(at).In(time.FixedZone("HKT", 8*3600)).Format("15:04:05.000"))
	}
	t.Logf("replayed %d events including state before final encounter; music sounds=%d", count, len(played))
	if len(played) != 2 {
		t.Fatalf("want one alert for each of two death removals, got %d", len(played))
	}
	for index, removal := range []int64{1790087965_000, 1790088127_000} {
		if played[index] < removal || played[index] > removal+4000 {
			t.Fatalf("death removal did not promptly warn: %v", played)
		}
	}
}
