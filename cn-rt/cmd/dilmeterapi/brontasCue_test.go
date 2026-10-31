package main

import "testing"

func TestBrontasActionCueNeverConfusesWaitingWithUnavailable(t *testing.T) {
	v := stationGuideFixture()
	v.Guidance, v.PaceUntilMs = "pace", 1200
	if c := brontasGuideCue(v, 1000); c.Mode != "wait" || c.Title != "等拍" || c.Detail != "起步准备" || c.Seconds != "0.2" {
		t.Fatal(c)
	}
	if c := brontasGuideCue(v, 1200); c.Mode != "unavailable" || c.Title != "暂无指引" {
		t.Fatal("clock released cue", c)
	}
	v.PaceUntilMs = 0
	if c := brontasGuideCue(v, 1000); c.Detail != "等脚下出线" {
		t.Fatal(c)
	}
	if c := brontasGuideCue(v, 1350); c.Mode != "unavailable" || c.Detail != "等待更新" {
		t.Fatal(c)
	}
	if c := brontasGuideCue(v, 1700); c.Detail != "定位中" {
		t.Fatal(c)
	}
	if c := brontasGuideCue(v, 5000); c.Detail != "信号中断" {
		t.Fatal(c)
	}
	v.Guidance = "blocked"
	if c := brontasGuideCue(v, 1000); c.Detail != "暂无可用路线" {
		t.Fatal(c)
	}
	v.Guidance, v.Player.Speed = "waiting", 0
	if c := brontasGuideCue(v, 1000); c.Detail != "等待测速" {
		t.Fatal(c)
	}
}

func TestBrontasActionCueFollowsActualDisplayedDirection(t *testing.T) {
	for _, facing := range []string{"outward", "inward", "north", "south"} {
		v := brontasStationPreview(100000, 103000, 9, facing)
		c := brontasGuideCue(v, 103000)
		if c.Mode != "go" || c.Title != "移动" {
			t.Fatal(c)
		}
		v.GuidanceKind = "escape"
		urgent := brontasGuideCue(v, 103000)
		if urgent.Mode != "urgent" || urgent.Title != "快躲" || urgent.Angle != c.Angle || urgent.Detail != c.Detail {
			t.Fatal("urgent cue changed route", urgent, c)
		}
		if v.StepAxis == "horizontal" {
			v.Player.Y += .2
		} else {
			v.Player.X += .2
		}
		if c := brontasGuideCue(v, 103000); c.Mode != "unavailable" || c.Detail != "位置变化" {
			t.Fatal("off-axis target still says go", c)
		}
	}
}
