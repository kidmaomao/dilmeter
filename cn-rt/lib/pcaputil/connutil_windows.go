//go:build windows

package pcaputil

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gopacket/gopacket/pcap"
	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"golang.org/x/sys/windows"
)

const (
	afInet                 = 2
	afUnspec               = 0
	tcpTableOwnerPIDAll    = 5
	processQueryLimitedInf = 0x1000
)

var (
	iphlpapiDLL             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapiDLL.NewProc("GetExtendedTcpTable")
	probeStateMu            sync.Mutex
	probesInFlight          = make(map[string]struct{})
	selectedFilterMu        sync.RWMutex
	selectedCaptureFilter   string
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPID  uint32
}

type mibTCPTableOwnerPID struct {
	NumEntries uint32
	Table      [1]mibTCPRowOwnerPID
}

// connectionInfo holds the details of a game server TCP connection.
type connectionInfo struct {
	ServerIP   string
	ServerPort string
	LocalPort  string
}

// connectionWithNic pairs a NIC name/friendly name with connection info.
type connectionWithNic struct {
	nicName      string
	friendlyName string
	connInfo     connectionInfo
	processName  string
	reason       string
	priority     int
}

// FindNic finds the network interface where Client.exe has TCP connections
// by scanning Client.exe TCP connections and testing each one for game packets.
func FindNic() (string, error) {
	return FindNicWithMode(false)
}

// FindNicWithMode adds a conservative accelerator fallback when enabled. It
// still prefers the direct Client.exe game-server connection. If that is not
// available, it tests Client.exe proxy/virtual-adapter connections and
// game-server connections owned by accelerator processes. A connection is only
// accepted after a valid game packet has been parsed from it.
func FindNicWithMode(acceleratorMode bool) (string, error) {
	return findNicWithOptions(acceleratorMode, false)
}

// FindNicWithDiagnosticMode enables a longer, narrowly scoped fallback on the
// already mapped game adapter. It is intentionally separate from the normal
// release path so diagnostic probing does not alter standard capture behavior.
func FindNicWithDiagnosticMode(acceleratorMode bool) (string, error) {
	return findNicWithOptions(acceleratorMode, true)
}

