package main

import (
	"strconv"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestHealingHandsRepeatedTargetRestartsCooldown(t *testing.T) {
	const caster uint64 = 4503599631566088
	publisher := newSkillTestPublisher()
	publisher.localEntityId = caster
	publisher.lastSentEventAt = time.Now()
	var sounds []nativeReminderSoundRequest
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{
		SkillCooldowns: nativeSkillCooldownSettings{Rules: map[uint16]nativeSkillCooldownRule{
			59000: {SkillID: 59000, Enabled: true, CooldownSeconds: 5, SoundMode: "default"},
		}},
	}, &sounds)
	runtime.localID = strconv.FormatUint(caster, 10)
	// Captured October 26 confirmations: same target, separate successful casts.
	for index, atMs := range []int64{1793017208570, 1793017219956} {
		p := &packet.GamePacket{At: time.UnixMilli(atMs), Id: caster, Msg: packet.Message{
			packet.NewMessageElemShort(59000), packet.NewMessageElemLong(caster),
			packet.NewMessageElemInt(0), packet.NewMessageElemInt(1),
		}}
		if err := publisher.publishSkillExecutePacket(p); err != nil {
			t.Fatal(err)
		}
		if len(publisher.pendingEvents) != index+1 {
			t.Fatalf("cast %d suppressed for repeated healing target", index+1)
		}
		action := publisher.pendingEvents[index].(*event.EventSkillAction)
		if action.SkillId != 59000 || !action.IsLocal || action.IsFallback || action.AtMs != atMs {
			t.Fatalf("unexpected healing action: %#v", action)
		}
		runtime.onEvent(action)
		got := runtime.skillCooldowns[59000]
		if got.UsedAtMs != atMs || got.ReadyAtMs != atMs+5000 || got.Generation != uint64(index+1) {
			t.Fatalf("cast %d cooldown = %#v", index+1, got)
		}
		// Adjacent observations must still produce one countdown and ready sound.
		runtime.onEvent(action)
		runtime.evaluateSkillCooldowns(time.UnixMilli(atMs + 5000))
		runtime.evaluateSkillCooldowns(time.UnixMilli(atMs + 5100))
		if len(sounds) != index+1 {
			t.Fatalf("cast %d ready sound count = %d", index+1, len(sounds))
		}
	}
}
