package packet

// Effect 879 is Brontas' arena ray, not Miel's skill 52401. These parameters
// are the wire shape verified in October CN captures. Unknown layouts fail
// closed instead of silently inventing geometry or a different timing model.
type BrontasRayPacket struct {
	Direction uint16
	Lane      uint32
	DelayMs   uint32
	LeadMs    uint32
}

func ParseBrontasRayPacket(p *GamePacket) (BrontasRayPacket, bool) {
	var result BrontasRayPacket
	if p == nil || p.Op != OpcodeEffectDelayed || len(p.Msg) != 11 {
		return result, false
	}
	types := [...]MessageElemType{3, 3, 1, 3, 3, 3, 3, 2, 3, 3, 3}
	for i, typ := range types {
		if p.Msg[i] == nil || p.Msg[i].Type() != typ {
			return result, false
		}
	}
	m := p.Msg
	integer := func(i int) uint32 { return m[i].Data().(uint32) }
	if integer(1) != 879 || m[2].Data().(uint8) != 0 || integer(3) != 19661 || integer(4) != 31238 ||
		integer(5) != 11000 || integer(6) != 846 || integer(9) != 1600 || integer(0) > 1600 || integer(0) != integer(10) {
		return result, false
	}
	result = BrontasRayPacket{Direction: m[7].Data().(uint16), Lane: integer(8), DelayMs: integer(0), LeadMs: integer(9)}
	return result, result.Direction <= 3 && result.Lane <= 12
}

func IsBrontasRayEndPacket(p *GamePacket) bool {
	if p == nil || p.Op != 37011 || len(p.Msg) != 4 {
		return false
	}
	for i, typ := range [...]MessageElemType{3, 1, 3, 3} {
		if p.Msg[i] == nil || p.Msg[i].Type() != typ {
			return false
		}
	}
	return p.Msg[0].Data().(uint32) == 879 && p.Msg[1].Data().(uint8) == 2 &&
		p.Msg[2].Data().(uint32) == 25161 && p.Msg[3].Data().(uint32) == 36738
}

// Movement starts are confirmed coordinates; the destination is not a current
// position. Both observed continuation flags (0 and 1) carry the same shape.
type BrontasPositionPacket struct {
	RegionID, X, Y uint32
	DestX, DestY   uint32
	Mode           uint64
	Moving         bool
}

func ParseBrontasPositionPacket(p *GamePacket) (BrontasPositionPacket, bool) {
	var v BrontasPositionPacket
	if p == nil {
		return v, false
	}
	if p.Op == 26004 || p.Op == 26009 {
		if len(p.Msg) != 4 {
			return v, false
		}
		for i, typ := range [...]MessageElemType{1, 3, 3, 3} {
			if p.Msg[i] == nil || p.Msg[i].Type() != typ {
				return v, false
			}
		}
		if p.Msg[0].Data().(uint8) != 1 {
			return v, false
		}
		v = BrontasPositionPacket{RegionID: p.Msg[1].Data().(uint32), X: p.Msg[2].Data().(uint32), Y: p.Msg[3].Data().(uint32)}
		return v, v.RegionID != 0 && v.X <= 1000000 && v.Y <= 1000000
	}
	if p.Op == OpcodeRunning || p.Op == OpcodeWalking {
		if len(p.Msg) != 6 {
			return v, false
		}
		for i, typ := range [...]MessageElemType{3, 3, 3, 3, 1, 1} {
			if p.Msg[i] == nil || p.Msg[i].Type() != typ {
				return v, false
			}
		}
		if p.Msg[4].Data().(uint8) != 1 || p.Msg[5].Data().(uint8) > 1 {
			return v, false
		}
		v.X, v.Y = p.Msg[0].Data().(uint32), p.Msg[1].Data().(uint32)
		v.DestX, v.DestY = p.Msg[2].Data().(uint32), p.Msg[3].Data().(uint32)
		v.Mode = uint64(p.Op)
		v.Moving = v.X != v.DestX || v.Y != v.DestY
		return v, v.X <= 1000000 && v.Y <= 1000000 && v.DestX <= 1000000 && v.DestY <= 1000000
	}
	return v, false
}
