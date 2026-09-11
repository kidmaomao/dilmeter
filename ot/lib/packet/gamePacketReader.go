package packet

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/util"
)

type GameServerPacketReader struct {
	// non-mutable
	ctx             context.Context
	packetCh        chan *GamePacket
	clientIpCh      chan string
	masterPayloadCh chan gamePacketPayload
	connectionEpoch atomic.Uint64

	// mutable (set on OpenNic/OpenFile)
	handle            *pcap.Handle
	driverHandle      io.Closer
	fd                *os.File
	logHandle         *pcapgo.NgWriter
	logFd             *os.File
	linkType          layers.LinkType
	logDir            string
	disableCaptureLog bool

	// statistics
	payloadCount    uint64
	parsedCount     uint64
	parseErrorCount uint64
	lastErrorTime   time.Time
}

type GameServerPacketReaderOpt struct {
	Ctx               context.Context
	LogDir            string
	DisableCaptureLog bool
}

type gamePacketPayload struct {
	relSeq          uint32
	data            []byte
	at              time.Time
	connectionEpoch uint64
}

// pendingTcpLayer buffers a TCP segment that arrived out of order.
type pendingTcpLayer struct {
	tcpLayer layers.TCP
	ci       gopacket.CaptureInfo
}

const pcapQueueSize = 100
const pcapBufferSize = 32 * 1024 * 1024
const pcapPromisc = true
const packetQueueSize = 100

var ErrTooShortPacket = errors.New("too short packet")

func NewGameServerPacketReader(opt *GameServerPacketReaderOpt) (*GameServerPacketReader, error) {
	if opt == nil {
		return nil, errors.New("opt is nil")
	}

	v := &GameServerPacketReader{
		ctx:               opt.Ctx,
		packetCh:          make(chan *GamePacket, packetQueueSize),
		clientIpCh:        make(chan string, 100),
		masterPayloadCh:   make(chan gamePacketPayload, pcapQueueSize),
		logDir:            opt.LogDir,
		disableCaptureLog: opt.DisableCaptureLog,
	}
	if v.logDir == "" {
		v.logDir = "logs"
	}

	go v.packetLoop(v.masterPayloadCh)

	return v, nil
}

// OpenNic opens the specified NIC for live capture.
func (t *GameServerPacketReader) OpenNic(name string) error {
	return t.OpenNicWithFilter(name, constants.GameServerFilter())
}

// OpenNicWithFilter opens a capture device with an explicit filter. It is used
// by connection discovery so several candidate flows can be tested in parallel
// without changing the process-wide active connection filter.
func (t *GameServerPacketReader) OpenNicWithFilter(name string, filter string) error {
	return t.openNicWithFilter(name, filter, pcap.BlockForever)
}

// OpenNicWithFilterTimeout is intended for short-lived connection probes.
// A finite read timeout lets the packet loop observe context cancellation even
// when a virtual adapter does not deliver any packets.
func (t *GameServerPacketReader) OpenNicWithFilterTimeout(name string, filter string, timeout time.Duration) error {
	return t.openNicWithFilter(name, filter, timeout)
}

func (t *GameServerPacketReader) openNicWithFilter(name string, filter string, timeout time.Duration) error {
	if t.handle != nil || t.driverHandle != nil {
		return fmt.Errorf("nic already opened")
	}

	logger.Printf("OpenNic %s filter=%s timeout=%v", name, filter, timeout)

	handle, err := pcap.OpenLive(name, pcapBufferSize, pcapPromisc, timeout)
	if err != nil {
		logger.Println(err)
		return err
	}
	t.handle = handle
	t.linkType = handle.LinkType()
	logger.Printf("Interface %s LinkType: %v", name, t.linkType)

	if err := handle.SetBPFFilter(filter); err != nil {
		handle.Close()
		t.handle = nil
		return err
	}

	if !t.disableCaptureLog {
		if err := t.openLog(); err != nil {
			logger.Println("openLog failed", err)
		}
	}

	payloadCh := make(chan gamePacketPayload, pcapQueueSize)
	epoch := t.connectionEpoch.Add(1)
	go t.readPacketLoop(handle, payloadCh, epoch)
	go t.forwardPayloads(payloadCh)

	return nil
}

