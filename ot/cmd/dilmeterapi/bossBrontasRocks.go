package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"math"
	"sort"
	"strconv"
	"time"
)

const brontasRockKey = "brontas-rocks"

type nativeBrontasRockSettings struct {
	Enabled         bool `json:"enabled"`
	GuidanceEnabled bool `json:"guidanceEnabled"`
	X               int  `json:"x"`
	Y               int  `json:"y"`
	ScalePercent    int  `json:"scalePercent"`
	Rotation        int  `json:"rotation"`
}
type nativeBrontasRock struct {
	BossID   string
	AtMs     int64
	Clock    int
	RegionID uint32
}
type nativeBrontasRockMarker struct {
	Clock int `json:"clock"`
}

func (t *eventPublisher) publishWorldPropPacket(p *packet.GamePacket) {
	// This stream intentionally includes only the relevant mineral prop. It
	// records real creation/removal, not inferred player counts or particles.
	switch p.Op {
	case 21200:
		prop, ok := packet.ParseWorldPropPacket(p.Msg)
		if !ok {
			return
		}
		if prop.PropID != 49205 {
			return
		}
		if t.worldProps == nil {
			t.worldProps = make(map[uint64]bool)
		}
		if len(t.worldProps) > 4096 {
			clear(t.worldProps)
		}
		t.worldProps[prop.ID] = true
		t.publish(&event.EventWorldProp{EventBase: event.EventBase{EventId: event.EventIdWorldProp, At: p.At.Unix(), Id: strconv.FormatUint(prop.ID, 10)}, AtMs: p.At.UnixMilli(), PropID: prop.PropID, RegionID: prop.RegionID, X: prop.X, Y: prop.Y, State: prop.State, Active: true})
	case 21201:
		if len(p.Msg) != 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
			return
		}
		id := p.Msg[0].Data().(uint64)
		if !t.worldProps[id] {
			return
		}
		delete(t.worldProps, id)
		t.publish(&event.EventWorldProp{EventBase: event.EventBase{EventId: event.EventIdWorldProp, At: p.At.Unix(), Id: strconv.FormatUint(id, 10)}, AtMs: p.At.UnixMilli(), PropID: 49205})
	}
}

// Four 50% mineral sockets seen in every captured arena instance. Random
// minerals reuse the same prop ID, so checking the exact sockets is essential.
func brontasRockClock(x, y float64) int {
	for _, s := range []struct {
		x, y  float64
		clock int
	}{{25156, 41145, 12}, {29556, 36745, 3}, {25156, 32345, 6}, {20756, 36745, 9}} {
		if math.Abs(x-s.x) <= 2 && math.Abs(y-s.y) <= 2 {
			return s.clock
		}
	}
	return 0
}

func (r *nativeReminderRuntime) observeBrontasProp(e *event.EventWorldProp) {
	if !e.Active {
		delete(r.brontasRocks, e.Id)
		return
	}
	// A reused prop ID at a random position must also clear the old marker.
	delete(r.brontasRocks, e.Id)
	clock := brontasRockClock(e.X, e.Y)
	if e.PropID != 49205 || e.State != "landing" || clock == 0 {
		return
	}
	var boss *nativeReminderEntity
	for _, entity := range r.entities {
		if entity.Known && entity.Active && !entity.Defeated && entity.RaceID == 7602 && entity.HealthKnown && entity.MaximumHealth > 0 && entity.CurrentHealth/entity.MaximumHealth >= 0.45 && entity.CurrentHealth/entity.MaximumHealth <= 0.505 && (boss == nil || entity.AppearedAt > boss.AppearedAt) {
			boss = entity
		}
	}
	if boss == nil {
		return
	}
	if r.brontasRocks == nil {
		r.brontasRocks = make(map[string]nativeBrontasRock)
	}
	r.brontasRocks[e.Id] = nativeBrontasRock{BossID: boss.ID, AtMs: nativeEventAtMs(e.At, e.AtMs), Clock: clock, RegionID: e.RegionID}
}

func (r *nativeReminderRuntime) brontasRockOverlay(now time.Time) *nativeBossMechanicOverlayItem {
	nowMs := now.UnixMilli()
	markers := []nativeBrontasRockMarker{}
	start, end := int64(0), int64(0)
	seen := map[int]bool{}
	for id, rock := range r.brontasRocks {
		boss := r.entities[rock.BossID]
		// Removal is normally received after about 29s. Bound stale state if a
		// removal is lost; never leave a direction on screen across encounters.
		if boss == nil || !boss.Active || boss.Defeated || nowMs >= rock.AtMs+45_000 {
			delete(r.brontasRocks, id)
			continue
		}
		if nowMs < rock.AtMs {
			continue
		}
		clock := (rock.Clock + r.settings.BossMechanics.BrontasRocks.Rotation/30) % 12
		if clock == 0 {
			clock = 12
		}
		if !seen[clock] {
			markers = append(markers, nativeBrontasRockMarker{Clock: clock})
			seen[clock] = true
		}
		if start == 0 || rock.AtMs < start {
			start = rock.AtMs
		}
		end = max(end, rock.AtMs+45_000)
	}
	s := r.settings.BossMechanics.BrontasRocks
	if !s.Enabled || len(markers) == 0 {
		return nil
	}
	sort.Slice(markers, func(i, j int) bool { return markers[i].Clock < markers[j].Clock })
	item := &nativeBossMechanicOverlayItem{Key: brontasRockKey, Name: "二王 50% 落石", Rocks: markers, HideCountdown: true, StartedAtMs: start, EndsAtMs: end, Generation: uint64(start), X: s.X, Y: s.Y, ScalePercent: s.ScalePercent}
	if s.GuidanceEnabled {
		item.BrontasGuide = r.brontasGuideView(nowMs)
	}
	return item
}
