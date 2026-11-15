package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCaptureConnectionKeepsReceivingDuringMissingSnapshots(t *testing.T) {
	start := time.Now()
	monitor := captureConnectionMonitor{lastPayloadAt: start}
	// Reproduce repeated false negatives over a longer interval than the
	// November 11 failure, while the capture continues to receive game data.
	for i := 1; i <= 700; i++ {
		if monitor.shouldClose(start.Add(time.Duration(i)*2*time.Second), uint64(i), false, nil) {
			t.Fatalf("healthy capture closed at poll %d", i)
		}
	}
}

func TestCaptureConnectionRequiresRepeatedAbsenceAndSilence(t *testing.T) {
	start := time.Now()
	monitor := captureConnectionMonitor{lastPayloadAt: start}
	for i := 1; i <= 4; i++ {
		closed := monitor.shouldClose(start.Add(time.Duration(i)*2*time.Second), 0, false, nil)
		if closed != (i == 4) {
			t.Fatalf("poll %d close=%t", i, closed)
		}
	}
	// An already idle but connected game must also get three separate checks.
	monitor = captureConnectionMonitor{lastPayloadAt: start.Add(-time.Minute)}
	for i := 1; i <= 3; i++ {
		closed := monitor.shouldClose(start.Add(time.Duration(i)*2*time.Second), 0, false, nil)
		if closed != (i == 3) {
			t.Fatalf("idle game poll %d close=%t", i, closed)
		}
	}
}

func TestCaptureConnectionErrorsAndRecoveryResetConfirmation(t *testing.T) {
	start := time.Now()
	monitor := captureConnectionMonitor{lastPayloadAt: start.Add(-time.Minute)}
	queryError := errors.New("GetExtendedTcpTable: win32=5")
	for i := 0; i < 5; i++ {
		if monitor.shouldClose(start, 0, false, nil) {
			t.Fatal("single missing snapshot closed capture")
		}
		if monitor.shouldClose(start, 0, false, queryError) || monitor.missingChecks != 0 {
			t.Fatal("unknown query result counted as disconnection")
		}
	}
	monitor.shouldClose(start, 0, false, nil)
	if monitor.shouldClose(start, 0, true, nil) || monitor.missingChecks != 0 {
		t.Fatal("confirmed idle connection did not reset absence counter")
	}
}

func TestCaptureConnectionDiagnosticsThrottleAndRecover(t *testing.T) {
	var output bytes.Buffer
	previous := logger.Writer()
	logger.SetOutput(&output)
	t.Cleanup(func() { logger.SetOutput(previous) })
	start := time.Now()
	monitor := captureConnectionMonitor{}
	err := errors.New("OpenProcess(pid=1234): win32=5")
	monitor.reportError(start, err)
	monitor.reportError(start.Add(2*time.Second), err)
	monitor.reportError(start.Add(31*time.Second), err)
	monitor.reportError(start.Add(32*time.Second), nil)
	if strings.Count(output.String(), "win32=5") != 2 || !strings.Contains(output.String(), "query recovered") {
		t.Fatalf("unexpected error diagnostics: %s", output.String())
	}
}
