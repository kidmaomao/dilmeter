package main

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func transformationTestPacket(skill uint16) *packet.GamePacket {
	return &packet.GamePacket{Op: packet.OpcodeSkillComplete, Id: 100, At: time.UnixMilli(100_123),
		Msg: packet.Message{packet.NewMessageElemShort(skill), packet.NewMessageElemString("")}}
}

func transformationTestRuntime(skill uint16) *nativeReminderRuntime {
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{
		EffectTimers: nativeEffectTimerSettings{Rules: map[string]nativeEffectTimerRule{
			"transform": {Enabled: true, SourceType: "skill", SourceID: uint32(skill), TargetMode: "self", DurationSeconds: 1800},
		}},
		SkillCooldowns: nativeSkillCooldownSettings{Rules: map[uint16]nativeSkillCooldownRule{
			skill: {Enabled: true, SkillID: skill, CooldownSeconds: 1800},
		}},
	}, &sounds)
	r.localID = "100"
	return r
}

func TestTransformationCompleteStartsDurationAndCooldown(t *testing.T) {
	for _, skill := range []uint16{40001, 41001, 42001, 43001, 44001} {
		t.Run(strconv.Itoa(int(skill)), func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId = 100
			publisher.lastSentEventAt = time.Now().Add(time.Hour)
			r := transformationTestRuntime(skill)
			p := transformationTestPacket(skill)
			publisher.publishTransformationComplete(p)
			publisher.publishTransformationComplete(p) // Same completion must not restart it.
			if len(publisher.pendingEvents) != 1 {
				t.Fatalf("actions = %d, want 1", len(publisher.pendingEvents))
			}
			action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
			if !ok || action.SkillId != skill || !action.IsLocal || action.IsFallback ||
				action.SourceId != "100" || action.Id != "100" || action.AtMs != p.At.UnixMilli() {
				t.Fatalf("unexpected transformation action: %#v", publisher.pendingEvents[0])
			}
			r.onEvent(action)
			state := r.effectTimers["transform"]
			if state.StartedAtMs != action.AtMs || state.EndsAtMs != action.AtMs+1_800_000 || state.Generation != 1 {
				t.Fatalf("duration timer = %#v", state)
			}
			if cd := r.skillCooldowns[skill]; cd.UsedAtMs != action.AtMs || cd.ReadyAtMs != state.EndsAtMs {
				t.Fatalf("cooldown = %#v", cd)
			}
			// A later genuine cast must still be accepted.
			p.At = p.At.Add(31 * time.Minute)
			publisher.publishTransformationComplete(p)
			if len(publisher.pendingEvents) != 2 {
				t.Fatal("later transformation was suppressed")
			}
			r.onEvent(publisher.pendingEvents[1])
			if state = r.effectTimers["transform"]; state.StartedAtMs != p.At.UnixMilli() || state.Generation != 2 {
				t.Fatalf("next duration timer = %#v", state)
			}
		})
	}
}

func TestTransformationCompleteRejectsOtherSignals(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*packet.GamePacket)
	}{
		{"prepare", func(p *packet.GamePacket) { p.Op = packet.OpcodeSkillPrepareReady }},
		{"cancel", func(p *packet.GamePacket) { p.Op = packet.OpcodeSkillPrepareEnd }},
		{"passive enable", func(p *packet.GamePacket) { p.Op = 27044 }},
		{"normal skill completion", func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemShort(21002) }},
		{"transformation passive", func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemShort(43011) }},
		{"transformation attack", func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemShort(43002) }},
		{"other player", func(p *packet.GamePacket) { p.Id = 200 }},
		{"zero source", func(p *packet.GamePacket) { p.Id = 0 }},
		{"truncated", func(p *packet.GamePacket) { p.Msg = p.Msg[:1] }},
		{"wrong skill type", func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemInt(43001) }},
		{"wrong result type", func(p *packet.GamePacket) { p.Msg[1] = packet.NewMessageElemByte(0) }},
		{"nonempty result", func(p *packet.GamePacket) { p.Msg[1] = packet.NewMessageElemString("failed") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publisher := newSkillTestPublisher()
			publisher.localEntityId = 100
			publisher.lastSentEventAt = time.Now().Add(time.Hour)
			p := transformationTestPacket(43001)
			tc.edit(p)
			publisher.publishTransformationComplete(p)
			publisher.publishTransformationComplete(nil)
			if len(publisher.pendingEvents) != 0 {
				t.Fatalf("unexpected action for %s", tc.name)
			}
		})
	}
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now().Add(time.Hour)
	publisher.publishTransformationComplete(transformationTestPacket(43001))
	if len(publisher.pendingEvents) != 0 {
		t.Fatal("unknown source was incorrectly inferred to be local")
	}
}

// Optional local regression. The user's original capture remains outside the
// repository; only the expected cast timestamp is retained here.
func TestTransformationCaptureReplay(t *testing.T) {
	file := os.Getenv("DILMETER_TRANSFORMATION_PCAP")
	if file == "" {
		t.Skip("set DILMETER_TRANSFORMATION_PCAP to replay the 2026-10-18 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	r := transformationTestRuntime(43001)
	r.localID = ""
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	const wantAt = int64(1792254730604)
	casts := 0
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	for {
		select {
		case batch := <-events:
			idle.Reset(4 * time.Second)
			for _, e := range batch {
				r.onEvent(e)
				if action, ok := e.(*event.EventSkillAction); ok && action.SkillId == 43001 {
					casts++
					if action.AtMs != wantAt || !action.IsLocal || action.IsFallback || action.SourceId != r.localID {
						t.Fatalf("unexpected captured transformation: %#v", action)
					}
				}
			}
		case <-idle.C:
			_, parsed, errors := reader.GetStats()
			if casts != 1 || parsed != 252 || errors != 0 {
				t.Fatalf("casts=%d parsed=%d errors=%d; want 1/252/0", casts, parsed, errors)
			}
			state := r.effectTimers["transform"]
			if state.StartedAtMs != wantAt || state.EndsAtMs != wantAt+1_800_000 || state.Generation != 1 {
				t.Fatalf("captured duration timer = %#v", state)
			}
			// Follow the native overlay output through a normal background refresh,
			// just before expiry, and expiry. This is the user's 1800-second rule.
			for _, offset := range []int64{200, 1_799_999, 1_800_000} {
				r.publishNativeSkillState(time.UnixMilli(wantAt + offset))
				skillOverlayState.RLock()
				data := append([]byte(nil), skillOverlayState.data...)
				skillOverlayState.RUnlock()
				var output nativeSkillOverlayMessage
				if err := json.Unmarshal(data, &output); err != nil {
					t.Fatal(err)
				}
				want := 1
				if offset == 1_800_000 {
					want = 0
				}
				if len(output.EffectTimers) != want {
					t.Fatalf("overlay at +%d ms: %#v", offset, output.EffectTimers)
				}
			}
			t.Logf("252 packets, 0 errors; one transformation at %d; 1800-second timer and overlay expiry passed", wantAt)
			return
		case <-ctx.Done():
			t.Fatal("transformation capture replay timed out")
		}
	}
}
