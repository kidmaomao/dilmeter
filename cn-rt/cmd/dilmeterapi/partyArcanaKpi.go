package main

import (
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// Public skill lifecycles are keyed by their actual caster. Local ACK-based
// parsers remain authoritative for self; a damage phase is never a new cast.
type partyArcanaState struct {
	sniper         *gunnerSniperCast
	heavy          *event.EventArcanaSignal
	chemical       *chemicalCarnivalCast
	interlude      *puppeteerInterludeCast
	combo          *fighterComboCast
	kick           *partyFighterKick
	uppercutAction uint32
	spend          *fighterSpendCast
	lastRelease    map[uint16]int64
}

type partyFighterKick struct {
	combo      *fighterComboCast
	launchAtMs int64
}

func (t *eventPublisher) partyArcanaActor(id uint64) *partyArcanaState {
	if t.localEntityId == 0 || id == t.localEntityId || !t.partySignalActor(id) {
		return nil
	}
	if t.partyArcana == nil {
		t.partyArcana = map[uint64]*partyArcanaState{}
	}
	if t.partyArcana[id] == nil {
		t.partyArcana[id] = &partyArcanaState{lastRelease: map[uint16]int64{}}
	}
	return t.partyArcana[id]
}

// Exact typed layouts, including finite geometry, prevent unrelated effects
// with a shared number or malformed records from becoming KPI evidence.
func arcanaShape(msg packet.Message, types ...packet.MessageElemType) bool {
	if len(msg) != len(types) {
		return false
	}
	for i, field := range msg {
		if field.Type() != types[i] {
			return false
		}
		if types[i] == packet.MessageElemTypeFloat && !finiteFighterValue(field.Data().(float32)) {
			return false
		}
	}
	return true
}

func (t *eventPublisher) partyArcanaSignal(p *packet.GamePacket, signal *event.EventArcanaSignal) {
	signal.EventBase = event.EventBase{EventId: event.EventIdArcanaSignal, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
	signal.AtMs = p.At.UnixMilli()
	t.publish(signal)
}

func (t *eventPublisher) partyArcanaRelease(p *packet.GamePacket, state *partyArcanaState, skill uint16) bool {
	at := p.At.UnixMilli()
	if previous, seen := state.lastRelease[skill]; seen && at <= previous {
		return false
	}
	state.lastRelease[skill] = at
	id := strconv.FormatUint(p.Id, 10)
	t.publish(&event.EventSkillAction{EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: p.At.Unix(), Id: id}, AtMs: at, SkillId: skill, SourceId: id})
	return true
}

func (t *eventPublisher) publishPartyArcanaPacket(p *packet.GamePacket) {
	if p == nil {
		return
	}
	s := t.partyArcanaActor(p.Id)
	if s == nil {
		return
	}
	m, at := p.Msg, p.At.UnixMilli()
	const (
		b = packet.MessageElemTypeByte
		h = packet.MessageElemTypeShort
		i = packet.MessageElemTypeInt
		l = packet.MessageElemTypeLong
		f = packet.MessageElemTypeFloat
	)
	if p.Op == 28006 && arcanaShape(m, b) && m[0].Data().(uint8) == 0 {
		s.sniper, s.heavy, s.chemical, s.interlude, s.combo = nil, nil, nil, nil, nil
		if s.spend != nil {
			t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "fighter-spend-end", SkillId: s.spend.skillID, CastAtMs: s.spend.atMs, Complete: true})
			s.spend = nil
		}
	}
	if p.Op == 37011 {
		// The observer receives a compact phase-7 aiming start; the caster's
		// broadcast can instead use the extended phase-4 layout. Both precede
		// the same phase-5 shot feedback. Do not confuse aiming phase 7 with
		// the terminal shot (which is field 2 of a phase-5 message).
		longStart := arcanaShape(m, i, b, l, f, f, i, i, i, b) && m[0].Data().(uint32) == 920 && m[1].Data().(uint8) == 4
		shortStart := arcanaShape(m, i, b, l, f, f) && m[0].Data().(uint32) == 920 && m[1].Data().(uint8) == 7
		if (longStart || shortStart) && m[2].Data().(uint64) != 0 {
			if t.partyArcanaRelease(p, s, 59123) {
				s.sniper = &gunnerSniperCast{atMs: at, targetID: m[2].Data().(uint64), executed: true}
				t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "sniper-counter", SkillId: 59123, CastAtMs: at, TargetId: strconv.FormatUint(s.sniper.targetID, 10), Phase: 2})
			}
		}
		if arcanaShape(m, i, b, i, l, f, f, f) && m[0].Data().(uint32) == 920 && m[1].Data().(uint8) == 5 {
			phase, cast := m[2].Data().(uint32), s.sniper
			if cast != nil && (phase == 6 || phase == 7) && at >= cast.atMs && m[3].Data().(uint64) == cast.targetID && cast.count < 100 {
				cast.count++
				// TCP retransmissions are removed by the packet reader. Distinct
				// shots can share a capture timestamp when a frame is coalesced.
				t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "sniper-counter", SkillId: 59123, CastAtMs: cast.atMs, TargetId: strconv.FormatUint(cast.targetID, 10), Count: cast.count, Phase: phase, Complete: phase == 7})
				if phase == 7 {
					s.sniper = nil
				}
			}
		}
		if isInterludePosition(m) {
			if s.interlude == nil || s.interlude.atMs != at {
				if t.partyArcanaRelease(p, s, 59165) {
					s.interlude = &puppeteerInterludeCast{atMs: at, actions: map[uint32]bool{}, targets: map[uint64]bool{}}
				}
			}
			if s.interlude != nil && s.interlude.atMs == at {
				s.interlude.positions++
			}
		}
		if arcanaShape(m, i, i, f, f, f, i) && m[0].Data().(uint32) == 816 && m[1].Data().(uint32) == 1 {
			t.partyArcanaRelease(p, s, 59045)
		}
		if arcanaShape(m, i, packet.MessageElemTypeString, b, l, h) && m[0].Data().(uint32) == 14 && m[1].Data().(string) == "thunder" && m[2].Data().(uint8) == 1 && m[3].Data().(uint64) != 0 && m[4].Data().(uint16) == 30102 {
			t.partyArcanaRelease(p, s, 30102)
		}
		if arcanaShape(m, i, b) && m[0].Data().(uint32) == 329 && s.combo != nil && at >= s.combo.atMs {
			switch m[1].Data().(uint8) {
			case 0:
				if s.combo.reverseAtMs == 0 {
					s.combo.reverseAtMs = at
				}
			case 1:
				if s.combo.reverseAtMs > 0 {
					s.combo.reverseEndAtMs = at
				}
			}
		}
		// Spend starts carry direction, not hit count. Resource values themselves
		// are only usable if the server also supplied this actor's gauge baseline.
		if arcanaShape(m, i, b, f, f) && m[1].Data().(uint8) == 2 {
			effect := m[0].Data().(uint32)
			if effect >= 950 && effect <= 952 {
				skill := uint16(59185 + effect - 950)
				if t.partyArcanaRelease(p, s, skill) {
					if s.spend != nil {
						t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "fighter-spend-end", SkillId: s.spend.skillID, CastAtMs: s.spend.atMs, Complete: true})
					}
					s.spend = &fighterSpendCast{atMs: at, skillID: skill}
					t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "fighter-spend-start", SkillId: skill, CastAtMs: at})
				}
			}
		}
	}
	if p.Op == packet.OpcodeEffectDelayed {
		if effect, ok := parseChemicalEffectCount(m); ok && t.partyArcanaRelease(p, s, 59144) {
			target, _ := strconv.ParseUint(effect.TargetId, 10, 64)
			s.chemical = &chemicalCarnivalCast{atMs: at, targetID: target, effectCount: effect.Count}
		}
		if ids, ok := parsePartyHeavyRelease(m, p.Id); ok && t.partyArcanaRelease(p, s, 59121) {
			s.heavy = &event.EventArcanaSignal{Signal: "domain-sample", SkillId: 59121, CastAtMs: at, Count: uint32(len(ids)), ObjectIds: ids, Complete: true}
		}
		if arcanaShape(m, i, i, i, l) && m[1].Data().(uint32) == 818 && m[2].Data().(uint32) == 2 && m[3].Data().(uint64) != 0 {
			t.partyArcanaRelease(p, s, 59040)
		}
		if s.combo != nil && isFighterKickReady(m) && at >= s.combo.atMs {
			s.combo.readyAtMs = at
		}
		if arcanaShape(m, i, i, b, l) && m[1].Data().(uint32) == 291 && m[2].Data().(uint8) == 0 && m[3].Data().(uint64) == p.Id {
			cast := s.combo
			s.combo = nil
			if cast != nil && cast.reverseAtMs >= cast.atMs && cast.reverseAtMs > 0 && cast.reverseEndAtMs >= cast.reverseAtMs && cast.readyAtMs >= cast.reverseEndAtMs && at >= cast.readyAtMs {
				s.kick = &partyFighterKick{combo: cast, launchAtMs: at}
			}
		}
	}
}