func findNicWithOptions(acceleratorMode bool, diagnosticMode bool) (string, error) {
	setSelectedCaptureFilter("")
	rows, err := getTCPRows()
	if err != nil {
		logger.Println("getTCPRows failed:", err)
		return "", err
	}

	devices := getCaptureDevices()
	deviceMap := getCaptureDeviceMap(devices)
	var candidates []connectionWithNic
	seen := make(map[string]struct{})

	for _, row := range rows {
		if row.State != 5 { // TCP_ESTABLISHED = 5
			continue
		}

		name, nameErr := processName(row.OwningPID)
		if nameErr != nil {
			continue
		}

		localIP := ipv4FromDWORD(row.LocalAddr)
		remoteIP := ipv4FromDWORD(row.RemoteAddr)
		if localIP == nil || remoteIP == nil || remoteIP.IsUnspecified() {
			continue
		}
		remotePort := portFromDWORD(row.RemotePort)
		localPort := portFromDWORD(row.LocalPort)
		remotePortText := fmt.Sprintf("%d", remotePort)
		isClient := strings.EqualFold(name, "Client.exe")
		isConfiguredServer := constants.MatchesConfiguredGameServer(remoteIP.String(), remotePortText)

		priority := -1
		reason := ""
		switch {
		case isClient && isConfiguredServer:
			priority = 0
			reason = "Client.exe 直连游戏服务器"
		case acceleratorMode && isClient && remoteIP.IsLoopback():
			priority = 1
			reason = "Client.exe 本地代理连接"
		case acceleratorMode && isClient && remoteIP.IsPrivate():
			priority = 2
			reason = "Client.exe 虚拟网卡连接"
		case acceleratorMode && isConfiguredServer:
			priority = 3
			reason = "加速器进程持有的游戏服务器连接"
		case acceleratorMode && isClient:
			priority = 4
			reason = "Client.exe 其他候选连接"
		default:
			continue
		}

		connInfo := connectionInfo{
			ServerIP:   remoteIP.String(),
			ServerPort: remotePortText,
			LocalPort:  fmt.Sprintf("%d", localPort),
		}
		appendCandidate := func(device captureDevice, candidateReason string, candidatePriority int) {
			key := fmt.Sprintf("%s|%s|%d|%d", device.name, remoteIP.String(), remotePort, localPort)
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
			candidates = append(candidates, connectionWithNic{
				nicName:      device.name,
				friendlyName: device.description,
				processName:  name,
				reason:       candidateReason,
				priority:     candidatePriority,
				connInfo:     connInfo,
			})
		}

		mappedDevice, mapped := deviceMap[localIP.String()]
		if mapped {
			appendCandidate(mappedDevice, reason, priority*10)
		} else {
			logger.Printf("Could not map local IP %s to an Npcap device for %s", localIP.String(), reason)
		}

		// Route-mode accelerators can keep the original game-server socket in
		// the TCP table while hiding the virtual interface address from Npcap.
		// In that case, test every visible Npcap device with the exact 4-tuple
		// filter. The strict filter and packet parser prevent unrelated traffic
		// from being selected.
		if acceleratorMode && (isConfiguredServer || !mapped) {
			for _, device := range devices {
				appendCandidate(device, reason+"（全网卡回退）", priority*10+1)
			}
		}
	}

	if len(candidates) == 0 {
		if acceleratorMode {
			return "", errors.New("未找到 Client.exe、加速器服务器连接或可用的 Npcap 网卡")
		}
		return "", errors.New("no Client.exe network connections found")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].priority < candidates[j].priority
	})
	const maxCandidates = 32
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	logger.Printf("Testing %d connection candidate(s) in parallel, accelerator mode=%v", len(candidates), acceleratorMode)
	testCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	resultCh := make(chan connectionWithNic, 1)
	var waitGroup sync.WaitGroup
	launched := 0

	for i, conn := range candidates {
		logger.Printf("[%d/%d] %s; process=%s; NIC=%s (%s); %s:%s -> local port %s",
			i+1, len(candidates), conn.reason, conn.processName, conn.nicName, conn.friendlyName,
			conn.connInfo.ServerIP, conn.connInfo.ServerPort, conn.connInfo.LocalPort)
		filter := connectionCaptureFilter(conn.connInfo)
		probeKey := conn.nicName + "|" + filter
		if !beginProbe(probeKey) {
			logger.Printf("Skipping probe still in progress: %s", conn.friendlyName)
			continue
		}
		launched++
		waitGroup.Add(1)
		go func(candidate connectionWithNic, candidateFilter string, candidateProbeKey string) {
			defer waitGroup.Done()
			defer endProbe(candidateProbeKey)
			if testNicForPackets(testCtx, candidate.nicName, candidateFilter) {
				select {
				case resultCh <- candidate:
					cancel()
				default:
				}
			}
		}(conn, filter, probeKey)
	}

	if launched == 0 {
		return "", errors.New("候选网卡的上一轮探测仍未返回，已跳过重复任务")
	}

	doneCh := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(doneCh)
	}()

	select {
	case winner := <-resultCh:
		updateFilterWithConnection(winner.connInfo)
		setSelectedCaptureFilter("")
		logger.Printf("Success: parsed game packets through %s", winner.reason)
		cancel()
		return winner.nicName, nil
	case <-doneCh:
		select {
		case winner := <-resultCh:
			updateFilterWithConnection(winner.connInfo)
			setSelectedCaptureFilter("")
			logger.Printf("Success: parsed game packets through %s", winner.reason)
			return winner.nicName, nil
		default:
		}
	case <-testCtx.Done():
		select {
		case winner := <-resultCh:
			updateFilterWithConnection(winner.connInfo)
			setSelectedCaptureFilter("")
			logger.Printf("Success: parsed game packets through %s", winner.reason)
			return winner.nicName, nil
		default:
		}
		logger.Printf("Connection probe deadline reached; returning without waiting for stuck devices")
	}

	if diagnosticMode && acceleratorMode {
		if winner, filter, ok := runDiagnosticFallback(candidates); ok {
			updateFilterWithConnection(winner.connInfo)
			setSelectedCaptureFilter(filter)
			logger.Printf("[DIAGNOSTIC] relaxed probe succeeded on %s with filter=%s", winner.friendlyName, filter)
			return winner.nicName, nil
		}
	}

	if acceleratorMode {
		return "", errors.New("已找到候选连接，但尚未解析到洛奇数据；请切换一次地图。若持续出现，UU 路由模式可能未向 Npcap 暴露虚拟网卡流量")
	}
	return "", errors.New("no game packets found on any Client.exe connection")
}

