//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whMouseLL  = 14
	hcAction   = 0
	pmNoRemove = 0
	// LLMHF_INJECTED marks mouse input synthesized by SendInput, mouse_event,
	// or another process. Only physical mouse edges may start a SkillBar
	// gesture or trigger its optional stop-key compensation.
	llmhfInjected = 0x00000001
)

// MSLLHOOKSTRUCT is shared by the reminder-overlay drag helper and SkillBar's
// low-level click barrier.
type nativeLowLevelMouseHook struct {
	Point     nativePoint
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type nativeReminderHitRegion struct {
	Kind string
	ID   string
	Rect nativeRect
	X    int
	Y    int
}

type nativeReminderDragTarget struct {
	Kind string
	ID   string
	X    int
	Y    int
}

type nativeReminderHitSnapshot struct {
	Regions []nativeReminderHitRegion
}

type nativeReminderHookEdge struct {
	Down   bool
	Point  nativePoint
	Target nativeReminderDragTarget
	Epoch  uint64
}

var (
	procSetWindowsHookExW         = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx       = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx            = user32.NewProc("CallNextHookEx")
	procPeekMessageW              = user32.NewProc("PeekMessageW")
	procPostThreadMessageW        = user32.NewProc("PostThreadMessageW")
	nativeReminderLocked          atomic.Bool
	nativeReminderMouseHook       atomic.Uintptr
	nativeReminderMouseHookThread atomic.Uint32
	nativeReminderMouseHookReady  atomic.Bool
	nativeReminderMouseHookProc   = syscall.NewCallback(nativeReminderLowLevelMouseProc)
	nativeReminderMouseHookDone   chan struct{}
	nativeReminderWebHitSnapshot  atomic.Pointer[nativeReminderHitSnapshot]
	nativeReminderHookHitSnapshot atomic.Pointer[nativeReminderHitSnapshot]
	nativeReminderHookOwned       atomic.Bool
	nativeReminderHookCursorX     atomic.Int32
	nativeReminderHookCursorY     atomic.Int32
	nativeReminderHookCancelEpoch atomic.Uint64
	nativeReminderHookEdgeQueue   = make(chan nativeReminderHookEdge, 32)
	nativeReminderHookMoveWake    = make(chan struct{}, 1)
	nativeReminderHookCancelWake  = make(chan struct{}, 1)
	nativeReminderWorkerStop      chan struct{}
	nativeReminderWorkerDone      chan struct{}
	nativeReminderInputMu         sync.Mutex
	nativeReminderDragActive      bool
	nativeReminderDragTargetAt    nativeReminderDragTarget
	nativeReminderDragStart       nativePoint
)

func init() { nativeReminderLocked.Store(true) }

func initializeNativeReminderOverlayInput() error {
	workerStop := make(chan struct{})
	workerDone := make(chan struct{})
	nativeReminderWorkerStop = workerStop
	nativeReminderWorkerDone = workerDone
	go runNativeReminderInputWorker(workerStop, workerDone)

	ready := make(chan error, 1)
	done := make(chan struct{})
	nativeReminderMouseHookDone = done
	go runNativeReminderMouseHookThread(ready, done)
	if err := <-ready; err != nil {
		close(workerStop)
		<-workerDone
		return err
	}
	return nil
}

func runNativeReminderMouseHookThread(ready chan<- error, done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)

	var module windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &module); err != nil {
		ready <- err
		return
	}
	// Explicitly create this thread's message queue before publishing its ID so
	// shutdown can always wake GetMessage with WM_QUIT.
	var message nativeWindowMessage
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmNoRemove)
	hook, _, hookErr := procSetWindowsHookExW.Call(whMouseLL, nativeReminderMouseHookProc, uintptr(module), 0)
	if hook == 0 {
		ready <- fmt.Errorf("install shared low-level mouse hook: %w", hookErr)
		return
	}
	nativeReminderMouseHook.Store(hook)
	nativeReminderMouseHookThread.Store(windows.GetCurrentThreadId())
	nativeReminderMouseHookReady.Store(true)
	logger.Printf("shared low-level mouse hook ready: thread=%d input=skillbar-barrier+reminder-drag", windows.GetCurrentThreadId())
	ready <- nil

	for {
		result, _, messageErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			logger.Println("shared low-level mouse hook message loop failed:", messageErr)
			break
		}
		if result == 0 || message.Message == wmQuit {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}

	nativeReminderMouseHookReady.Store(false)
	nativeReminderMouseHookThread.Store(0)
	handleNativeSkillBarMouseHookUnavailable("shared WH_MOUSE_LL message loop stopped")
	if installed := nativeReminderMouseHook.Swap(0); installed != 0 {
		procUnhookWindowsHookEx.Call(installed)
	}
}

