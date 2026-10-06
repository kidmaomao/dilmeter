package packet

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestGamePacketParserResetsAcrossCaptureGap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r := &GameServerPacketReader{ctx: ctx, packetCh: make(chan *GamePacket, 1)}
	payloads := make(chan gamePacketPayload, 2)
	// An incomplete 100-byte game message must not consume a later message
	// after the TCP recovery buffer explicitly reports missing bytes.
	payloads <- gamePacketPayload{data: []byte{0xaa, 100, 0, 0, 0, 1}, connectionEpoch: 1}
	want := []byte{0xaa, 8, 0, 0, 0, 1, 0x12, 0x34}
	payloads <- gamePacketPayload{data: want, connectionEpoch: 1, gapBefore: true}
	done := make(chan struct{})
	go func() { defer close(done); r.packetLoop(payloads) }()
	defer func() { cancel(); <-done }()
	select {
	case p := <-r.packetCh:
		if !bytes.Equal(p.RawPacket, want) || p.ConnectionEpoch != 1 {
			t.Fatalf("partial pre-gap message contaminated recovery: %+v", p)
		}
	case <-ctx.Done():
		t.Fatal("parser stalled on the pre-gap incomplete message")
	}
}
