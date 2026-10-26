package main

import (
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

type spiritWeaponState struct {
	weaponType uint16
	charge     uint32
	observed   bool
	chargeAtMs int64
}

// In all three captures the named signal follows its gauge delta by 0–5 ms.
// Only associate a unique, recent matching slot, never equipment or an index
// remembered from a different session.
const spiritSignalWindowMs int64 = 100

func (t *eventPublisher) publishSpiritWeaponPacket(p *packet.GamePacket) {
	if !t.acceptsLocalCooldownPacket(p) {
		return
	}
	updates, err := packet.ParseSpiritWeaponUpdates(p.Msg)
	if err != nil {
		return
	}
	if t.spiritOwner != p.Id {
		t.spiritWeapons = nil
		t.spiritOwner = p.Id
	}
	before := spiritWeaponEnergies(t.spiritWeapons)
	if len(t.spiritWeapons) != len(updates) {
		// A changed roster invalidates positional identity. Never apply a
		// delta to an old slot merely because its index happens to match.
		t.spiritWeapons = make([]spiritWeaponState, len(updates))
	}
	for slot, update := range updates {
		state := &t.spiritWeapons[slot]
		if update.WeaponType != nil {
			if state.weaponType != *update.WeaponType {
				*state = spiritWeaponState{weaponType: *update.WeaponType}
			}
		}
		if update.Charge != nil {
			state.charge, state.observed = *update.Charge, true
			state.chargeAtMs = p.At.UnixMilli()
		}
	}
	t.publishSpiritEnergyChanges(p, before)
}

func (t *eventPublisher) publishSpiritEnergyChanges(p *packet.GamePacket, before map[uint16]event.EventSkillEnergy) {
	after := spiritWeaponEnergies(t.spiritWeapons)
	for id, old := range before {
		if _, ok := after[id]; !ok {
			old.Active = false
			after[id] = old
		}
	}
	for id, energy := range after {
		if old, ok := before[id]; ok && old.Percent == energy.Percent && old.Active == energy.Active {
			continue
		}
		energy.EventBase = event.EventBase{EventId: event.EventIdSkillEnergy, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}
		t.publish(&energy)
	}
}

// A named full-charge notification or successful cast can identify an unknown
// slot when capture started after the roster. Known identities win; stale,
// ambiguous and conflicting deltas must not create a mapping.
func (t *eventPublisher) spiritSlotForSignal(weaponType uint16, charge uint32, atMs int64) int {
	for slot, state := range t.spiritWeapons {
		if state.weaponType == weaponType {
			return slot
		}
	}
	match := -1
	for slot, state := range t.spiritWeapons {
		age := atMs - state.chargeAtMs
		if state.weaponType != 0 || !state.observed || state.charge != charge || state.chargeAtMs <= 0 || age < 0 || age > spiritSignalWindowMs {
			continue
		}
		if match != -1 {
			return -1
		}
		match = slot
	}
	return match
}

func (t *eventPublisher) publishSpiritReadyPacket(p *packet.GamePacket) {
	if !t.acceptsLocalCooldownPacket(p) || t.spiritOwner != p.Id {
		return
	}
	weaponType, err := packet.ParseSpiritAwakeningReady(p.Msg)
	if err != nil {
		return
	}
	slot := t.spiritSlotForSignal(weaponType, packet.SpiritAwakeningFullCharge, p.At.UnixMilli())
	if slot < 0 {
		return
	}
	before := spiritWeaponEnergies(t.spiritWeapons)
	t.spiritWeapons[slot] = spiritWeaponState{weaponType: weaponType, charge: packet.SpiritAwakeningFullCharge, observed: true, chargeAtMs: p.At.UnixMilli()}
	t.publishSpiritEnergyChanges(p, before)
}

func (t *eventPublisher) observeSpiritAwakeningExecute(p *packet.GamePacket, skillID uint16) {
	if !packet.IsSpiritAwakeningSkill(skillID) || p.Id != t.localEntityId || t.spiritOwner != p.Id {
		return
	}
	weaponType := skillID - 35110
	slot := t.spiritSlotForSignal(weaponType, 0, p.At.UnixMilli())
	if slot < 0 || t.spiritWeapons[slot].weaponType != 0 {
		return
	}
	// A normal cast has an immediately preceding zero delta. A 58015 proc
	// does not: keep its full charge instead of clearing on every execution.
	before := spiritWeaponEnergies(t.spiritWeapons)
	t.spiritWeapons[slot].weaponType = weaponType
	t.publishSpiritEnergyChanges(p, before)
}

func spiritWeaponEnergies(slots []spiritWeaponState) map[uint16]event.EventSkillEnergy {
	result := make(map[uint16]event.EventSkillEnergy)
	for _, slot := range slots {
		if slot.weaponType < 1 || slot.weaponType > 20 || !slot.observed {
			continue
		}
		id := 35110 + slot.weaponType
		result[id] = event.EventSkillEnergy{SkillId: id, Active: true,
			Percent: float64(slot.charge) * 100 / float64(packet.SpiritAwakeningFullCharge)}
	}
	return result
}