func shutdownNativeReminderOverlayInput() {
	nativeReminderMouseHookReady.Store(false)
	if thread := nativeReminderMouseHookThread.Load(); thread != 0 {
		posted, _, postErr := procPostThreadMessageW.Call(uintptr(thread), wmQuit, 0, 0)
		if posted == 0 {
			logger.Println("stop shared low-level mouse hook thread failed:", postErr)
		}
	}
	if done := nativeReminderMouseHookDone; done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			logger.Println("shared low-level mouse hook thread did not stop within 2 seconds")
		}
	}
	nativeReminderHookCancelEpoch.Add(1)
	nativeReminderHookOwned.Store(false)
	signalNativeReminderHookCancel()
	if stop := nativeReminderWorkerStop; stop != nil {
		close(stop)
		nativeReminderWorkerStop = nil
	}
	if done := nativeReminderWorkerDone; done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			logger.Println("native reminder input worker did not stop within 2 seconds")
		}
		nativeReminderWorkerDone = nil
	}
	nativeReminderWebHitSnapshot.Store(nil)
	nativeReminderHookHitSnapshot.Store(nil)
}

func nativeLowLevelMouseHookAvailable() bool { return nativeReminderMouseHookReady.Load() }

func nativeReminderOverlaysAreLocked() bool { return nativeReminderLocked.Load() }

func setNativeReminderOverlaysLocked(locked bool) {
	nativeReminderLocked.Store(locked)
	buffOverlayLocked.Store(locked)
	if locked {
		nativeReminderHookCancelEpoch.Add(1)
		signalNativeReminderHookCancel()
	}
	refreshWebViewReminderHitRegions()
}

func setNativeReminderHitRegions(regions []nativeReminderHitRegion) {
	snapshot := &nativeReminderHitSnapshot{Regions: append([]nativeReminderHitRegion(nil), regions...)}
	nativeReminderWebHitSnapshot.Store(snapshot)
	publishNativeReminderHookHitSnapshot()
}

