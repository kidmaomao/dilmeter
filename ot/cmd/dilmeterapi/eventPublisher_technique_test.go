package main

import (
	"context"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestIntensiveProvocationOnlyCasterStartsEffectTimer(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{EffectTimers: nativeEffectTimerSettings{Rules: map[string]nativeEffectTimerRule{
		"provocation": {Enabled: true, SourceType: "skill", SourceID: 58012, DurationSeconds: 7.5, TargetMode: "self"},
	}}}, &sounds)
	at := time.UnixMilli(1_789_313_628_199)
	for _, step := range []struct {
		name    string
		id      uint64
		ccID    uint32
		enabled bool
		offset  time.Duration
		want    int
	}{
		{"remote caster", 999, 517, true, 0, 0},
		{"local recipient", 100, 518, true, 0, 0},
		{"affected monster", 200, 519, true, 0, 0},
		{"inactive caster", 100, 517, false, 0, 0},
		{"local caster", 100, 517, true, 0, 1},
		{"duplicate update", 100, 517, true, 10 * time.Millisecond, 1},
		{"recipient update", 100, 518, true, 20 * time.Millisecond, 1},
		{"caster ends", 100, 517, false, 6 * time.Second, 1},
		{"next cast", 100, 517, true, 31 * time.Second, 2},
	} {
		t.Run(step.name, func(t *testing.T) {
			previous := len(publisher.pendingEvents)
			publisher.publishTechniqueConditionSkillAction(at.Add(step.offset), &packet.CharacterConditionPacket{
				Id: step.id, IsEnable: step.enabled,
				EntityCharacterCondition: packet.EntityCharacterCondition{CCId: step.ccID},
			})
			if len(publisher.pendingEvents) != step.want {
				t.Fatalf("skill events = %d, want %d", len(publisher.pendingEvents), step.want)
			}
			for _, raw := range publisher.pendingEvents[previous:] {
				action, ok := raw.(*event.EventSkillAction)
				if !ok || action.SkillId != 58012 || !action.IsLocal || action.IsFallback || action.Id != "100" || action.SourceId != "100" {
					t.Fatalf("caster action = %#v", raw)
				}
				runtime.onEvent(action)
				state, exists := runtime.effectTimers["provocation"]
				wantAt := at.Add(step.offset).UnixMilli()
				if !exists || state.StartedAtMs != wantAt || state.EndsAtMs != wantAt+7500 || state.TargetID != "100" || state.Generation != uint64(step.want) {
					t.Fatalf("effect timer = %#v", state)
				}
			}
		})
	}
}

// Opt-in replay of packet_capture_1789313480.pcapng. Keep the user's raw
// capture outside the repository; verify all three casts through the live path.
func TestReplayIntensiveProvocationEffectTimers(t *testing.T) {
	file := os.Getenv("DILMETER_PROVOCATION_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PROVOCATION_TEST_PCAP to replay the 2026-09-13 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{EffectTimers: nativeEffectTimerSettings{Rules: map[string]nativeEffectTimerRule{
		"provocation": {Enabled: true, SourceType: "skill", SourceID: 58012, DurationSeconds: 7.5, TargetMode: "self"},
		"arrow":       {Enabled: true, SourceType: "skill", SourceID: 21014, DurationSeconds: 5, TargetMode: "self"},
	}}}, &sounds)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	wantAt := []int64{1_789_313_628_199, 1_789_313_659_230, 1_789_313_767_489}
	casts, arrows := 0, 0
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	for {
		select {
		case events := <-eventCh:
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(4 * time.Second)
			for _, raw := range events {
				runtime.onEvent(raw)
				if action, ok := raw.(*event.EventSkillAction); ok && (action.SkillId == 58012 || action.SkillId == 21014) {
					key, duration := "provocation", int64(7500)
					if action.SkillId == 58012 {
						if casts >= len(wantAt) || action.AtMs != wantAt[casts] || action.IsFallback {
							t.Fatalf("unexpected provocation cast #%d: %#v", casts+1, action)
						}
						casts++
					} else {
						key, duration = "arrow", 5000
						arrows++
					}
					state, exists := runtime.effectTimers[key]
					if !action.IsLocal || action.Id != runtime.localID || !exists || state.StartedAtMs != action.AtMs || state.EndsAtMs != action.AtMs+duration || state.TargetID != runtime.localID {
						t.Fatalf("%s action %#v produced timer %#v for local player %s", key, action, state, runtime.localID)
					}
				}
			}
		case <-idle.C:
			if casts != 3 || arrows != 5 {
				t.Fatalf("replay: provocation=%d arrow=%d, want 3 provocation casts and 5 arrow triggers", casts, arrows)
			}
			t.Logf("replayed %d provocation casts and %d arrow timer triggers", casts, arrows)
			return
		case <-ctx.Done():
			t.Fatalf("replay timed out: provocation=%d arrow=%d", casts, arrows)
		}
	}
}
