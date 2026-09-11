//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const dilmeterSingleInstanceName = `Local\Noginogi.Dilmeter.Port8030.SingleInstance`

var (
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetClassNameW        = user32.NewProc("GetClassNameW")
	procIsWindow             = user32.NewProc("IsWindow")
	procFlashWindow          = user32.NewProc("FlashWindow")
	mainWindowHandle         atomic.Uintptr
	mainWindowCanActivate    atomic.Bool
	mainWindowActivateQueued atomic.Bool

	activateCurrentMainWindow = activateOrQueueCurrentMainWindow
)

func activateOrQueueCurrentMainWindow() bool {
	hwnd := mainWindowHandle.Load()
	if hwnd == 0 || !mainWindowCanActivate.Load() {
		// The HTTP server starts before the WebView/tray. Treat activation as
		// accepted during that short interval and honor it when the page is ready.
		mainWindowActivateQueued.Store(true)
		return true
	}
	return showAndActivateMainWindow(hwnd)
}

func acquireDilmeterSingleInstance() (windows.Handle, bool, error) {
	return acquireSingleInstanceNamed(dilmeterSingleInstanceName)
}

func acquireSingleInstanceNamed(name string) (windows.Handle, bool, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}
	handle, err := windows.CreateMutex(nil, false, namePtr)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return handle, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	return handle, false, nil
}

func checkTCPListenAvailable(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return listener.Close()
}

func handleAppActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-Dilmeter-Action") != "activate" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !activateCurrentMainWindow() {
		http.Error(w, "window is not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func tryActivateExistingDilmeter() bool {
	for attempt := 0; attempt < 3; attempt++ {
		activated, endpointAvailable := activateExistingDilmeterHTTP()
		if activated {
			return true
		}
		// Enumeration exists only for old releases without /api/activate. A new
		// release may deliberately keep its window hidden until WebView is ready.
		if !endpointAvailable {
			if hwnd := findExistingDilmeterMainWindow(); hwnd != 0 {
				return showAndActivateMainWindow(hwnd)
			}
		}
		time.Sleep(180 * time.Millisecond)
	}
	return false
}

func activateExistingDilmeterHTTP() (activated bool, endpointAvailable bool) {
	client := &http.Client{Timeout: 450 * time.Millisecond}
	response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/app_info", _port))
	if err != nil {
		return false, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, false
	}
	var info struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil || !isDilmeterApplicationName(info.Name) {
		return false, false
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/activate", _port), nil)
	if err != nil {
		return false, false
	}
	request.Header.Set("X-Dilmeter-Action", "activate")
	activateResponse, err := client.Do(request)
	if err != nil {
		return false, false
	}
	defer activateResponse.Body.Close()
	if activateResponse.StatusCode == http.StatusNotFound {
		return false, false
	}
	return activateResponse.StatusCode == http.StatusNoContent, true
}

func resetMainWindowActivationState() {
	mainWindowHandle.Store(0)
	mainWindowCanActivate.Store(false)
	mainWindowActivateQueued.Store(false)
}

func markMainWindowReadyForActivation() bool {
	mainWindowCanActivate.Store(true)
	return mainWindowActivateQueued.Swap(false)
}

func setMainWindowHandle(hwnd uintptr) {
	mainWindowHandle.Store(hwnd)
}

func isDilmeterApplicationName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "dilmetercn", "dilmeterrt", "dilmeterot":
		return true
	default:
		return false
	}
}

func isDilmeterMainWindowTitle(title string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	if strings.Contains(title, " overlay") {
		return false
	}
	return strings.HasPrefix(title, "dilmetercn v") ||
		strings.HasPrefix(title, "dilmeterrt v") ||
		strings.HasPrefix(title, "dilmeterot v")
}

func findExistingDilmeterMainWindow() uintptr {
	var found uintptr
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if found != 0 || windowClassName(hwnd) != "webview" {
			return 1
		}
		if isDilmeterMainWindowTitle(windowTitleText(hwnd)) {
			found = hwnd
			return 0
		}
		return 1
	})
	procEnumWindows.Call(callback, 0)
	return found
}

func windowTitleText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return windows.UTF16ToString(buffer)
}

func windowClassName(hwnd uintptr) string {
	buffer := make([]uint16, 128)
	length, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:length])
}

func showAndActivateMainWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	valid, _, _ := procIsWindow.Call(hwnd)
	if valid == 0 {
		return false
	}
	procShowWindow.Call(hwnd, swRestore)
	foreground, _, _ := procSetForegroundWindow.Call(hwnd)
	if foreground == 0 {
		procFlashWindow.Call(hwnd, 1)
	}
	return true
}

func requestMainWindowClose() bool {
	hwnd := mainWindowHandle.Load()
	if hwnd == 0 {
		return false
	}
	posted, _, _ := procPostMessageW.Call(hwnd, wmClose, 0, 0)
	return posted != 0
}

func consumeVersionLaunchMarker() bool {
	marker := filepath.Join(appDataDir(), ".last-launched-version")
	previous, _ := os.ReadFile(marker)
	changed := strings.TrimSpace(string(previous)) != strings.TrimSpace(AppVersion)
	if err := os.WriteFile(marker, []byte(strings.TrimSpace(AppVersion)+"\n"), 0644); err != nil {
		logger.Println("write version launch marker failed:", err)
	}
	return changed
}
