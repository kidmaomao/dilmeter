package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func healerDungeonSettings() healerSettings {
	s := defaultHealerSettings()
	s.Enabled, s.AutoAddInDungeon, s.AutoAddTemplateID = true, true, "dungeon"
	s.SoundEnabled = false
	s.Templates = []healerMemberTemplate{{
		ID: "dungeon", Name: "副本队友", Health: true,
		SkillSettings: &healerMemberSkillSettings{
			Enabled: true, Overlay: healerPosition{Enabled: true},
			Rules: []healerSkillRule{{SkillID: 59005, Name: "崩坏波动", CooldownSeconds: 20}},
		},
	}}
	return normalizeHealerSettings(s)
}

func TestHealerAutoRosterFollowsLocalDungeonTransfer(t *testing.T) {
	r, _ := healerFixture(t)
	h := r.healer
	h.settings = healerDungeonSettings()
	h.settings.Members = []healerMemberSelection{{Name: "固定队友", Favorite: true, Included: true}}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "100", At: 101}, Reliable: true})
	p, batches := perlonPacketTestPublisher()
	p.localEntityId, p.localEntityReliable = 100, true
	send := func(op packet.OpCode, id uint64, at int64, msg packet.Message) {
		p.publishHealerRegion(&packet.GamePacket{Op: op, Id: id, At: time.Unix(at, 0), Msg: msg})
		p.publishNow()
		for len(batches) > 0 {
			for _, e := range <-batches {
				r.onEvent(e)
			}
		}
	}
	warp := func(id uint64, region uint32, at int64) {
		send(healerWarpOpcode, id, at, packet.Message{
			packet.NewMessageElemByte(1), packet.NewMessageElemInt(region),
			packet.NewMessageElemInt(7638), packet.NewMessageElemInt(71361),
		})
	}
	profile := func(region, kind uint32, at int64) {
		send(healerRegionProfileOpcode, 1, at, packet.Message{
			packet.NewMessageElemByte(0), packet.NewMessageElemInt(region), packet.NewMessageElemByte(0),
			packet.NewMessageElemInt(kind), packet.NewMessageElemByte(0),
		})
	}
	appear := func(id, name, owner string, region, race uint32, at int64) {
		e := liveTestAppear(id, name, race, at)
		e.RegionID, e.OwnerId = region, owner
		r.onEvent(e)
	}
	check := func(at int64, want ...string) {
		t.Helper()
		healerTick(r, at*1000)
		var got []string
		for _, member := range h.state.Members {
			if member.Automatic {
				got = append(got, member.ID)
			}
		}
		if len(got) != len(want) || len(want) > 0 && got[0] != want[0] {
			t.Fatalf("at %d automatic roster = %v, want %v", at, got, want)
		}
		if len(h.settings.Members) != 1 || !h.settings.Members[0].Favorite {
			t.Fatal("automatic selection changed saved membership")
		}
	}

	profile(4016, 200138, 101)
	warp(100, 4016, 101)
	appear("town", "城镇路人", "", 4016, 10001, 101)
	check(101)
	appear("auto", "临时队友", "", 35000, 9001, 102)
	warp(100, 35000, 102)
	appear("wrong-area", "外部玩家", "", 4016, 9001, 102)
	appear("fixed", "固定队友", "", 35000, 9001, 102)
	appear("pet", "宠物", "auto", 35000, 9001, 102)
	appear("boss", "首领", "", 35000, perlonRaceID, 102)
	check(102) // Need the profile or ready signal in addition to the destination.
	profile(35000, 0x80000001, 103)
	warp(123, 4016, 103) // Another actor cannot change the local region.
	r.onEvent(healerHP("auto", 103, 755, 1000))
	r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "auto", At: 103}, SkillId: 59005, AtMs: 103000})
	check(103, "auto")
	row := h.overlaySnapshot(103000).Groups[0].Rows[0]
	if row.Key != "instance:auto" || row.Health.Value != "76%" || row.Skills[0].Value != "20s" {
		t.Fatalf("template did not provide health and skill settings: %+v", row)
	}
	r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "auto", At: 104}})
	check(104)
	appear("auto", "临时队友", "", 35000, 9001, 105)
	check(105, "auto")
	warp(100, 4016, 106)
	check(106)
	warp(100, 35000, 110)
	check(110) // Expired evidence and old appearances cannot re-add the player.
}