func nativeReminderLowLevelMouseProc(code, message, hookData uintptr) uintptr {
	if int32(code) < hcAction || hookData == 0 {
		return callNextNativeReminderMouseHook(code, message, hookData)
	}
	data := (*nativeLowLevelMouseHook)(unsafe.Pointer(hookData))
	// Once Reminder owns a left-button gesture, resolve its paired events before
	// routing new gestures to SkillBar. This keeps the matching button-up from
	// being intercepted if overlay visibility or overlap changes mid-gesture.
	if nativeReminderHookOwned.Load() {
		switch message {
		case wmMouseMove:
			nativeReminderStoreHookCursor(data.Point)
			signalNativeReminderHookMove()
			// Only the button edges are owned. Let ordinary cursor movement
			// continue downstream; without LDown it cannot become a game drag.
			return callNextNativeReminderMouseHook(code, message, hookData)
		case wmLButtonDown:
			return 1
		case wmLButtonUp:
			if nativeReminderHookOwned.CompareAndSwap(true, false) {
				nativeReminderStoreHookCursor(data.Point)
				nativeReminderQueueHookEdge(nativeReminderHookEdge{
					Point: data.Point, Epoch: nativeReminderHookCancelEpoch.Load(),
				})
				return 1
			}
		}
	}
	if nativeSkillBarLowLevelMouseHandled(message, data.Point, data.Flags) {
		return 1
	}
	if message != wmLButtonDown {
		return callNextNativeReminderMouseHook(code, message, hookData)
	}
	downEpoch := nativeReminderHookCancelEpoch.Load()
	if nativeReminderLocked.Load() {
		return callNextNativeReminderMouseHook(code, message, hookData)
	}
	target, ok := nativeReminderHookTargetAt(data.Point, nativeReminderHookHitSnapshot.Load())
	if !ok || !nativeReminderHookOwned.CompareAndSwap(false, true) {
		return callNextNativeReminderMouseHook(code, message, hookData)
	}
	nativeReminderStoreHookCursor(data.Point)
	// Keep the epoch captured before the lock check and ownership CAS. If a
	// concurrent lock/cancel invalidated this down, the worker rejects this old
	// epoch while ownership remains set so the matching LUp is still swallowed.
	nativeReminderQueueHookEdge(nativeReminderHookEdge{
		Down: true, Point: data.Point, Target: target, Epoch: downEpoch,
	})
	if nativeReminderHookDownInvalidated(nativeReminderLocked.Load(), downEpoch, nativeReminderHookCancelEpoch.Load()) {
		signalNativeReminderHookCancel()
	}
	return 1
}

func callNextNativeReminderMouseHook(code, message, hookData uintptr) uintptr {
	result, _, _ := procCallNextHookEx.Call(nativeReminderMouseHook.Load(), code, message, hookData)
	return result
}

func nativeReminderStoreHookCursor(point nativePoint) {
	nativeReminderHookCursorX.Store(point.X)
	nativeReminderHookCursorY.Store(point.Y)
}

func nativeReminderHookCursor() nativePoint {
	return nativePoint{X: nativeReminderHookCursorX.Load(), Y: nativeReminderHookCursorY.Load()}
}

func signalNativeReminderHookMove() {
	select {
	case nativeReminderHookMoveWake <- struct{}{}:
	default:
	}
}

func signalNativeReminderHookCancel() {
	select {
	case nativeReminderHookCancelWake <- struct{}{}:
	default:
	}
}

func nativeReminderQueueHookEdge(event nativeReminderHookEdge) {
	select {
	case nativeReminderHookEdgeQueue <- event:
	default:
		// The hook must never block. Invalidating the epoch makes every queued
		// edge from this gesture stale; the worker then restores the starting
		// position instead of committing a gesture with a missing edge.
		nativeReminderHookCancelEpoch.Add(1)
		signalNativeReminderHookCancel()
	}
}

func runNativeReminderInputWorker(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	publishNativeReminderHookHitSnapshot()

	var active bool
	var activeEpoch uint64
	for {
		select {
		case <-stop:
			finishNativeReminderDrag(true)
			return
		case <-nativeReminderHookCancelWake:
			currentEpoch := nativeReminderHookCancelEpoch.Load()
			if nativeReminderWorkerGestureIsStale(active, activeEpoch, currentEpoch) {
				finishNativeReminderDrag(true)
				active = false
				activeEpoch = 0
			}
		case event := <-nativeReminderHookEdgeQueue:
			currentEpoch := nativeReminderHookCancelEpoch.Load()
			if event.Epoch != currentEpoch {
				if nativeReminderWorkerGestureIsStale(active, activeEpoch, currentEpoch) {
					finishNativeReminderDrag(true)
					active = false
					activeEpoch = 0
				}
				continue
			}
			if event.Down {
				finishNativeReminderDrag(true)
				if nativeReminderLocked.Load() {
					active = false
					activeEpoch = 0
					continue
				}
				beginNativeReminderDrag(event.Target, event.Point)
				active = true
				activeEpoch = event.Epoch
				if cursor := nativeReminderHookCursor(); cursor != event.Point {
					moveNativeReminderDrag(cursor, true)
				}
				continue
			}
			if !active || activeEpoch != currentEpoch || nativeReminderLocked.Load() {
				finishNativeReminderDrag(true)
				active = false
				activeEpoch = 0
				continue
			}
			moveNativeReminderDrag(event.Point, false)
			finishNativeReminderDrag(false)
			active = false
			activeEpoch = 0
		case <-nativeReminderHookMoveWake:
			if !active {
				continue
			}
			if activeEpoch != nativeReminderHookCancelEpoch.Load() || nativeReminderLocked.Load() {
				finishNativeReminderDrag(true)
				active = false
				activeEpoch = 0
				continue
			}
			moveNativeReminderDrag(nativeReminderHookCursor(), true)
		case <-ticker.C:
			publishNativeReminderHookHitSnapshot()
		}
	}
}

