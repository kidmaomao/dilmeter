package packet

import (
	"encoding/binary"
	"math"
)

// Prop appearance (21200) is distinct from a character appearance. Coordinates
// in its 72-byte transform are X, height, Y; the region is an instance ID.
type WorldProp struct {
	ID               uint64
	PropID, RegionID uint32
	X, Y             float64
	State            string
}

func ParseWorldPropPacket(msg Message) (WorldProp, bool) {
	var result WorldProp
	types := []MessageElemType{MessageElemTypeLong, MessageElemTypeInt, MessageElemTypeString, MessageElemTypeString, MessageElemTypeBin, MessageElemTypeString, MessageElemTypeLong, MessageElemTypeByte, MessageElemTypeInt, MessageElemTypeShort}
	if len(msg) != len(types) {
		return result, false
	}
	for i, t := range types {
		if msg[i].Type() != t {
			return result, false
		}
	}
	data := msg[4].Data().([]byte)
	if len(data) != 72 || binary.LittleEndian.Uint32(data[:4]) != msg[1].Data().(uint32) {
		return result, false
	}
	result = WorldProp{ID: msg[0].Data().(uint64), PropID: msg[1].Data().(uint32), RegionID: binary.LittleEndian.Uint32(data[4:8]), State: msg[5].Data().(string), X: float64(math.Float32frombits(binary.LittleEndian.Uint32(data[8:12]))), Y: float64(math.Float32frombits(binary.LittleEndian.Uint32(data[16:20])))}
	if result.ID == 0 || result.RegionID == 0 || math.IsNaN(result.X) || math.IsNaN(result.Y) || math.IsInf(result.X, 0) || math.IsInf(result.Y, 0) || math.Abs(result.X) > 1e7 || math.Abs(result.Y) > 1e7 {
		return WorldProp{}, false
	}
	return result, true
}
