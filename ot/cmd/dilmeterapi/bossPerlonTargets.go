package main

import (
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

type nativePerlonTornadoMonitoring struct {
	Self      bool `json:"self"`
	Teammates bool `json:"teammates"`
}

// The deployment contains an effect-instance ID, not a player ID. Snapshot
// Perlon's explicit combat selection when the deployment arrives instead.
type perlonTornadoTarget struct {
	BossID, TargetID uint64
	AtMs             int64
}

func (t *eventPublisher) observePerlonTarget(p *packet.GamePacket) {
	if p.Op == packet.OpcodeEntityDisappear {
		if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
			return
		}
		id := p.Msg[0].Data().(uint64)
		if id == t.perlonTarget.BossID || id == t.perlonTarget.TargetID {
			t.perlonTarget = perlonTornadoTarget{}
		}
		return
	}
	if p.Op != packet.OpcodeSetCombatTarget {
		return
	}
	boss := t.entityCache[p.Id]
	if boss == nil || boss.EntityInfo == nil || boss.RaceId != perlonRaceID || boss.disappearAt != 0 {
		return
	}
	if p.At.UnixMilli() < t.perlonTarget.AtMs {
		return
	}
	target, err := packet.ParseSetCombatTargetPacket(p.Msg)
	if err != nil {
		target = 0
	}
	t.perlonTarget = perlonTornadoTarget{BossID: p.Id, TargetID: target, AtMs: p.At.UnixMilli()}
}

func (t *eventPublisher) perlonDeploymentTarget(p *packet.GamePacket) string {
	selected := t.perlonTarget
	if selected.BossID != p.Id || selected.TargetID == 0 || selected.AtMs > p.At.UnixMilli() {
		return ""
	}
	target := t.entityCache[selected.TargetID]
	if target == nil || target.EntityInfo == nil || target.disappearAt != 0 || target.OwnerId != 0 || !battleRecordPCRace(target.RaceId) {
		return "" // Unknown entities, pets and monsters are not teammates.
	}
	return strconv.FormatUint(selected.TargetID, 10)
}

func (r *nativeReminderRuntime) perlonTornadoEnabled() bool {
	rule := r.settings.BossMechanics.Rules[perlonTornadoKey]
	if !rule.Enabled {
		return false
	}
	if rule.TornadoMonitoring == nil {
		return true // Legacy settings monitored every deployment.
	}
	selection := rule.TornadoMonitoring
	switch r.perlonTornadoAudience() {
	case "self":
		return selection.Self
	case "teammate":
		return selection.Teammates
	default:
		return selection.Self && selection.Teammates
	}
}

func (r *nativeReminderRuntime) perlonTornadoAudience() string {
	if r.localID == "" || r.localID == "0" || r.perlon.TornadoTargetID == "" {
		return "unknown"
	}
	if r.perlon.TornadoTargetID == r.localID {
		return "self"
	}
	return "teammate"
}
