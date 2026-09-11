//go:build windows

package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

var (
	debuffOverlayHWND      uintptr
	debuffOverlayActive    atomic.Bool
	debuffOverlayClosing   atomic.Bool
	debuffOverlayCount     atomic.Int32
	debuffOverlaySize      atomic.Int32
	debuffOverlayScale     atomic.Int32
	debuffOverlayHasBoss   atomic.Bool
	debuffOverlayHasHealth atomic.Bool
	debuffOverlayMoveSeq   atomic.Int64
)

type debuffOverlayRequest struct {
	Active       *bool `json:"active,omitempty"`
	ItemCount    *int  `json:"itemCount,omitempty"`
	IconSize     *int  `json:"iconSize,omitempty"`
	ScalePercent *int  `json:"scalePercent,omitempty"`
	HasBoss      *bool `json:"hasBoss,omitempty"`
	HasHealth    *bool `json:"hasHealth,omitempty"`
}

func initializeDebuffOverlay(view webview2.WebView, cfg config) {
	debuffOverlayHWND = uintptr(view.Window())
	debuffOverlayClosing.Store(false)
	debuffOverlaySize.Store(30)
	debuffOverlayScale.Store(100)

	style, _, _ := procGetWindowLongW.Call(debuffOverlayHWND, signedWindowLongIndex(gwlStyle))
	style &^= wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu
	procSetWindowLongW.Call(debuffOverlayHWND, signedWindowLongIndex(gwlStyle), style)
	exStyle, _, _ := procGetWindowLongW.Call(debuffOverlayHWND, signedWindowLongIndex(gwlExStyle))
	exStyle |= wsExTopmost | wsExToolWindow | wsExLayered | wsExNoActivate | wsExTransparent
	procSetWindowLongW.Call(debuffOverlayHWND, signedWindowLongIndex(gwlExStyle), exStyle)
	procSetLayeredWindowAttributes.Call(debuffOverlayHWND, 0x000000, 255, lwaColorKey)

	x, y := 40, 160
	if cfg.DebuffOverlayPositionSet {
		x, y = cfg.DebuffOverlayX, cfg.DebuffOverlayY
	}
	procSetWindowPos.Call(debuffOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 520, 100,
		swpNoActivate|swpFrameChanged)
	paintBuffOverlayBackground(debuffOverlayHWND)
	procShowWindow.Call(debuffOverlayHWND, swHide)
	go monitorDebuffOverlayForeground(debuffOverlayHWND)
}

func shutdownDebuffOverlay() {
	debuffOverlayClosing.Store(true)
	debuffOverlayActive.Store(false)
	debuffOverlayHWND = 0
}

func setNativeDebuffOverlayActive(active bool) { debuffOverlayActive.Store(active) }

