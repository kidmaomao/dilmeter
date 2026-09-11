package packet

import "fmt"

//go:generate go run gen_opcode_string.go

type OpCode uint32

const (
	OpcodeEntityAppear         = 21004
	OpcodeEntityDisappear      = 21005
	OpcodeCreatureBodyUpdate   = 21006
	OpcodeItemAppear           = 21009
	OpcodeItemDisappear        = 21010
	OpcodeChat                 = 21100
	OpcodeNotice               = 21101
	OpcodeUnknownWarp          = 21102
	OpcodeEntitiesAppear       = 21300
	OpcodeEntitiesDisappear    = 21301
	OpcodeIsNowDead            = 21500
	OpcodeEquipmentChanged     = 23014
	OpcodeUnequipment          = 23015
	OpcodeItemDurabilityUpdate = 23500
	OpcodeForceWalk            = 26011
	OpcodeFlying               = 26031
	// OpcodeSkillExecute is the server confirmation that a locally requested
	// skill actually executed. Unlike CombatActionPack, it is also emitted for
	// non-damaging and deployable skills such as Smoke Screen and Hydra.
	OpcodeSkillPrepareReady = 27013
	OpcodeSkillExecute      = 27016
	OpcodeSkillPrepareEnd   = 27028
	// OpcodeSkillCooldownReset names the affected skill directly and resets
	// its current cooldown. OpcodeSkillCooldownReduce carries one or more
	// (skill id, milliseconds) reductions.
	OpcodeSkillCooldownReset  = 27049
	OpcodeSkillCooldownReduce = 27069
	OpcodeChangeStanceRes     = 28201
	OpcodeChangeStance        = 28202
	OpcodeStatUpdatePrivate   = 30000
	OpcodeStatUpdatePublic    = 30002
	OpcodeCombatTargetUpdate  = 31002
	OpcodeSkillTarget         = 31006
	OpcodeSetCombatTarget     = 31008
	OpcodeSetFinisher         = 31009
	OpcodeSetFinisher2        = 31010
	OpcodeCombatActionPack    = 31014
	OpcodeCombatAttackRes     = 32001
	OpcodeEffect              = 37009
	OpcodeEffectDelayed       = 37013
	OpcodeConditionUpdate     = 41000
	OpcodeSharpMind           = 42014
	// OpcodeSkillListState carries local SLST<skill-id> state updates. The
	// combat-astrology Grand Conjunction critical-CD effect uses this signal.
	OpcodeSkillListState = 135688
	OpcodeWalking        = 0xfd13021
	OpcodeRunning        = 0xf44bba3
)

func (t OpCode) String() string {
	if s, ok := OpCodeStringMap[uint32(t)]; ok {
		return s
	}

	return fmt.Sprintf("%v", uint32(t))
}
