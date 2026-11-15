//go:build windows

package pcaputil

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"unsafe"

	"gitlab.com/prilus/mabidilmeter/constants"
	"golang.org/x/sys/windows"
)

func testConnectionRow(port uint32) mibTCPRowOwnerPID {
	return mibTCPRowOwnerPID{
		State: 5, LocalAddr: binary.LittleEndian.Uint32(net.IPv4(192, 168, 50, 103).To4()),
		LocalPort:  (port&255)<<8 | port>>8,
		RemoteAddr: binary.LittleEndian.Uint32(net.IPv4(211, 147, 76, 32).To4()),
		RemotePort: (11021&255)<<8 | 11021>>8, OwningPID: 1234,
	}
}

func configureConnectionTest(t *testing.T) {
	t.Helper()
	if err := constants.ConfigureGameServer("211.147.76.0/24", []string{"11021"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = constants.ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"}) })
}

func TestValidatedConnectionSurvivesProcessInspectionFailure(t *testing.T) {
	configureConnectionTest(t)
	constants.UseActiveConnection("211.147.76.32", "11021", "6683")
	status, err := checkCaptureConnections([]mibTCPRowOwnerPID{testConnectionRow(6683)}, false, func(uint32) (string, error) {
		t.Fatal("validated flow should not require another process-handle query")
		return "", windows.ERROR_ACCESS_DENIED
	})
	if err != nil || !status.GameConnected || !status.ActiveConnected {
		t.Fatalf("established game flow lost: status=%+v err=%v", status, err)
	}
}

func TestConnectionCheckDistinguishesUnknownFromDisconnected(t *testing.T) {
	configureConnectionTest(t)
	status, err := checkCaptureConnections([]mibTCPRowOwnerPID{testConnectionRow(6683)}, false, func(pid uint32) (string, error) {
		return "", windowsCallError("OpenProcess", windows.ERROR_ACCESS_DENIED)
	})
	if status.GameConnected || !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("expected unknown process status: %+v %v", status, err)
	}
	for _, detail := range []string{"win32=5", "pid=1234", "192.168.50.103:6683", "211.147.76.32:11021", "OpenProcess"} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("missing diagnostic %q: %v", detail, err)
		}
	}
	status, err = checkCaptureConnections(nil, false, processName)
	if err != nil || status.GameConnected {
		t.Fatalf("empty successful snapshot should confirm absence: %+v %v", status, err)
	}
}

func TestLocalServicesDoNotCountAsGameConnections(t *testing.T) {
	configureConnectionTest(t)
	row := testConnectionRow(8030)
	row.RemoteAddr = binary.LittleEndian.Uint32(net.IPv4(127, 0, 0, 1).To4())
	row.RemotePort = (8030&255)<<8 | 8030>>8
	status, err := checkCaptureConnections([]mibTCPRowOwnerPID{row}, false, func(uint32) (string, error) {
		t.Fatal("unrelated local services should not need process inspection")
		return "", windows.ERROR_ACCESS_DENIED
	})
	if err != nil || status.GameConnected {
		t.Fatalf("local service mistaken for game: %+v %v", status, err)
	}
}

func TestConnectionCheckTracksChannelChangesAndAccelerators(t *testing.T) {
	configureConnectionTest(t)
	constants.UseActiveConnection("211.147.76.32", "11021", "6683")
	lookup := func(uint32) (string, error) { return "Client.exe", nil }
	status, err := checkCaptureConnections([]mibTCPRowOwnerPID{testConnectionRow(6684)}, false, lookup)
	if err != nil || !status.GameConnected || status.ActiveConnected {
		t.Fatalf("new channel not distinguished from old capture: %+v %v", status, err)
	}
	row := testConnectionRow(6683)
	row.State = 8 // CLOSE_WAIT must not keep a closed game flow alive.
	status, err = checkCaptureConnections([]mibTCPRowOwnerPID{row}, false, lookup)
	if err != nil || status.GameConnected || status.ActiveConnected {
		t.Fatalf("closed flow retained: %+v %v", status, err)
	}
	row = testConnectionRow(6684)
	lookup = func(uint32) (string, error) { return "accelerator.exe", nil }
	status, err = checkCaptureConnections([]mibTCPRowOwnerPID{row}, true, lookup)
	if err != nil || !status.GameConnected {
		t.Fatalf("accelerator connection lost: %+v %v", status, err)
	}
}

func TestTCPTableRetriesGrowthAndPreservesErrors(t *testing.T) {
	calls := 0
	rows, err := readTCPRows(func(buffer *byte, size *uint32, _ bool, _, _, _ uint32) error {
		calls++
		if calls < 3 {
			*size += 2048
			return windows.ERROR_INSUFFICIENT_BUFFER
		}
		table := (*mibTCPTableOwnerPID)(unsafe.Pointer(buffer))
		table.NumEntries = 1
		table.Table[0] = testConnectionRow(6683)
		*size = 28
		return nil
	})
	if err != nil || calls != 3 || len(rows) != 1 || portFromDWORD(rows[0].LocalPort) != 6683 {
		t.Fatalf("failed to recover growing TCP table: calls=%d rows=%v err=%v", calls, rows, err)
	}
	_, err = readTCPRows(func(*byte, *uint32, bool, uint32, uint32, uint32) error { return windows.ERROR_ACCESS_DENIED })
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) || !strings.Contains(err.Error(), "win32=5") {
		t.Fatalf("lost Windows error: %v", err)
	}
	calls = 0
	_, err = readTCPRows(func(_ *byte, size *uint32, _ bool, _, _, _ uint32) error {
		calls++
		*size += 2048
		return windows.ERROR_INSUFFICIENT_BUFFER
	})
	if calls != 5 || !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		t.Fatalf("retry limit/error lost: calls=%d err=%v", calls, err)
	}
}

func TestTCPTableRetriesWhenLocalServicesStart(t *testing.T) {
	var listeners []net.Listener
	t.Cleanup(func() {
		for _, listener := range listeners {
			listener.Close()
		}
	})
	calls := 0
	sawGrowth := false
	rows, err := readTCPRows(func(buffer *byte, size *uint32, order bool, af, class, reserved uint32) error {
		calls++
		if calls == 2 {
			// Grow the actual Windows table after sizing, exceeding the spare rows.
			for i := 0; i < 128; i++ {
				listener, err := net.Listen("tcp4", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				listeners = append(listeners, listener)
			}
		}
		err := getExtendedTcpTable(buffer, size, order, af, class, reserved)
		if buffer != nil && errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
			sawGrowth = true
		}
		return err
	})
	if err != nil || !sawGrowth || len(rows) < len(listeners) {
		t.Fatalf("real local-service growth not recovered: calls=%d growth=%t rows=%d err=%v", calls, sawGrowth, len(rows), err)
	}
	t.Logf("recovered actual Windows connection-table growth with %d local listeners in %d calls", len(listeners), calls)
}
