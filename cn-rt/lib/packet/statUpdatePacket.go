package packet

import (
	"fmt"
	"math"
)

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

// ReadPrivateManaMaximum reads the limit of the character's default MP
// regeneration. Current CN stat updates often send just the MP modifier in
// the stat list, then send the actual limit here after an equipment change.
func ReadPrivateManaMaximum(msg Message) float64 {
	const manaRegenID, manaStatID = 2, 32
	if len(msg) < 2 || msg[0].Type() != MessageElemTypeByte || msg[0].Data().(uint8) != 3 ||
		msg[1].Type() != MessageElemTypeInt {
		return 0
	}
	stats := msg[1].Data().(uint32)
	if stats > uint32((len(msg)-2)/2) {
		return 0
	}
	added, remaining, ok := splitStatRegenList(msg[2+int(stats)*2:], 7)
	if !ok {
		return 0
	}
	_, remaining, ok = splitStatRegenList(remaining, 1)
	if !ok {
		return 0
	}
	updated, remaining, ok := splitStatRegenList(remaining, 3)
	if !ok || len(remaining) != 0 {
		return 0
	}
	maximum := 0.0
	for i := 0; i < len(added); i += 7 {
		record := added[i : i+7]
		if record[0].Type() == MessageElemTypeInt && record[0].Data().(uint32) == manaRegenID &&
			record[3].Type() == MessageElemTypeInt && record[3].Data().(uint32) == manaStatID {
			maximum = statManaRegenLimit(record[5])
		}
	}
	for i := 0; i < len(updated); i += 3 {
		record := updated[i : i+3]
		if record[0].Type() == MessageElemTypeInt && record[0].Data().(uint32) == manaRegenID &&
			record[1].Type() == MessageElemTypeFloat {
			maximum = statManaRegenLimit(record[2])
		}
	}
	return maximum
}

func splitStatRegenList(msg Message, width int) (Message, Message, bool) {
	if len(msg) == 0 || msg[0].Type() != MessageElemTypeInt {
		return nil, nil, false
	}
	count := msg[0].Data().(uint32)
	if count > uint32((len(msg)-1)/width) {
		return nil, nil, false
	}
	end := 1 + int(count)*width
	return msg[1:end], msg[end:], true
}

func statManaRegenLimit(elem IMessageElem) float64 {
	if elem.Type() != MessageElemTypeFloat {
		return 0
	}
	maximum := float64(elem.Data().(float32))
	if maximum <= 0 || math.IsNaN(maximum) || math.IsInf(maximum, 0) {
		return 0
	}
	return maximum
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
