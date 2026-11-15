//go:build windows

package packet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
)

func TestLiveCaptureReconnectRetainsEverySegment(t *testing.T) {
	if os.Getenv("DILMETER_TEST_LIVE_CAPTURE") != "1" {
		t.Skip("set DILMETER_TEST_LIVE_CAPTURE=1 to exercise the installed Npcap driver")
	}
	devices, err := pcap.FindAllDevs()
	if err != nil {
		t.Fatal(err)
	}
	nic := ""
	for _, device := range devices {
		if strings.Contains(strings.ToLower(device.Name), "loopback") {
			nic = device.Name
			break
		}
	}
	if nic == "" {
		t.Fatal("Npcap loopback interface unavailable")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	reader, err := NewGameServerPacketReader(&GameServerPacketReaderOpt{Ctx: ctx, LogDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	filter := fmt.Sprintf("tcp and src host 127.0.0.1 and src port %d and dst port %d",
		listener.Addr().(*net.TCPAddr).Port, client.LocalAddr().(*net.TCPAddr).Port)
	for i := byte(0); i < 3; i++ {
		if err := reader.OpenNicWithFilter(nic, filter); err != nil {
			t.Fatal(err)
		}
		frame := []byte{0xaa, 8, 0, 0, 0, 1, i, 0x34}
		if _, err := server.Write(frame); err != nil {
			t.Fatal(err)
		}
		select {
		case p := <-reader.PacketCh():
			if !p.IsShortPacket || !bytes.Equal(p.ShortBody, frame[6:]) {
				t.Fatalf("wrong packet after reconnect %d: %+v", i, p)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no live parsed packet after reconnect %d", i)
		}
		reader.CloseNic()
	}
	if reader.LivePayloadCount() < 3 {
		t.Fatalf("live activity missing: %d", reader.LivePayloadCount())
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.pcapng"))
	if err != nil || len(files) != 3 {
		t.Fatalf("live reconnect logs lost: %v %v", files, err)
	}
	found := make(map[byte]bool)
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		capture, err := pcapgo.NewNgReader(file, pcapgo.DefaultNgReaderOptions)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		for {
			data, _, readErr := capture.ReadPacketData()
			if readErr != nil {
				if readErr != io.EOF {
					t.Error(readErr)
				}
				break
			}
			for i := byte(0); i < 3; i++ {
				if bytes.Contains(data, []byte{0xaa, 8, 0, 0, 0, 1, i, 0x34}) {
					found[i] = true
				}
			}
		}
		file.Close()
	}
	if len(found) != 3 {
		t.Fatalf("saved live captures missing data: %v", found)
	}
	t.Log("three live Npcap sessions received, parsed and retained all test packets")
}