func TestHealerLifeCurtainUsesBothPoolsAndClears(t *testing.T) {
	r, _ := healerFixture(t)
	h := r.healer
	h.settings.Members = []healerMemberSelection{{ID: "ally", Included: true}}
	grant := func(id, metadata string, now, end int64) {
		r.onEvent(&event.EventCharacterConditionEnable{
			EventBase: event.EventBase{Id: id, At: now / 1000},
			AtMs:      now, CCId: lifeCurtainConditionID, Metadata: metadata, DisableAtMs: end,
		})
	}
	check := func(now int64, key, value string) {
		t.Helper()
		healerTick(r, now)
		got := ""
		for _, group := range h.overlaySnapshot(now).Groups {
			for _, row := range group.Rows {
				if row.Key == key {
					got = row.Curtain
				}
			}
		}
		if got != value {
			t.Fatalf("at %d curtain for %s = %q, want %q", now, key, got, value)
		}
	}
	grant("ally", "SBBV:f:4387.007813;SBBSB:f:1876.03125;SBBDDR:f:.473488;", 100100, 108000)
	check(100250, "player:队友", "6263")
	grant("ally", "SBBSB:f:770.638428;SBBV:f:0;", 100300, 108000)
	check(100500, "player:队友", "771")
	r.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 100}, AtMs: 100600, CCId: lifeCurtainConditionID})
	check(100750, "player:队友", "") // Consumption can remove CC999 in the same second as an update.
	grant("self", "SBBV:f:500;SBBSB:f:0;", 101000, 102000)
	check(101000, "self", "500")
	check(102000, "self", "")
	grant("ally", "SBBV:f:10;SBBSB:f:0;", 103000, 109000)
	r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "ally", At: 103}})
	r.onEvent(healerHP("ally", 104, 1000, 1000))
	check(104000, "player:队友", "")
}

func TestHealerVitalsKeepCurtainThroughInitialHealthAndIdle(t *testing.T) {
	runtime, _ := healerFixture(t)
	h := runtime.healer
	h.settings.Members = []healerMemberSelection{{ID: "ally", Included: true, Health: true, Mana: true}}
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{Id: "ally", At: 100}, AtMs: 100050,
		CCId: lifeCurtainConditionID, DisableAtMs: 160050, Metadata: "SBBV:f:5300.4;SBBSB:f:0;",
	})
	rowAt := func(at int64) healerTeamRow {
		t.Helper()
		healerTick(runtime, at)
		frame := h.overlaySnapshot(at)
		if len(frame.Groups) != 1 || len(frame.Groups[0].Rows) != 1 {
			t.Fatalf("ally row missing at %d: %+v", at, frame)
		}
		return frame.Groups[0].Rows[0]
	}
	if row := rowAt(100050); row.Curtain != "5300" || row.Health.Value != "—" || row.Mana != nil {
		t.Fatalf("shield should be visible before the first life stat: %+v", row)
	}
	stats := healerHP("ally", 100, 760, 1000)
	stats.AtMs = 100150
	stats.Stats = append(stats.Stats,
		event.EventStatUpdateEntry{StatId: 32, Value: 821.7},
		event.EventStatUpdateEntry{StatId: 33, Value: 840},
		event.EventStatUpdateEntry{StatId: 34, Value: 160})
	runtime.onEvent(stats)
	if row := rowAt(100150); row.Curtain != "5300" || row.Health.Value != "76%" || row.Mana != nil {
		t.Fatalf("first stat update covered the shield or exposed teammate mana: %+v", row)
	}
	// An appearance snapshot can repeat this condition without pool values.
	runtime.onEvent(&event.EventCharacterConditionEnable{
		EventBase: event.EventBase{Id: "ally", At: 101}, AtMs: 101150,
		CCId: lifeCurtainConditionID, Snapshot: true, Metadata: "SBBDDR:f:0.65;",
	})
	runtime.onEvent(&event.EventCharacterConditionDisable{
		EventBase: event.EventBase{Id: "ally", At: 100}, AtMs: 100010, CCId: lifeCurtainConditionID,
	})
	if row := rowAt(145000); row.Curtain != "5300" || row.Mana != nil {
		t.Fatalf("idle time hid the shield or exposed a teammate mana placeholder: %+v", row)
	}
	runtime.onEvent(healerHP("ally", 160, 700, 1000))
	if row := rowAt(160050); row.Curtain != "" || row.Health.Value != "70%" || row.Mana != nil {
		t.Fatalf("expired shield did not reveal the new life percentage: %+v", row)
	}
}

