//go:build windows && dilmeter_rt

package packet

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

type replayWinDivertReceiver struct {
	packets [][]byte
}

func (r *replayWinDivertReceiver) Recv(dst []byte) (int, error) {
	if len(r.packets) == 0 {
		return 0, io.EOF
	}
	n := copy(dst, r.packets[0])
	r.packets = r.packets[1:]
	return n, nil
}

func testWinDivertPacket(t *testing.T, server string, seq uint32, data []byte) []byte {
	t.Helper()
	ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP,
		SrcIP: net.ParseIP(server).To4(), DstIP: net.IPv4(192, 0, 2, 1)}
	tcp := &layers.TCP{SrcPort: 11020, DstPort: 40000, Seq: seq, ACK: true}
	if err := tcp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatal(err)
	}
	b := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, tcp, gopacket.Payload(data)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func replayWinDivert(t *testing.T, packets [][]byte) []gamePacketPayload {
	t.Helper()
	r := &GameServerPacketReader{ctx: context.Background(), clientIpCh: make(chan string, 10)}
	r.connectionEpoch.Store(1)
	out := make(chan gamePacketPayload, len(packets)+1)
	r.readWinDivertPacketLoop(&replayWinDivertReceiver{packets}, out, 1)
	var parts []gamePacketPayload
	for p := range out {
		parts = append(parts, p)
	}
	return parts
}

func TestWinDivertReordersAndDeduplicatesCapture(t *testing.T) {
	// Real raw-IP decoding and reusable receive buffers, including a partial
	// retransmission that closes a gap behind the queue's former FIFO head.
	var packets [][]byte
	for _, p := range []struct {
		seq  uint32
		data string
	}{{100, "ab"}, {108, "ij"}, {106, "gh"}, {106, "gh"}, {104, "ef"}, {101, "bcde"}, {100, "abcdefghij"}} {
		packets = append(packets, testWinDivertPacket(t, "198.51.100.1", p.seq, []byte(p.data)))
	}
	var got []byte
	for _, p := range replayWinDivert(t, packets) {
		if p.gapBefore || p.connectionEpoch != 1 || int(p.relSeq) != len(got) {
			t.Fatalf("discontinuous capture output: %+v", p)
		}
		got = append(got, p.data...)
	}
	if string(got) != "abcdefghij" {
		t.Fatalf("got %q", got)
	}
}

func TestWinDivertBeyondOldQueueLimit(t *testing.T) {
	packets := [][]byte{testWinDivertPacket(t, "198.51.100.1", 0, []byte{0})}
	for i := 300; i >= 2; i-- {
		packets = append(packets, testWinDivertPacket(t, "198.51.100.1", uint32(i), []byte{byte(i)}))
	}
	packets = append(packets, testWinDivertPacket(t, "198.51.100.1", 1, []byte{1}))
	var got []byte
	for _, p := range replayWinDivert(t, packets) {
		if p.gapBefore {
			t.Fatal("complete capture was treated as missing bytes")
		}
		got = append(got, p.data...)
	}
	want := make([]byte, 301)
	for i := range want {
		want[i] = byte(i)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %d bytes, want %d in sequence order", len(got), len(want))
	}
}

func TestWinDivertConnectionChangeDiscardsOldPendingData(t *testing.T) {
	parts := replayWinDivert(t, [][]byte{
		testWinDivertPacket(t, "198.51.100.1", 100, []byte("ab")),
		testWinDivertPacket(t, "198.51.100.1", 104, []byte("old")),
		// Same ports and next expected sequence, but a different server address.
		testWinDivertPacket(t, "198.51.100.2", 102, []byte("new")),
	})
	if len(parts) != 2 || string(parts[1].data) != "new" || parts[1].relSeq != 0 || parts[1].connectionEpoch != 2 {
		t.Fatalf("old connection contaminated new capture: %+v", parts)
	}
}

func TestWinDivertInitialFourByteSegmentIsNotDiscarded(t *testing.T) {
	want := []byte{0xaa, 8, 0, 0, 0, 1, 0x12, 0x34}
	parts := replayWinDivert(t, [][]byte{
		testWinDivertPacket(t, "198.51.100.1", 100, want[:4]),
		testWinDivertPacket(t, "198.51.100.1", 104, want[4:]),
	})
	var got []byte
	for _, p := range parts {
		got = append(got, p.data...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ordinary initial segment discarded: got %x want %x", got, want)
	}
}
