package main

import (
	"context"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func twLaserRuntime(sounds *[]nativeReminderSoundRequest) *nativeReminderRuntime {
	return newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{
		Volume: 80, Rules: map[string]nativeBossMechanicRule{"miel-laser": {
			Key: "miel-laser", BossRaceIDs: []uint32{7603, 7615}, Trigger: "skill-action", SkillID: 52401,
			TriggerRaceIDs: []uint32{7604, 7605, 7606, 7607, 7608, 7616, 7617, 7618, 7619, 7620},
			Enabled:        true, CountdownSeconds: 5, SoundMode: "dedicated",
		}},
	}}, sounds)
}

func TestTWLaserAFFACluster(t *testing.T) {
	for _, bossRace := range []uint32{7603, 7615} {
		publisher := newSkillTestPublisher()
		publisher.lastSentEventAt = time.Now()
		const bossID = uint64(9000)
		publisher.entityCache[bossID] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: bossID, RaceId: bossRace}}
		shardRace := uint32(7604)
		if bossRace == 7615 {
			shardRace = 7616
		}
		at := time.UnixMilli(1790606383566)
		for i := uint64(0); i < 3; i++ {
			id := 9100 + i
			publisher.entityCache[id] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: id, RaceId: shardRace + uint32(i), OwnerId: bossID}}
			p := divineSwordDeployPacket(at.Add(time.Duration(i*4)*time.Millisecond), id)
			p.Op = packet.OpCode(0xaffa)
			if !publisher.publishBossLaserPacket(p) {
				t.Fatal("TW opcode not dispatched")
			}
		}
		if len(publisher.pendingEvents) != 1 {
			t.Fatalf("race %d: got %d actions, want one", bossRace, len(publisher.pendingEvents))
		}
		action := publisher.pendingEvents[0].(*event.EventSkillAction)
		if action.SkillId != 52401 || action.AtMs != at.UnixMilli() || action.IsFallback || action.IsLocal {
			t.Fatalf("unexpected TW action: %+v", action)
		}
		for _, msg := range []packet.Message{
			{packet.NewMessageElemShort(52402), packet.NewMessageElemByte(1)},
			{packet.NewMessageElemShort(0), packet.NewMessageElemByte(1)},
			{packet.NewMessageElemShort(52401), packet.NewMessageElemByte(1)},
			{packet.NewMessageElemInt(52401), packet.NewMessageElemByte(0)},
			{packet.NewMessageElemShort(52401), packet.NewMessageElemShort(0)},
			{nil, nil}, nil,
		} {
			publisher.publishBossLaserPacket(&packet.GamePacket{At: at.Add(time.Second), Id: 9100, Op: packet.OpCode(0xaffa), Msg: msg})
		}
		if len(publisher.pendingEvents) != 1 {
			t.Fatal("TW cancellation/unrelated payload triggered a laser")
		}
		p := divineSwordDeployPacket(at.Add(10*time.Second), 9100)
		p.Op = packet.OpCode(0xaffa)
		publisher.publishBossLaserPacket(p)
		if len(publisher.pendingEvents) != 2 {
			t.Fatal("subsequent TW cast was suppressed")
		}
	}
}

// User captures remain outside Git. This regression exercises parsing, entity
// ownership, dispatch, deduplication and the native sound/countdown together.
func TestReplayTWSeptember2026Laser(t *testing.T) {
	file := os.Getenv("DILMETER_TW_LASER_PCAP")
	if file == "" {
		t.Skip("set DILMETER_TW_LASER_PCAP to the September 28 TW capture")
	}
	if err := constants.ConfigureGameServer("210.208.80.25/32", []string{"11022"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = constants.ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"}) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	var sounds []nativeReminderSoundRequest
	runtime := twLaserRuntime(&sounds)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(6 * time.Second)
	defer idle.Stop()
	casts := 0
	for {
		select {
		case batch := <-events:
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			// Initial publisher state can arrive before OpenFile's delayed
			// replay starts; do not let that shorten the first-packet timeout.
			idle.Reset(6 * time.Second)
			for _, e := range batch {
				runtime.onEvent(e)
				if action, ok := e.(*event.EventSkillAction); ok && action.SkillId == 52401 && !action.IsLocal && !action.IsFallback {
					casts++
					state := runtime.bossMechanics["miel-laser"]
					if state.StartedAtMs != 1790606383566 || state.EndsAtMs-state.StartedAtMs != 5000 {
						t.Fatalf("TW countdown: %+v", state)
					}
				}
			}
		case <-idle.C:
			if casts != 1 || len(sounds) != 1 {
				t.Fatalf("TW replay: casts=%d sounds=%d; want 1 each", casts, len(sounds))
			}
			return
		case <-ctx.Done():
			t.Fatal("TW replay timed out")
		}
	}
}
