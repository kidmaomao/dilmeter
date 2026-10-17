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

func TestTransformationCancelPacket(t *testing.T) {
	for _, skill := range []uint16{40001, 41001, 42001, 43001, 44001} {
		t.Run(strconv.Itoa(int(skill)), func(t *testing.T) {
			p := newSkillTestPublisher()
			p.localEntityId = 100
			p.lastSentEventAt = time.Now().Add(time.Hour)
			raw := transformationTestPacket(skill)
			raw.Op = packet.OpcodeSkillCancel
			p.publishTransformationCancel(raw)
			if len(p.pendingEvents) != 1 {
				t.Fatalf("events: %v", p.pendingEvents)
			}
			state, ok := p.pendingEvents[0].(*event.EventSkillState)
			if !ok || state.Active || state.Scope != "active" || state.SkillId != skill || state.Id != "100" || state.AtMs != raw.At.UnixMilli() {
				t.Fatalf("cancel must emit inactive state, not a cast: %#v", p.pendingEvents[0])
			}
		})
	}
	for _, edit := range []func(*packet.GamePacket){
		func(p *packet.GamePacket) { p.Id = 200 },
		func(p *packet.GamePacket) { p.Id = 0 },
		func(p *packet.GamePacket) { p.Op = packet.OpcodeSkillPrepareEnd },
		func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemShort(43002) },
		func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemShort(59046) },
		func(p *packet.GamePacket) { p.Msg[0] = packet.NewMessageElemInt(43001) },
		func(p *packet.GamePacket) { p.Msg = p.Msg[:1] },
		func(p *packet.GamePacket) { p.Msg[1] = packet.NewMessageElemString("failed") },
	} {
		publisher := newSkillTestPublisher()
		publisher.localEntityId = 100
		publisher.lastSentEventAt = time.Now().Add(time.Hour)
		p := transformationTestPacket(43001)
		p.Op = packet.OpcodeSkillCancel
		edit(p)
		publisher.publishTransformationCancel(p)
		publisher.publishTransformationCancel(nil)
		if len(publisher.pendingEvents) != 0 {
			t.Fatal("invalid cancellation accepted")
		}
	}
}

func TestTransformationDurationEndsWithoutResettingCooldown(t *testing.T) {
	for _, skill := range []uint16{40001, 41001, 42001, 43001, 44001} {
		for _, reason := range []string{"cancel", "death", "channel", "character"} {
			t.Run(strconv.Itoa(int(skill))+"/"+reason, func(t *testing.T) {
				r := transformationTestRuntime(skill)
				r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "100", At: 100}, SkillId: skill, AtMs: 100123, IsLocal: true, SourceId: "100"})
				cd := r.skillCooldowns[skill]
				var end event.IEvent
				switch reason {
				case "cancel":
					end = &event.EventSkillState{EventBase: event.EventBase{Id: "100", At: 100}, AtMs: 100500, SkillId: skill, Scope: "active"}
				case "death":
					end = &event.EventFinish{EventBase: event.EventBase{Id: "100", At: 100}}
				case "channel":
					end = &event.EventLocalEntity{EventBase: event.EventBase{Id: "0", At: 101}, Reset: true}
				case "character":
					end = &event.EventLocalEntity{EventBase: event.EventBase{Id: "200", At: 101}}
				}
				r.onEvent(end)
				r.onEvent(end)
				if _, exists := r.effectTimers["transform"]; exists {
					t.Fatal("transformation duration remained active")
				}
				if r.skillCooldowns[skill] != cd {
					t.Fatal("ending transformation changed cooldown")
				}
				r.publishNativeSkillState(time.UnixMilli(101000))
				skillOverlayState.RLock()
				data := append([]byte(nil), skillOverlayState.data...)
				skillOverlayState.RUnlock()
				var overlay nativeSkillOverlayMessage
				if err := json.Unmarshal(data, &overlay); err != nil {
					t.Fatal(err)
				}
				if len(overlay.EffectTimers) != 0 {
					t.Fatalf("cancelled bar remained in overlay: %#v", overlay.EffectTimers)
				}
			})
		}
	}
}

