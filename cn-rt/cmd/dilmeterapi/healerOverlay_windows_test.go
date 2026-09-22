//go:build windows

package main

import "testing"

func TestHealerWebViewFrameKeepsIndependentCoordinates(t *testing.T) {
	frame := buildHealerOverlayFrame(16, 30, []healerCard{
		{MemberKey: "one", Name: "Oneforall", Category: "health", X: -500, Y: 220, Value: "20%"},
		{MemberKey: "one", Name: "Oneforall", Category: "buff", X: 40, Y: 160, Value: "1380s", CCID: 680},
		{MemberKey: "one", Name: "Oneforall", Category: "buff", X: 40, Y: 160, Value: "25s", CCID: 192},
		{MemberKey: "two", Name: "第二位队友", Category: "buff", X: 40, Y: 240, Value: "9s", CCID: 680},
	}, false, 1000)
	if len(frame.Groups) != 3 || frame.X != -512 || frame.Y != 148 {
		t.Fatalf("incorrect independent positions: %+v", frame)
	}
	if frame.Groups[1].Cards[0].Value != "1380" || frame.Groups[1].Height != 66 || len(frame.Groups[1].Cards) != 2 {
		t.Fatal("Buffs must share a compact per-member row")
	}
	for _, group := range frame.Groups {
		if group.X < frame.X || group.Y < frame.Y || group.X+group.Width > frame.X+frame.Width || group.Y+group.Height > frame.Y+frame.Height {
			t.Fatal("clipped overlay group")
		}
	}
}
