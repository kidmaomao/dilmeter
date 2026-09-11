package packet

import "fmt"

// ParseSetCombatTargetPacket returns the entity selected by the packet actor.
// A zero target clears the current selection. The trailing mode and label vary
// by client action and are not needed for target-health tracking.
func ParseSetCombatTargetPacket(msg Message) (uint64, error) {
	if len(msg) < 1 || msg[0].Type() != MessageElemTypeLong {
		return 0, fmt.Errorf("invalid combat target packet")
	}
	return msg[0].Data().(uint64), nil
}
