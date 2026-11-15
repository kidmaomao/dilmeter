package main

import (
	"fmt"
	"testing"
)

func TestHealerStatusGridFitsAllCardsAndKeepsWidth(t *testing.T) {
	for _, fontSize := range []int{12, 16, 24, 72} {
		for _, iconSize := range []int{16, 30, 80} {
			cards := []healerCard{}
			for _, kind := range []string{"buff", "skill"} {
				for i := range 16 {
					cards = append(cards, healerCard{Key: fmt.Sprintf("%s-%d", kind, i), MemberKey: "a", Name: "七罪", Category: kind, Value: "未观测"})
				}
			}
			text := healerTextSettings{FontSize: fontSize, Width: 660}
			frame := buildHealerOverlayFrame(text, iconSize, cards, nil, false, 1000)
			group := frame.Groups[0]
			if len(group.Rows) != 1 || len(group.Rows[0].Skills) != 16 || len(group.Rows[0].Buffs) != 16 {
				t.Fatalf("missing status icons: %+v", group)
			}
			rows := (len(cards) + group.Columns - 1) / group.Columns
			if group.Height != 16+rows*max(fontSize+4, iconSize)+(rows-1)*4 || frame.Height < group.Height+24 {
				t.Fatalf("wrapped rows clipped at font=%d icon=%d: %+v", fontSize, iconSize, frame)
			}
			for i := range cards {
				cards[i].Value = "1s"
			}
			after := buildHealerOverlayFrame(text, iconSize, cards, nil, false, 1100).Groups[0]
			if after.Width != group.Width || after.Height != group.Height {
				t.Fatal("status/countdown change moved the grid")
			}
		}
	}
}

func TestHealerOverlayPutsTeammatesInOneWindow(t *testing.T) {
	cards := []healerCard{
		{MemberKey: "one", Name: "七罪", Category: "buff", Value: "371s", X: 40, Y: 160},
		{MemberKey: "one", Name: "七罪", Category: "skill", Value: "就绪", X: 40, Y: 160},
		{MemberKey: "two", Name: "另一位队友", Category: "skill", Value: "未观测", X: 40, Y: 232},
		{MemberKey: "three", Name: "侧边队友", Category: "buff", Value: "10s", X: 800, Y: 232},
	}
	text := healerTextSettings{FontSize: 12, Width: 660, X: -40, Y: 160}
	frame := buildHealerOverlayFrame(text, 30, cards, nil, false, 1000)
	if len(frame.Groups) != 1 {
		t.Fatalf("teammates split across panels: %+v", frame)
	}
	group := frame.Groups[0]
	if group.Kind != "team" || len(group.Rows) != 3 || group.X != -40 || group.Y != 160 || frame.X != -52 || frame.Y != 148 {
		t.Fatalf("team panel did not use shared coordinates: %+v", frame)
	}
	if group.Rows[0].Skills[0].Value != "就绪" || group.Rows[0].Buffs[0].Value != "371s" || group.Rows[2].Name != "侧边队友" {
		t.Fatalf("status icons mixed across member rows: %+v", group.Rows)
	}
}
