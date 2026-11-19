package main

import (
	"fmt"
	"testing"
)

func TestHealerStatusGridFitsAllCardsAndKeepsWidth(t *testing.T) {
	for _, fontSize := range []int{12, 16, 24, 72} {
		for _, iconSize := range []int{16, 28, 30, 80} {
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
			captionExtension := min(iconSize/4, (max(10, fontSize*3/4)+2)/2)
			expectedHeight := 6 + 8 + fontSize + 8 + 4 + rows*(iconSize+captionExtension) + (rows-1)*2
			if group.Height != expectedHeight || group.CellWidth != iconSize || group.CellHeight != iconSize+captionExtension || frame.Height != group.Height+24 {
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

func TestHealerTeamWrapsOneHorizontalStripAtFixedWidth(t *testing.T) {
	for _, limit := range []int{280, 660} {
		base := healerOverlayGroup{Rows: []healerTeamRow{{Name: "短", PartySlot: 1}}}
		layoutHealerTeam(&base, 16, 28, limit)
		for _, tc := range []struct{ skills, buffs, rows int }{
			{0, 0, 0}, {1, 0, 1}, {0, 2, 1}, {1, 1, 1}, {6, 0, 1}, {0, 6, 1}, {3, 3, 1}, {3, 4, 2}, {6, 6, 2}, {6, 7, 3}, {16, 16, 6},
		} {
			member := healerTeamRow{Name: "带有很长名字的队友", PartySlot: 16, Skills: make([]healerCard, tc.skills), Buffs: make([]healerCard, tc.buffs)}
			team := healerOverlayGroup{Rows: []healerTeamRow{member}}
			layoutHealerTeam(&team, 16, 28, limit)
			stripHeight := 0
			if tc.rows > 0 {
				stripHeight = 4 + tc.rows*35 + (tc.rows-1)*2
			}
			if team.Columns != 6 || team.Width != 224 || team.Width != base.Width || team.NameWidth != base.NameWidth || team.Height != base.Height+stripHeight {
				t.Fatalf("width limit %d, %d skills / %d Buffs: got %d columns and %dx%d", limit, tc.skills, tc.buffs, team.Columns, team.Width, team.Height)
			}
			team.Rows = append(team.Rows, healerTeamRow{Name: "新队友", Health: &healerCard{}, Mana: &healerCard{}, Curtain: "9223372036854775807"})
			layoutHealerTeam(&team, 16, 28, limit)
			if team.Width != base.Width || team.Height != base.Height+stripHeight+52 {
				t.Fatalf("a shield and mana row should add 52 pixels of height at the same width: %+v", team)
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
