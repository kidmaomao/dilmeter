//go:build windows

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

const (
	gwlStyle   = -16
	gwlExStyle = -20

	wsCaption       = 0x00C00000
	wsThickFrame    = 0x00040000
	wsMinimizeBox   = 0x00020000
	wsMaximizeBox   = 0x00010000
	wsSysMenu       = 0x00080000
	wsExTopmost     = 0x00000008
	wsExTransparent = 0x00000020
	wsExToolWindow  = 0x00000080
	wsExLayered     = 0x00080000
	wsExNoActivate  = 0x08000000

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040

	swHide           = 0
	swShowNoActivate = 4
	lwaColorKey      = 0x00000001
	rgnOr            = 2
	overlayPadding   = 8
	overlayGap       = 6
	overlayCountdown = 18
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowLongW             = user32.NewProc("GetWindowLongW")
	procSetWindowLongW             = user32.NewProc("SetWindowLongW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procFillRect                   = user32.NewProc("FillRect")
	procSetWindowRgn               = user32.NewProc("SetWindowRgn")
	gdi32                          = windows.NewLazySystemDLL("gdi32.dll")
	procCreateSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	procCreateRectRgn              = gdi32.NewProc("CreateRectRgn")
	procCombineRgn                 = gdi32.NewProc("CombineRgn")
	procDeleteObject               = gdi32.NewProc("DeleteObject")

	buffOverlayView    webview2.WebView
	buffOverlayHWND    uintptr
	buffOverlayLocked  atomic.Bool
	buffOverlayActive  atomic.Bool
	buffOverlayClosing atomic.Bool
	buffOverlayCount   atomic.Int32
	buffOverlaySize    atomic.Int32
	buffOverlayScale   atomic.Int32
	buffOverlayMoveSeq atomic.Int64
)

type nativeRect struct{ Left, Top, Right, Bottom int32 }
type nativePoint struct{ X, Y int32 }

type buffOverlayRequest struct {
	Locked       *bool `json:"locked,omitempty"`
	Active       *bool `json:"active,omitempty"`
	ItemCount    *int  `json:"itemCount,omitempty"`
	IconSize     *int  `json:"iconSize,omitempty"`
	ScalePercent *int  `json:"scalePercent,omitempty"`
}

type overlayRegionRect struct{ left, top, right, bottom int }
type buffOverlayMetrics struct {
	width, height int
	regions       []overlayRegionRect
}
type buffOverlayPositionRequest struct {
	X        int   `json:"x"`
	Y        int   `json:"y"`
	Save     bool  `json:"save"`
	Sequence int64 `json:"sequence"`
}

func initializeBuffOverlay(view webview2.WebView, cfg config) {
	buffOverlayView = view
	buffOverlayHWND = uintptr(view.Window())
	buffOverlayClosing.Store(false)
	buffOverlayLocked.Store(nativeReminderOverlaysAreLocked())
	buffOverlaySize.Store(20)
	buffOverlayScale.Store(100)

	style, _, _ := procGetWindowLongW.Call(buffOverlayHWND, signedWindowLongIndex(gwlStyle))
	style &^= wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu
	procSetWindowLongW.Call(buffOverlayHWND, signedWindowLongIndex(gwlStyle), style)
	applyBuffOverlayExtendedStyle(true)
	procSetLayeredWindowAttributes.Call(buffOverlayHWND, 0x000000, 255, lwaColorKey)

	x, y := 40, 100
	if cfg.BuffOverlayPositionSet {
		x, y = cfg.BuffOverlayX, cfg.BuffOverlayY
	}
	procSetWindowPos.Call(buffOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 520, 100,
		swpNoActivate|swpFrameChanged)
	paintBuffOverlayBackground(buffOverlayHWND)
	procShowWindow.Call(buffOverlayHWND, swHide)
	go monitorBuffOverlayForeground(buffOverlayHWND)
}

func shutdownBuffOverlay() {
	buffOverlayClosing.Store(true)
	buffOverlayActive.Store(false)
	buffOverlayHWND = 0
	buffOverlayView = nil
}

func applyBuffOverlayExtendedStyle(_ bool) {
	if buffOverlayHWND == 0 {
		return
	}
	exStyle, _, _ := procGetWindowLongW.Call(buffOverlayHWND, signedWindowLongIndex(gwlExStyle))
	exStyle |= wsExTopmost | wsExToolWindow | wsExLayered | wsExNoActivate | wsExTransparent
	procSetWindowLongW.Call(buffOverlayHWND, signedWindowLongIndex(gwlExStyle), exStyle)
	procSetWindowPos.Call(buffOverlayHWND, ^uintptr(0), 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpFrameChanged)
}

func setNativeBuffOverlayActive(active bool) { buffOverlayActive.Store(active) }

func handleBuffOverlay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"locked": nativeReminderOverlaysAreLocked(), "active": buffOverlayActive.Load(), "scale": buffOverlayScale.Load()})
	case http.MethodPut:
		var request buffOverlayRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid overlay settings", http.StatusBadRequest)
			return
		}
		if request.Active != nil {
			buffOverlayActive.Store(*request.Active)
		}
		if request.ItemCount != nil {
			buffOverlayCount.Store(int32(max(0, min(64, *request.ItemCount))))
		}
		if request.IconSize != nil {
			buffOverlaySize.Store(int32(max(16, min(80, *request.IconSize))))
		}
		if request.ScalePercent != nil {
			buffOverlayScale.Store(int32(max(50, min(500, *request.ScalePercent))))
		}
		if request.Locked != nil {
			buffOverlayLocked.Store(*request.Locked)
			setNativeReminderOverlaysLocked(*request.Locked)
			_ = updateConfig(func(cfg *config) { cfg.BuffOverlayLocked = *request.Locked })
		}
		applyBuffOverlayRegion()
		paintBuffOverlayBackground(buffOverlayHWND)
		_ = json.NewEncoder(w).Encode(map[string]any{"locked": nativeReminderOverlaysAreLocked(), "active": buffOverlayActive.Load(), "count": buffOverlayCount.Load(), "size": buffOverlaySize.Load(), "scale": buffOverlayScale.Load()})
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func applyBuffOverlayRegion() {
	if buffOverlayHWND == 0 {
		return
	}
	count := int(buffOverlayCount.Load())
	size := int(buffOverlaySize.Load())
	scalePercent := int(buffOverlayScale.Load())
	if size < 16 {
		size = 20
	}
	if scalePercent < 50 {
		scalePercent = 100
	}
	if count <= 0 {
		procSetWindowRgn.Call(buffOverlayHWND, 0, 1)
		return
	}
	metrics := calculateBuffOverlayMetrics(count, size, scalePercent, 0)
	procSetWindowPos.Call(buffOverlayHWND, 0, 0, 0, uintptr(metrics.width), uintptr(metrics.height),
		swpNoMove|swpNoActivate)

	combined, _, _ := procCreateRectRgn.Call(0, 0, 0, 0)
	if combined == 0 {
		return
	}
	for _, region := range metrics.regions {
		part, _, _ := procCreateRectRgn.Call(
			uintptr(region.left), uintptr(region.top), uintptr(region.right), uintptr(region.bottom),
		)
		if part == 0 {
			continue
		}
		procCombineRgn.Call(combined, combined, part, rgnOr)
		procDeleteObject.Call(part)
	}
	if result, _, _ := procSetWindowRgn.Call(buffOverlayHWND, combined, 1); result == 0 {
		procDeleteObject.Call(combined)
	}
}

