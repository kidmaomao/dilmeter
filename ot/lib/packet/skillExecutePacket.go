package packet

import "fmt"

// SkillExecutePacket is the server acknowledgement for an executed skill.
// The second field varies by skill: most skills carry either a uint32 action
// value or a uint64 execution token. The remaining fields are not needed for
// cooldown tracking.
type SkillExecutePacket struct {
	SkillId  uint16
	ActionId uint32
}

func ParseSkillExecutePacket(msg Message) (*SkillExecutePacket, error) {
	if len(msg) < 1 || msg[0].Type() != MessageElemTypeShort {
		return nil, fmt.Errorf("skill execute: missing skill id")
	}

	result := &SkillExecutePacket{SkillId: msg[0].Data().(uint16)}
	if result.SkillId == 0 {
		return nil, fmt.Errorf("skill execute: zero skill id")
	}
	if len(msg) < 2 {
		return result, nil
	}

	switch msg[1].Type() {
	case MessageElemTypeInt:
		// CN Thunder repeats (100,1) on every confirmed release. Like the
		// archery mode below, 100 is not a unique execution identifier.
		if result.SkillId == 30102 && len(msg) == 3 && msg[1].Data().(uint32) == 100 && msg[2].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 1 {
			return result, nil
		}
		// CN archery repeats (520,1) for every shot; 520 is a mode flag,
		// not a unique action id. Normal archery must not suppress Magnum.
		if (result.SkillId == 21001 || result.SkillId == 21002) && len(msg) == 3 && msg[1].Data().(uint32) == 520 && msg[2].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 1 {
			return result, nil
		}
		result.ActionId = msg[1].Data().(uint32)
	case MessageElemTypeLong:
		// These two-field packets carry a target entity, not an execution
		// token. Hashing that target suppressed later skills at the same boss.
		// Heavy Artillery also appends the captured (0,1) flags to a target;
		// other multi-field forms retain execution-token deduplication.
		heavyTarget := result.SkillId == 59121 && len(msg) == 4 && msg[2].Type() == MessageElemTypeInt && msg[3].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 0 && msg[3].Data().(uint32) == 1
		alchemyTarget := (result.SkillId == 59144 || result.SkillId == 59145) && len(msg) == 5 && msg[2].Type() == MessageElemTypeInt && msg[3].Type() == MessageElemTypeInt && msg[4].Type() == MessageElemTypeShort && msg[2].Data().(uint32) == 0 && msg[3].Data().(uint32) == 1 && msg[4].Data().(uint16) == 2
		// Act 7 appends four movement floats after its selected target.
		puppetTarget := result.SkillId == 54105 && len(msg) == 8 && msg[2].Type() == MessageElemTypeInt && msg[3].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 0 && msg[3].Data().(uint32) == 1
		if puppetTarget {
			for _, field := range msg[4:] {
				if field.Type() != MessageElemTypeFloat {
					puppetTarget = false
					break
				}
			}
		}
		fighterTarget := (len(msg) == 2 && (result.SkillId == 24101 || result.SkillId == 24201 || result.SkillId == 24301 || result.SkillId == 59181 || result.SkillId == 59182)) || (len(msg) == 4 && (result.SkillId == 59180 || result.SkillId == 59185 || result.SkillId == 59186 || result.SkillId == 59187) && msg[2].Type() == MessageElemTypeInt && msg[3].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 0 && msg[3].Data().(uint32) == 1)
		// Confirmed Alchemic Stinger packets put the selected boss in field
		// two, followed by (0,1) and three elemental shorts. Reusing a boss
		// must not deduplicate Flame Burst against Hydro Pierce or later casts.
		stingerTarget := result.SkillId >= 59060 && result.SkillId <= 59064 && len(msg) == 7 && msg[2].Type() == MessageElemTypeInt && msg[3].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 0 && msg[3].Data().(uint32) == 1
		if stingerTarget {
			for _, field := range msg[4:] {
				if field.Type() != MessageElemTypeShort {
					stingerTarget = false
					break
				}
			}
		}
		archeryTarget := (result.SkillId == 21014 || result.SkillId == 59064) && len(msg) == 4 && msg[2].Type() == MessageElemTypeInt && msg[3].Type() == MessageElemTypeInt && msg[2].Data().(uint32) == 0 && msg[3].Data().(uint32) == 1
		if result.SkillId == 59047 || heavyTarget || alchemyTarget || puppetTarget || fighterTarget || stingerTarget || archeryTarget || (len(msg) == 2 && (result.SkillId == 54302 || result.SkillId == 59121 || result.SkillId == 59122 || result.SkillId == 59123)) {
			return result, nil
		}
		token := msg[1].Data().(uint64)
		result.ActionId = uint32(token) ^ uint32(token>>32)
	}
	return result, nil
}
