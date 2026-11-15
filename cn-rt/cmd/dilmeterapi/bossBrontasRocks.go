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

// HP reaches the 50% gate before the server sends the actual rock positions.
// Keep this phase after removal/timeout so HP refreshes cannot reopen the map.
type nativeBrontasRockPhase struct {
	AtMs      int64
	Confirmed bool
}

func (r *nativeReminderRuntime) clearBrontasBoss(id string) {
	delete(r.brontasRockPhases, id)
	for propID, rock := range r.brontasRocks {
		if rock.BossID == id {
			delete(r.brontasRocks, propID)
		}
	}
}

func (r *nativeReminderRuntime) observeBrontasHealth(entity *nativeReminderEntity, previous, previousMaximum float64, previouslyKnown bool, atMs int64) {
	if !entity.Known || !entity.Active || entity.Defeated || entity.RaceID != 7602 || !entity.HealthKnown || entity.MaximumHealth <= 0 {
		return
	}
	if _, exists := r.brontasRockPhases[entity.ID]; exists {
		return
	}
	ratio := entity.CurrentHealth / entity.MaximumHealth
	// Float32 HP can put the gate just above 0.5 (observed 0.5000000437).
	const epsilon = 0.000001
	if math.IsNaN(ratio) || ratio < .45 || ratio > .5+epsilon {
		return
	}
	if previouslyKnown && previousMaximum > 0 {
		if previous/previousMaximum <= .5+epsilon {
			return
		}
	} else if math.Abs(ratio-.5) > epsilon {
		return // Joining after the gate does not predict an already finished phase.
	}
	if r.brontasRockPhases == nil {
		r.brontasRockPhases = make(map[string]nativeBrontasRockPhase)
	}
	r.brontasRockPhases[entity.ID] = nativeBrontasRockPhase{AtMs: atMs}
}

type nativeBrontasRockSettings struct {
	Enabled         bool   `json:"enabled"`
	GuidanceEnabled bool   `json:"guidanceEnabled"`
	X               int    `json:"x"`
	Y               int    `json:"y"`
	ScalePercent    int    `json:"scalePercent"`
	Rotation        int    `json:"rotation"` // legacy saved field
	Facing          string `json:"facing"`
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
	defer func() {
		if len(r.brontasRocks) == 0 {
			r.clearBrontasLiveRays()
		}
	}()
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
	if r.brontasRockPhases == nil {
		r.brontasRockPhases = make(map[string]nativeBrontasRockPhase)
	}
	phase, exists := r.brontasRockPhases[boss.ID]
	if !exists {
		phase.AtMs = nativeEventAtMs(e.At, e.AtMs)
	}
	phase.Confirmed = true
	r.brontasRockPhases[boss.ID] = phase
}

func (r *nativeReminderRuntime) brontasRockOverlay(now time.Time) *nativeBossMechanicOverlayItem {
	nowMs := now.UnixMilli()
	markers := []nativeBrontasRockMarker{}
	start, end := int64(0), int64(0)
	seen := map[int]bool{}
	for id, phase := range r.brontasRockPhases {
		boss := r.entities[id]
		if boss == nil || !boss.Active || boss.Defeated {
			r.clearBrontasBoss(id)
			continue
		}
		// Pending directions are deliberately empty and never flash. Stop a
		// missed/cancelled phase after 30s without guessing any positions.
		if !phase.Confirmed && nowMs >= phase.AtMs && nowMs < phase.AtMs+30_000 {
			if start == 0 || phase.AtMs < start {
				start = phase.AtMs
			}
			end = max(end, phase.AtMs+30_000)
		}
	}
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
		clock := rock.Clock
		if !seen[clock] {
			markers = append(markers, nativeBrontasRockMarker{Clock: clock})
			seen[clock] = true
		}
		if start == 0 || rock.AtMs < start {
			start = rock.AtMs
		}
		if phase, ok := r.brontasRockPhases[rock.BossID]; ok && phase.AtMs < start {
			start = phase.AtMs // Keep the same card when the first location arrives.
		}
		end = max(end, rock.AtMs+45_000)
	}
	s := r.settings.BossMechanics.BrontasRocks
	if !s.Enabled || end == 0 {
		return nil
	}
	sort.Slice(markers, func(i, j int) bool { return markers[i].Clock < markers[j].Clock })
	item := &nativeBossMechanicOverlayItem{Key: brontasRockKey, Name: "二王 50% 落石", Rocks: markers, HideCountdown: true, StartedAtMs: start, EndsAtMs: end, Generation: uint64(start), X: s.X, Y: s.Y, ScalePercent: s.ScalePercent}
	if s.GuidanceEnabled {
		item.BrontasGuide = r.brontasGuideView(nowMs)
	}
	return item
}
