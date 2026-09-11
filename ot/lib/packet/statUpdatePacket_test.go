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
	if err != nil { t.Fatal(err) }
	if len(entries) != 3 || entries[0].StatId != 28 || entries[0].Value != 123.5 ||
		entries[1].StatId != 30 || entries[1].Value != 500 || entries[2].Value != 7 {
		t.Fatalf("unexpected stat entries: %#v", entries)
	}
}

func TestParseStatUpdatePacketRejectsTruncatedCount(t *testing.T) {
	_, err := ParseStatUpdatePacket(Message{NewMessageElemByte(4), NewMessageElemInt(2)})
	if err == nil { t.Fatal("truncated stat packet was accepted") }
}
