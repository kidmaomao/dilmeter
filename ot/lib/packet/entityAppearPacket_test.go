package packet

import (
	"encoding/binary"
	"testing"
)

// Minimal public character snapshot, independent of private capture data.
func batchTestCharacter(id uint64) []byte {
	msg := make(Message, 143)
	for i := range msg {
		msg[i] = NewMessageElemByte(0)
	}
	msg[0], msg[1], msg[2] = NewMessageElemLong(id), NewMessageElemByte(5), NewMessageElemString("TestPlayer")
	msg[5] = NewMessageElemInt(9001)
	msg[7], msg[9] = NewMessageElemShort(0), NewMessageElemShort(0)
	for _, i := range []int{13, 14, 15, 16} {
		msg[i] = NewMessageElemFloat(1)
	}
	for _, i := range []int{39, 40, 41, 43, 44, 45, 50, 51, 55, 78} {
		msg[i] = NewMessageElemInt(0)
	}
	msg[81] = NewMessageElemString("")
	msg[137], msg[141] = NewMessageElemLong(0), NewMessageElemLong(0)
	body := binary.BigEndian.AppendUint32(nil, OpcodeEntityAppear)
	body = binary.BigEndian.AppendUint64(body, id)
	body = binary.AppendUvarint(body, uint64(len(msg.Bytes())))
	return append(body, msg.Bytes()...)
}

func TestEntitiesAppearContinuesPastNonCharactersAndBadEntries(t *testing.T) {
	valid := func(id uint64) Message {
		return Message{NewMessageElemShort(16), NewMessageElemInt(0), NewMessageElemBin(batchTestCharacter(id))}
	}
	for _, skipped := range []Message{
		{NewMessageElemShort(32), NewMessageElemInt(0), NewMessageElemBin(nil)},
		{NewMessageElemShort(16), NewMessageElemInt(0), NewMessageElemBin([]byte{0})},
		{NewMessageElemByte(16), NewMessageElemInt(0), NewMessageElemBin(nil)},
	} {
		msg := Message{NewMessageElemShort(4)}
		msg = append(msg, skipped...)
		msg = append(msg, valid(123)...)
		msg = append(msg, skipped...)
		msg = append(msg, valid(456)...)
		entities, err := ParseEntitiesAppearPacket(&GamePacket{Msg: msg})
		if err != nil || len(entities) != 2 || entities[0].Id != 123 || entities[1].Id != 456 {
			t.Fatalf("later teammates lost: entities=%v err=%v", entities, err)
		}
	}
}

func TestEntitiesAppearRejectsTruncatedBatch(t *testing.T) {
	if _, err := ParseEntitiesAppearPacket(&GamePacket{Msg: Message{NewMessageElemShort(1), NewMessageElemShort(16)}}); err == nil {
		t.Fatal("truncated entry accepted")
	}
}
