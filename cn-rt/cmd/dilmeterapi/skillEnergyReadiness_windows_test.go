//go:build windows

package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestDarkEnergyReadinessIgnoresDigitsAndEarlyThreshold(t *testing.T) {
	for _, digits := range []bool{true, false} {
		for _, tc := range []struct {
			name             string
			percent          float64
			active, observed bool
			atMs             int64
			ready            bool
		}{
			{"partial energy", 95, true, true, 130000, false},
			{"cooling", 100, true, true, 129999, false},
			{"unknown cooldown", 100, true, false, 130000, false},
			{"inactive", 100, false, true, 130000, false},
			{"both ready", 100, true, true, 130000, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var sounds []nativeReminderSoundRequest
				r := newNativeReminderRuntimeForTest(nativeReminderSettings{
					SkillCooldowns: nativeSkillCooldownSettings{Rules: map[uint16]nativeSkillCooldownRule{
						darkEnergySkillID: {SkillID: darkEnergySkillID, Enabled: true, CooldownSeconds: 30, ProgressThresholdPercent: 80, ShowEnergyPercent: &digits, SoundMode: "default"},
					}},
				}, &sounds)
				rule := r.settings.SkillCooldowns.Rules[darkEnergySkillID]
				if rule.ProgressThresholdPercent != 100 {
					t.Fatal("legacy threshold was not normalized to full energy")
				}
				// Also defend against an unnormalized rule reaching evaluation.
				rule.ProgressThresholdPercent = 80
				r.settings.SkillCooldowns.Rules[darkEnergySkillID] = rule
				r.darkEnergy = event.EventSkillEnergy{Percent: tc.percent, Active: tc.active}
				if tc.observed {
					r.observeSkillCooldown(darkEnergySkillID, 100000, false)
				}
				r.evaluateEnergySkills(time.UnixMilli(tc.atMs))
				r.evaluateEnergySkills(time.UnixMilli(tc.atMs))
				if r.energyReady[darkEnergySkillID] != tc.ready || (len(sounds) == 1) != tc.ready || len(sounds) > 1 {
					t.Fatalf("digits=%t ready=%t sounds=%d, want ready=%t", digits, r.energyReady[darkEnergySkillID], len(sounds), tc.ready)
				}
			})
		}
	}
}

func TestDarkEnergyReminderStaysDimUntilBothReady(t *testing.T) {
	progress, observed, digits := 100.0, true, false
	item := nativeSkillOverlayItem{
		SkillID: darkEnergySkillID, EnergyGate: true, EnergyActive: true,
		CooldownObserved: true, ProgressPercent: &progress, ProgressObserved: &observed,
		ShowEnergyPercent: &digits, UsedAtMs: 100000, ReadyAtMs: 130000,
		X: 20, Y: 20,
	}
	render := func() (*image.RGBA, string) {
		canvas := image.NewRGBA(image.Rect(0, 0, 120, 140))
		texts := drawNativeSkillReminderItem(canvas, item, image.Point{}, 48, 125000)
		var label string
		for _, text := range texts {
			label += text.text
		}
		return canvas, label
	}
	waiting, label := render()
	if !strings.Contains(label, "能量已满") || !strings.Contains(label, "CD 5s") {
		t.Fatalf("hidden digits must still explain the unmet CD: %q", label)
	}
	item.EnergyReady, item.ReadyAtMs = true, 120000
	ready, _ := render()
	// Compare the actual icon interior, excluding its border/glow.
	brightness := func(canvas *image.RGBA) int {
		total := 0
		for y := 24; y < 64; y++ {
			for x := 24; x < 64; x++ {
				c := canvas.RGBAAt(x, y)
				total += int(c.R) + int(c.G) + int(c.B)
			}
		}
		return total
	}
	if brightness(waiting) >= brightness(ready) {
		t.Fatal("full energy with CD still running looks as bright as ready")
	}
	progress, item.EnergyReady = 95, false
	_, label = render()
	if !strings.Contains(label, "能量未满") {
		t.Fatalf("hidden digits conceal incomplete energy: %q", label)
	}
}