func setSelectedCaptureFilter(filter string) {
	selectedFilterMu.Lock()
	selectedCaptureFilter = filter
	selectedFilterMu.Unlock()
}

// SelectedCaptureFilter returns the relaxed filter selected by the diagnostic
// fallback. An empty value means the normal configured server filter is used.
func SelectedCaptureFilter() string {
	selectedFilterMu.RLock()
	defer selectedFilterMu.RUnlock()
	return selectedCaptureFilter
}

func runDiagnosticFallback(candidates []connectionWithNic) (connectionWithNic, string, bool) {
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		// Priority zero is the Client.exe game-server connection mapped directly
		// to its local capture device. Avoid broad probes on unrelated adapters.
		if candidate.priority != 0 {
			continue
		}
		key := candidate.nicName + "|" + candidate.connInfo.LocalPort
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		filters := []struct {
			label  string
			filter string
		}{
			{
				label:  "inbound-local-port",
				filter: fmt.Sprintf("tcp and dst port %s", candidate.connInfo.LocalPort),
			},
			{
				label:  "bidirectional-local-port",
				filter: fmt.Sprintf("tcp and port %s", candidate.connInfo.LocalPort),
			},
			{
				label:  "all-tcp-on-mapped-device",
				filter: "tcp",
			},
		}

		for _, probe := range filters {
			logger.Printf("[DIAGNOSTIC] probing %s (%s) for 12s with filter=%s", candidate.friendlyName, probe.label, probe.filter)
			probeCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			found := testNicForPackets(probeCtx, candidate.nicName, probe.filter)
			cancel()
			if found {
				return candidate, probe.filter, true
			}
			logger.Printf("[DIAGNOSTIC] no valid game packet on %s (%s)", candidate.friendlyName, probe.label)
		}
	}
	return connectionWithNic{}, "", false
}

func beginProbe(key string) bool {
	probeStateMu.Lock()
	defer probeStateMu.Unlock()
	if _, exists := probesInFlight[key]; exists {
		return false
	}
	probesInFlight[key] = struct{}{}
	return true
}

func endProbe(key string) {
	probeStateMu.Lock()
	delete(probesInFlight, key)
	probeStateMu.Unlock()
}

// HasGameConnection reports whether Client.exe currently owns at least one
// established TCP connection. It intentionally does not open a capture device,
// so it is cheap enough for the desktop wait/reconnect loop.
func HasGameConnection() bool {
	return HasGameConnectionWithMode(false)
}

// HasGameConnectionWithMode also recognizes Client.exe proxy connections and
// accelerator-owned connections to the configured game server.
func HasGameConnectionWithMode(acceleratorMode bool) bool {
	rows, err := getTCPRows()
	if err != nil {
		return false
	}

	for _, row := range rows {
		if row.State != 5 { // TCP_ESTABLISHED
			continue
		}
		name, err := processName(row.OwningPID)
		remoteIP := ipv4FromDWORD(row.RemoteAddr)
		remotePort := fmt.Sprintf("%d", portFromDWORD(row.RemotePort))
		isClient := err == nil && strings.EqualFold(name, "Client.exe")
		isConfiguredServer := constants.MatchesConfiguredGameServer(remoteIP.String(), remotePort)
		if isClient && isConfiguredServer {
			return true
		}
		if acceleratorMode && (isClient || isConfiguredServer) {
			return true
		}
	}

	return false
}

