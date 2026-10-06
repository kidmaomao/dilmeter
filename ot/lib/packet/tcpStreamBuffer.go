package packet

import (
	"sort"
	"time"
)

// tcpStreamBuffer emits each byte once and only in TCP sequence order. The
// capture may contain overlapping retransmissions and several reordered runs.
// Sorting only the final capture is insufficient: live capture needs the same
// ordering before the game packet parser sees any bytes.
type tcpStreamBuffer struct {
	initialized bool
	next        uint32
	pending     []gamePacketPayload
}

const maxPendingTCPSegments = 4096

func (b *tcpStreamBuffer) push(seq uint32, data []byte, at time.Time) []gamePacketPayload {
	if len(data) == 0 {
		return nil
	}
	if !b.initialized {
		b.initialized = true
		b.next = seq
	}
	// Sequence subtraction also handles a TCP sequence-number wrap.
	offset := int32(seq - b.next)
	if offset < 0 {
		skip := int64(-int64(offset))
		if skip >= int64(len(data)) {
			return nil
		}
		data = data[skip:]
		seq = b.next
	}
	// Identical retransmissions must not fill the recovery queue.
	for _, p := range b.pending {
		if p.relSeq == seq && len(p.data) >= len(data) {
			return nil
		}
	}
	owned := append([]byte(nil), data...)
	b.pending = append(b.pending, gamePacketPayload{relSeq: seq, data: owned, at: at})
	sort.SliceStable(b.pending, func(i, j int) bool { return int32(b.pending[i].relSeq-b.next) < int32(b.pending[j].relSeq-b.next) })
	output := b.drain()
	if len(b.pending) > maxPendingTCPSegments {
		// A genuinely missing segment cannot stall capture indefinitely. Recover at
		// the smallest available sequence, explicitly discarding the partial game
		// message across the gap instead of concatenating unrelated bytes.
		b.next = b.pending[0].relSeq
		recovered := b.drain()
		if len(recovered) > 0 {
			recovered[0].gapBefore = true
		}
		output = append(output, recovered...)
	}
	return output
}

func (b *tcpStreamBuffer) drain() []gamePacketPayload {
	var output []gamePacketPayload
	for len(b.pending) > 0 {
		p := b.pending[0]
		offset := int32(p.relSeq - b.next)
		if offset > 0 {
			break
		}
		b.pending = b.pending[1:]
		skip := int64(-int64(offset))
		if skip >= int64(len(p.data)) {
			continue
		}
		p.data = p.data[skip:]
		p.relSeq = b.next
		b.next += uint32(len(p.data))
		output = append(output, p)
	}
	return output
}

func (b *tcpStreamBuffer) finish() []gamePacketPayload {
	var output []gamePacketPayload
	for len(b.pending) > 0 {
		gap := b.pending[0].relSeq != b.next
		b.next = b.pending[0].relSeq
		part := b.drain()
		if len(part) > 0 {
			part[0].gapBefore = gap
		}
		output = append(output, part...)
	}
	return output
}
