package main

import (
	"fmt"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// Packets describe persistent scheduled effects, not fresh player positions.
// Allow normal ~600ms wave gaps; a stalled stream stops live guidance after
// 2.5s. Individual endings below use an explicitly named experimental model.
const brontasRayStreamTimeoutMs int64 = 2500

// October 29 video review supports finite visibility, not an exact collision
// duration. 600ms is an experimental tail allowance, never a decoded end event.
const brontasRayExperimentalTailMs int64 = 600
const brontasRayLifetimeModel = "video-tail-600-v1"

type nativeBrontasLiveRays struct {
	BossID              string
	PhaseAtMs, LastAtMs int64
	Ended               bool
	Rays                []nativeBrontasPacketRay
	Seen                map[string]bool
	Target              *brontasPoint
	NextTarget          *brontasPoint
	Rhythm              *brontasRhythmPlan
}

type nativeBrontasPacketRay struct {
	brontasGuideRay
	Lane uint32
}

func (t *eventPublisher) publishBrontasRayPacket(p *packet.GamePacket) {
	if p == nil {
		return
	}
	ray, start := packet.ParseBrontasRayPacket(p)
	position, positioned := packet.ParseBrontasPositionPacket(p)
	end := packet.IsBrontasRayEndPacket(p)
	death := p.Op == 21500 && len(p.Msg) == 0
	if !start && !end && !death && !positioned {
		return
	}
	t.Lock()
	entity := t.entityCache[p.Id]
	boss := entity != nil && entity.EntityInfo != nil && entity.RaceId == 7602 && entity.disappearAt == 0
	localDeath := death && p.Id != 0 && p.Id == t.localEntityId
	localPosition := positioned && p.Id != 0 && p.Id == t.localEntityId
	t.Unlock()
	if (!boss && !localDeath && !localPosition) || (positioned && !localPosition) {
		return
	}
	kind := "ray"
	if localPosition {
		kind = "position"
	}
	if end {
		kind = "end"
	}
	if death {
		kind = "death"
	}
	t.publish(&event.EventBrontasRay{
		EventBase: event.EventBase{EventId: event.EventIdBrontasRay, Id: strconv.FormatUint(p.Id, 10), At: p.At.Unix()},
		AtMs:      p.At.UnixMilli(), Kind: kind, Direction: ray.Direction, Lane: ray.Lane, DelayMs: ray.DelayMs, LeadMs: ray.LeadMs,
		Live:     p.LiveCapture,
		RegionID: position.RegionID, X: position.X, Y: position.Y, Moving: position.Moving,
		DestX: position.DestX, DestY: position.DestY, MoveMode: position.Mode,
	})
	// Deliver this deadline and any preceding mineral event in order, without
	// waiting for the ordinary 100ms statistics batch.
	t.publishNow()
}

func (r *nativeReminderRuntime) brontasLivePhase(bossID string, atMs int64) (int64, bool) {
	boss := r.entities[bossID]
	if boss == nil || !boss.Known || !boss.Active || boss.Defeated || boss.RaceID != 7602 {
		return 0, false
	}
	first := int64(0)
	for _, rock := range r.brontasRocks {
		if rock.BossID == bossID && atMs >= rock.AtMs && atMs < rock.AtMs+45000 && (first == 0 || rock.AtMs < first) {
			first = rock.AtMs
		}
	}
	if first != 0 && r.brontasLiveRays.BossID == bossID && r.brontasLiveRays.PhaseAtMs > 0 && first >= r.brontasLiveRays.PhaseAtMs && first < r.brontasLiveRays.PhaseAtMs+45000 {
		return r.brontasLiveRays.PhaseAtMs, true
	}
	return first, first != 0
}

func (r *nativeReminderRuntime) observeBrontasRay(e *event.EventBrontasRay) {
	// Offline files can still log decoded events, but never animate a current
	// game overlay. Replay tests opt in on a separate, private runtime.
	if !e.Live || e.AtMs <= 0 {
		return
	}
	if e.Kind == "position" {
		if e.Id == r.localID && e.AtMs >= r.brontasPosition.AtMs && e.X <= 1000000 && e.Y <= 1000000 {
			region := e.RegionID
			if region == 0 {
				region = r.brontasPosition.RegionID
			}
			next := nativeBrontasPosition{AtMs: e.AtMs, RegionID: region, X: float64(e.X), Y: float64(e.Y), Moving: e.Moving, DestX: float64(e.DestX), DestY: float64(e.DestY), Mode: e.MoveMode}
			if next.Mode == 0 {
				next.Mode = r.brontasPosition.Mode
			}
			next.Speed, next.SpeedSamples, next.SpeedAtMs = brontasPositionSpeed(r.brontasPosition, next)
			brontasRememberOwnRun(r.brontasPosition, &next)
			r.brontasPosition = next
		}
		return
	}
	s := &r.brontasLiveRays
	if e.Kind == "death" && e.Id == r.localID {
		if s.BossID == "" {
			for _, rock := range r.brontasRocks {
				if phase, ok := r.brontasLivePhase(rock.BossID, e.AtMs); ok {
					*s = nativeBrontasLiveRays{BossID: rock.BossID, PhaseAtMs: phase}
					break
				}
			}
		}
		if e.AtMs >= s.PhaseAtMs {
			s.Ended = true
			s.Rays = nil
			r.brontasGuide = brontasGuideFrame{}
		}
		return
	}
	phase, ok := r.brontasLivePhase(e.Id, e.AtMs)
	if !ok {
		return
	}
	if s.BossID != e.Id || s.PhaseAtMs != phase {
		*s = nativeBrontasLiveRays{BossID: e.Id, PhaseAtMs: phase, Seen: make(map[string]bool)}
	}
	if e.Kind == "end" || e.Kind == "death" {
		s.Ended = true
		s.Rays = nil
		r.brontasGuide = brontasGuideFrame{}
		return
	}
	if e.Kind != "ray" || s.Ended || e.Direction > 3 || e.Lane > 12 || e.LeadMs != 1600 || e.DelayMs > 1600 {
		return
	}
	id := fmt.Sprintf("%d:%d:%d:%d", e.AtMs, e.Direction, e.Lane, e.DelayMs)
	if s.Seen[id] || len(s.Seen) >= 512 {
		return
	}
	s.Seen[id] = true
	kept := s.Rays[:0]
	for _, old := range s.Rays {
		if old.EndsAtMs == 0 || old.EndsAtMs+brontasGuideGuardMs > e.AtMs {
			kept = append(kept, old)
		}
	}
	s.Rays = kept
	axis := "vertical"
	if e.Direction%2 == 1 {
		axis = "horizontal"
	}
	// Full 11000-unit arena frame. This is deliberately NOT the PR's local
	// radius=1 mineral frame, whose actual radius has not been calibrated.
	offset, width := (float64(e.Lane)*846+423-5500)/5500, 423.0/5500
	warning := e.AtMs + int64(e.DelayMs)
	s.Rays = append(s.Rays, nativeBrontasPacketRay{Lane: e.Lane, brontasGuideRay: brontasGuideRay{ID: id, Axis: axis, Offset: &offset, HalfWidth: &width, WarningAtMs: warning, LethalAtMs: warning + int64(e.LeadMs), EndsAtMs: warning + int64(e.LeadMs) + brontasRayExperimentalTailMs}})
	s.LastAtMs = max(s.LastAtMs, e.AtMs)
}

func (r *nativeReminderRuntime) brontasLiveRayView(nowMs int64) *nativeBrontasGuideView {
	s := &r.brontasLiveRays
	bossID, phase := s.BossID, s.PhaseAtMs
	if start, ok := r.brontasLivePhase(bossID, nowMs); !ok || start != phase {
		bossID, phase = "", 0
		for _, rock := range r.brontasRocks {
			if start, ok := r.brontasLivePhase(rock.BossID, nowMs); ok && start > phase {
				bossID, phase = rock.BossID, start
			}
		}
	}
	if bossID == "" {
		return nil
	}
	v := &nativeBrontasGuideView{Source: "packet", RayLifetimeModel: brontasRayLifetimeModel, Stage: "map", Status: "watch", ValidUntilMs: nowMs + brontasRayStreamTimeoutMs}
	if s.BossID == bossID && s.PhaseAtMs == phase && s.Ended {
		return v
	}
	v.Clock = r.brontasStationClock(bossID, phase, nowMs)
	if v.Clock == 0 {
		return v
	}
	v.Stage = "station"
	v.Player, v.PlayerValidUntilMs = r.brontasStationPlayer(v.Clock, nowMs)
	v.Motion = brontasStationMotion(r.brontasPosition, v.Clock, nowMs)
	if s.BossID == bossID && s.PhaseAtMs == phase && len(s.Rays) > 0 {
		v.ValidUntilMs = s.LastAtMs + brontasRayStreamTimeoutMs
		if nowMs >= v.ValidUntilMs {
			v.Status = "waiting"
			v.Player = nil
			orientBrontasStation(v, r.settings.BossMechanics.BrontasRocks.Facing)
			return v
		}
		for _, worldRay := range s.Rays {
			if worldRay.EndsAtMs > 0 && nowMs >= worldRay.EndsAtMs+brontasGuideGuardMs {
				continue
			}
			ray, covered := brontasStationRay(worldRay, v.Clock)
			if !covered {
				continue
			}
			v.ReceivedCount++
			v.Rays = append(v.Rays, ray)
			if ray.LethalAtMs > nowMs {
				v.PendingCount++
			}
		}
	}
	v.GuidanceValidUntilMs = min(v.ValidUntilMs, r.brontasPosition.AtMs+brontasGuidePositionFreshMs)
	if !r.brontasPosition.Moving {
		// A confirmed stop remains usable during a deliberate bait pause. Moving
		// samples still expire after 700 ms; do not extend their projection.
		v.GuidanceValidUntilMs = min(v.ValidUntilMs, r.brontasPosition.AtMs+3000)
	}
	v.SourceAtMs = r.brontasPosition.AtMs
	v.ComputedAtMs = nowMs
	v.Target = s.Target
	v.NextTarget = s.NextTarget
	v.rhythm = s.Rhythm
	planBrontasStationGuide(v, nowMs)
	s.Rhythm = v.rhythm
	s.Target = v.Target
	s.NextTarget = v.NextTarget
	orientBrontasStation(v, r.settings.BossMechanics.BrontasRocks.Facing)
	h, vertical := brontasFootRayDeadlines(v, nowMs)
	v.LethalAtMs = h
	if vertical > 0 && (h == 0 || vertical < h) {
		v.LethalAtMs = vertical
	}
	if v.LethalAtMs > 0 {
		v.Status = "axis"
	}
	return v
}

func (r *nativeReminderRuntime) clearBrontasLiveRays() {
	r.brontasLiveRays = nativeBrontasLiveRays{}
	r.brontasGuide = brontasGuideFrame{}
}