func handleDebuffOverlay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"active": debuffOverlayActive.Load(), "scale": debuffOverlayScale.Load(), "hasBoss": debuffOverlayHasBoss.Load(), "hasHealth": debuffOverlayHasHealth.Load()})
	case http.MethodPut:
		var request debuffOverlayRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid overlay settings", http.StatusBadRequest)
			return
		}
		if request.Active != nil {
			debuffOverlayActive.Store(*request.Active)
		}
		if request.ItemCount != nil {
			debuffOverlayCount.Store(int32(max(0, min(64, *request.ItemCount))))
		}
		if request.IconSize != nil {
			debuffOverlaySize.Store(int32(max(16, min(80, *request.IconSize))))
		}
		if request.ScalePercent != nil {
			debuffOverlayScale.Store(int32(max(50, min(500, *request.ScalePercent))))
		}
		if request.HasBoss != nil {
			debuffOverlayHasBoss.Store(*request.HasBoss)
		}
		if request.HasHealth != nil {
			debuffOverlayHasHealth.Store(*request.HasHealth)
		}
		applyDebuffOverlayRegion()
		paintBuffOverlayBackground(debuffOverlayHWND)
		_ = json.NewEncoder(w).Encode(map[string]any{"active": debuffOverlayActive.Load(), "count": debuffOverlayCount.Load(), "size": debuffOverlaySize.Load(), "scale": debuffOverlayScale.Load(), "hasBoss": debuffOverlayHasBoss.Load(), "hasHealth": debuffOverlayHasHealth.Load()})
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func applyDebuffOverlayRegion() {
	if debuffOverlayHWND == 0 {
		return
	}
	count := int(debuffOverlayCount.Load())
	size := int(debuffOverlaySize.Load())
	scalePercent := int(debuffOverlayScale.Load())
	hasBoss := debuffOverlayHasBoss.Load()
	hasHealth := debuffOverlayHasHealth.Load()
	if size < 16 {
		size = 30
	}
	if scalePercent < 50 {
		scalePercent = 100
	}
	if count <= 0 && !hasBoss && !hasHealth {
		procSetWindowRgn.Call(debuffOverlayHWND, 0, 1)
		return
	}
	metrics := calculateDebuffOverlayMetrics(count, size, scalePercent, hasBoss, hasHealth)
	procSetWindowPos.Call(debuffOverlayHWND, 0, 0, 0, uintptr(metrics.width), uintptr(metrics.height),
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
	if result, _, _ := procSetWindowRgn.Call(debuffOverlayHWND, combined, 1); result == 0 {
		procDeleteObject.Call(combined)
	}
}

const (
	debuffBossHeaderWidth  = 420
	debuffBossHeaderHeight = 22
	debuffBossHeaderGap    = 4
	debuffHealthBarHeight  = 38
	debuffHealthBarGap     = 4
)

func calculateDebuffOverlayMetrics(count, iconSize, scalePercent int, hasBoss, hasHealth bool) buffOverlayMetrics {
	if !hasBoss && !hasHealth {
		return calculateBuffOverlayMetrics(count, iconSize, scalePercent, 0)
	}
	count = max(0, count)
	iconSize = max(1, iconSize)
	scalePercent = max(50, min(500, scalePercent))
	iconsWidth := 0
	if count > 0 {
		iconsWidth = count*iconSize + (count-1)*overlayGap
	}
	contentWidth := max(debuffBossHeaderWidth, iconsWidth)
	logicalWidth := overlayPadding*2 + contentWidth
	logicalHeight := overlayPadding * 2
	if hasHealth {
		logicalHeight += debuffHealthBarHeight
	}
	if hasBoss {
		if hasHealth {
			logicalHeight += debuffHealthBarGap
		}
		logicalHeight += debuffBossHeaderHeight
		if count > 0 {
			logicalHeight += debuffBossHeaderGap + iconSize + overlayCountdown
		}
	}
	return buffOverlayMetrics{width: scaleOverlayCeil(logicalWidth, scalePercent), height: scaleOverlayCeil(logicalHeight, scalePercent), regions: []overlayRegionRect{{
		left: scaleOverlayFloor(overlayPadding, scalePercent), top: scaleOverlayFloor(overlayPadding, scalePercent), right: scaleOverlayCeil(overlayPadding+contentWidth, scalePercent), bottom: scaleOverlayCeil(logicalHeight-overlayPadding, scalePercent),
	}}}
}

func handleDebuffOverlayPosition(w http.ResponseWriter, r *http.Request) {
	if debuffOverlayHWND == 0 {
		http.Error(w, "overlay is unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		var rect nativeRect
		if ok, _, _ := procGetWindowRect.Call(debuffOverlayHWND, uintptr(unsafe.Pointer(&rect))); ok == 0 {
			http.Error(w, "cannot read overlay position", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"x": rect.Left, "y": rect.Top})
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
	if !acceptOverlayMoveSequence(&debuffOverlayMoveSeq, request.Sequence) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	x := max(-32000, min(32000, request.X))
	y := max(-32000, min(32000, request.Y))
	procSetWindowPos.Call(debuffOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 0, 0, swpNoSize|swpNoActivate)
	if request.Save {
		if err := updateConfig(func(cfg *config) { cfg.DebuffOverlayX, cfg.DebuffOverlayY, cfg.DebuffOverlayPositionSet = x, y, true }); err != nil {
			logger.Println("save Debuff overlay position failed:", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func monitorDebuffOverlayForeground(hwnd uintptr) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	visible := false
	for range ticker.C {
		if hwnd == 0 || debuffOverlayClosing.Load() || debuffOverlayHWND != hwnd {
			return
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		shouldShow := debuffOverlayActive.Load() && !trayDebuffOverlayHidden.Load() &&
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