func parsePartyHeavyRelease(m packet.Message, caster uint64) ([]string, bool) {
	const (
		b = packet.MessageElemTypeByte
		i = packet.MessageElemTypeInt
		l = packet.MessageElemTypeLong
		f = packet.MessageElemTypeFloat
	)
	if len(m) < 4 || !arcanaShape(m[:4], i, i, b, i) || m[1].Data().(uint32) != 918 || m[2].Data().(uint8) != 2 {
		return nil, false
	}
	n := m[3].Data().(uint32)
	if n < 1 || n > 33 || len(m) != 4+int(n)*11 {
		return nil, false
	}
	ids := []string{}
	seen := map[uint64]bool{}
	bases := 0
	for offset := 4; offset < len(m); offset += 11 {
		g := m[offset : offset+11]
		if !arcanaShape(g, i, l, i, f, f, i, f, f, i, i, f) {
			return nil, false
		}
		kind, id := g[0].Data().(uint32), g[1].Data().(uint64)
		if id == 0 || seen[id] {
			return nil, false
		}
		seen[id] = true
		if kind == 0 && id == caster {
			bases++
		} else if kind == 1 && id != caster {
			ids = append(ids, strconv.FormatUint(id, 10))
		} else {
			return nil, false
		}
	}
	return ids, bases == 1
}