// HasActiveCaptureConnection reports whether the exact TCP flow selected by
// FindNic is still established. Merely finding another Client.exe connection
// is not sufficient: changing channel creates a new local TCP port, while the
// current BPF filter remains pinned to the old port and would otherwise keep
// the UI in a misleading "capturing" state forever.
func HasActiveCaptureConnection() bool {
	serverIP, serverPort, localPort, selected := constants.ActiveConnection()
	if !selected {
		// Relaxed/alternate backends may not have selected an exact flow.
		return true
	}

	rows, err := getTCPRows()
	if err != nil {
		// A transient Windows table-query failure must not tear down a healthy
		// capture; the next status poll will try again.
		return true
	}
	for _, row := range rows {
		if row.State != 5 { // TCP_ESTABLISHED
			continue
		}
		if ipv4FromDWORD(row.RemoteAddr).String() != serverIP {
			continue
		}
		if fmt.Sprintf("%d", portFromDWORD(row.RemotePort)) != serverPort {
			continue
		}
		if fmt.Sprintf("%d", portFromDWORD(row.LocalPort)) != localPort {
			continue
		}
		return true
	}
	return false
}

type captureDevice struct {
	name        string
	description string
	addresses   []string
}

func getCaptureDevices() []captureDevice {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		logger.Println("pcap.FindAllDevs failed:", err)
		return nil
	}

	result := make([]captureDevice, 0, len(devices))
	for _, pcapDevice := range devices {
		description := strings.TrimSpace(pcapDevice.Description)
		if description == "" {
			description = pcapDevice.Name
		}
		device := captureDevice{
			name:        pcapDevice.Name,
			description: description,
		}
		for _, address := range pcapDevice.Addresses {
			ip4 := address.IP.To4()
			if ip4 == nil {
				continue
			}
			device.addresses = append(device.addresses, ip4.String())
		}
		result = append(result, device)
	}
	return result
}

func getCaptureDeviceMap(devices []captureDevice) map[string]captureDevice {
	result := make(map[string]captureDevice)
	for _, device := range devices {
		for _, address := range device.addresses {
			result[address] = device
		}
	}
	return result
}

func updateFilterWithConnection(connInfo connectionInfo) {
	constants.UseActiveConnection(connInfo.ServerIP, connInfo.ServerPort, connInfo.LocalPort)
	logger.Printf("Filter updated with connection: %s:%s -> 0.0.0.0:%s",
		connInfo.ServerIP, connInfo.ServerPort, connInfo.LocalPort)
}

func connectionCaptureFilter(connInfo connectionInfo) string {
	return fmt.Sprintf(
		"tcp and src host %s and src port %s and dst port %s",
		connInfo.ServerIP,
		connInfo.ServerPort,
		connInfo.LocalPort,
	)
}

func testNicForPackets(ctx context.Context, nicName string, filter string) bool {
	probeCtx, cancel := context.WithCancel(ctx)
	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:               probeCtx,
		DisableCaptureLog: true,
	})
	if err != nil {
		cancel()
		logger.Printf("  Error creating reader: %v", err)
		return false
	}
	defer func() {
		cancel()
		// The finite pcap read timeout lets readPacketLoop leave before the
		// handle is closed, avoiding a cross-thread Close/Read deadlock.
		time.Sleep(350 * time.Millisecond)
		r.Close()
	}()

	if err := r.OpenNicWithFilterTimeout(nicName, filter, 250*time.Millisecond); err != nil {
		logger.Printf("  Error opening NIC %s: %v", nicName, err)
		return false
	}

	select {
	case <-probeCtx.Done():
		return false

	case <-r.PacketCh():
		return true
	}
}

func processName(pid uint32) (string, error) {
	h, err := windows.OpenProcess(processQueryLimitedInf, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}

	full := windows.UTF16ToString(buf[:size])
	return filepath.Base(full), nil
}

