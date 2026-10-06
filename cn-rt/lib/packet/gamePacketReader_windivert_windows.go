//go:build windows && dilmeter_rt

package packet

import (
	"fmt"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"gitlab.com/prilus/mabidilmeter/lib/util"
	"gitlab.com/prilus/mabidilmeter/lib/windivert"
)

const winDivertPacketBufferSize = 0xFFFF

// OpenWinDivert opens the experimental WFP capture backend. The filter uses
// WinDivert syntax, not libpcap/BPF syntax.
func (t *GameServerPacketReader) OpenWinDivert(dllPath string, filter string) error {
	if t.handle != nil || t.driverHandle != nil {
		return fmt.Errorf("capture source already opened")
	}

	logger.Printf("OpenWinDivert dll=%s filter=%s", dllPath, filter)
	handle, err := windivert.Open(dllPath, filter)
	if err != nil {
		return err
	}
	t.driverHandle = handle
	t.linkType = layers.LinkTypeIPv4

	if !t.disableCaptureLog {
		if err := t.openLog(); err != nil {
			logger.Println("openLog failed", err)
		}
	}

	payloadCh := make(chan gamePacketPayload, pcapQueueSize)
	epoch := t.connectionEpoch.Add(1)
	go t.readWinDivertPacketLoop(handle, payloadCh, epoch)
	go t.forwardPayloads(payloadCh)
	return nil
}

type winDivertReceiver interface {
	Recv([]byte) (int, error)
}

func (t *GameServerPacketReader) readWinDivertPacketLoop(handle winDivertReceiver, ch chan<- gamePacketPayload, connectionEpoch uint64) {
	defer close(ch)

	ip4 := layers.IPv4{}
	tcp := layers.TCP{}
	payload := gopacket.Payload{}
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4, &tcp, &payload)
	decodedLayers := make([]gopacket.LayerType, 0, 4)
	packetBuffer := make([]byte, winDivertPacketBufferSize)

	baseSeq := uint32(0)
	stream := tcpStreamBuffer{}
	flow := ""
	clientIPSeen := make(map[string]struct{})

	forward := func(parts []gamePacketPayload) {
		for _, p := range parts {
			p.relSeq -= baseSeq
			p.connectionEpoch = connectionEpoch
			ch <- p
		}
	}

	logger.Println("WinDivert receive loop started")
	for i := 0; t.ctx.Err() == nil; i++ {
		n, err := handle.Recv(packetBuffer)
		if err != nil {
			if t.ctx.Err() == nil {
				logger.Println("WinDivert receive stopped:", err)
			}
			break
		}
		if n < 1 {
			continue
		}

		packetData := packetBuffer[:n]
		capturedAt := time.Now()
		captureInfo := gopacket.CaptureInfo{
			Timestamp:     capturedAt,
			CaptureLength: n,
			Length:        n,
		}
		if t.logHandle != nil {
			_ = t.logHandle.WritePacket(captureInfo, packetData)
		}

		decodedLayers = decodedLayers[:0]
		if err := parser.DecodeLayers(packetData, &decodedLayers); err != nil {
			logger.Println("WinDivert decode failed:", err)
			continue
		}

		hasTCP := false
		for _, layerType := range decodedLayers {
			if layerType == layers.LayerTypeTCP {
				hasTCP = true
				break
			}
		}
		if !hasTCP || len(tcp.Payload) < 1 {
			continue
		}

		dstIP := ip4.DstIP.String()
		if _, seen := clientIPSeen[dstIP]; !seen {
			clientIPSeen[dstIP] = struct{}{}
			util.TrySend(t.clientIpCh, dstIP)
			logger.Printf("WinDivert detected client ip %v", dstIP)
		}

		packetFlow := fmt.Sprintf("%s:%d>%s:%d", ip4.SrcIP, tcp.SrcPort, ip4.DstIP, tcp.DstPort)
		if flow != packetFlow {
			switchingConnection := flow != ""
			if flow != "" {
				connectionEpoch = t.connectionEpoch.Add(1)
			}
			flow = packetFlow
			stream = tcpStreamBuffer{}
			baseSeq = tcp.Seq
			if switchingConnection && len(tcp.Payload) == 4 {
				stream.initialized = true
				stream.next = tcp.Seq + 4
				continue
			}
		}
		forward(stream.push(tcp.Seq, tcp.Payload, capturedAt))
	}

	// Preserve real gap boundaries when the capture source stops.
	forward(stream.finish())
	logger.Println("WinDivert receive loop exited")
}
