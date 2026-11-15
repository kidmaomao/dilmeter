package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestPerlonTornadoFlushPreservesEntityOrder(t *testing.T) {
	p, batches := perlonPacketTestPublisher()
	p.entityCache[100] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 100, RaceId: perlonRaceID}}
	appearance := liveTestAppear("100", "佩隆", perlonRaceID, 100)
	p.publish(appearance)
	if len(batches) != 0 {
		t.Fatal("ordinary entity update should still be queued")
	}
	p.publishPerlonPacket(&packet.GamePacket{At: time.UnixMilli(100500), Id: 100, Op: 37011, Msg: packet.Message{
		packet.NewMessageElemInt(967), packet.NewMessageElemInt(1), packet.NewMessageElemLong(900),
		packet.NewMessageElemFloat(97780), packet.NewMessageElemFloat(137046), packet.NewMessageElemFloat(300),
		packet.NewMessageElemInt(1000), packet.NewMessageElemInt(3000),
	}})
	select {
	case batch := <-batches:
		if len(batch) != 2 || batch[0] != appearance || batch[1].(*event.EventSkillAction).MechanicSignal != perlonTornadoKey {
			t.Fatalf("urgent cast lost preceding boss appearance: %+v", batch)
		}
	default:
		t.Fatal("tornado waited for the next batch timer")
	}
}

func TestPerlonTornadoDelayedFrameAndCleanup(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := perlonTestRuntime(&sounds)
	r.onEvent(liveTestAppear("boss", "佩隆", perlonRaceID, 100))
	cast := &event.EventSkillAction{EventBase: event.EventBase{Id: "boss", At: 101}, AtMs: 101000, MechanicSignal: perlonTornadoKey, MechanicDurationMs: 1000}
	r.onEvent(cast)
	for _, ms := range []int64{101000, 101999, 102000, 102500, 103999} {
		items := r.perlonOverlay(time.UnixMilli(ms))
		if len(items) != 1 || items[0].TornadoStartsAtMs != 102000 || items[0].EndsAtMs != 104000 {
			t.Fatalf("at %d: display retention changed the real startup or lost the alert: %+v", ms, items)
		}
		message := nativeSkillOverlayMessage{Mechanics: items}
		if !nativeSkillOverlayMessageVisible(message, ms) {
			t.Fatalf("at %d: delayed overlay frame was discarded", ms)
		}
	}
	for _, ms := range []int64{100999, 104000, 110000} {
		if len(r.perlonOverlay(time.UnixMilli(ms))) != 0 {
			t.Fatalf("alert visible outside its window at %d", ms)
		}
	}
	r.onEvent(cast) // Repeated evidence must not replay sound or extend the deadline.
	if len(sounds) != 1 || len(r.perlonOverlay(time.UnixMilli(104001))) != 0 {
		t.Fatal("duplicate cast extended or replayed the warning")
	}
	r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "boss", At: 102}})
	if len(r.perlonOverlay(time.UnixMilli(102500))) != 0 {
		t.Fatal("boss death retained the post-start warning")
	}
}

func TestPerlonTornadoUsesIndependentSoundChannel(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := perlonTestRuntime(&sounds)
	for _, mode := range []string{"dedicated", "custom", "off"} {
		rule := r.settings.BossMechanics.Rules[perlonTornadoKey]
		rule.SoundMode, rule.CustomSoundID = mode, "custom-file"
		r.settings.BossMechanics.Rules[perlonTornadoKey] = rule
		r.playPerlonSound(perlonTornadoKey)
	}
	r.playPerlonSound(perlonResonanceKey)
	if len(sounds) != 3 || !sounds[0].Tornado || !sounds[1].Tornado || sounds[2].Tornado ||
		sounds[0].Kind != "skill-ready" || sounds[1].SoundID != "custom-file" || !sounds[2].Priority {
		t.Fatalf("sound modes or channel isolation changed: %+v", sounds)
	}
}
