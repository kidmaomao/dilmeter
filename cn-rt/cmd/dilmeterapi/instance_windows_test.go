//go:build windows

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestDilmeterWindowTitleClassification(t *testing.T) {
	for _, title := range []string{
		"DilmeterCN v1.3.2",
		"DilmeterRT v1.3.2",
		"DilmeterOT v1.3.2",
		"DilmeterCN v1.3.2-diagnostic",
	} {
		if !isDilmeterMainWindowTitle(title) {
			t.Fatalf("main-window title %q was rejected", title)
		}
	}
	for _, title := range []string{
		"DilmeterCN v1.3.2 Buff Overlay",
		"DilmeterRT v1.3.2 Skill Overlay",
		"unrelated webview",
	} {
		if isDilmeterMainWindowTitle(title) {
			t.Fatalf("non-main title %q was accepted", title)
		}
	}
}

func TestSingleInstanceMutexIsShared(t *testing.T) {
	name := `Local\Noginogi.Dilmeter.Test.` + strconv.Itoa(os.Getpid()) + `.` + strconv.FormatInt(time.Now().UnixNano(), 10)
	first, already, err := acquireSingleInstanceNamed(name)
	if err != nil || already || first == 0 {
		t.Fatalf("first mutex acquisition = handle %v, already %v, err %v", first, already, err)
	}
	second, already, err := acquireSingleInstanceNamed(name)
	if err != nil || !already || second == 0 {
		windows.CloseHandle(first)
		t.Fatalf("second mutex acquisition = handle %v, already %v, err %v", second, already, err)
	}
	windows.CloseHandle(second)
	windows.CloseHandle(first)

	third, already, err := acquireSingleInstanceNamed(name)
	if err != nil || already || third == 0 {
		t.Fatalf("mutex after all handles closed = handle %v, already %v, err %v", third, already, err)
	}
	windows.CloseHandle(third)
}

func TestTCPListenPreflightDetectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := checkTCPListenAvailable(address); err == nil {
		listener.Close()
		t.Fatal("occupied port was reported as available")
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkTCPListenAvailable(address); err != nil {
		t.Fatalf("released port was not available: %v", err)
	}
}

func TestHandleAppActivate(t *testing.T) {
	original := activateCurrentMainWindow
	t.Cleanup(func() { activateCurrentMainWindow = original })
	activated := false
	activateCurrentMainWindow = func() bool {
		activated = true
		return true
	}
	request := httptest.NewRequest(http.MethodPost, "/api/activate", nil)
	request.Header.Set("X-Dilmeter-Action", "activate")
	response := httptest.NewRecorder()
	handleAppActivate(response, request)
	if response.Code != http.StatusNoContent || !activated {
		t.Fatalf("activation response = %d, activated = %v", response.Code, activated)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/activate", nil)
	response = httptest.NewRecorder()
	handleAppActivate(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing activation header response = %d", response.Code)
	}
}

func TestActivationQueuesBeforeWindowExists(t *testing.T) {
	resetMainWindowActivationState()
	t.Cleanup(resetMainWindowActivationState)
	if !activateOrQueueCurrentMainWindow() {
		t.Fatal("activation during startup was not accepted")
	}
	if !markMainWindowReadyForActivation() {
		t.Fatal("startup activation was not queued for the ready window")
	}
}