func TestTransformationEndKeepsUnrelatedAndNewerTimers(t *testing.T) {
	r := transformationTestRuntime(43001)
	cast := &event.EventSkillAction{EventBase: event.EventBase{Id: "100", At: 100}, SkillId: 43001, AtMs: 100123, IsLocal: true, SourceId: "100"}
	r.onEvent(cast)
	for _, e := range []event.IEvent{
		&event.EventSkillState{EventBase: event.EventBase{Id: "200", At: 101}, AtMs: 101000, SkillId: 43001, Scope: "active"},
		&event.EventSkillState{EventBase: event.EventBase{Id: "100", At: 101}, AtMs: 101000, SkillId: 44001, Scope: "active"},
		&event.EventSkillState{EventBase: event.EventBase{Id: "100", At: 99}, AtMs: 99999, SkillId: 43001, Scope: "active"},
		&event.EventSkillState{EventBase: event.EventBase{Id: "100", At: 101}, AtMs: 101000, SkillId: 43001, Scope: "prepare"},
		&event.EventSkillState{EventBase: event.EventBase{Id: "100", At: 101}, AtMs: 101000, SkillId: 43001, Scope: "active", Active: true},
		&event.EventFinish{EventBase: event.EventBase{Id: "200", At: 101}},
		&event.EventStatUpdate{EventBase: event.EventBase{Id: "100", At: 101}, Stats: []event.EventStatUpdateEntry{{StatId: 28, Value: -1}}},
		&event.EventEntityDisappear{EventBase: event.EventBase{Id: "100", At: 101}},
		&event.EventLocalEntity{EventBase: event.EventBase{Id: "100", At: 101}},
	} {
		r.onEvent(e)
		if _, ok := r.effectTimers["transform"]; !ok {
			t.Fatalf("unrelated event cleared transformation: %#v", e)
		}
	}
	r.settings.EffectTimers.Rules["ordinary"] = nativeEffectTimerRule{SourceType: "skill", SourceID: 21002, TargetMode: "self"}
	r.settings.EffectTimers.Rules["monster"] = nativeEffectTimerRule{SourceType: "skill", SourceID: 43001, TargetMode: "monster"}
	for _, key := range []string{"ordinary", "monster"} {
		r.effectTimers[key] = nativeEffectTimerRuntime{StartedAtMs: 100123, EndsAtMs: 999999, TargetID: "100"}
	}
	r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "100", At: 102}, AtMs: 102000, SkillId: 43001, Scope: "active"})
	if len(r.effectTimers) != 2 {
		t.Fatalf("unrelated timers removed: %#v", r.effectTimers)
	}
	cast.AtMs = 103123
	cast.At = 103
	r.onEvent(cast)
	if r.effectTimers["transform"].StartedAtMs != 103123 {
		t.Fatal("later transformation did not restart")
	}
}

func TestTransformationCancelCaptureReplay(t *testing.T) {
	file := os.Getenv("DILMETER_TRANSFORMATION_CANCEL_PCAP")
	if file == "" {
		t.Skip("set DILMETER_TRANSFORMATION_CANCEL_PCAP to replay the cancellation capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	r := transformationTestRuntime(43001)
	r.localID = ""
	if err = reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	const startAt = int64(1792255760995)
	const endAt = int64(1792255774682)
	starts, ends := 0, 0
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	for {
		select {
		case batch := <-events:
			idle.Reset(4 * time.Second)
			for _, e := range batch {
				r.onEvent(e)
				switch v := e.(type) {
				case *event.EventSkillAction:
					if v.SkillId != 43001 {
						continue
					}
					starts++
					if v.AtMs != startAt || r.effectTimers["transform"].StartedAtMs != startAt {
						t.Fatal("wrong transformation start")
					}
				case *event.EventSkillState:
					if v.SkillId != 43001 {
						continue
					}
					ends++
					if v.AtMs != endAt || v.Active || v.Scope != "active" {
						t.Fatalf("wrong cancellation: %#v", v)
					}
					if _, ok := r.effectTimers["transform"]; ok {
						t.Fatal("capture cancellation left bar active")
					}
					r.publishNativeSkillState(time.UnixMilli(endAt + 200))
					skillOverlayState.RLock()
					data := append([]byte(nil), skillOverlayState.data...)
					skillOverlayState.RUnlock()
					var overlay nativeSkillOverlayMessage
					if err := json.Unmarshal(data, &overlay); err != nil {
						t.Fatal(err)
					}
					if len(overlay.EffectTimers) != 0 {
						t.Fatal("cancelled bar reappeared during refresh")
					}
				}
			}
		case <-idle.C:
			_, parsed, errors := reader.GetStats()
			if starts != 1 || ends != 1 || parsed != 140 || errors != 0 {
				t.Fatalf("start/end/packets/errors = %d/%d/%d/%d", starts, ends, parsed, errors)
			}
			if r.skillCooldowns[43001].ReadyAtMs != startAt+1800000 {
				t.Fatal("cancellation changed cooldown")
			}
			t.Logf("140 packets, 0 errors; start %d -> cancel %d; duration ended after 13.687 seconds; cooldown preserved", startAt, endAt)
			return
		case <-ctx.Done():
			t.Fatal("capture replay timed out")
		}
	}
}