// CloseNic stops live capture and closes the handle.
func (t *GameServerPacketReader) CloseNic() {
	if t.handle != nil {
		if stats, err := t.handle.Stats(); err == nil {
			logger.Printf("[Close Pcap Stats] Received: %d, Dropped: %d, InterfaceDropped: %d",
				stats.PacketsReceived, stats.PacketsDropped, stats.PacketsIfDropped)
		}
		t.handle.Close()
		t.handle = nil
	}
	if t.driverHandle != nil {
		if err := t.driverHandle.Close(); err != nil {
			logger.Println("close driver capture failed:", err)
		}
		t.driverHandle = nil
	}
	if t.logHandle != nil {
		t.logHandle.Flush()
		t.logHandle = nil
	}
	if t.logFd != nil {
		t.logFd.Close()
		t.logFd = nil
	}
}

// SetClientIp updates the BPF filter to restrict capture to a specific client IP.
func (t *GameServerPacketReader) SetClientIp(ip string) {
	if t.handle == nil || ip == "" {
		return
	}
	filter := constants.GameServerFilter() + " and dst host " + ip
	if err := t.handle.SetBPFFilter(filter); err != nil {
		logger.Println("SetClientIp SetBPFFilter failed:", err)
	}
}

// OpenFile opens a pcapng file for replay.
func (t *GameServerPacketReader) OpenFile(file string) error {
	if t.handle != nil || t.driverHandle != nil {
		return fmt.Errorf("nic already opened")
	}

	fd, err := os.OpenFile(file, os.O_RDONLY, 0644)
	if err != nil {
		logger.Println(err)
		return err
	}
	t.fd = fd

	// libpcap uses multibyte fopen for file names, causing issues with non-ASCII paths.
	handle, err := pcap.OpenOfflineFile(fd)
	if err != nil {
		logger.Println(err)
		return err
	}

	filter := constants.GameServerFilter()
	if err := handle.SetBPFFilter(filter); err != nil {
		logger.Println(err)
	}

	t.handle = handle
	t.linkType = handle.LinkType()
	logger.Printf("File LinkType: %v", t.linkType)

	payloadCh := make(chan gamePacketPayload, pcapQueueSize)
	epoch := t.connectionEpoch.Add(1)
	go t.forwardPayloads(payloadCh)

	// Delay replay 20 seconds so a WebSocket client has time to connect.
	time.AfterFunc(2*time.Second, func() {
		logger.Println("start readPacketLoop", file)
		go t.readPacketLoop(handle, payloadCh, epoch)
	})

	return nil
}

// OpenStdin opens stdin as a pcap source.
func (t *GameServerPacketReader) OpenStdin() error {
	if t.handle != nil || t.driverHandle != nil {
		return fmt.Errorf("nic already opened")
	}

	handle, err := pcap.OpenOfflineFile(os.Stdin)
	if err != nil {
		logger.Println(err)
		return err
	}

	filter := constants.GameServerFilter()
	if err := handle.SetBPFFilter(filter); err != nil {
		logger.Println(err)
	}

	t.handle = handle
	t.linkType = handle.LinkType()

	payloadCh := make(chan gamePacketPayload, pcapQueueSize)
	epoch := t.connectionEpoch.Add(1)
	go t.readPacketLoop(handle, payloadCh, epoch)
	go t.forwardPayloads(payloadCh)

	return nil
}

// forwardPayloads drains payloadCh into masterPayloadCh.
// Exits when payloadCh is closed (i.e., when readPacketLoop ends).
func (t *GameServerPacketReader) forwardPayloads(payloadCh <-chan gamePacketPayload) {
	for p := range payloadCh {
		t.masterPayloadCh <- p
	}
}

