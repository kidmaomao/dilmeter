package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReminderPreviewsSurviveLiveFramesAndRestoreLatestState(t *testing.T) {
	preserveBurstPreviewState(t)
	const now int64 = 100000
	live := nativeSkillOverlayMessage{Type: "skill-cooldown-state", Settings: nativeSkillOverlaySettings{Opacity: 100},
		Items:        []nativeSkillOverlayItem{{SkillID: 59000, ReadyAtMs: 150000, Generation: 7}},
		AimReminder:  &nativeAimReminderOverlayItem{TargetID: "live-target", ReadyAtMs: 160000},
		EffectTimers: []nativeEffectTimerOverlayItem{{Key: "buff", EndsAtMs: 170000}},
		StackAlerts:  []nativeBuffStackOverlayItem{{CCID: 10, Stack: 2}},
		TargetHealth: &nativeTargetHealthOverlayItem{EntityID: "live-boss", CurrentHealth: 90, MaximumHealth: 100},
	}
	refresh := func(at int64) {
		t.Helper()
		raw, _ := json.Marshal(live)
		if !setSkillOverlayStateAt(raw, at) {
			t.Fatal("live frame rejected")
		}
	}
	refresh(now)
	setReminderPreview(reminderPreviewRequest{AtMs: now - 3600000, DurationMs: 3600,
		Items:        []nativeSkillOverlayItem{{SkillID: 59000, UsedAtMs: now - 3601000, ReadyAtMs: now - 3599100}},
		AimReminder:  &nativeAimReminderOverlayItem{Active: true, ReadyAtMs: now - 3598000},
		EffectTimers: []nativeEffectTimerOverlayItem{{Key: "buff", EndsAtMs: now - 3596400}},
		StackAlerts:  []nativeBuffStackOverlayItem{{CCID: 10, Stack: 9, Persistent: true}},
		TargetHealth: &nativeTargetHealthOverlayItem{EntityID: "sample-boss", CurrentHealth: 50, MaximumHealth: 100},
	}, now)
	// Both existing preview systems must compose with all the new preview kinds.
	setBurstReminderPreview(nativeBossMechanicOverlayItem{SkillID: 59005, Phase: "cast"}, now)
	setBossMechanicPreview(bossMechanicPreviewRequest{Key: "miel-orb", DurationSeconds: 8}, now)
	for _, elapsed := range []int64{0, 50, 200, 900, 2000, 3599} {
		refresh(now + elapsed)
		m := readBurstPreviewState(t)
		if len(m.Items) != 1 || m.Items[0].ReadyAtMs != now+900 || !nativeSkillReminderItemVisible(m.Items[0], now+elapsed) {
			t.Fatalf("skill preview lost at %d: %+v", elapsed, m.Items)
		}
		if m.AimReminder.ReadyAtMs != now+2000 || m.StackAlerts[0].Stack != 9 || !m.EffectTimers[0].Enabled || m.TargetHealth.EntityID != "sample-boss" || len(m.Mechanics) != 2 {
			t.Fatalf("preview collision at %d: %+v", elapsed, m)
		}
		if !nativeSkillOverlayMessageVisible(m, now+elapsed) {
			t.Fatal("preview hidden by disabled live overlays")
		}
	}
	// Live changes received beneath a preview must be restored, not the old frame.
	live.Items[0].ReadyAtMs = 155000
	live.TargetHealth.CurrentHealth = 75
	refresh(now + 3600)
	m := readBurstPreviewState(t)
	if m.Items[0].ReadyAtMs != 155000 || m.Items[0].Generation != 7 || m.AimReminder.TargetID != "live-target" ||
		m.EffectTimers[0].EndsAtMs != 170000 || m.StackAlerts[0].Stack != 2 || m.TargetHealth.CurrentHealth != 75 || len(m.Mechanics) != 2 {
		t.Fatalf("live state not restored: %+v", m)
	}
	refresh(now + 8000)
	if len(readBurstPreviewState(t).Mechanics) != 0 {
		t.Fatal("expired burst/boss preview retained")
	}
}

func TestReminderPreviewClicksHaveIndependentDeadlines(t *testing.T) {
	preserveBurstPreviewState(t)
	request := func(skill uint16, at int64) {
		setReminderPreview(reminderPreviewRequest{AtMs: at, DurationMs: 3600, Items: []nativeSkillOverlayItem{{SkillID: skill, ReadyAtMs: at + 900}}}, at)
	}
	request(59000, 100000)
	request(59001, 101000)
	request(59000, 102000)
	setBurstReminderPreview(nativeBossMechanicOverlayItem{SkillID: 58014, Phase: "effect"}, 104600)
	m := readBurstPreviewState(t)
	if len(m.Items) != 1 || m.Items[0].SkillID != 59000 || m.Items[0].PreviewExpiresAtMs != 105600 || len(m.Mechanics) != 1 {
		t.Fatalf("preview click renewed unrelated item or resurrected expired sample: %+v", m)
	}
	if nativeSkillReminderItemVisible(m.Items[0], 105600) {
		t.Fatal("stale preview visible without another live frame")
	}
}

func TestReminderPreviewHTTPValidation(t *testing.T) {
	preserveBurstPreviewState(t)
	for _, body := range []string{`{}`, `{"atMs":1,"durationMs":120001,"items":[{"skillId":59000}]}`, `{"atMs":1,"durationMs":3600}`, `{"atMs":1,"durationMs":3600,"items":[{"skillId":59000}]}`} {
		response := httptest.NewRecorder()
		handleReminderPreview(response, httptest.NewRequest("POST", "/api/reminder_preview", strings.NewReader(body)))
		want := 400
		if strings.Contains(body, `"durationMs":3600,"items"`) {
			want = 204
		}
		if response.Code != want {
			t.Fatalf("%s: status %d, want %d", body, response.Code, want)
		}
	}
}

func TestExpiredPreviewCannotKeepOverlayOrPersistentItemVisible(t *testing.T) {
	const now int64 = 100000
	enabled := nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100}
	for _, message := range []nativeSkillOverlayMessage{
		{Settings: enabled, Items: []nativeSkillOverlayItem{{AlwaysVisible: true, PreviewExpiresAtMs: now}}},
		{Settings: enabled, AimReminder: &nativeAimReminderOverlayItem{Active: true, AlwaysVisible: true, PreviewExpiresAtMs: now}},
		{Settings: enabled, EffectTimers: []nativeEffectTimerOverlayItem{{Enabled: true, AlwaysVisible: true, EndsAtMs: now + 50000, PreviewExpiresAtMs: now}}},
		{Settings: enabled, StackAlerts: []nativeBuffStackOverlayItem{{Persistent: true, PreviewExpiresAtMs: now}}},
	} {
		if nativeSkillOverlayMessageVisible(message, now) {
			t.Fatalf("expired sample still visible: %+v", message)
		}
	}
	// The window and individual icon must agree for real fully charged energy.
	full, observed := 100.0, true
	item := nativeSkillOverlayItem{EnergyGate: true, EnergyReady: true, ProgressPercent: &full, ProgressObserved: &observed}
	if !nativeSkillOverlayMessageVisible(nativeSkillOverlayMessage{Settings: enabled, Items: []nativeSkillOverlayItem{item}}, now) {
		t.Fatal("fully charged energy icon was visible but its overlay window was hidden")
	}
}
