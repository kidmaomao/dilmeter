package constants

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultGameServerNetwork = "211.147.76.0/24"

var defaultGameServerPorts = []string{"11020", "11021", "11023"}

var gameServerFilter = struct {
	sync.RWMutex
	network       string
	ports         []string
	activeIP      string
	activeSrcPort string
	activeDstPort string
}{
	network: defaultGameServerNetwork,
	ports:   append([]string(nil), defaultGameServerPorts...),
}

// ConfigureGameServer selects the only CN server network and ports that the
// desktop monitor is allowed to inspect. Selecting a server also clears any
// connection-specific filter left over from the previous server.
func ConfigureGameServer(network string, ports []string) error {
	ip, _, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("invalid game server network %q: %w", network, err)
	}
	if ip.To4() == nil {
		return fmt.Errorf("game server network must be IPv4: %q", network)
	}
	if len(ports) == 0 {
		return fmt.Errorf("game server ports cannot be empty")
	}
	for _, port := range ports {
		port = strings.TrimSpace(port)
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("invalid game server port %q", port)
		}
	}

	gameServerFilter.Lock()
	gameServerFilter.network = network
	gameServerFilter.ports = append([]string(nil), ports...)
	gameServerFilter.activeIP = ""
	gameServerFilter.activeSrcPort = ""
	gameServerFilter.activeDstPort = ""
	gameServerFilter.Unlock()
	return nil
}

// MatchesConfiguredGameServer reports whether a TCP connection belongs to the
// currently selected CN server. This prevents unrelated Client.exe connections
// from bypassing the user's server selection.
func MatchesConfiguredGameServer(ip string, port string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	gameServerFilter.RLock()
	network := gameServerFilter.network
	ports := append([]string(nil), gameServerFilter.ports...)
	gameServerFilter.RUnlock()

	_, subnet, err := net.ParseCIDR(network)
	if err != nil || !subnet.Contains(parsedIP) {
		return false
	}
	for _, allowedPort := range ports {
		if port == allowedPort {
			return true
		}
	}
	return false
}

// UseActiveConnection narrows packet capture to the exact selected-server TCP
// connection after the correct NIC has been identified.
func UseActiveConnection(serverIP string, serverPort string, localPort string) {
	gameServerFilter.Lock()
	gameServerFilter.activeIP = serverIP
	gameServerFilter.activeSrcPort = serverPort
	gameServerFilter.activeDstPort = localPort
	gameServerFilter.Unlock()
}

// ActiveConnection returns the exact TCP flow currently used by packet
// capture. The boolean is false before connection discovery has selected a
// concrete game connection.
func ActiveConnection() (serverIP string, serverPort string, localPort string, ok bool) {
	gameServerFilter.RLock()
	defer gameServerFilter.RUnlock()
	if gameServerFilter.activeIP == "" || gameServerFilter.activeSrcPort == "" || gameServerFilter.activeDstPort == "" {
		return "", "", "", false
	}
	return gameServerFilter.activeIP, gameServerFilter.activeSrcPort, gameServerFilter.activeDstPort, true
}

// GameServerFilter returns a consistent BPF expression for the current server.
func GameServerFilter() string {
	gameServerFilter.RLock()
	defer gameServerFilter.RUnlock()

	server := gameServerFilter.network
	ports := gameServerFilter.ports
	dstPort := ""
	if gameServerFilter.activeIP != "" {
		server = gameServerFilter.activeIP
		ports = []string{gameServerFilter.activeSrcPort}
		dstPort = gameServerFilter.activeDstPort
	}

	filter := fmt.Sprintf("tcp and src net %s", server)
	if len(ports) > 0 {
		filter += fmt.Sprintf(" and src port ( %s )", strings.Join(ports, " or "))
	}
	if dstPort != "" {
		filter += fmt.Sprintf(" and dst port %s", dstPort)
	}
	return filter
}

// GameServerRoamingFilter returns a capture filter suitable for a long-lived
// desktop session. For a direct configured-server connection it intentionally
// omits the local TCP port so a channel switch is captured immediately. Proxy
// or accelerator connections outside the configured server range keep the
// exact discovered flow for safety. The boolean reports whether the returned
// filter follows all configured connections.
func GameServerRoamingFilter() (string, bool) {
	gameServerFilter.RLock()
	network := gameServerFilter.network
	ports := append([]string(nil), gameServerFilter.ports...)
	activeIP := gameServerFilter.activeIP
	activeSrcPort := gameServerFilter.activeSrcPort
	activeDstPort := gameServerFilter.activeDstPort
	gameServerFilter.RUnlock()

	directConfiguredConnection := activeIP == ""
	if activeIP != "" {
		parsedIP := net.ParseIP(activeIP)
		_, subnet, err := net.ParseCIDR(network)
		portAllowed := false
		for _, port := range ports {
			if port == activeSrcPort {
				portAllowed = true
				break
			}
		}
		directConfiguredConnection = err == nil && parsedIP != nil && subnet.Contains(parsedIP) && portAllowed
	}

	if !directConfiguredConnection {
		filter := fmt.Sprintf("tcp and src host %s and src port %s", activeIP, activeSrcPort)
		if activeDstPort != "" {
			filter += fmt.Sprintf(" and dst port %s", activeDstPort)
		}
		return filter, false
	}

	filter := fmt.Sprintf("tcp and src net %s", network)
	if len(ports) > 0 {
		filter += fmt.Sprintf(" and src port ( %s )", strings.Join(ports, " or "))
	}
	return filter, true
}

var SERVER_START_AT = time.Now().Unix()
var SERVER_START_AT_STR = time.Now().Format("2006-01-02_15-04-05")
