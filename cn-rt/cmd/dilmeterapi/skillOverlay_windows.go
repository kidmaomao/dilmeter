//go:build windows

package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

const (
	skillOverlayDefaultX      = 600
	skillOverlayDefaultY      = 180
	skillOverlayDefaultWidth  = 300
	skillOverlayDefaultHeight = 260
)

var (
	skillOverlayView     webview2.WebView
	skillOverlayHWND     uintptr
	skillOverlayActive   atomic.Bool
	skillOverlayClosing  atomic.Bool
	skillOverlayX        atomic.Int32
	skillOverlayY        atomic.Int32
	skillOverlayWidth    atomic.Int32
	skillOverlayHeight   atomic.Int32
	skillOverlayStateSeq atomic.Int64
	skillOverlayMoveSeq  atomic.Int64
	skillOverlayBoundsMu sync.Mutex
)

func setNativeSkillOverlayActive(active bool) { skillOverlayActive.Store(active) }

type skillOverlayRequest struct {
	Active   *bool `json:"active,omitempty"`
	X        *int  `json:"x,omitempty"`
	Y        *int  `json:"y,omitempty"`
	Width    *int  `json:"width,omitempty"`
	Height   *int  `json:"height,omitempty"`
	Sequence int64 `json:"sequence,omitempty"`
}

type skillOverlayPositionRequest struct {
	X        int   `json:"x"`
	Y        int   `json:"y"`
	Save     bool  `json:"save"`
	Sequence int64 `json:"sequence"`
}

func initializeSkillOverlay(view webview2.WebView, cfg config) {
	skillOverlayClosing.Store(false)
	skillOverlayView = view
	skillOverlayHWND = uintptr(view.Window())

	style, _, _ := procGetWindowLongW.Call(skillOverlayHWND, signedWindowLongIndex(gwlStyle))
	style &^= wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu
	procSetWindowLongW.Call(skillOverlayHWND, signedWindowLongIndex(gwlStyle), style)
	exStyle, _, _ := procGetWindowLongW.Call(skillOverlayHWND, signedWindowLongIndex(gwlExStyle))
	exStyle |= wsExTopmost | wsExToolWindow | wsExLayered | wsExNoActivate | wsExTransparent
	procSetWindowLongW.Call(skillOverlayHWND, signedWindowLongIndex(gwlExStyle), exStyle)
	procSetWindowPos.Call(skillOverlayHWND, ^uintptr(0), 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpFrameChanged)
	procSetLayeredWindowAttributes.Call(skillOverlayHWND, 0x000000, 255, lwaColorKey)

	x, y := skillOverlayDefaultX, skillOverlayDefaultY
	if cfg.SkillOverlayPositionSet {
		x, y = cfg.SkillOverlayX, cfg.SkillOverlayY
	}
	applySkillOverlayBounds(x, y, skillOverlayDefaultWidth, skillOverlayDefaultHeight, true)
	paintSkillOverlayBackground()
	procShowWindow.Call(skillOverlayHWND, swHide)
	go monitorSkillOverlayForeground(skillOverlayHWND)
}

func shutdownSkillOverlay() {
	skillOverlayClosing.Store(true)
	skillOverlayActive.Store(false)
	skillOverlayHWND = 0
	skillOverlayView = nil
}

func handleSkillOverlay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if skillOverlayHWND == 0 || skillOverlayClosing.Load() {
		http.Error(w, "overlay is unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeSkillOverlayState(w)
	case http.MethodPut:
		var request skillOverlayRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid overlay settings", http.StatusBadRequest)
			return
		}
		skillOverlayBoundsMu.Lock()
		defer skillOverlayBoundsMu.Unlock()
		if request.Sequence > 0 && request.Sequence <= skillOverlayStateSeq.Load() {
			writeSkillOverlayState(w)
			return
		}
		if request.Sequence > 0 {
			skillOverlayStateSeq.Store(request.Sequence)
		}
		if request.Active != nil {
			skillOverlayActive.Store(*request.Active)
		}
		x, y, width, height := currentSkillOverlayBounds()
		if request.X != nil {
			x = *request.X
		}
		if request.Y != nil {
			y = *request.Y
		}
		if request.Width != nil {
			width = *request.Width
		}
		if request.Height != nil {
			height = *request.Height
		}
		if request.X != nil || request.Y != nil || request.Width != nil || request.Height != nil {
			applySkillOverlayBounds(x, y, width, height, false)
		}
		writeSkillOverlayState(w)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeSkillOverlayState(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]any{"active": skillOverlayActive.Load(), "x": skillOverlayX.Load(), "y": skillOverlayY.Load(), "width": skillOverlayWidth.Load(), "height": skillOverlayHeight.Load()})
}