func nativeReminderWorkerGestureIsStale(active bool, activeEpoch, currentEpoch uint64) bool {
	return active && activeEpoch != currentEpoch
}

func nativeReminderHookDownInvalidated(locked bool, downEpoch, currentEpoch uint64) bool {
	return locked || downEpoch != currentEpoch
}

func beginNativeReminderDrag(target nativeReminderDragTarget, point nativePoint) {
	nativeReminderInputMu.Lock()
	nativeReminderDragActive = true
	nativeReminderDragTargetAt = target
	nativeReminderDragStart = point
	nativeReminderInputMu.Unlock()
	publishNativeReminderDrag(target.Kind, target.ID, target.X, target.Y, true, false)
}

func publishNativeReminderHookHitSnapshot() {
	webSnapshot := nativeReminderWebHitSnapshot.Load()
	regionCount := 2
	if webSnapshot != nil {
		regionCount += len(webSnapshot.Regions)
	}
	regions := make([]nativeReminderHitRegion, 0, regionCount)
	if webSnapshot != nil {
		regions = append(regions, webSnapshot.Regions...)
	}
	// Append in reverse hit-test priority because nativeReminderHookTargetAt
	// scans from the end, preserving the previous buff > debuff > WebView order.
	if region, ok := captureNativeWholeWindowHitRegion("debuff", "", debuffOverlayHWND); ok {
		regions = append(regions, region)
	}
	if region, ok := captureNativeWholeWindowHitRegion("buff", "", buffOverlayHWND); ok {
		regions = append(regions, region)
	}
	nativeReminderHookHitSnapshot.Store(&nativeReminderHitSnapshot{Regions: regions})
}

func nativeReminderHookTargetAt(point nativePoint, snapshot *nativeReminderHitSnapshot) (nativeReminderDragTarget, bool) {
	if snapshot == nil {
		return nativeReminderDragTarget{}, false
	}
	for index := len(snapshot.Regions) - 1; index >= 0; index-- {
		region := snapshot.Regions[index]
		if nativePointInRect(point, region.Rect) {
			return nativeReminderDragTarget{Kind: region.Kind, ID: region.ID, X: region.X, Y: region.Y}, true
		}
	}
	return nativeReminderDragTarget{}, false
}

func captureNativeWholeWindowHitRegion(kind, id string, hwnd uintptr) (nativeReminderHitRegion, bool) {
	if hwnd == 0 {
		return nativeReminderHitRegion{}, false
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return nativeReminderHitRegion{}, false
	}
	var rect nativeRect
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return nativeReminderHitRegion{}, false
	}
	return nativeReminderHitRegion{Kind: kind, ID: id, X: int(rect.Left), Y: int(rect.Top), Rect: rect}, true
}

func nativePointInRect(point nativePoint, rect nativeRect) bool {
	return point.X >= rect.Left && point.X < rect.Right && point.Y >= rect.Top && point.Y < rect.Bottom
}

