package main

import "testing"

func TestNativeSkillOverlayMessageVisible(t *testing.T) {
	now := int64(100_000)
	progress, threshold, observed := 97.0, 95.0, true
	tests := []struct {
		name    string
		message nativeSkillOverlayMessage
		want    bool
	}{
		{
			name: "explicitly disabled",
			message: nativeSkillOverlayMessage{
				Settings:    nativeSkillOverlaySettings{Opacity: 100},
				AimReminder: &nativeAimReminderOverlayItem{Active: true},
			},
		},
		{
			name: "legacy message keeps overlay enabled",
			message: nativeSkillOverlayMessage{
				AimReminder: &nativeAimReminderOverlayItem{Active: true},
			},
			want: true,
		},
		{
			name: "active aim",
			message: nativeSkillOverlayMessage{
				Settings:    nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				AimReminder: &nativeAimReminderOverlayItem{Active: true},
			},
			want: true,
		},
		{
			name: "target health remains independently visible",
			message: nativeSkillOverlayMessage{
				Settings:     nativeSkillOverlaySettings{Opacity: 100},
				TargetHealth: &nativeTargetHealthOverlayItem{CurrentHealth: 1, MaximumHealth: 2},
			},
			want: true,
		},
		{
			name: "expired target health preview",
			message: nativeSkillOverlayMessage{
				TargetHealth: &nativeTargetHealthOverlayItem{CurrentHealth: 1, MaximumHealth: 2, PreviewEndsAt: now},
			},
		},
		{
			name: "future mechanic",
			message: nativeSkillOverlayMessage{
				Settings:  nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				Mechanics: []nativeBossMechanicOverlayItem{{EndsAtMs: now + 1}},
			},
			want: true,
		},
		{
			name: "future stack alert",
			message: nativeSkillOverlayMessage{
				Settings:    nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				StackAlerts: []nativeBuffStackOverlayItem{{EndsAtMs: now + 1}},
			},
			want: true,
		},
		{
			name: "active effect timer",
			message: nativeSkillOverlayMessage{
				Settings:     nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				EffectTimers: []nativeEffectTimerOverlayItem{{Enabled: true, EndsAtMs: now + 1}},
			},
			want: true,
		},
		{
			name: "persistent expired effect timer",
			message: nativeSkillOverlayMessage{
				Settings:     nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				EffectTimers: []nativeEffectTimerOverlayItem{{Enabled: true, AlwaysVisible: true}},
			},
			want: true,
		},
		{
			name: "ready burst",
			message: nativeSkillOverlayMessage{
				Settings: nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				Items:    []nativeSkillOverlayItem{{ReadyAtMs: now - 1}},
			},
			want: true,
		},
		{
			name: "expired ready burst",
			message: nativeSkillOverlayMessage{
				Settings: nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				Items:    []nativeSkillOverlayItem{{ReadyAtMs: now - 2600}},
			},
		},
		{
			name: "observed progress over threshold",
			message: nativeSkillOverlayMessage{
				Settings: nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100},
				Items: []nativeSkillOverlayItem{{
					ProgressPercent: &progress, ProgressThresholdPercent: &threshold, ProgressObserved: &observed,
				}},
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeSkillOverlayMessageVisible(test.message, now); got != test.want {
				t.Fatalf("visible = %v, want %v", got, test.want)
			}
		})
	}
}
