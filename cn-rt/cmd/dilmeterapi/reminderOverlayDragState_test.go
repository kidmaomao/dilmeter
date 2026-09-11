package main

import "testing"

func TestNormalizeNativeReminderLockDefaultsAndPreservesUnlock(t *testing.T) {
	defaults := normalizeNativeReminderSettings(nativeReminderSettings{})
	if !nativeReminderSettingsLocked(defaults) {
		t.Fatal("legacy settings must default to locked/mouse-through")
	}
	unlocked := false
	settings := normalizeNativeReminderSettings(nativeReminderSettings{Buff: nativeBuffSettings{Locked: &unlocked}})
	if nativeReminderSettingsLocked(settings) {
		t.Fatal("an explicit unlocked setting was discarded")
	}
}

func TestApplyNativeReminderPosition(t *testing.T) {
	settings := nativeReminderSettings{
		Buff: nativeBuffSettings{Rules: map[uint32]nativeBuffRule{1080: {CCID: 1080}}},
		SkillCooldowns: nativeSkillCooldownSettings{
			Rules: map[uint16]nativeSkillCooldownRule{21002: {SkillID: 21002}},
		},
		BossMechanics: nativeBossMechanicSettings{Rules: map[string]nativeBossMechanicRule{
			"miel-orb": {Key: "miel-orb"},
		}},
		EffectTimers: nativeEffectTimerSettings{Rules: map[string]nativeEffectTimerRule{
			"boost": {Key: "boost", SourceID: 123},
		}},
	}
	updates := []nativeReminderPositionUpdate{
		{Kind: "skill", ID: "21002", X: 101, Y: 202},
		{Kind: "aim", ID: "aim", X: 303, Y: 404},
		{Kind: "mechanic", ID: "miel-orb", X: 505, Y: 606},
		{Kind: "target-health", ID: "miel-shard", X: 607, Y: 708},
		{Kind: "effect", ID: "boost", X: 609, Y: 710},
		{Kind: "stack", ID: "1080", X: 707, Y: 808},
	}
	for _, update := range updates {
		if !applyNativeReminderPosition(&settings, update) {
			t.Fatalf("position update was rejected: %#v", update)
		}
	}
	if rule := settings.SkillCooldowns.Rules[21002]; rule.X != 101 || rule.Y != 202 {
		t.Fatalf("skill position = (%d,%d)", rule.X, rule.Y)
	}
	if aim := settings.SkillCooldowns.AimReminder; aim.X != 303 || aim.Y != 404 {
		t.Fatalf("aim position = (%d,%d)", aim.X, aim.Y)
	}
	if rule := settings.BossMechanics.Rules["miel-orb"]; rule.X != 505 || rule.Y != 606 {
		t.Fatalf("mechanic position = (%d,%d)", rule.X, rule.Y)
	}
	if boss := settings.BossMechanics; boss.MielShardHealthBarX != 607 || boss.MielShardHealthBarY != 708 {
		t.Fatalf("target health position = (%d,%d)", boss.MielShardHealthBarX, boss.MielShardHealthBarY)
	}
	if rule := settings.EffectTimers.Rules["boost"]; rule.X != 609 || rule.Y != 710 {
		t.Fatalf("effect position = (%d,%d)", rule.X, rule.Y)
	}
	if rule := settings.Buff.Rules[1080]; rule.StackX != 707 || rule.StackY != 808 {
		t.Fatalf("stack position = (%d,%d)", rule.StackX, rule.StackY)
	}
}

func TestApplyNativeReminderDragOverride(t *testing.T) {
	previous := currentNativeReminderDrag()
	defer func() {
		nativeReminderDragState.Lock()
		nativeReminderDragState.value = previous
		nativeReminderDragState.Unlock()
	}()
	publishNativeReminderDrag("skill", "21002", 444, 555, true, false)
	message := nativeSkillOverlayMessage{Items: []nativeSkillOverlayItem{{SkillID: 21002, X: 1, Y: 2}}}
	applyNativeReminderDragOverride(&message)
	if message.Items[0].X != 444 || message.Items[0].Y != 555 {
		t.Fatalf("drag override position = (%d,%d)", message.Items[0].X, message.Items[0].Y)
	}
	publishNativeReminderDrag("target-health", "miel-60", 666, 777, true, false)
	message.TargetHealth = &nativeTargetHealthOverlayItem{X: 3, Y: 4}
	applyNativeReminderDragOverride(&message)
	if message.TargetHealth.X != 666 || message.TargetHealth.Y != 777 {
		t.Fatalf("target health drag override position = (%d,%d)", message.TargetHealth.X, message.TargetHealth.Y)
	}
	publishNativeReminderDrag("effect", "boost", 888, 999, true, false)
	message.EffectTimers = []nativeEffectTimerOverlayItem{{Key: "boost", X: 5, Y: 6}}
	applyNativeReminderDragOverride(&message)
	if message.EffectTimers[0].X != 888 || message.EffectTimers[0].Y != 999 {
		t.Fatalf("effect drag override position = (%d,%d)", message.EffectTimers[0].X, message.EffectTimers[0].Y)
	}
}
