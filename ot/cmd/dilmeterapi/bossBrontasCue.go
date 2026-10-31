package main

import (
	"fmt"
	"math"
)

type brontasActionCue struct {
	Mode, Title, Detail, Seconds string
	Angle                        float64
}

// Mirrors the WebView cue, including render-time expiry. A missing arrow is
// explicitly unavailable; only the planner's fresh pace/hold means wait.
func brontasGuideCue(v *nativeBrontasGuideView, now int64) brontasActionCue {
	unavailable := func(detail string) brontasActionCue {
		return brontasActionCue{Mode: "unavailable", Title: "暂无指引", Detail: detail}
	}
	wait := func(detail string) brontasActionCue {
		return brontasActionCue{Mode: "wait", Title: "等拍", Detail: detail}
	}
	if now >= v.ValidUntilMs {
		return unavailable("信号中断")
	}
	if v.Player == nil || v.PlayerValidUntilMs > 0 && now >= v.PlayerValidUntilMs {
		return unavailable("定位中")
	}
	if v.Source == "packet" && now >= v.GuidanceValidUntilMs {
		return unavailable("等待更新")
	}
	if v.Guidance == "pace" {
		if v.PaceUntilMs > 0 && now >= v.PaceUntilMs {
			return unavailable("等待更新")
		}
		cue := wait("等脚下出线")
		if v.PaceUntilMs > now {
			cue.Detail, cue.Seconds = "起步准备", fmt.Sprintf("%.1f", math.Ceil(float64(v.PaceUntilMs-now)/100)/10)
		}
		return cue
	}
	if v.Guidance == "hold" {
		return wait("等脚下出线")
	}
	if v.Guidance == "move" || v.Source != "packet" {
		if target, _ := brontasDisplayedStep(v); target != nil {
			dx, dy := target.X-v.Player.X, target.Y-v.Player.Y
			if math.Hypot(dx, dy) < .06 {
				return wait("等下一步")
			}
			cue := brontasActionCue{Mode: "go", Title: "移动", Detail: "向", Angle: math.Atan2(-dy, dx)}
			if v.GuidanceKind == "escape" {
				cue.Mode, cue.Title = "urgent", "快躲"
			}
			if math.Abs(dx) > .04 {
				if dx > 0 {
					cue.Detail += "右"
				} else {
					cue.Detail += "左"
				}
			}
			if math.Abs(dy) > .04 {
				if dy > 0 {
					cue.Detail += "上"
				} else {
					cue.Detail += "下"
				}
			}
			return cue
		}
	}
	if v.Guidance == "blocked" {
		return unavailable("暂无可用路线")
	}
	if v.Player.Speed <= 0 {
		return unavailable("等待测速")
	}
	if v.Guidance == "move" {
		return unavailable("位置变化")
	}
	return unavailable("等待更新")
}
