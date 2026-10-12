package packet

import (
	"context"
	"testing"
	"time"
)

func TestObsReceiveTimeSurvivesTCPReorderAndParserQueue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := &GameServerPacketReader{ctx: ctx, packetCh: make(chan *GamePacket, 3)}
	ch := make(chan gamePacketPayload, 4)
	stream := tcpStreamBuffer{}
	data := []byte{0xaa, 8, 0, 0, 0, 1, 0x12, 0x34}
	base := time.Now()
	capture := base.Add(time.Hour)
	// The second packet is observed before the gap closes. Retain that receive
	// time even though it waits in the reorder queue and is parsed later.
	parts := stream.pushObserved(100, data, capture, base)
	parts = append(parts, stream.pushObserved(116, data, capture.Add(time.Second), base.Add(time.Millisecond))...)
	parts = append(parts, stream.pushObserved(108, data, capture.Add(2*time.Second), base.Add(2*time.Millisecond))...)
	for _, p := range parts {
		p.connectionEpoch = 1
		p.serverScope = "test"
		ch <- p
	}
	done := make(chan struct{})
	go func() { defer close(done); r.packetLoop(ch) }()
	defer func() { cancel(); <-done }()
	for _, want := range []time.Time{base, base.Add(2 * time.Millisecond), base.Add(time.Millisecond)} {
		select {
		case p := <-r.packetCh:
			if !p.ObservedAt.Equal(want) || p.At.Sub(p.ObservedAt) < 59*time.Minute {
				t.Fatalf("receive/capture clock mixed: %+v", p)
			}
		case <-ctx.Done():
			t.Fatal("missing parsed packet")
		}
	}
}
