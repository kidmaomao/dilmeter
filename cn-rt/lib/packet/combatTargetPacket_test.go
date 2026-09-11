package packet

import "testing"

func TestParseSetCombatTargetPacket(t *testing.T) {
	want := uint64(4767482431088776)
	got, err := ParseSetCombatTargetPacket(Message{
		NewMessageElemLong(want), NewMessageElemByte(0), NewMessageElemString(""),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("target = %d, want %d", got, want)
	}
}

func TestParseSetCombatTargetPacketAcceptsClear(t *testing.T) {
	got, err := ParseSetCombatTargetPacket(Message{NewMessageElemLong(0)})
	if err != nil || got != 0 {
		t.Fatalf("clear target = %d, err = %v", got, err)
	}
}

func TestParseSetCombatTargetPacketRejectsInvalidMessage(t *testing.T) {
	if _, err := ParseSetCombatTargetPacket(Message{NewMessageElemInt(1)}); err == nil {
		t.Fatal("invalid combat target packet was accepted")
	}
}