func (t *eventPublisher) observePartyArcanaCombat(p *packet.GamePacket, pack *packet.CombatActionPackPacket) {
	if p == nil || pack == nil || pack.CombatActionId == 0 {
		return
	}
	var attacker *packet.CombatActionPacket
	for _, a := range pack.SubPackets {
		if a != nil && a.Attacker != nil && a.Hit == nil {
			if attacker != nil {
				return
			}
			attacker = a
		}
	}
	if attacker == nil {
		return
	}
	s := t.partyArcanaActor(attacker.EntityId)
	if s == nil {
		return
	}
	// Combat packs use a transport ID: emitted signals must use the attacker.
	observation := *p
	observation.Id = attacker.EntityId
	p = &observation
	at := p.At.UnixMilli()
	if attacker.SkillId == 24201 && publicActiveSuccess(attacker) {
		if s.uppercutAction != pack.CombatActionId {
			s.uppercutAction = pack.CombatActionId
			s.combo = &fighterComboCast{atMs: at, targetID: attacker.Attacker.TargetId}
			s.kick = nil
		}
	}
	if attacker.SkillId == 24301 && publicActiveSuccess(attacker) && s.kick != nil {
		kick := s.kick
		s.kick = nil
		cast := kick.combo
		if at >= kick.launchAtMs && attacker.Attacker.TargetId == cast.targetID {
			// The impact confirms the target; timing remains the earlier public
			// launch, so projectile travel cannot inflate the animation interval.
			t.publish(&event.EventArcanaSignal{EventBase: event.EventBase{EventId: event.EventIdArcanaSignal, At: kick.launchAtMs / 1000, Id: strconv.FormatUint(p.Id, 10)}, AtMs: kick.launchAtMs,
				Signal: "fighter-combo", SkillId: 24201, CastAtMs: cast.atMs, TargetId: strconv.FormatUint(cast.targetID, 10), ReverseAtMs: cast.reverseAtMs, ReverseEndAtMs: cast.reverseEndAtMs, ReadyAtMs: cast.readyAtMs, Count: 1, Complete: true})
		}
	}
	if attacker.SkillId == 59121 && s.heavy != nil && at >= s.heavy.CastAtMs {
		// The shot geometry proves the linked count (including explicit zero).
		// Require the actual target hit before attributing it to a boss.
		confirmed := false
		for _, a := range pack.SubPackets {
			if a.Hit != nil && a.EntityId != 0 {
				v := *s.heavy
				v.TargetId = strconv.FormatUint(a.EntityId, 10)
				t.partyArcanaSignal(p, &v)
				confirmed = true
			}
		}
		if confirmed {
			s.heavy = nil
		}
	}
	if attacker.SkillId == 59144 && s.chemical != nil && at >= s.chemical.atMs {
		cast := s.chemical
		for _, a := range pack.SubPackets {
			if a.Hit != nil && a.EntityId == cast.targetID {
				if cast.firstHitAtMs == 0 {
					cast.firstHitAtMs = at
				}
				cast.lastHitAtMs = at
				if a.Hit.Options&packet.CombatActionHitOptionsMultiHit != 0 {
					cast.hitCount = a.Hit.MultiHitCount
					if cast.hitCount != cast.effectCount {
						cast.invalid = true
					}
				}
			}
		}
		if !cast.invalid && cast.firstHitAtMs > 0 && (cast.effectCount == 1 || cast.hitCount == cast.effectCount) {
			t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "chemical-sample", SkillId: 59144, CastAtMs: cast.atMs, FirstHitAtMs: cast.firstHitAtMs, TargetId: strconv.FormatUint(cast.targetID, 10), Count: cast.effectCount, Complete: true})
			s.chemical = nil
		}
	}
	if attacker.SkillId == 59165 && s.interlude != nil && at >= s.interlude.atMs {
		cast := s.interlude
		if cast.actions[pack.CombatActionId] {
			return
		}
		cast.actions[pack.CombatActionId] = true
		if cast.firstAttackAtMs == 0 {
			cast.firstAttackAtMs = at
		}
		cast.lastAttackAtMs = at
		for _, a := range pack.SubPackets {
			if a.Hit != nil && a.EntityId != 0 {
				cast.targets[a.EntityId] = true
			}
		}
		if cast.positions > 0 && cast.positions <= 100 && len(cast.actions) == int(cast.positions) {
			for target := range cast.targets {
				t.partyArcanaSignal(p, &event.EventArcanaSignal{Signal: "interlude-sample", SkillId: 59165, CastAtMs: cast.atMs, FirstHitAtMs: cast.firstAttackAtMs, TargetId: strconv.FormatUint(target, 10), Count: cast.positions, Complete: true})
			}
			s.interlude = nil
		}
	}
}

func publicActiveSuccess(a *packet.CombatActionPacket) bool {
	const flags = packet.CombatActionTypeAttacker | packet.CombatActionTypeSkillActive | packet.CombatActionTypeSkillSuccess | packet.CombatActionTypeSkillPlayerCharacter
	return a != nil && a.Hit == nil && a.Attacker != nil && a.Attacker.TargetId != 0 && a.Type&flags == flags
}

func partyArcanaPublicSkill(skill uint16) bool {
	switch skill {
	case 20017, 21002, 30102, 59026, 59028, 59040, 59045, 59060, 59105, 59106, 59121, 59123, 59144, 59165, 59185, 59186, 59187:
		return true
	}
	return false
}

// Only explicit active/success records confirm these releases. Multi-hit
// phases lacking those flags remain compatibility fallbacks.
func publicKpiAction(a *packet.CombatActionPacket) bool {
	if !publicActiveSuccess(a) {
		return false
	}
	switch a.SkillId {
	case 20017, 21002, 59026, 59028, 59105, 59106:
		return true
	}
	return false
}