func currentSkillOverlayBounds() (x, y, width, height int) {
	return int(skillOverlayX.Load()), int(skillOverlayY.Load()), int(skillOverlayWidth.Load()), int(skillOverlayHeight.Load())
}

func applySkillOverlayBounds(x, y, width, height int, frameChanged bool) {
	if skillOverlayHWND == 0 {
		return
	}
	x = max(-32000, min(32000, x))
	y = max(-32000, min(32000, y))
	width = max(1, min(32000, width))
	height = max(1, min(32000, height))
	skillOverlayX.Store(int32(x))
	skillOverlayY.Store(int32(y))
	skillOverlayWidth.Store(int32(width))
	skillOverlayHeight.Store(int32(height))
	flags := uintptr(swpNoActivate)
	if frameChanged {
		flags |= swpFrameChanged
	}
	procSetWindowPos.Call(skillOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), uintptr(width), uintptr(height), flags)
	paintSkillOverlayBackground()
}

func paintSkillOverlayBackground() {
	if skillOverlayHWND == 0 {
		return
	}
	var rect nativeRect
	if ok, _, _ := procGetClientRect.Call(skillOverlayHWND, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	dc, _, _ := procGetDC.Call(skillOverlayHWND)
	if dc == 0 {
		return
	}
	defer procReleaseDC.Call(skillOverlayHWND, dc)
	brush, _, _ := procCreateSolidBrush.Call(0x000000)
	if brush == 0 {
		return
	}
	defer procDeleteObject.Call(brush)
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&rect)), brush)
}

func handleSkillOverlayPosition(w http.ResponseWriter, r *http.Request) {
	if skillOverlayHWND == 0 {
		http.Error(w, "overlay is unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]int32{"x": skillOverlayX.Load(), "y": skillOverlayY.Load(), "width": skillOverlayWidth.Load(), "height": skillOverlayHeight.Load()})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request skillOverlayPositionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid overlay position", http.StatusBadRequest)
		return
	}
	if !acceptOverlayMoveSequence(&skillOverlayMoveSeq, request.Sequence) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	skillOverlayBoundsMu.Lock()
	x := max(-32000, min(32000, request.X))
	y := max(-32000, min(32000, request.Y))
	_, _, width, height := currentSkillOverlayBounds()
	applySkillOverlayBounds(x, y, width, height, false)
	skillOverlayBoundsMu.Unlock()
	if request.Save {
		if err := updateConfig(func(cfg *config) { cfg.SkillOverlayX, cfg.SkillOverlayY, cfg.SkillOverlayPositionSet = x, y, true }); err != nil {
			logger.Println("save skill overlay position failed:", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func monitorSkillOverlayForeground(hwnd uintptr) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	visible := false
	for range ticker.C {
		if hwnd == 0 || skillOverlayClosing.Load() || skillOverlayHWND != hwnd {
			return
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		shouldShow := skillOverlayActive.Load() && !trayPlayerOverlayHidden.Load() &&
			(isCurrentProcessWindow(foreground) || isMabinogiWindow(foreground))
		if shouldShow {
			visible = true
			paintSkillOverlayBackground()
			procShowWindow.Call(hwnd, swShowNoActivate)
			procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
		} else if visible {
			visible = false
			procShowWindow.Call(hwnd, swHide)
		}
	}
}
