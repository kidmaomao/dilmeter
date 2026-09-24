package main

import (
	"context"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestHealerReplayCrombasVisibilityAndMusic(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_VISIBILITY_PCAP")
	if path == "" {
		t.Skip("set DILMETER_HEALER_VISIBILITY_PCAP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	runtime, _ := healerFixture(t)
	runtime.healer.settings.Members = nil
	for i, name := range []string{"栗艾", "小米羔"} {
		member := normalizeHealerMember(healerMemberSelection{Name: name, Favorite: true, Included: true, Overture: true}, runtime.healer.settings, i)
		runtime.healer.settings.Members = append(runtime.healer.settings.Members, member)
	}
	zone := time.FixedZone("HKT", 8*3600)
	checkpoints := []time.Time{}
	for _, clock := range []string{"22:28:03", "22:30:40", "22:40:43", "22:43:25", "22:53:25", "22:57:10"} {
		at, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-09-23 "+clock, zone)
		checkpoints = append(checkpoints, at)
	}
	if err := reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(8 * time.Second)
	defer idle.Stop()
	checked, count := 0, 0
	for {
		select {
		case batch := <-events:
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(8 * time.Second)
			for _, current := range batch {
				base, ok := current.(interface{ GetEventBase() *event.EventBase })
				if !ok || base.GetEventBase().Sequence == 0 {
					continue
				}
				at := base.GetEventBase().At
				for checked < len(checkpoints) && at > checkpoints[checked].Unix() {
					check := checkpoints[checked]
					healerTick(runtime, check.UnixMilli())
					for i := 0; i < 2; i++ {
						member := runtime.healer.state.Members[i]
						if !member.Active || member.Overture.State != "active" || member.Overture.RemainingSeconds == nil || *member.Overture.RemainingSeconds <= 25 {
							t.Fatalf("%s teammate/music missing: %+v", check.Format("15:04:05"), member)
						}
						t.Logf("%s %s recognized; music=%ds", check.Format("15:04:05"), member.Name, *member.Overture.RemainingSeconds)
					}
					frame := runtime.healer.overlaySnapshot(check.UnixMilli())
					if len(frame.Groups) != 2 {
						t.Fatalf("%s overlay groups=%d, want both teammates", check.Format("15:04:05"), len(frame.Groups))
					}
					checked++
				}
				runtime.onEvent(current)
				count++
			}
		case <-idle.C:
			if checked != len(checkpoints) {
				t.Fatalf("only checked %d checkpoints", checked)
			}
			t.Logf("replayed %d events; all %d map/Boss checkpoints have both teammate Buff frames", count, checked)
			return
		case <-ctx.Done():
			t.Fatal("replay timed out")
		}
	}
}

func TestHealerMusicSnapshotAfterMapChange(t *testing.T) {
	runtime, sounds := healerFixture(t)
	runtime.healer.settings.Members[0].Name = "队友"
	cache := make(entityCache)
	entity := &packet.EntityInfo{Id: 123, Name: "队友", RaceId: 10001, CharacterConditionMap: map[uint32]*packet.EntityCharacterCondition{
		680: {CCId: 680, DisableAt: 700, DisableAtMs: 700000},
	}}
	publishAppearance := func(at int64) {
		cache.add(entity, time.Unix(at, 0))
		runtime.onEvent(toEventEntityAppear(at, entity))
		for _, current := range toEventListEntityContains(at, entity, &cache) {
			runtime.onEvent(current)
		}
	}
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "ally", At: 100}})
	publishAppearance(100)
	healerTick(runtime, 100000)
	if runtime.healer.state.Members[0].Overture.State != "active" {
		t.Fatal("initial music missing")
	}
	cache.disappear(123, time.Unix(110, 0))
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "123", At: 110}})
	healerTick(runtime, 110000)
	if len(runtime.healer.state.Cards) != 0 {
		t.Fatal("departed teammate still visible")
	}
	// The server sends the exact same music expiry in the new map snapshot.
	publishAppearance(120)
	healerTick(runtime, 120000)
	member := runtime.healer.state.Members[0]
	if !member.Active || member.Overture.State != "active" || member.Overture.RemainingSeconds == nil || *member.Overture.RemainingSeconds != 580 || len(runtime.healer.state.Cards) != 2 {
		t.Fatalf("map entry lost active music: member=%+v cards=%+v", member, runtime.healer.state.Cards)
	}
	if card := runtime.healer.state.Cards[1]; card.CCID != 192 || card.State != "missing" || card.Value != "补充" || member.Vivace.State != "unknown" {
		t.Fatalf("map entry did not restore the configured unobserved music slot: %+v", card)
	}
	if len(*sounds) != 0 {
		t.Fatal("map entry invented a music warning")
	}
	if repeated := toEventListEntityContains(121, entity, &cache); len(repeated) != 0 {
		t.Fatal("duplicate filtering stopped working within one visible interval")
	}
	// Buffs removed off screen must not survive in the publisher cache.
	cache.disappear(123, time.Unix(130, 0))
	empty := &packet.EntityInfo{Id: 123, Name: "队友", RaceId: 10001}
	cache.add(empty, time.Unix(140, 0))
	if len(cache[123].characterConditionMap) != 0 {
		t.Fatal("stale off-screen conditions retained")
	}
}
