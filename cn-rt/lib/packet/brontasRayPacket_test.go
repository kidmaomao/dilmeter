package packet

import "testing"

func brontasWireFixture() *GamePacket {
	return &GamePacket{Op: 37013, Msg: Message{NewMessageElemInt(90), NewMessageElemInt(879), NewMessageElemByte(0), NewMessageElemInt(19661), NewMessageElemInt(31238), NewMessageElemInt(11000), NewMessageElemInt(846), NewMessageElemShort(1), NewMessageElemInt(10), NewMessageElemInt(1600), NewMessageElemInt(90)}}
}

func TestBrontasRayStrictWireShape(t *testing.T) {
	p := brontasWireFixture()
	r, ok := ParseBrontasRayPacket(p)
	if !ok || r.Direction != 1 || r.Lane != 10 || r.DelayMs != 90 || r.LeadMs != 1600 {
		t.Fatalf("decode: %+v %v", r, ok)
	}
	for i := 0; i < len(p.Msg); i++ {
		bad := *p
		bad.Msg = p.Msg[:i]
		if _, ok := ParseBrontasRayPacket(&bad); ok {
			t.Fatalf("accepted truncation %d", i)
		}
		bad.Msg = append(Message(nil), p.Msg...)
		bad.Msg[i] = NewMessageElemString("bad")
		if _, ok := ParseBrontasRayPacket(&bad); ok {
			t.Fatalf("accepted wrong type %d", i)
		}
	}
	for i, v := range map[int]IMessageElem{1: NewMessageElemInt(878), 2: NewMessageElemByte(1), 3: NewMessageElemInt(0), 4: NewMessageElemInt(0), 5: NewMessageElemInt(0), 6: NewMessageElemInt(0), 7: NewMessageElemShort(4), 8: NewMessageElemInt(13), 9: NewMessageElemInt(1000), 10: NewMessageElemInt(91)} {
		bad := brontasWireFixture()
		bad.Msg[i] = v
		if _, ok := ParseBrontasRayPacket(bad); ok {
			t.Fatalf("accepted unknown parameter %d", i)
		}
	}
	p.Op = 37011
	if _, ok := ParseBrontasRayPacket(p); ok {
		t.Fatal("wrong opcode accepted")
	}
	p.Msg = Message{NewMessageElemInt(879), NewMessageElemByte(2), NewMessageElemInt(25161), NewMessageElemInt(36738)}
	if !IsBrontasRayEndPacket(p) {
		t.Fatal("verified whole-phase end rejected")
	}
	p.Msg[1] = NewMessageElemByte(1)
	if IsBrontasRayEndPacket(p) {
		t.Fatal("unverified phase treated as end")
	}
}

func TestBrontasPositionConfirmedStartAndShapes(t *testing.T) {
	p := &GamePacket{Op: 26004, Msg: Message{NewMessageElemByte(1), NewMessageElemInt(35041), NewMessageElemInt(25160), NewMessageElemInt(32540)}}
	v, ok := ParseBrontasPositionPacket(p)
	if !ok || v.RegionID != 35041 || v.Moving {
		t.Fatal(v, ok)
	}
	p.Op = OpcodeRunning
	p.Msg = Message{NewMessageElemInt(25160), NewMessageElemInt(32540), NewMessageElemInt(21000), NewMessageElemInt(36700), NewMessageElemByte(1), NewMessageElemByte(1)}
	v, ok = ParseBrontasPositionPacket(p)
	if !ok || v.X != 25160 || v.Y != 32540 || !v.Moving || v.RegionID != 0 {
		t.Fatal("destination used as present point", v, ok)
	}
	for i := range p.Msg {
		bad := *p
		bad.Msg = append(Message(nil), p.Msg...)
		bad.Msg[i] = NewMessageElemString("bad")
		if _, ok := ParseBrontasPositionPacket(&bad); ok {
			t.Fatal("wrong type accepted", i)
		}
	}
	p.Msg[5] = NewMessageElemByte(2)
	if _, ok := ParseBrontasPositionPacket(p); ok {
		t.Fatal("unknown movement flag accepted")
	}
	p.Op = 26004
	p.Msg = Message{NewMessageElemByte(0), NewMessageElemInt(1), NewMessageElemInt(2)}
	if _, ok := ParseBrontasPositionPacket(p); ok {
		t.Fatal("unknown correction shape accepted")
	}
}