func TestHealerDungeonCaptureReplay(t *testing.T) {
	path := os.Getenv("DILMETER_TEAM_DUNGEON_PCAP")
	if path == "" {
		t.Skip("set DILMETER_TEAM_DUNGEON_PCAP to the 2026-11-15 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	batches := make(chan []event.IEvent, 4096)
	publisher.addClient(ctx, batches)
	if err = reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	var sounds []nativeReminderSoundRequest
	r := perlonTestRuntime(&sounds)
	r.healer = newHealerMonitor(filepath.Join(t.TempDir(), "healer.json"))
	r.healer.settings = healerDungeonSettings()
	idle := time.NewTimer(5 * time.Second)
	defer idle.Stop()
	regionsWithRoster := map[uint32]bool{}
	tornado := map[string]int{}
	var dungeonTransfers, townTransfers, greenRows int
	var nowMs int64
	for {
		select {
		case <-ctx.Done():
			t.Fatal("capture replay timed out")
		case <-idle.C:
			t.Logf("dungeon transfers=%d with roster=%d, town transfers=%d, green row samples=%d, tornado targets=%v",
				dungeonTransfers, len(regionsWithRoster), townTransfers, greenRows, tornado)
			if dungeonTransfers != 10 || len(regionsWithRoster) != 10 || townTransfers != 5 || greenRows == 0 ||
				tornado["self"] != 21 || tornado["teammate"] != 41 {
				t.Fatalf("capture evidence failed to reach the team and tornado overlays. Active rosters in: %v", regionsWithRoster)
			}
			return
		case batch := <-batches:
			idle.Reset(5 * time.Second)
			for _, raw := range batch {
				base, ok := raw.(interface{ GetEventBase() *event.EventBase })
				if !ok || base.GetEventBase().Sequence == 0 {
					continue // The subscription's initial snapshot uses wall-clock time.
				}
				r.onEvent(raw)
				switch e := raw.(type) {
				case *event.EventLocalRegion:
					if e.Dungeon {
						dungeonTransfers++
					} else if e.RegionID == 4016 {
						townTransfers++
					}
				case *event.EventSkillAction:
					if e.MechanicSignal == perlonTornadoKey {
						for _, item := range r.perlonOverlay(time.UnixMilli(e.AtMs)) {
							if item.Key == perlonTornadoKey {
								tornado[item.TornadoTarget]++
							}
						}
					}
				}
				nowMs = max(nowMs, base.GetEventBase().At*1000)
			}
			before := r.healer.lastTickMs
			healerTick(r, nowMs)
			if before == r.healer.lastTickMs {
				continue
			}
			for _, member := range r.healer.state.Members {
				if !member.Automatic {
					continue
				}
				if !r.healer.state.InDungeon || r.healer.regionID == 4016 {
					if member.PartySlot == 0 {
						t.Fatal("a non-party player was automatically monitored in town")
					}
					continue
				}
				regionsWithRoster[r.healer.regionID] = true
				if member.CurtainAmount != nil {
					greenRows++
				}
			}
		}
	}
}
