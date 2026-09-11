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

func (t *GameServerPacketReader) readWinDivertPacketLoop(handle *windivert.Handle, ch chan<- gamePacketPayload, connectionEpoch uint64) {
	defer close(ch)

	ip4 := layers.IPv4{}
	tcp := layers.TCP{}
	payload := gopacket.Payload{}
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4, &tcp, &payload)
	decodedLayers := make([]gopacket.LayerType, 0, 4)
	packetBuffer := make([]byte, winDivertPacketBufferSize)

	baseSeq := uint32(0)
	nextSeq, prevDstPort := uint32(0), layers.TCPPort(0)
	pendingTcpLayers := make([]pendingTcpLayer, 0, packetQueueSize)
	clientIPSeen := make(map[string]struct{})

	emit := func(seq uint32, data []byte, at time.Time) {
		dataCopy := append([]byte(nil), data...)
		ch <- gamePacketPayload{
			relSeq:          seq - baseSeq,
			data:            dataCopy,
			at:              at,
			connectionEpoch: connectionEpoch,
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

		if i == 0 {
			baseSeq = tcp.Seq
		}

		dstIP := ip4.DstIP.String()
		if _, seen := clientIPSeen[dstIP]; !seen {
			clientIPSeen[dstIP] = struct{}{}
			util.TrySend(t.clientIpCh, dstIP)
			logger.Printf("WinDivert detected client ip %v", dstIP)
		}

		if nextSeq != 0 && tcp.Seq != nextSeq {
			if prevDstPort != tcp.DstPort {
				connectionEpoch = t.connectionEpoch.Add(1)
				pendingTcpLayers = pendingTcpLayers[:0]
				prevDstPort = tcp.DstPort
				baseSeq = tcp.Seq
				nextSeq = tcp.Seq + uint32(len(tcp.Payload))
				if len(tcp.Payload) == 4 {
					continue
				}
				emit(tcp.Seq, tcp.Payload, capturedAt)
				continue
			}

			if tcp.Seq < nextSeq {
				if tcp.Seq+uint32(len(tcp.Payload)) > nextSeq {
					newData := tcp.Payload[nextSeq-tcp.Seq:]
					if len(newData) > 0 {
						emit(nextSeq, newData, capturedAt)
					}
					nextSeq = tcp.Seq + uint32(len(tcp.Payload))
				}
				continue
			}

			logger.Println("WinDivert packet out of order", i, nextSeq, tcp.Seq)
			if len(pendingTcpLayers) >= packetQueueSize {
				for _, pending := range pendingTcpLayers {
					emit(pending.tcpLayer.Seq, pending.tcpLayer.Payload, pending.ci.Timestamp)
				}
				pendingTcpLayers = pendingTcpLayers[:0]
				emit(tcp.Seq, tcp.Payload, capturedAt)
				nextSeq = tcp.Seq + uint32(len(tcp.Payload))
				continue
			}

			payloadCopy := append([]byte(nil), tcp.Payload...)
			tcpCopy := tcp
			tcpCopy.Payload = payloadCopy
			pendingTcpLayers = append(pendingTcpLayers, pendingTcpLayer{
				tcpLayer: tcpCopy,
				ci:       captureInfo,
			})
			continue
		}

		emit(tcp.Seq, tcp.Payload, capturedAt)
		nextSeq = tcp.Seq + uint32(len(tcp.Payload))
		prevDstPort = tcp.DstPort

		for len(pendingTcpLayers) > 0 {
			pending := pendingTcpLayers[0]
			if pending.tcpLayer.Seq == nextSeq {
				emit(pending.tcpLayer.Seq, pending.tcpLayer.Payload, pending.ci.Timestamp)
				nextSeq = pending.tcpLayer.Seq + uint32(len(pending.tcpLayer.Payload))
				pendingTcpLayers = pendingTcpLayers[1:]
				continue
			}
			if pending.tcpLayer.Seq < nextSeq {
				if pending.tcpLayer.Seq+uint32(len(pending.tcpLayer.Payload)) <= nextSeq {
					pendingTcpLayers = pendingTcpLayers[1:]
					continue
				}
				newData := pending.tcpLayer.Payload[nextSeq-pending.tcpLayer.Seq:]
				if len(newData) > 0 {
					emit(nextSeq, newData, pending.ci.Timestamp)
				}
				nextSeq = pending.tcpLayer.Seq + uint32(len(pending.tcpLayer.Payload))
				pendingTcpLayers = pendingTcpLayers[1:]
				continue
			}
			break
		}
	}

	for _, pending := range pendingTcpLayers {
		emit(pending.tcpLayer.Seq, pending.tcpLayer.Payload, pending.ci.Timestamp)
	}
	logger.Println("WinDivert receive loop exited")
}