func (t *GameServerPacketReader) openLog() error {
	logDir := t.logDir
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		logger.Println(err)
		return err
	}
	fileName := filepath.Join(logDir, fmt.Sprintf("packet_capture_%v.pcapng", constants.SERVER_START_AT))
	fd, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logger.Println(err)
		return err
	}
	t.logFd = fd

	linkType := t.linkType
	if linkType == 0 {
		linkType = layers.LinkTypeNull
	}

	handle, err := pcapgo.NewNgWriter(fd, linkType)
	if err != nil {
		logger.Println(err)
		return err
	}
	t.logHandle = handle
	logger.Printf("pcapng writer initialized with LinkType: %v", linkType)
	return nil
}

func (t *GameServerPacketReader) packetLoop(payloadCh <-chan gamePacketPayload) {
	buffer := bytes.NewBuffer(nil)
	lastRelSeq, lastAt := uint32(0), time.Now()
	payloads := make([]gamePacketPayload, 0, packetQueueSize)
	currentEpoch := uint64(0)

	skipPayload := func(n int) {
		for n > 0 && len(payloads) > 0 {
			if n < len(payloads[0].data) {
				lastRelSeq, lastAt = payloads[0].relSeq, payloads[0].at
				payloads[0].data = payloads[0].data[n:]
				return
			}
			n -= len(payloads[0].data)
			lastRelSeq, lastAt = payloads[0].relSeq, payloads[0].at
			payloads = payloads[1:]
		}
	}

	nextPayload := func() {
		buffer.Reset()
		if len(payloads) < 1 {
			return
		}
		payloads = payloads[1:]
		if len(payloads) < 1 {
			return
		}
		for _, v := range payloads {
			buffer.Write(v.data)
		}
		lastRelSeq, lastAt = payloads[0].relSeq, payloads[0].at
	}

	pushPayload := func(payloadData gamePacketPayload) {
		if buffer.Len() < 1 {
			buffer.Reset()
		}
		if len(payloads) < 1 {
			lastRelSeq, lastAt = payloadData.relSeq, payloadData.at
		}
		payloads = append(payloads, payloadData)
		buffer.Write(payloadData.data)
	}

	for {
		select {
		case <-t.ctx.Done():
			logger.Printf("[Stats] Payloads: %d, Parsed: %d, Errors: %d",
				t.payloadCount, t.parsedCount, t.parseErrorCount)
			return

		case payloadData := <-payloadCh:
			// A capture reconnect or channel switch starts an independent TCP
			// byte stream. Never append it to an incomplete packet from the old
			// connection, and discard any old payload that arrives late while its
			// reader goroutine is winding down.
			if payloadData.connectionEpoch < currentEpoch {
				continue
			}
			if payloadData.connectionEpoch > currentEpoch {
				currentEpoch = payloadData.connectionEpoch
				buffer.Reset()
				payloads = payloads[:0]
				lastRelSeq = 0
				lastAt = payloadData.at
			}
			t.payloadCount++
			pushPayload(payloadData)
		}

	readerLoop:
		for {
			msg, consumed, err := parseGamePacket(buffer.Bytes(), lastAt)
			if err != nil {
				if err == io.EOF {
					break readerLoop
				}
				t.parseErrorCount++
				t.lastErrorTime = time.Now()
				logger.Printf("[ParseError #%d] relSeq=%d %v", t.parseErrorCount, lastRelSeq, err)
				nextPayload()
				continue
			}

			if msg != nil {
				buffer.Next(consumed)
				t.parsedCount++
				msg.ConnectionEpoch = currentEpoch
				t.packetCh <- msg
				skipPayload(consumed)
			}
		}
	}
}

