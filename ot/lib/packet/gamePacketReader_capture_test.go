package packet

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
)

func TestPcapInitialFourByteSegmentIsNotDiscarded(t *testing.T) {
	// Capturing an established connection may start at an ordinary game header
	// split across TCP segments. A four-byte segment alone is not a key packet.
	want := []byte{0xaa, 8, 0, 0, 0, 1, 0x12, 0x34}
	path := filepath.Join(t.TempDir(), "split-header.pcap")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := pcapgo.NewWriter(f)
	if err := w.WriteFileHeader(65535, layers.LinkTypeRaw); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolTCP,
			SrcIP: net.IPv4(198, 51, 100, 1), DstIP: net.IPv4(192, 0, 2, 1)}
		tcp := &layers.TCP{SrcPort: 11020, DstPort: 40000, Seq: uint32(100 + i*4), ACK: true}
		if err := tcp.SetNetworkLayerForChecksum(ip); err != nil {
			t.Fatal(err)
		}
		buf := gopacket.NewSerializeBuffer()
		if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, tcp, gopacket.Payload(want[i*4:(i+1)*4])); err != nil {
			t.Fatal(err)
		}
		data := buf.Bytes()
		if err := w.WritePacket(gopacket.CaptureInfo{Timestamp: time.Unix(100, int64(i)), CaptureLength: len(data), Length: len(data)}, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	handle, err := pcap.OpenOffline(path)
	if err != nil {
		// This integration test uses the native capture driver. Hosted Windows
		// runners do not ship Npcap; driver-independent stream tests still run.
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "couldn't load wpcap.dll") {
			t.Skipf("Npcap is not installed: %v", err)
		}
		t.Fatal(err)
	}
	defer handle.Close()
	r := &GameServerPacketReader{ctx: context.Background(), clientIpCh: make(chan string, 1), linkType: layers.LinkTypeRaw}
	out := make(chan gamePacketPayload, 3)
	r.readPacketLoop(handle, out, 1)
	var got []byte
	for p := range out {
		got = append(got, p.data...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ordinary initial segment discarded: got %x want %x", got, want)
	}
}
