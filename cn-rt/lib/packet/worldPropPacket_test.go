package packet

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestWorldPropTransformAndStrictShape(t *testing.T) {
	data := make([]byte, 72)
	binary.LittleEndian.PutUint32(data, 49205)
	binary.LittleEndian.PutUint32(data[4:], 35027)
	binary.LittleEndian.PutUint32(data[8:], math.Float32bits(25156))
	binary.LittleEndian.PutUint32(data[16:], math.Float32bits(41145))
	msg := Message{NewMessageElemLong(123), NewMessageElemInt(49205), NewMessageElemString(""), NewMessageElemString(""), NewMessageElemBin(data), NewMessageElemString("landing"), NewMessageElemLong(0), NewMessageElemByte(0), NewMessageElemInt(0), NewMessageElemShort(0)}
	prop, ok := ParseWorldPropPacket(msg)
	if !ok || prop.ID != 123 || prop.RegionID != 35027 || prop.X != 25156 || prop.Y != 41145 || prop.State != "landing" {
		t.Fatalf("transform: %+v %v", prop, ok)
	}
	for i := 0; i < len(msg); i++ {
		if _, ok := ParseWorldPropPacket(msg[:i]); ok {
			t.Fatalf("accepted truncated %d", i)
		}
	}
	bad := append(Message(nil), msg...)
	bad[1] = NewMessageElemInt(49206)
	if _, ok := ParseWorldPropPacket(bad); ok {
		t.Fatal("accepted mismatched type")
	}
	bad[1] = NewMessageElemShort(49205)
	if _, ok := ParseWorldPropPacket(bad); ok {
		t.Fatal("accepted wrong field type")
	}
	binary.LittleEndian.PutUint32(data[8:], math.Float32bits(float32(math.NaN())))
	if _, ok := ParseWorldPropPacket(msg); ok {
		t.Fatal("accepted nonfinite coordinate")
	}
}