func (t *GameServerPacketReader) readPacketLoop(handle *pcap.Handle, ch chan<- gamePacketPayload, connectionEpoch uint64) {
	defer close(ch)

	eth := layers.Ethernet{}
	ip4 := layers.IPv4{}
	tcp := layers.TCP{}
	payload := gopacket.Payload{}

	var layerParser *gopacket.DecodingLayerParser
	skipAddressFamilyHeader := false
	switch t.linkType {
	case layers.LinkTypeNull, layers.LinkTypeLoop:
		// Loopback: skip 4-byte AF header manually; parse from IPv4 directly.
		layerParser = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4, &tcp, &payload)
		skipAddressFamilyHeader = true
	case layers.LinkTypeRaw, layers.LinkTypeIPv4:
		// Layer-3 tunnel adapters may expose a raw IPv4 packet without an
		// Ethernet header. This is common for route-mode accelerator adapters.
		layerParser = gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4, &tcp, &payload)
	default:
		layerParser = gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &eth, &ip4, &tcp, &payload)
	}
	packetLayers := []gopacket.LayerType(nil)

	baseSeq := uint32(0)
	nextSeq, prevDstPort := uint32(0), layers.TCPPort(0)
	pendingTcpLayers := make([]pendingTcpLayer, 0, packetQueueSize)
	clientIPSeen := make(map[string]struct{})

	lastDropped := 0
	for i := 0; t.ctx.Err() == nil; i++ {
		b, ci, err := handle.ReadPacketData()
		if err != nil {
			if err == pcap.NextErrorTimeoutExpired {
				continue
			}
			logger.Println(err, i)
			break
		}

		if t.logHandle != nil {
			_ = t.logHandle.WritePacket(ci, b)
		}

		if i%100 == 0 {
			if stats, err := handle.Stats(); err == nil {
				if stats.PacketsDropped > lastDropped {
					logger.Printf("[pcap] received=%d dropped=%d ifdropped=%d (+%d new drops)",
						stats.PacketsReceived, stats.PacketsDropped, stats.PacketsIfDropped,
						stats.PacketsDropped-lastDropped)
					lastDropped = stats.PacketsDropped
				}
			}
		}

		// Loopback: skip 4-byte address-family header.
		packetData := b
		if skipAddressFamilyHeader {
			if len(b) > 4 {
				packetData = b[4:]
			} else {
				continue
			}
		}

		if err := layerParser.DecodeLayers(packetData, &packetLayers); err != nil {
			logger.Println(err)
			continue
		}

		if i == 0 {
			baseSeq = tcp.Seq
		}

		for _, layer := range packetLayers {
			if layer != layers.LayerTypeTCP || len(tcp.Payload) < 1 {
				continue
			}

			// Detect client IP (destination of server→client packets).
			dstIP := ip4.DstIP.String()
			if _, seen := clientIPSeen[dstIP]; !seen {
				clientIPSeen[dstIP] = struct{}{}
				util.TrySend(t.clientIpCh, dstIP)
				logger.Printf("detected client ip %v", dstIP)
			}

			if nextSeq != 0 && tcp.Seq != nextSeq {
				// Connection switch (different dst port = different channel/session).
				if prevDstPort != tcp.DstPort {
					connectionEpoch = t.connectionEpoch.Add(1)
					// Discard pending segments from the old connection — they are no
					// longer relevant and may contain retransmissions that would
					// produce duplicate events.
					pendingTcpLayers = pendingTcpLayers[:0]
					prevDstPort = tcp.DstPort
					baseSeq = tcp.Seq
					nextSeq = tcp.Seq + uint32(len(tcp.Payload))
					if len(tcp.Payload) == 4 {
						// Encryption key packet on a new connection; skip.
						continue
					}
					ch <- gamePacketPayload{
						relSeq:          tcp.Seq - baseSeq,
						data:            tcp.Payload,
						at:              ci.Timestamp,
						connectionEpoch: connectionEpoch,
					}
					continue
				}

				if tcp.Seq < nextSeq {
					// Retransmission or overlap.
					if tcp.Seq+uint32(len(tcp.Payload)) > nextSeq {
						// Partial overlap: only the bytes past nextSeq are new.
						p := tcp.Payload[nextSeq-tcp.Seq:]
						if len(p) > 0 {
							ch <- gamePacketPayload{
								relSeq:          nextSeq - baseSeq,
								data:            p,
								at:              ci.Timestamp,
								connectionEpoch: connectionEpoch,
							}
						}
						nextSeq = tcp.Seq + uint32(len(tcp.Payload))
					}
					// Pure retransmission (Seq+len <= nextSeq): all bytes already
					// seen — discard silently to prevent duplicate events.
					continue
				}

				// tcp.Seq > nextSeq: true out-of-order packet, buffer it.
				logger.Println("packet out of order", i, nextSeq, tcp.Seq)

				if len(pendingTcpLayers) >= packetQueueSize {
					// Buffer full: flush and reset to current packet as new base.
					for _, v := range pendingTcpLayers {
						ch <- gamePacketPayload{
							relSeq:          v.tcpLayer.Seq - baseSeq,
							data:            v.tcpLayer.Payload,
							at:              v.ci.Timestamp,
							connectionEpoch: connectionEpoch,
						}
					}
					pendingTcpLayers = pendingTcpLayers[:0]
					ch <- gamePacketPayload{
						relSeq:          tcp.Seq - baseSeq,
						data:            tcp.Payload,
						at:              ci.Timestamp,
						connectionEpoch: connectionEpoch,
					}
					nextSeq = tcp.Seq + uint32(len(tcp.Payload))
					continue
				}

				// Out of order: buffer (must copy payload since tcp is reused).
				payloadCopy := make([]byte, len(tcp.Payload))
				copy(payloadCopy, tcp.Payload)
				tcpCopy := tcp
				tcpCopy.Payload = payloadCopy
				pendingTcpLayers = append(pendingTcpLayers, pendingTcpLayer{
					tcpLayer: tcpCopy,
					ci:       ci,
				})
				continue
			}

			// In-order: forward immediately.
			ch <- gamePacketPayload{
				relSeq:          tcp.Seq - baseSeq,
				data:            tcp.Payload,
				at:              ci.Timestamp,
				connectionEpoch: connectionEpoch,
			}
			nextSeq = tcp.Seq + uint32(len(tcp.Payload))
			prevDstPort = tcp.DstPort

			// Drain buffered out-of-order segments now contiguous with nextSeq.
			for len(pendingTcpLayers) > 0 {
				v := pendingTcpLayers[0]
				if v.tcpLayer.Seq == nextSeq {
					ch <- gamePacketPayload{
						relSeq:          v.tcpLayer.Seq - baseSeq,
						data:            v.tcpLayer.Payload,
						at:              v.ci.Timestamp,
						connectionEpoch: connectionEpoch,
					}
					nextSeq = v.tcpLayer.Seq + uint32(len(v.tcpLayer.Payload))
					pendingTcpLayers = pendingTcpLayers[1:]
					continue
				}
				if v.tcpLayer.Seq < nextSeq {
					if v.tcpLayer.Seq+uint32(len(v.tcpLayer.Payload)) < nextSeq {
						pendingTcpLayers = pendingTcpLayers[1:]
						continue
					}
					p := v.tcpLayer.Payload[nextSeq-v.tcpLayer.Seq:]
					if len(p) > 0 {
						ch <- gamePacketPayload{
							relSeq:          nextSeq - baseSeq,
							data:            p,
							at:              v.ci.Timestamp,
							connectionEpoch: connectionEpoch,
						}
					}
					nextSeq = v.tcpLayer.Seq + uint32(len(v.tcpLayer.Payload))
					pendingTcpLayers = pendingTcpLayers[1:]
					continue
				}
				break
			}
		}

		if i&((1<<10)-1) == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Flush remaining pending segments.
	for _, v := range pendingTcpLayers {
		ch <- gamePacketPayload{
			relSeq:          v.tcpLayer.Seq - baseSeq,
			data:            v.tcpLayer.Payload,
			at:              v.ci.Timestamp,
			connectionEpoch: connectionEpoch,
		}
	}
}

