//go:build windows

package main

import "testing"

func TestHealerWebViewScalesOneTeamPanelWithSelfMana(t *testing.T) {
	text := healerTextSettings{FontSize: 16, ScalePercent: 155, Width: 660, X: -500, Y: 160}
	frame := buildHealerOverlayFrame(text, 20, []healerCard{
		{MemberKey: "self", Name: "Oneforall", Category: "mana", X: 40, Y: 220, Value: "20%"},
		{MemberKey: "one", Name: "另一位队友", Category: "buff", Value: "1380s", CCID: 680},
	}, []healerMemberState{
		{Key: "self", Name: "Oneforall", PartySlot: 2, showHealth: true},
		{Key: "one", Name: "另一位队友", PartySlot: 1, showHealth: true},
	}, false, 1000)
	if len(frame.Groups) != 1 || frame.X != -512 || frame.Y != 148 || frame.ScalePercent != 155 {
		t.Fatalf("incorrect overlay bounds: %+v", frame)
	}
	panel := frame.Groups[0]
	if len(panel.Rows) != 2 || panel.Rows[0].PartySlot != 1 || panel.Rows[1].Key != "self" ||
		panel.Rows[1].Mana == nil || panel.Rows[1].Mana.Value != "20%" || panel.Rows[1].Health == nil {
		t.Fatalf("ordered team rows did not include the local mana bar: %+v", panel.Rows)
	}
	if frame.Width != (panel.Width*155+99)/100+24 || frame.Height != (panel.Height*155+99)/100+24 {
		t.Fatal("scaled overlay bounds do not fit the entire panel")
	}
}

func TestHealerWindowUsesSharedDPIWithoutMovingItsDesktopAnchor(t *testing.T) {
	runtime, _ := healerFixture(t)
	h := runtime.healer
	h.previewText = healerTextSettings{X: -500, Y: 160, Width: 660, ScalePercent: 155, FontSize: 16}
	h.previewIconSize, h.previewUntilMs = 20, 2000
	for _, dpi := range []int{0, 125, 150, 200, 100} {
		runtime.settings.Buff.DPIPercent = dpi
		frame := runtime.healerOverlaySnapshot(1000)
		wantDPI := dpi
		if dpi == 0 {
			wantDPI = nativeReminderScalePercent(healerOverlayWindow.Load(), 0)
		}
		if frame.DPIPercent != wantDPI || frame.ScalePercent != 155 || !frame.Preview {
			t.Fatalf("DPI %d: incorrect preview settings %+v", dpi, frame)
		}
		x, y, width, height := frame.desktopBounds()
		margin := (12*wantDPI + 99) / 100
		if x+margin != -500 || y+margin != 160 || width != (frame.Width*wantDPI+99)/100 || height != (frame.Height*wantDPI+99)/100 {
			t.Fatalf("DPI %d: unexpected desktop bounds (%d,%d) %dx%d", dpi, x, y, width, height)
		}
	}
}
