//go:build windows

package main

import (
	"context"
	"sync/atomic"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

var healerOverlayWindow atomic.Uintptr

func healerOverlayDPI(configured int) int {
	return nativeReminderScalePercent(healerOverlayWindow.Load(), configured)
}

// Keep the panel anchored to its saved desktop position as its monitor or DPI changes.
func (frame healerOverlayFrame) desktopBounds() (x, y, width, height int) {
	team := frame.Groups[0]
	x = team.X - scaleOverlayCeil(team.X-frame.X, frame.DPIPercent)
	y = team.Y - scaleOverlayCeil(team.Y-frame.Y, frame.DPIPercent)
	return x, y, scaleOverlayCeil(frame.Width, frame.DPIPercent), scaleOverlayCeil(frame.Height, frame.DPIPercent)
}

// Match the existing Boss/skill WebView: transparent outside the content,
// topmost, non-activating, and click-through. Monitoring/audio stay in Go.
func initializeHealerOverlay(ctx context.Context, view webview2.WebView, dispatch func(func())) func() {
	hwnd := uintptr(view.Window())
	healerOverlayWindow.Store(hwnd)
	style, _, _ := procGetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlStyle))
	style &^= wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu
	procSetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlStyle), style)
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlExStyle))
	procSetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlExStyle), exStyle|wsExTopmost|wsExToolWindow|wsExLayered|wsExNoActivate|wsExTransparent)
	procSetLayeredWindowAttributes.Call(hwnd, 0, 255, lwaColorKey)
	paintBuffOverlayBackground(hwnd)
	procShowWindow.Call(hwnd, swHide)
	stop, done := make(chan struct{}), make(chan struct{})
	var closing, pending atomic.Bool
	go func() {
		defer close(done)
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		visible := false
		var lastX, lastY, lastWidth, lastHeight int
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
			}
			runtime := currentNativeReminderRuntime()
			if runtime == nil || runtime.healer == nil {
				continue
			}
			if !pending.CompareAndSwap(false, true) {
				continue
			}
			// Dispatch through the main window that owns Run(), not this
			// overlay's unpumped queue. All HWND mutations stay on its thread.
			dispatch(func() {
				defer pending.Store(false)
				if closing.Load() {
					return
				}
				frame := runtime.healerOverlaySnapshot(time.Now().UnixMilli())
				foreground, _, _ := procGetForegroundWindow.Call()
				show := len(frame.Groups) > 0 && !trayPlayerOverlayHidden.Load() && (frame.Preview || isCurrentProcessWindow(foreground) || isMabinogiWindow(foreground))
				if show {
					x, y, width, height := frame.desktopBounds()
					if lastX != x || lastY != y || lastWidth != width || lastHeight != height {
						lastX, lastY, lastWidth, lastHeight = x, y, width, height
						procSetWindowPos.Call(hwnd, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), uintptr(min(32000, width)), uintptr(min(32000, height)), swpNoActivate|swpFrameChanged)
					}
					paintBuffOverlayBackground(hwnd)
					procShowWindow.Call(hwnd, swShowNoActivate)
					procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
				} else if visible {
					procShowWindow.Call(hwnd, swHide)
				}
				visible = show
			})
		}
	}()
	return func() {
		closing.Store(true)
		close(stop)
		<-done
		healerOverlayWindow.Store(0)
		procShowWindow.Call(hwnd, swHide)
	}
}