func calculateBuffOverlayMetrics(count, iconSize, scalePercent, margin int) buffOverlayMetrics {
	count = max(0, count)
	iconSize = max(1, iconSize)
	scalePercent = max(50, min(500, scalePercent))
	logicalWidth := overlayPadding * 2
	if count > 0 {
		logicalWidth += count*iconSize + (count-1)*overlayGap
	}
	logicalItemHeight := iconSize + overlayCountdown
	logicalHeight := overlayPadding*2 + logicalItemHeight
	metrics := buffOverlayMetrics{width: scaleOverlayCeil(logicalWidth, scalePercent), height: scaleOverlayCeil(logicalHeight, scalePercent)}
	for index := 0; index < count; index++ {
		x := overlayPadding + index*(iconSize+overlayGap)
		metrics.regions = append(metrics.regions, overlayRegionRect{
			left: scaleOverlayFloor(max(0, x-margin), scalePercent), top: scaleOverlayFloor(max(0, overlayPadding-margin), scalePercent),
			right: scaleOverlayCeil(min(logicalWidth, x+iconSize+margin), scalePercent), bottom: scaleOverlayCeil(min(logicalHeight, overlayPadding+logicalItemHeight+margin), scalePercent),
		})
	}
	return metrics
}

func scaleOverlayFloor(value, scalePercent int) int { return value * scalePercent / 100 }
func scaleOverlayCeil(value, scalePercent int) int  { return (value*scalePercent + 99) / 100 }

