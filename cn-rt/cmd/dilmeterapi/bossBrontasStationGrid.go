package main

import "math"

type nativeBrontasPosition struct {
	DestX, DestY float64
	Mode         uint64
	AtMs         int64
	RegionID     uint32
	X, Y         float64
	Moving       bool
	Speed        float64
	SpeedSamples int
	SpeedAtMs    int64
	RunSpeed     float64
	RunSpeedAtMs int64
}

// Both renderers and the planner use this same bounded projection. The packet
// still supplies the confirmed start; destination alone never places a player.
type brontasGuideMotion struct {
	From    brontasPoint `json:"from"`
	To      brontasPoint `json:"to"`
	AtMs    int64        `json:"atMs"`
	UntilMs int64        `json:"untilMs"`
	Speed   float64      `json:"speed"`
}

func brontasPositionAt(p nativeBrontasPosition, nowMs int64) nativeBrontasPosition {
	if !p.Moving || p.Mode == 0 || p.SpeedSamples < 2 || nowMs < p.AtMs || nowMs-p.SpeedAtMs >= 3000 {
		return p
	}
	dx, dy := p.DestX-p.X, p.DestY-p.Y
	d := math.Hypot(dx, dy)
	if d <= 0 {
		return p
	}
	travel := math.Min(d, p.Speed*float64(min(nowMs-p.AtMs, brontasMovingPositionFreshMs))/1000)
	p.X += dx / d * travel
	p.Y += dy / d * travel
	return p
}

func brontasStationMotion(p nativeBrontasPosition, clock int, nowMs int64) *brontasGuideMotion {
	if !p.Moving || p.Mode == 0 || p.SpeedSamples < 2 || nowMs-p.SpeedAtMs >= 3000 {
		return nil
	}
	cx, cy, ok := brontasStationGrid(clock)
	if !ok {
		return nil
	}
	point := func(x, y float64) brontasPoint {
		return brontasPoint{X: (x - float64(19661+cx*846+423)) / 1269, Y: (y - float64(31238+cy*846+423)) / 1269}
	}
	return &brontasGuideMotion{From: point(p.X, p.Y), To: point(p.DestX, p.DestY), AtMs: p.AtMs, UntilMs: p.AtMs + brontasMovingPositionFreshMs, Speed: p.Speed / 1269}
}

// The three adjacent wire channels surrounding each mineral's center channel.
// This is a display crop, not a claimed mineral protection/collision radius.
func brontasStationGrid(clock int) (x, y int, ok bool) {
	switch clock {
	case 12:
		return 6, 11, true
	case 3:
		return 11, 6, true
	case 6:
		return 6, 1, true
	case 9:
		return 1, 6, true
	default:
		return 0, 0, false
	}
}

func brontasStationContains(clock int, x, y float64) bool {
	cx, cy, ok := brontasStationGrid(clock)
	if !ok {
		return false
	}
	left, bottom := float64(19661+(cx-1)*846), float64(31238+(cy-1)*846)
	return x >= left && x < left+2538 && y >= bottom && y < bottom+2538
}

func brontasStationRay(world nativeBrontasPacketRay, clock int) (brontasGuideRay, bool) {
	x, y, ok := brontasStationGrid(clock)
	if !ok {
		return brontasGuideRay{}, false
	}
	center := x
	if world.Axis == "horizontal" {
		center = y
	}
	if int(world.Lane) < center-1 || int(world.Lane) > center+1 {
		return brontasGuideRay{}, false
	}
	ray := world.brontasGuideRay
	offset, halfWidth := float64(int(world.Lane)-center)*2/3, 1.0/3
	ray.Offset, ray.HalfWidth = &offset, &halfWidth
	return ray, true
}

// A new movement sample invalidates the old lane immediately. Moving samples
// expire after 700ms. Projection requires measured speed and stops at destination.
const brontasMovingPositionFreshMs int64 = 700

func (r *nativeReminderRuntime) brontasStationClock(bossID string, phase, nowMs int64) int {
	p := brontasPositionAt(r.brontasPosition, nowMs)
	if p.AtMs < phase || p.AtMs > nowMs {
		return 0
	}
	for _, rock := range r.brontasRocks {
		if rock.BossID == bossID && rock.AtMs <= nowMs && rock.AtMs+45000 > nowMs && p.RegionID == rock.RegionID && brontasStationContains(rock.Clock, p.X, p.Y) {
			return rock.Clock
		}
	}
	return 0
}

func (r *nativeReminderRuntime) brontasStationPlayer(clock int, nowMs int64) (*brontasGuidePlayer, int64) {
	p := brontasPositionAt(r.brontasPosition, nowMs)
	until := nowMs + brontasRayStreamTimeoutMs
	if p.Moving {
		until = p.AtMs + brontasMovingPositionFreshMs
	}
	if nowMs >= until {
		return nil, until
	}
	cx, cy, ok := brontasStationGrid(clock)
	if !ok || !brontasStationContains(clock, p.X, p.Y) {
		return nil, 0
	}
	return &brontasGuidePlayer{brontasPoint: brontasPoint{
		X: (p.X - float64(19661+cx*846+423)) / 1269,
		Y: (p.Y - float64(31238+cy*846+423)) / 1269,
	}, Radius: .055, Speed: func() float64 {
		if p.SpeedSamples >= 1 && nowMs-p.SpeedAtMs < 3000 {
			// The first measured interval can already guide a conservative step.
			// Requiring a second interval postponed initial advice by another packet.
			factor := .75
			if p.SpeedSamples == 1 {
				factor = .65
			}
			return p.Speed * factor / 1269
		}
		if brontasRecentOwnRun(p, nowMs) {
			// Lower, guidance-only estimate after a short ride/stop. This never
			// feeds position projection or uses the mount's velocity.
			return p.RunSpeed * .60 / 1269
		}
		return 0
	}()}, until
}

// One pending deadline per axis under the player's feet. Half-open channels
// make a boundary belong to exactly one lane, independent of display rotation.
func brontasFootRayDeadlines(v *nativeBrontasGuideView, nowMs int64) (horizontal, vertical int64) {
	if v.Player == nil || nowMs >= v.ValidUntilMs || (v.PlayerValidUntilMs > 0 && nowMs >= v.PlayerValidUntilMs) {
		return
	}
	for _, ray := range v.Rays {
		if ray.Offset == nil || ray.HalfWidth == nil || ray.LethalAtMs <= nowMs || (ray.EndsAtMs > 0 && ray.EndsAtMs <= nowMs) {
			continue
		}
		coordinate, deadline := v.Player.X, &vertical
		if ray.Axis == "horizontal" {
			coordinate, deadline = v.Player.Y, &horizontal
		}
		// Tiny tolerance only for equivalent normalized boundary representations.
		if coordinate >= *ray.Offset-*ray.HalfWidth-1e-9 && coordinate < *ray.Offset+*ray.HalfWidth-1e-9 && (*deadline == 0 || ray.LethalAtMs < *deadline) {
			*deadline = ray.LethalAtMs
		}
	}
	return
}
