package packet

import "testing"

func TestCombatMultiHitCountRetainsServerCount(t *testing.T) {
	makeHit := func(multi bool) Message {
		options := uint32(32)
		if multi {
			options |= uint32(CombatActionHitOptionsMultiHit)
		}
		msg := Message{NewMessageElemInt(2556890), NewMessageElemLong(200), NewMessageElemByte(1), NewMessageElemShort(2000), NewMessageElemShort(23002), NewMessageElemShort(0), NewMessageElemShort(0),
			NewMessageElemInt(options), NewMessageElemFloat(238693.12), NewMessageElemFloat(0), NewMessageElemInt(0), NewMessageElemInt(0), NewMessageElemInt(0), NewMessageElemFloat(251), NewMessageElemFloat(-295)}
		if multi {
			msg = append(msg, NewMessageElemInt(5), NewMessageElemInt(150), NewMessageElemInt(0), NewMessageElemInt(0))
		}
		return append(msg, NewMessageElemByte(32), NewMessageElemInt(0), NewMessageElemLong(100), NewMessageElemInt(24), NewMessageElemLong(100), NewMessageElemByte(0))
	}
	multi, err := parseCombatActionPacket(200, makeHit(true))
	if err != nil || multi.Hit == nil || multi.Hit.MultiHitCount != 5 {
		t.Fatalf("multi hit count not retained: %#v, %v", multi, err)
	}
	if multi.Hit.Damage != float32(238693.12) {
		t.Fatal("retaining count changed aggregate damage")
	}
	ordinary, err := parseCombatActionPacket(200, makeHit(false))
	if err != nil || ordinary.Hit.MultiHitCount != 0 {
		t.Fatal("ordinary hit became multi-hit count")
	}
}
