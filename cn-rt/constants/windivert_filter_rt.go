//go:build dilmeter_rt

package constants

import (
	"fmt"
	"net"
	"strings"
)

// WinDivertGameServerFilter returns an equivalent read-only WinDivert filter.
// It is compiled only into the DilmeterRT build.
func WinDivertGameServerFilter() (string, error) {
	gameServerFilter.RLock()
	network := gameServerFilter.network
	ports := append([]string(nil), gameServerFilter.ports...)
	activeIP := gameServerFilter.activeIP
	activePort := gameServerFilter.activeSrcPort
	gameServerFilter.RUnlock()

	var addressFilter string
	if activeIP != "" {
		if net.ParseIP(activeIP) == nil {
			return "", fmt.Errorf("invalid active game server IP %q", activeIP)
		}
		addressFilter = fmt.Sprintf("remoteAddr == %s", activeIP)
		if activePort != "" {
			ports = []string{activePort}
		}
	} else {
		ip, subnet, err := net.ParseCIDR(network)
		if err != nil {
			return "", fmt.Errorf("invalid game server network %q: %w", network, err)
		}
		first := ip.Mask(subnet.Mask).To4()
		if first == nil {
			return "", fmt.Errorf("DilmeterRT WinDivert capture currently supports IPv4 only")
		}
		last := append(net.IP(nil), first...)
		for i := range last {
			last[i] |= ^subnet.Mask[i]
		}
		addressFilter = fmt.Sprintf("remoteAddr >= %s and remoteAddr <= %s", first.String(), last.String())
	}

	if len(ports) == 0 {
		return "", fmt.Errorf("game server ports cannot be empty")
	}
	portTests := make([]string, 0, len(ports))
	for _, port := range ports {
		port = strings.TrimSpace(port)
		if port != "" {
			portTests = append(portTests, "remotePort == "+port)
		}
	}
	if len(portTests) == 0 {
		return "", fmt.Errorf("game server ports cannot be empty")
	}

	return fmt.Sprintf(
		"inbound and ip and tcp and tcp.PayloadLength > 0 and (%s) and (%s)",
		addressFilter,
		strings.Join(portTests, " or "),
	), nil
}