func getInterfaceNicMap() map[string]string {
	result := map[string]string{}

	nics, err := pcap.FindAllDevs()
	if err != nil {
		logger.Println("pcap.FindAllDevs failed:", err)
		return result
	}

	var size uint32
	_ = windows.GetAdaptersAddresses(afUnspec, windows.GAA_FLAG_INCLUDE_PREFIX, 0, nil, &size)
	if size == 0 {
		return result
	}

	buf := make([]byte, size)
	addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(afUnspec, windows.GAA_FLAG_INCLUDE_PREFIX, 0, addr, &size); err != nil {
		return result
	}

	ipToAdapter := make(map[string]*windows.IpAdapterAddresses)
	for a := addr; a != nil; a = a.Next {
		for u := a.FirstUnicastAddress; u != nil; u = u.Next {
			ip := sockaddrToIP(u.Address)
			if ip != nil {
				ipToAdapter[ip.String()] = a
			}
		}
	}

	for _, nic := range nics {
		if len(nic.Addresses) > 0 {
			for _, nicAddr := range nic.Addresses {
				if adapter, ok := ipToAdapter[nicAddr.IP.String()]; ok {
					friendlyName := windows.UTF16PtrToString(adapter.FriendlyName)
					result[friendlyName] = nic.Name
					logger.Printf("Mapped friendly name '%s' to NIC '%s'", friendlyName, nic.Name)
					break
				}
			}
		}
	}

	return result
}

func getTCPRows() ([]mibTCPRowOwnerPID, error) {
	var size uint32
	_ = getExtendedTcpTable(nil, &size, false, afInet, tcpTableOwnerPIDAll, 0)
	if size == 0 {
		return nil, fmt.Errorf("empty table size")
	}

	buf := make([]byte, size)
	if err := getExtendedTcpTable(&buf[0], &size, false, afInet, tcpTableOwnerPIDAll, 0); err != nil {
		return nil, err
	}

	table := (*mibTCPTableOwnerPID)(unsafe.Pointer(&buf[0]))
	rows := make([]mibTCPRowOwnerPID, 0, table.NumEntries)
	first := unsafe.Pointer(&table.Table[0])
	rowSize := unsafe.Sizeof(table.Table[0])

	for i := uint32(0); i < table.NumEntries; i++ {
		row := *(*mibTCPRowOwnerPID)(unsafe.Pointer(uintptr(first) + uintptr(i)*rowSize))
		rows = append(rows, row)
	}

	return rows, nil
}

func getExtendedTcpTable(table *byte, size *uint32, order bool, af uint32, tableClass uint32, reserved uint32) error {
	if err := iphlpapiDLL.Load(); err != nil {
		return err
	}

	var orderFlag uintptr
	if order {
		orderFlag = 1
	}

	r1, _, _ := procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(table)),
		uintptr(unsafe.Pointer(size)),
		orderFlag,
		uintptr(af),
		uintptr(tableClass),
		uintptr(reserved),
	)

	if r1 == 0 {
		return nil
	}
	if r1 == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return windows.ERROR_INSUFFICIENT_BUFFER
	}
	return windows.Errno(r1)
}

func getInterfaceMap() map[string]string {
	result := map[string]string{}

	var size uint32
	_ = windows.GetAdaptersAddresses(afUnspec, windows.GAA_FLAG_INCLUDE_PREFIX, 0, nil, &size)
	if size == 0 {
		return result
	}

	buf := make([]byte, size)
	addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(afUnspec, windows.GAA_FLAG_INCLUDE_PREFIX, 0, addr, &size); err != nil {
		return result
	}

	for a := addr; a != nil; a = a.Next {
		name := windows.UTF16PtrToString(a.FriendlyName)
		for u := a.FirstUnicastAddress; u != nil; u = u.Next {
			ip := sockaddrToIP(u.Address)
			if ip == nil {
				continue
			}
			result[ip.String()] = name
		}
	}

	return result
}

func sockaddrToIP(sa windows.SocketAddress) net.IP {
	if sa.Sockaddr == nil {
		return nil
	}
	switch sa.Sockaddr.Addr.Family {
	case windows.AF_INET:
		sa4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(sa.Sockaddr))
		return net.IPv4(sa4.Addr[0], sa4.Addr[1], sa4.Addr[2], sa4.Addr[3])
	case windows.AF_INET6:
		sa6 := (*windows.RawSockaddrInet6)(unsafe.Pointer(sa.Sockaddr))
		return net.IP(sa6.Addr[:])
	default:
		return nil
	}
}

func ipv4FromDWORD(addr uint32) net.IP {
	return net.IPv4(byte(addr), byte(addr>>8), byte(addr>>16), byte(addr>>24))
}

func portFromDWORD(port uint32) int {
	b := (*[2]byte)(unsafe.Pointer(&port))
	return int(binary.BigEndian.Uint16(b[:]))
}