func moveNativeReminderDrag(cursor nativePoint, dragging bool) {
	nativeReminderInputMu.Lock()
	if !nativeReminderDragActive {
		nativeReminderInputMu.Unlock()
		return
	}
	target := nativeReminderDragTargetAt
	start := nativeReminderDragStart
	nativeReminderInputMu.Unlock()
	x := max(-32000, min(32000, target.X+int(cursor.X-start.X)))
	y := max(-32000, min(32000, target.Y+int(cursor.Y-start.Y)))
	publishNativeReminderDrag(target.Kind, target.ID, x, y, dragging, !dragging)
	switch target.Kind {
	case "buff":
		procSetWindowPos.Call(buffOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 0, 0, swpNoSize|swpNoActivate|swpShowWindow)
		publishNativeReminderHookHitSnapshot()
	case "debuff":
		procSetWindowPos.Call(debuffOverlayHWND, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 0, 0, swpNoSize|swpNoActivate|swpShowWindow)
		publishNativeReminderHookHitSnapshot()
	default:
		refreshWebViewReminderHitRegions()
	}
}

func finishNativeReminderDrag(cancel bool) {
	nativeReminderInputMu.Lock()
	if !nativeReminderDragActive {
		nativeReminderInputMu.Unlock()
		return
	}
	target := nativeReminderDragTargetAt
	nativeReminderDragActive = false
	nativeReminderInputMu.Unlock()
	snapshot := currentNativeReminderDrag()
	if cancel {
		snapshot.X, snapshot.Y = target.X, target.Y
	}
	publishNativeReminderDrag(target.Kind, target.ID, snapshot.X, snapshot.Y, false, !cancel)
	if cancel {
		switch target.Kind {
		case "buff":
			procSetWindowPos.Call(buffOverlayHWND, ^uintptr(0), uintptr(int32(target.X)), uintptr(int32(target.Y)), 0, 0, swpNoSize|swpNoActivate|swpShowWindow)
		case "debuff":
			procSetWindowPos.Call(debuffOverlayHWND, ^uintptr(0), uintptr(int32(target.X)), uintptr(int32(target.Y)), 0, 0, swpNoSize|swpNoActivate|swpShowWindow)
		default:
			refreshWebViewReminderHitRegions()
		}
		publishNativeReminderHookHitSnapshot()
		return
	}
	publishNativeReminderHookHitSnapshot()
	switch target.Kind {
	case "buff":
		_ = updateConfig(func(cfg *config) {
			cfg.BuffOverlayX, cfg.BuffOverlayY, cfg.BuffOverlayPositionSet = snapshot.X, snapshot.Y, true
		})
		markNativeReminderDragCommitted(target.Kind, target.ID)
	case "debuff":
		_ = updateConfig(func(cfg *config) {
			cfg.DebuffOverlayX, cfg.DebuffOverlayY, cfg.DebuffOverlayPositionSet = snapshot.X, snapshot.Y, true
		})
		markNativeReminderDragCommitted(target.Kind, target.ID)
	default:
		queueNativeReminderPositionUpdate(nativeReminderPositionUpdate{Kind: target.Kind, ID: target.ID, X: snapshot.X, Y: snapshot.Y})
	}
}

func nativeReminderRegionForSkill(item nativeSkillOverlayItem, iconSize int) nativeReminderHitRegion {
	return nativeReminderHitRegion{Kind: "skill", ID: strconv.Itoa(int(item.SkillID)), X: item.X, Y: item.Y,
		Rect: nativeRect{Left: int32(item.X), Top: int32(item.Y), Right: int32(item.X + iconSize), Bottom: int32(item.Y + iconSize + 22)}}
}

