package packet

import "fmt"

// StatUpdateEntry is one public/private entity attribute update. CN packets
// encode a one-byte mode, an integer count, then count pairs of stat id and a
// typed numeric value.
type StatUpdateEntry struct {
	StatId uint32
	Value  float64
}

func ParseStatUpdatePacket(msg Message) ([]StatUpdateEntry, error) {
	if len(msg) < 2 || msg[0].Type() != MessageElemTypeByte || msg[1].Type() != MessageElemTypeInt {
		return nil, fmt.Errorf("invalid stat update header")
	}
	count := int(msg[1].Data().(uint32))
	if count < 0 || len(msg) < 2+count*2 {
		return nil, fmt.Errorf("stat update count %d exceeds message length %d", count, len(msg))
	}
	entries := make([]StatUpdateEntry, 0, count)
	for index := 0; index < count; index++ {
		base := 2 + index*2
		if msg[base].Type() != MessageElemTypeInt {
			return nil, fmt.Errorf("stat update id %d has type %d", index, msg[base].Type())
		}
		value, ok := messageNumericValue(msg[base+1])
		if !ok {
			return nil, fmt.Errorf("stat update value %d has type %d", index, msg[base+1].Type())
		}
		entries = append(entries, StatUpdateEntry{
			StatId: msg[base].Data().(uint32),
			Value:  value,
		})
	}
	return entries, nil
}

func messageNumericValue(value IMessageElem) (float64, bool) {
	switch value.Type() {
	case MessageElemTypeByte:
		return float64(value.Data().(uint8)), true
	case MessageElemTypeShort:
		return float64(value.Data().(uint16)), true
	case MessageElemTypeInt:
		return float64(value.Data().(uint32)), true
	case MessageElemTypeLong:
		return float64(value.Data().(uint64)), true
	case MessageElemTypeFloat:
		return float64(value.Data().(float32)), true
	default:
		return 0, false
	}
}
