package packet

import "fmt"

const OpcodeSpiritWeaponUpdate OpCode = 44582
const OpcodeSpiritAwakeningReady OpCode = 44610
const SpiritAwakeningFullCharge uint32 = 60000

func IsSpiritAwakeningSkill(id uint16) bool { return id >= 35111 && id <= 35130 }

// 44610 names the weapon type whose awakening gauge has just reached full.
func ParseSpiritAwakeningReady(msg Message) (uint16, error) {
	if len(msg) != 1 || msg[0].Type() != MessageElemTypeShort {
		return 0, fmt.Errorf("spirit ready: invalid weapon type field")
	}
	typ := msg[0].Data().(uint16)
	if typ < 1 || typ > 20 {
		return 0, fmt.Errorf("spirit ready: unknown weapon type")
	}
	return typ, nil
}

// SpiritWeaponUpdate is one positional slot in 44582. Unchanged slots still
// carry a zero mask; their position, rather than the equipped weapon, identifies
// the spirit. A complete snapshot supplies the weapon type used by later deltas.
type SpiritWeaponUpdate struct {
	WeaponType *uint16
	Charge     *uint32
}

func ParseSpiritWeaponUpdates(msg Message) ([]SpiritWeaponUpdate, error) {
	if len(msg) == 0 || msg[0].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("spirit update: missing slot count")
	}
	count := int(msg[0].Data().(uint32))
	if count > 64 || len(msg) < count+1 {
		return nil, fmt.Errorf("spirit update: invalid slot count")
	}
	// Full 0xffffffff snapshots contain these 16 fields. Partial masks select
	// the same fields in order; bit 14 is the awakening gauge, not affection,
	// experience, spirit level, or skill points.
	types := [...]MessageElemType{
		MessageElemTypeShort, MessageElemTypeShort, MessageElemTypeShort,
		MessageElemTypeLong, MessageElemTypeString, MessageElemTypeShort,
		MessageElemTypeInt, MessageElemTypeInt, MessageElemTypeShort,
		MessageElemTypeShort, MessageElemTypeShort, MessageElemTypeString,
		MessageElemTypeString, MessageElemTypeShort, MessageElemTypeInt, MessageElemTypeLong,
	}
	updates := make([]SpiritWeaponUpdate, count)
	cursor := 1
	for slot := range updates {
		if cursor >= len(msg) || msg[cursor].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("spirit update: missing slot mask")
		}
		mask := msg[cursor].Data().(uint32)
		cursor++
		if mask != 0xffffffff && mask & ^uint32(0xffff) != 0 {
			return nil, fmt.Errorf("spirit update: unknown mask")
		}
		for bit, typ := range types {
			if mask&(1<<bit) == 0 {
				continue
			}
			if cursor >= len(msg) || msg[cursor].Type() != typ {
				return nil, fmt.Errorf("spirit update: invalid field %d", bit)
			}
			switch bit {
			case 0:
				value := msg[cursor].Data().(uint16)
				updates[slot].WeaponType = &value
			case 14:
				value := msg[cursor].Data().(uint32)
				if value > SpiritAwakeningFullCharge {
					return nil, fmt.Errorf("spirit update: invalid charge")
				}
				updates[slot].Charge = &value
			}
			cursor++
		}
	}
	if cursor != len(msg) {
		return nil, fmt.Errorf("spirit update: trailing fields")
	}
	return updates, nil
}