// refreshWebViewReminderHitRegions keeps the unlock-and-drag hit testing from
// the native experiment without drawing any native pixels. All visible content
// is still rendered by the transparent WebView; this function only mirrors its
// absolute screen rectangles for the low-level mouse hook.
func refreshWebViewReminderHitRegions() {
	if skillOverlayHWND == 0 {
		setNativeReminderHitRegions(nil)
		return
	}
	skillOverlayState.RLock()
	data := append([]byte(nil), skillOverlayState.data...)
	skillOverlayState.RUnlock()
	var message nativeSkillOverlayMessage
	if json.Unmarshal(data, &message) != nil {
		setNativeReminderHitRegions(nil)
		return
	}
	applyNativeReminderDragOverride(&message)
	nowMs := time.Now().UnixMilli()
	if !nativeSkillOverlayMessageVisible(message, nowMs) {
		setNativeReminderHitRegions(nil)
		return
	}
	scalePercent := nativeReminderScalePercent(skillOverlayHWND, message.Settings.DPIPercent)
	dpiScale := float64(scalePercent) / 100
	iconSize := max(24, int(math.Ceil(float64(max(24, message.Settings.IconSize))*dpiScale)))
	regions := make([]nativeReminderHitRegion, 0, len(message.Items)+len(message.EffectTimers)+len(message.Mechanics)+len(message.StackAlerts)+2)
	for _, item := range message.Items {
		if nativeSkillReminderItemVisible(item, nowMs) {
			regions = append(regions, nativeReminderRegionForSkill(item, iconSize))
		}
	}
	if aim := message.AimReminder; aim != nil && (aim.Active || aim.AlwaysVisible) {
		factor := dpiScale * float64(max(50, min(200, aim.ScalePercent))) / 100
		width, height := max(146, int(math.Ceil(292*factor))), max(32, int(math.Ceil(64*factor)))
		regions = append(regions, nativeReminderHitRegion{Kind: "aim", ID: "aim", X: aim.X, Y: aim.Y,
			Rect: nativeRect{Left: int32(aim.X), Top: int32(aim.Y), Right: int32(aim.X + width), Bottom: int32(aim.Y + height)}})
	}
	if target := message.TargetHealth; target != nil && target.MaximumHealth > 0 && target.CurrentHealth >= 0 &&
		(target.PreviewEndsAt == 0 || target.PreviewEndsAt > nowMs) {
		factor := dpiScale * float64(max(50, min(200, target.ScalePercent))) / 100
		width, height := max(210, int(math.Ceil(420*factor))), max(19, int(math.Ceil(38*factor)))
		regions = append(regions, nativeReminderHitRegion{Kind: "target-health", ID: "miel-shard", X: target.X, Y: target.Y,
			Rect: nativeRect{Left: int32(target.X), Top: int32(target.Y), Right: int32(target.X + width), Bottom: int32(target.Y + height)}})
	}
	for _, item := range message.EffectTimers {
		if !item.Enabled || (!item.AlwaysVisible && item.EndsAtMs <= nowMs) {
			continue
		}
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		baseWidth, baseHeight := 292, 64
		if item.Orientation == "vertical" {
			baseWidth, baseHeight = 72, 220
		}
		width, height := max(36, int(math.Ceil(float64(baseWidth)*factor))), max(32, int(math.Ceil(float64(baseHeight)*factor)))
		regions = append(regions, nativeReminderHitRegion{Kind: "effect", ID: item.Key, X: item.X, Y: item.Y,
			Rect: nativeRect{Left: int32(item.X), Top: int32(item.Y), Right: int32(item.X + width), Bottom: int32(item.Y + height)}})
	}
	for _, item := range message.Mechanics {
		if item.EndsAtMs <= nowMs {
			continue
		}
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		size := max(59, int(math.Ceil(118*factor)))
		regions = append(regions, nativeReminderHitRegion{Kind: "mechanic", ID: item.Key, X: item.X, Y: item.Y,
			Rect: nativeRect{Left: int32(item.X), Top: int32(item.Y), Right: int32(item.X + size), Bottom: int32(item.Y + size)}})
	}
	for _, item := range message.StackAlerts {
		if !item.Persistent && item.EndsAtMs <= nowMs {
			continue
		}
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		width, height := max(110, int(math.Ceil(220*factor))), max(48, int(math.Ceil(96*factor)))
		kind, id := nativeStackReminderIdentity(item)
		regions = append(regions, nativeReminderHitRegion{Kind: kind, ID: id, X: item.X, Y: item.Y,
			Rect: nativeRect{Left: int32(item.X), Top: int32(item.Y), Right: int32(item.X + width), Bottom: int32(item.Y + height)}})
	}
	setNativeReminderHitRegions(regions)
}
