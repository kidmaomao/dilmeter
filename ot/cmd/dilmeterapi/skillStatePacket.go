package main

import "gitlab.com/prilus/mabidilmeter/lib/packet"

const (
	magnumShotSkillID uint16 = 21002
	finalShotSkillID  uint16 = 23030
	latikaSecretCCID  uint32 = 407
	rapidAimCCID      uint32 = 477
)

func parsePreparedSkillID(msg packet.Message) (uint16, bool) {
	if len(msg) < 1 || msg[0].Type() != packet.MessageElemTypeShort {
		return 0, false
	}
	return msg[0].Data().(uint16), true
}

func parseEndedSkillID(msg packet.Message) (uint16, bool) {
	if len(msg) < 1 || msg[len(msg)-1].Type() != packet.MessageElemTypeShort {
		return 0, false
	}
	return msg[len(msg)-1].Data().(uint16), true
}

// Skill target update (31006) begins as
// [byte(1), long(target), short(skill), byte(...)] and clears as [byte(0)].
func parseSkillTarget(msg packet.Message) (active bool, targetID uint64, skillID uint16, ok bool) {
	if len(msg) < 1 || msg[0].Type() != packet.MessageElemTypeByte {
		return false, 0, 0, false
	}
	if msg[0].Data().(uint8) == 0 {
		return false, 0, 0, true
	}
	if len(msg) < 3 || msg[1].Type() != packet.MessageElemTypeLong || msg[2].Type() != packet.MessageElemTypeShort {
		return false, 0, 0, false
	}
	targetID = msg[1].Data().(uint64)
	skillID = msg[2].Data().(uint16)
	return targetID != 0, targetID, skillID, true
}