func paintBuffOverlayBackground(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	var rect nativeRect
	if ok, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	dc, _, _ := procGetDC.Call(hwnd)
	if dc == 0 {
		return
	}
	defer procReleaseDC.Call(hwnd, dc)
	brush, _, _ := procCreateSolidBrush.Call(0x000000)
	if brush == 0 {
		return
	}
	defer procDeleteObject.Call(brush)
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&rect)), brush)
}

func handleBuffOverlayDrag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if nativeReminderOverlaysAreLocked() {
		http.Error(w, "unlock reminder overlays before dragging", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleBuffOverlayPosition(w http.ResponseWriter, r *http.Request) {
	if buffOverlayHWND == 0 {
		http.Error(w, "overlay is unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		var rect nativeRect
		if ok, _, _ := procGetWindowRect.Call(buffOverlayHWND, uintptr(unsafe.Pointer(&rect))); ok == 0 {
			http.Error(w, "cannot read overlay position", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"x": rect.Left, "y": rect.Top, "locked": nativeReminderOverlaysAreLocked()})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request buffOverlayPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid overlay position", http.StatusBadRequest)
		return
	}
	if !acceptOverlayMoveSequence(&buffOverlayMoveSeq, request.Sequence) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	x := max(-32000, min(32000, request.X))
	y := max(-32000, min(32000, request.Y))
	procSetWindowPos.Call(buffOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 0, 0, swpNoSize|swpNoActivate)
	if request.Save {
		if err := updateConfig(func(cfg *config) { cfg.BuffOverlayX, cfg.BuffOverlayY, cfg.BuffOverlayPositionSet = x, y, true }); err != nil {
			logger.Println("save Buff overlay position failed:", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func acceptOverlayMoveSequence(counter *atomic.Int64, sequence int64) bool {
	if sequence <= 0 {
		return true
	}
	for {
		previous := counter.Load()
		if sequence <= previous {
			return false
		}
		if counter.CompareAndSwap(previous, sequence) {
			return true
		}
	}
}

func handleBuffOverlayResetPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if buffOverlayHWND == 0 {
		http.Error(w, "overlay is unavailable", http.StatusServiceUnavailable)
		return
	}
	const x, y = 40, 100
	procSetWindowPos.Call(buffOverlayHWND, ^uintptr(0), x, y, 0, 0, swpNoSize|swpNoActivate)
	_ = updateConfig(func(cfg *config) { cfg.BuffOverlayX, cfg.BuffOverlayY, cfg.BuffOverlayPositionSet = x, y, true })
	w.WriteHeader(http.StatusNoContent)
}

func monitorBuffOverlayForeground(hwnd uintptr) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	visible := false
	for range ticker.C {
		if hwnd == 0 || buffOverlayClosing.Load() || buffOverlayHWND != hwnd {
			return
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		shouldShow := buffOverlayActive.Load() && !trayPlayerOverlayHidden.Load() &&
			(isCurrentProcessWindow(foreground) || isMabinogiWindow(foreground))
		if shouldShow {
			visible = true
			paintBuffOverlayBackground(hwnd)
			procShowWindow.Call(hwnd, swShowNoActivate)
			procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
		} else if visible {
			visible = false
			procShowWindow.Call(hwnd, swHide)
		}
	}
}

func isCurrentProcessWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err != nil {
		return false
	}
	return pid == uint32(os.Getpid())
}

func isMabinogiWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err != nil || pid == 0 {
		return false
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(process)
	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return false
	}
	return strings.EqualFold(filepath.Base(syscall.UTF16ToString(buffer[:size])), "Client.exe")
}

func signedWindowLongIndex(index int32) uintptr { return uintptr(index) }
