//go:build windows

package pcaputil

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/prilus/mabidilmeter/constants"
	"golang.org/x/sys/windows"
)

// ConnectionStatus is computed from a single Windows TCP-table snapshot.
// A non-nil error means absence could not be established reliably.
type ConnectionStatus struct {
	GameConnected   bool
	ActiveConnected bool
}

func CheckCaptureConnections(acceleratorMode bool) (ConnectionStatus, error) {
	rows, err := getTCPRows()
	if err != nil {
		return ConnectionStatus{}, err
	}
	return checkCaptureConnections(rows, acceleratorMode, processName)
}

func checkCaptureConnections(rows []mibTCPRowOwnerPID, acceleratorMode bool, lookup func(uint32) (string, error)) (ConnectionStatus, error) {
	serverIP, serverPort, localPort, selected := constants.ActiveConnection()
	status := ConnectionStatus{ActiveConnected: !selected}
	// Discovery already validated this flow by parsing game traffic. Its
	// established TCP row remains useful even if process inspection is denied.
	for _, row := range rows {
		if selected && row.State == 5 && ipv4FromDWORD(row.RemoteAddr).String() == serverIP &&
			fmt.Sprint(portFromDWORD(row.RemotePort)) == serverPort && fmt.Sprint(portFromDWORD(row.LocalPort)) == localPort {
			return ConnectionStatus{GameConnected: true, ActiveConnected: true}, nil
		}
	}

	var inspectionError error
	for _, row := range rows {
		if row.State != 5 {
			continue
		}
		remoteIP := ipv4FromDWORD(row.RemoteAddr).String()
		remotePort := portFromDWORD(row.RemotePort)
		configured := constants.MatchesConfiguredGameServer(remoteIP, fmt.Sprint(remotePort))
		if acceleratorMode && configured {
			status.GameConnected = true
			return status, nil
		}
		if !configured && !acceleratorMode {
			continue
		}
		name, err := lookup(row.OwningPID)
		if err != nil {
			if inspectionError == nil {
				inspectionError = fmt.Errorf("inspect connection pid=%d local=%s:%d remote=%s:%d: %w",
					row.OwningPID, ipv4FromDWORD(row.LocalAddr), portFromDWORD(row.LocalPort), remoteIP, remotePort, err)
			}
			continue
		}
		if strings.EqualFold(name, "Client.exe") {
			status.GameConnected = true
			return status, nil
		}
	}
	return status, inspectionError
}

func windowsCallError(operation string, err error) error {
	var code windows.Errno
	if errors.As(err, &code) {
		return fmt.Errorf("%s: win32=%d: %w", operation, uint32(code), err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
