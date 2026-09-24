package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestHealerConfiguredBuffWindowAppearsBeforeBuff(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	h.settings.Text.Enabled = true
	h.settings.Members[0].Health = false
	checkFrame := func(at int64, vivaceState, vivaceValue string) {
		t.Helper()
		healerTick(runtime, at)
		frame := h.overlaySnapshot(at)
		if len(frame.Groups) != 1 || frame.Groups[0].Name != "队友" || len(frame.Groups[0].Cards) != 2 {
			t.Fatalf("configured teammate window missing: %+v", frame)
		}
		cards := frame.Groups[0].Cards
		if cards[0].CCID != 680 || cards[0].State != "missing" || cards[0].Value != "补充" || !cards[0].Flash {
			t.Fatalf("unobserved overture must use expired appearance: %+v", cards[0])
		}
		if cards[1].CCID != 192 || cards[1].State != vivaceState || cards[1].Value != vivaceValue {
			t.Fatalf("vivace card incorrect: %+v", cards[1])
		}
		if h.state.Members[0].Overture.State != "unknown" || h.state.Members[0].Overture.RemainingSeconds != nil {
			t.Fatal("display placeholder fabricated an observed Buff")
		}
	}
	checkFrame(100_000, "missing", "补充")
	checkFrame(104_000, "missing", "补充")
	if len(*sounds) != 0 || len(h.state.Alerts) != 0 || len(h.alerts) != 0 {
		t.Fatal("unobserved placeholders created an alert or consumed a sound quota")
	}

	// Match the report: overture stays unobserved while vivace has a real timer.
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 105}, CCId: 192, DisableAt: 700})
	checkFrame(105_000, "active", "595")
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "ally", At: 106}, CCId: 192})
	checkFrame(106_000, "missing", "补充")
	if len(*sounds) != 0 {
		t.Fatal("keeping the missing card visible bypassed sound debounce")
	}
	checkFrame(110_000, "missing", "补充")
	if len(*sounds) != 1 || (*sounds)[0].Kind != "healer-music" {
		t.Fatal("confirmed removal must retain its normal sound reminder")
	}

	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "ally", At: 111}})
	healerTick(runtime, 111_000)
	if len(h.overlaySnapshot(111_000).Groups) != 0 {
		t.Fatal("out-of-view teammate retained a Buff window")
	}
	runtime.onEvent(liveTestAppear("ally", "队友", 10001, 112))
	checkFrame(112_000, "missing", "补充")
	checkFrame(114_000, "missing", "补充")
	if len(*sounds) != 1 {
		t.Fatal("reappearance with unknown Buffs repeated a sound")
	}
}

func TestHealerUnobservedBuffWindowRespectsControls(t *testing.T) {
	for _, scenario := range []string{"global-off", "buff-off", "window-off", "icon-off", "flash-off", "capture-off", "unselected"} {
		t.Run(scenario, func(t *testing.T) {
			runtime, sounds := healerFixture(t)
			h := runtime.healer
			h.settings.Text.Enabled = true
			member := normalizeHealerMember(healerMemberSelection{ID: "ally", Name: "队友", Included: true, Overture: true}, h.settings, 0)
			member.BuffSettings.Overlay = healerPosition{Enabled: true, X: -530, Y: 690}
			h.settings.Members = []healerMemberSelection{member}
			no := false
			switch scenario {
			case "global-off":
				h.settings.Enabled = false
			case "buff-off":
				member.BuffSettings.Enabled = false
			case "window-off":
				member.BuffSettings.Overlay.Enabled = false
			case "icon-off":
				member.BuffSettings.Rules[0].OverlayEnabled = &no
			case "flash-off":
				member.BuffSettings.Rules[0].FlashEnabled = &no
			case "unselected":
				h.settings.Members = nil
			}
			h.evaluate(runtime, time.Unix(100, 0), scenario != "capture-off")
			frame := h.overlaySnapshot(100_000)
			if scenario == "flash-off" {
				if len(frame.Groups) != 1 || len(frame.Groups[0].Cards) != 1 || frame.Groups[0].Cards[0].Flash || frame.Groups[0].X != -530 || frame.Groups[0].Y != 690 {
					t.Fatalf("placeholder ignored flash/position settings: %+v", frame)
				}
			} else if len(frame.Groups) != 0 {
				t.Fatalf("%s still displayed a window: %+v", scenario, frame)
			}
			if len(*sounds) != 0 || len(h.state.Alerts) != 0 {
				t.Fatal("unknown Buff raised an alert")
			}
		})
	}
}