func (t *GameServerPacketReader) Close() {
	logger.Printf("[Close Stats] Payloads: %d, Parsed: %d, Errors: %d",
		t.payloadCount, t.parsedCount, t.parseErrorCount)

	t.CloseNic()

	if t.fd != nil {
		t.fd.Close()
		t.fd = nil
	}
}

func (t *GameServerPacketReader) PacketCh() <-chan *GamePacket {
	return t.packetCh
}

func (t *GameServerPacketReader) ClientIpCh() <-chan string {
	return t.clientIpCh
}

func (t *GameServerPacketReader) GetStats() (payloads, parsed, errors uint64) {
	return t.payloadCount, t.parsedCount, t.parseErrorCount
}

// parseGamePacket attempts to parse a single game packet from data.
// Returns (packet, consumed_bytes, error).
// On io.EOF: more data needed. On other errors: malformed data (do not consume).
func parseGamePacket(data []byte, at time.Time) (*GamePacket, int, error) {
	const headerSize = 6

	if len(data) < headerSize {
		return nil, 0, io.EOF
	}

	sign := data[0]
	length := le.Uint32(data[1:5])
	flag := data[5]

	if length == 0 || length > 0x100_0000 {
		return nil, 0, fmt.Errorf("invalid packet length %v", length)
	}
	if flag > 4 {
		return nil, 0, fmt.Errorf("invalid flag %v", flag)
	}

	isShortPacket := flag == 1 || flag == 2

	if isShortPacket {
		if len(data) < int(length) {
			return nil, 0, io.EOF
		}
		if int(length) < headerSize {
			return nil, 0, fmt.Errorf("short packet length %v too small", length)
		}
		shortBody := make([]byte, int(length)-headerSize)
		copy(shortBody, data[headerSize:int(length)])
		rawPacket := make([]byte, int(length))
		copy(rawPacket, data[:int(length)])

		return &GamePacket{
			At:            at,
			Sign:          sign,
			Length:        length,
			Flag:          flag,
			IsShortPacket: true,
			ShortBody:     shortBody,
			RawPacket:     rawPacket,
		}, int(length), nil
	}

	if int(length) < headerSize+0xd {
		return nil, 0, ErrTooShortPacket
	}
	if len(data) < int(length) {
		return nil, 0, io.EOF
	}

	body := data[headerSize:int(length)]

	op := be.Uint32(body)
	body = body[4:]
	id := be.Uint64(body)
	body = body[8:]

	_, lenbytes := binary.Uvarint(body)
	if lenbytes <= 0 {
		return nil, 0, fmt.Errorf("invalid message length %v", lenbytes)
	}
	if len(body) < lenbytes {
		return nil, 0, fmt.Errorf("invalid message length %v %v", len(body), lenbytes)
	}
	body = body[lenbytes:]

	msg, err := NewMessage(bytes.NewReader(body))
	if err != nil {
		// Header may be a false positive; do NOT consume bytes.
		return nil, 0, err
	}

	rawPacket := make([]byte, int(length))
	copy(rawPacket, data[:int(length)])

	return &GamePacket{
		At:        at,
		Sign:      sign,
		Length:    length,
		Flag:      flag,
		Op:        OpCode(op),
		Id:        id,
		Msg:       msg,
		RawPacket: rawPacket,
	}, int(length), nil
}

// GamePacketBodyReader reads a packet body from r (op, id, message).
func GamePacketBodyReader(r io.Reader) (uint32, uint64, Message, error) {
	b := make([]byte, 8)

	if _, err := io.ReadFull(r, b[:4]); err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}
	op := be.Uint32(b[:4])

	if _, err := io.ReadFull(r, b[:8]); err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}
	id := be.Uint64(b[:8])

	_, lenbytes, err := util.ReadUvarint(r)
	if err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}
	_ = lenbytes

	msg, err := NewMessage(r)
	if err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	return op, id, msg, nil
}
