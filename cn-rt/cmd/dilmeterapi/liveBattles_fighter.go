package main

import (
	"math"
	"strings"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

// Retain a bounded gauge state, not an ever-growing list of previous fights'
// deltas. Its timestamp is the last observation, so natural growth continues
// through the next window without inventing an observation at the first hit.
type liveFighterEnergy struct {
	atMs                 int64
	value, rate, maximum float64
	upper                float64
	inferred             bool
}

func (g *liveFighterEnergy) baseline(id string) *event.EventArcanaSignal {
	signal := &event.EventArcanaSignal{
		EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: g.atMs / 1000, Id: id},
		Signal:    "fighter-energy-baseline", AtMs: g.atMs, Kind: 3,
		Value: float32(g.value), Rate: float32(g.rate), Maximum: float32(g.maximum),
	}
	if g.inferred {
		signal.Signal, signal.UpperValue = "fighter-energy-bounds", float32(g.upper)
	}
	return signal
}

func (s *liveBattleIndex) observeFighterEnergy(v *event.EventArcanaSignal) {
	if !strings.HasPrefix(v.Signal, "fighter-energy-") && !strings.HasPrefix(v.Signal, "fighter-spend-") {
		return
	}
	a := s.actor(v.Id)
	switch v.Signal {
	case "fighter-energy-reset":
		a.fighterEnergy, a.fighterSpend = nil, nil
	case "fighter-energy-baseline":
		a.fighterEnergy = nil
		if v.AtMs > 0 && v.AtMs/1000 == v.At && finiteFighterValue(v.Value) && finiteFighterValue(v.Rate) && finiteFighterValue(v.Maximum) && v.Value >= 0 && v.Maximum >= 100 && v.Value <= v.Maximum && v.Rate >= 0 {
			a.fighterEnergy = &liveFighterEnergy{atMs: v.AtMs, value: float64(v.Value), rate: float64(v.Rate), maximum: float64(v.Maximum), upper: float64(v.Value)}
		}
	case "fighter-energy-delta":
		g := a.fighterEnergy
		if g == nil {
			// User-confirmed CN parameters allow bounds when capture starts
			// after initialization; never publish them as an observed baseline.
			g = &liveFighterEnergy{atMs: v.AtMs, rate: 3, maximum: 400, upper: 400, inferred: true}
			a.fighterEnergy = g
		}
		if !finiteFighterValue(v.Value) || v.AtMs < g.atMs || v.AtMs/1000 != v.At {
			a.fighterEnergy = nil
			return
		}
		growth, delta := float64(v.AtMs-g.atMs)/1000*g.rate, float64(v.Value)
		if g.inferred {
			lower := math.Max(math.Min(g.maximum, g.value+growth), -delta)
			upper := math.Min(math.Min(g.maximum, g.upper+growth), g.maximum-delta)
			if lower > upper+0.1 {
				a.fighterEnergy = nil
				return
			}
			g.value, g.upper, g.atMs = math.Max(0, math.Min(lower, upper)+delta), math.Min(g.maximum, upper+delta), v.AtMs
			return
		}
		next := math.Min(g.maximum, g.value+growth) + delta
		// Match the KPI's missing-history check; an unexplained overdraft
		// must never become an apparently complete zero-valued baseline.
		if next < -0.1 {
			a.fighterEnergy = nil
			return
		}
		g.value, g.atMs = math.Max(0, math.Min(g.maximum, next)), v.AtMs
		g.upper = g.value
	case "fighter-spend-start":
		if v.AtMs > 0 && v.AtMs == v.CastAtMs && (v.SkillId == 59185 || v.SkillId == 59186 || v.SkillId == 59187) {
			copy := *v
			copy.Sequence = 0
			a.fighterSpend = &copy
		}
	case "fighter-spend-end":
		if start := a.fighterSpend; start != nil && v.SkillId == start.SkillId && v.CastAtMs == start.CastAtMs && v.AtMs >= start.AtMs {
			a.fighterSpend = nil
		}
	}
}
