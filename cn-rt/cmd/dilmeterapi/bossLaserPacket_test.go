package main

import (
	"context"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// The raw file stays outside the repository. This opt-in regression replays
// the complete September 12 capture through the parser, publisher and native
// sound/countdown runtime, including the real fragment-owner relationships.
func TestReplaySeptember2026BossLaserAndDorcha(t *testing.T) {
	file := os.Getenv("DILMETER_SEPTEMBER_SIGNALS_PCAP")
	if file == "" {
		t.Skip("set DILMETER_SEPTEMBER_SIGNALS_PCAP to the September 12 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx: ctx, DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{
		BossMechanics: nativeBossMechanicSettings{
			Volume: 80,
			Rules: map[string]nativeBossMechanicRule{"miel-laser": {
				Key: "miel-laser", BossRaceIDs: []uint32{7603, 7615},
				Trigger: "skill-action", SkillID: 52401, Enabled: true,
				TriggerRaceIDs:   []uint32{7604, 7605, 7606, 7607, 7608, 7616, 7617, 7618, 7619, 7620},
				CountdownSeconds: 5, SoundMode: "dedicated",
			}},
		},
	}, &sounds)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(12 * time.Second)
	defer idle.Stop()
	casts, energyUpdates := 0, 0
	lastAt := int64(0)
	energyMin, energyMax := 15.0, 0.0
	for {
		select {
		case batch := <-events:
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(4 * time.Second)
			for _, e := range batch {
				runtime.onEvent(e)
				if b, ok := e.(interface{ GetEventBase() *event.EventBase }); ok {
					lastAt = max(lastAt, b.GetEventBase().At)
				}
				if action, ok := e.(*event.EventSkillAction); ok && action.SkillId == 52401 && !action.IsLocal && !action.IsFallback {
					casts++
					state := runtime.bossMechanics["miel-laser"]
					if state.EndsAtMs-state.StartedAtMs != 5000 {
						t.Fatalf("beam countdown did not start at the cast: %+v", state)
					}
				}
				if update, ok := e.(*event.EventStatUpdate); ok && update.Private && update.Id == runtime.localID {
					for _, stat := range update.Stats {
						if stat.StatId == 196 {
							energyUpdates++
							energyMin, energyMax = min(energyMin, stat.Value), max(energyMax, stat.Value)
						}
					}
				}
			}
		case <-idle.C:
			if lastAt < 1789229400 || casts != 67 || len(sounds) != 67 {
				t.Fatalf("incomplete replay: last=%d casts=%d sounds=%d; want all 67 casts and sounds", lastAt, casts, len(sounds))
			}
			if energyUpdates < 1500 || energyMin != 0 || energyMax != 15 {
				t.Fatalf("Dorcha replay: updates=%d range=%g..%g", energyUpdates, energyMin, energyMax)
			}
			t.Logf("All 67 beam casts produced one sound and a 5-second countdown each; Dorcha updates=%d, range=0..15", energyUpdates)
			return
		case <-ctx.Done():
			t.Fatalf("replay timed out: last=%d casts=%d sounds=%d", lastAt, casts, len(sounds))
		}
	}
}

func TestPublishBossLaserPacketSeptember2026AFF0(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	// Captured on 2026-09-12: three owned fragments announce one beam.
	const bossID = uint64(4767482428236900)
	publisher.entityCache[bossID] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: bossID, RaceId: 7603}}
	for i, id := range []uint64{4767482428236933, 4767482428236937, 4767482428236950} {
		publisher.entityCache[id] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{
			Id: id, RaceId: uint32(7604 + i), OwnerId: bossID,
		}}
		p := divineSwordDeployPacket(time.UnixMilli(1789221047551), id)
		p.Op = packet.OpCode(45040)
		if !publisher.publishBossLaserPacket(p) {
			t.Fatal("new Divine Sword opcode was not dispatched")
		}
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("fragment cluster published %d actions, want 1", len(publisher.pendingEvents))
	}
	action := publisher.pendingEvents[0].(*event.EventSkillAction)
	if action.SkillId != 52401 || action.IsLocal || action.IsFallback || action.AtMs != 1789221047551 {
		t.Fatalf("new beam lost its identity or cast timestamp: %+v", action)
	}
	// The new opcode also carries cancellation, Divine Spear, and other
	// mechanics. Retain the exact skill-and-phase validation.
	for _, msg := range []packet.Message{
		{packet.NewMessageElemShort(0), packet.NewMessageElemByte(1)},
		{packet.NewMessageElemShort(52402), packet.NewMessageElemByte(1)},
		{packet.NewMessageElemShort(52409), packet.NewMessageElemByte(1)},
		{packet.NewMessageElemShort(52401), packet.NewMessageElemByte(1)},
		{packet.NewMessageElemInt(52401), packet.NewMessageElemByte(0)},
		{packet.NewMessageElemShort(52401), packet.NewMessageElemShort(0)},
		nil,
	} {
		publisher.publishBossLaserPacket(&packet.GamePacket{
			At: time.UnixMilli(1789221072240), Id: 4767482428236933, Op: packet.OpCode(45040), Msg: msg,
		})
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("unrelated/cancelled AFF0 action produced a beam alert")
	}
}
