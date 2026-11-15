package packet

import "testing"

func TestParseStatUpdatePacket(t *testing.T) {
	msg := Message{
		NewMessageElemByte(4), NewMessageElemInt(3),
		NewMessageElemInt(28), NewMessageElemFloat(123.5),
		NewMessageElemInt(30), NewMessageElemInt(500),
		NewMessageElemInt(198), NewMessageElemShort(7),
	}
	entries, err := ParseStatUpdatePacket(msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].StatId != 28 || entries[0].Value != 123.5 ||
		entries[1].StatId != 30 || entries[1].Value != 500 || entries[2].Value != 7 {
		t.Fatalf("unexpected stat entries: %#v", entries)
	}
}

func TestParseStatUpdatePacketRejectsTruncatedCount(t *testing.T) {
	_, err := ParseStatUpdatePacket(Message{NewMessageElemByte(4), NewMessageElemInt(2)})
	if err == nil {
		t.Fatal("truncated stat packet was accepted")
	}
}

func TestReadPrivateManaMaximumFollowsDefaultRegen(t *testing.T) {
	head := Message{
		NewMessageElemByte(3), NewMessageElemInt(2),
		NewMessageElemInt(34), NewMessageElemFloat(2913),
		NewMessageElemInt(35), NewMessageElemFloat(4918),
	}
	withUpdates := append(append(Message(nil), head...),
		NewMessageElemInt(0), NewMessageElemInt(0), NewMessageElemInt(3),
		NewMessageElemInt(1), NewMessageElemFloat(.12), NewMessageElemFloat(9169.279),
		NewMessageElemInt(2), NewMessageElemFloat(.05), NewMessageElemFloat(6557),
		NewMessageElemInt(3), NewMessageElemFloat(.4), NewMessageElemFloat(4918))
	if maximum := ReadPrivateManaMaximum(withUpdates); maximum != 6557 {
		t.Fatalf("read maximum %g from the private weapon-change packet, want 6557", maximum)
	}
	withAdded := append(append(Message(nil), head...), NewMessageElemInt(2),
		NewMessageElemInt(2), NewMessageElemFloat(.05), NewMessageElemInt(4294967295), NewMessageElemInt(32),
		NewMessageElemByte(0), NewMessageElemFloat(5597), NewMessageElemByte(1),
		NewMessageElemInt(130), NewMessageElemFloat(25), NewMessageElemInt(4294967295), NewMessageElemInt(32),
		NewMessageElemByte(0), NewMessageElemFloat(4200), NewMessageElemByte(1),
		NewMessageElemInt(0), NewMessageElemInt(0))
	if maximum := ReadPrivateManaMaximum(withAdded); maximum != 5597 {
		t.Fatalf("read %g after adding the standard and a temporary mana regen, want 5597", maximum)
	}
	noDefault := append(append(Message(nil), head...),
		NewMessageElemInt(0), NewMessageElemInt(0), NewMessageElemInt(1),
		NewMessageElemInt(130), NewMessageElemFloat(25), NewMessageElemFloat(4200))
	for _, msg := range []Message{noDefault, withUpdates[:len(withUpdates)-1], head} {
		if maximum := ReadPrivateManaMaximum(msg); maximum != 0 {
			t.Fatalf("trusted a mana limit without a complete standard regen: %g", maximum)
		}
	}
}
