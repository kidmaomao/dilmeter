package packet

import (
	"bytes"
	"math/rand"
	"testing"
	"time"
)

func TestTCPStreamReordersNestedGapsAndRetransmissions(t *testing.T) {
	b := tcpStreamBuffer{}
	var got []byte
	push := func(seq uint32, s string) {
		for _, p := range b.push(seq, []byte(s), time.Time{}) {
			if p.gapBefore {
				t.Fatal("unexpected gap")
			}
			got = append(got, p.data...)
		}
	}
	push(100, "ab")
	push(108, "ij") // A later sequence arrives first, formerly blocking the FIFO.
	push(106, "gh")
	push(106, "gh")
	push(104, "ef")
	push(101, "bcde") // Partial retransmission closes the gap and must drain.
	push(100, "abcdefghij")
	if string(got) != "abcdefghij" || len(b.pending) != 0 {
		t.Fatalf("got %q pending=%d", got, len(b.pending))
	}
}

func TestTCPStreamRandomizedSegmentsPreserveEveryByte(t *testing.T) {
	// Compare reconstructed data with an independent known byte stream, across
	// varied segmentation, permutations, duplicate and partially overlapping
	// retransmissions. Half of the cases also wrap the uint32 TCP sequence.
	for seed := int64(0); seed < 1000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		want := make([]byte, 4096)
		_, _ = rng.Read(want)
		base := rng.Uint32()
		if seed%2 == 0 {
			base = ^uint32(0) - 2048
		}
		type segment struct{ start, end int }
		var segments []segment
		for start := 1; start < len(want); {
			end := min(len(want), start+1+rng.Intn(128))
			segments = append(segments, segment{start, end})
			start = end
		}
		for i := 0; i < 160; i++ {
			start := rng.Intn(len(want))
			segments = append(segments, segment{start, min(len(want), start+1+rng.Intn(256))})
		}
		rng.Shuffle(len(segments), func(i, j int) { segments[i], segments[j] = segments[j], segments[i] })
		b := tcpStreamBuffer{}
		var got []byte
		check := func(parts []gamePacketPayload) {
			for _, p := range parts {
				if p.gapBefore || p.relSeq != base+uint32(len(got)) {
					t.Fatalf("seed %d emitted a gap or duplicate at byte %d", seed, len(got))
				}
				got = append(got, p.data...)
			}
		}
		check(b.push(base, want[:1], time.Time{}))
		for _, s := range segments {
			check(b.push(base+uint32(s.start), want[s.start:s.end], time.Time{}))
		}
		if len(b.pending) != 0 || !bytes.Equal(got, want) {
			t.Fatalf("seed %d reconstructed %d/%d bytes, pending=%d", seed, len(got), len(want), len(b.pending))
		}
	}
}

func TestTCPStreamInOrderHasNoQueueDelay(t *testing.T) {
	b := tcpStreamBuffer{}
	for i := 0; i < 10_000; i++ {
		data := []byte{byte(i), byte(i >> 8)}
		parts := b.push(uint32(i*2), data, time.Time{})
		if len(parts) != 1 || !bytes.Equal(parts[0].data, data) || len(b.pending) != 0 {
			t.Fatalf("in-order segment %d was delayed or changed", i)
		}
	}
}

func TestTCPStreamOwnsQueuedPayloadAndWrapsSequence(t *testing.T) {
	b := tcpStreamBuffer{}
	b.push(0xfffffffc, []byte("ab"), time.Time{})
	data := []byte("ef")
	b.push(0, data, time.Time{})
	data[0] = 'x'
	parts := b.push(0xfffffffe, []byte("cd"), time.Time{})
	var got []byte
	for _, p := range parts {
		got = append(got, p.data...)
	}
	if string(got) != "cdef" {
		t.Fatalf("got %q", got)
	}
	if len(b.push(0xfffffffc, []byte("abcdef"), time.Time{})) != 0 {
		t.Fatal("duplicate emitted across wrap")
	}
}

func TestTCPStreamMoreThanOldQueueLimit(t *testing.T) {
	b := tcpStreamBuffer{}
	b.push(0, []byte{0}, time.Time{})
	for i := 300; i >= 2; i-- {
		if len(b.push(uint32(i), []byte{byte(i)}, time.Time{})) != 0 {
			t.Fatal("out of order bytes emitted")
		}
	}
	parts := b.push(1, []byte{1}, time.Time{})
	var got []byte
	for _, p := range parts {
		if p.gapBefore {
			t.Fatal("unexpected gap")
		}
		got = append(got, p.data...)
	}
	want := make([]byte, 300)
	for i := range want {
		want[i] = byte(i + 1)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %d bytes", len(got))
	}
}

func TestTCPStreamRecoveryAndEOFMarkMissingBytes(t *testing.T) {
	b := tcpStreamBuffer{}
	b.push(0, []byte("a"), time.Time{})
	var recovered []gamePacketPayload
	for i := 2; i <= maxPendingTCPSegments+2; i++ {
		recovered = append(recovered, b.push(uint32(i), []byte("b"), time.Time{})...)
	}
	if len(recovered) != maxPendingTCPSegments+1 || !recovered[0].gapBefore {
		t.Fatalf("unsafe recovery: %d parts", len(recovered))
	}
	b.push(uint32(maxPendingTCPSegments+10), []byte("z"), time.Time{})
	parts := b.finish()
	if len(parts) != 1 || !parts[0].gapBefore || string(parts[0].data) != "z" {
		t.Fatalf("unsafe EOF: %+v", parts)
	}
}
