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
				for i := 0; i < 16; i++ {
					cards = append(cards, healerCard{Key: fmt.Sprintf("%s-%d", kind, i), MemberKey: "a", Name: "七罪", Category: kind, Value: "未观测", X: 40, Y: 160})
				}
			}
			frame := buildHealerOverlayFrame(fontSize, iconSize, cards, false, 1000)
			group := frame.Groups[0]
			if group.Columns != 4 || len(group.Sections) != 2 || group.Sections[0].Kind != "skill" || len(group.Sections[1].Cards) != 16 {
				t.Fatalf("missing or mixed sections: %+v", group)
			}
			// Two sections, four rows each, with all last-row text inside the window.
			if group.Height < 14+8*group.CellHeight+6*6+7 || frame.Height < group.Height+24 {
				t.Fatalf("wrapped rows clipped at font=%d icon=%d: %+v", fontSize, iconSize, frame)
			}
			for i := range cards {
				cards[i].Value = "1"
			}
			after := buildHealerOverlayFrame(fontSize, iconSize, cards, false, 1100).Groups[0]
			if after.Width != group.Width || after.Height != group.Height {
				t.Fatal("status/countdown change moved the grid")
			}
			short := buildHealerOverlayFrame(fontSize, iconSize, cards[:4], false, 1100).Groups[0]
			if short.Width != group.Width {
				t.Fatal("adding more rows made the window wider")
			}
		}
	}
}

func TestHealerStatusGridCompactNameAndNonOverlappingMembers(t *testing.T) {
	cards := []healerCard{
		{MemberKey: "one", Name: "七罪", Category: "buff", Value: "371", X: 40, Y: 160},
		{MemberKey: "one", Name: "七罪", Category: "skill", Value: "就绪", X: 40, Y: 160},
		{MemberKey: "two", Name: "另一位队友", Category: "skill", Value: "未观测", X: 40, Y: 232},
		{MemberKey: "three", Name: "侧边队友", Category: "buff", Value: "10", X: 800, Y: 232},
	}
	frame := buildHealerOverlayFrame(12, 30, cards, false, 1000)
	first, second, side := frame.Groups[0], frame.Groups[1], frame.Groups[2]
	if first.NameWidth != 28 || first.Width >= 180 {
		t.Fatalf("short name kept a large blank column: %+v", first)
	}
	if second.Y < first.Y+first.Height+6 || side.Y != 232 || cards[2].Y != 232 {
		t.Fatalf("panels overlap or saved anchor was changed: %+v", frame.Groups)
	}
	for _, group := range frame.Groups {
		if group.X+group.Width > frame.X+frame.Width || group.Y+group.Height > frame.Y+frame.Height {
			t.Fatal("native frame clips a repositioned panel")
		}
	}
}
