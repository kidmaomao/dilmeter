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
		result.ActionId = msg[1].Data().(uint32)
	case MessageElemTypeLong:
		token := msg[1].Data().(uint64)
		result.ActionId = uint32(token) ^ uint32(token>>32)
	}
	return result, nil
}
