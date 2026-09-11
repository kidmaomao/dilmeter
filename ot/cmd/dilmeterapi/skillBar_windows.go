//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	skillBarDefaultX      = 600
	skillBarDefaultY      = 520
	skillBarDefaultWidth  = 520
	skillBarDefaultHeight = 58

	inputKeyboard        = 1
	mapvkVKToVSC         = 0
	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002
	keyeventfScanCode    = 0x0008

	wsPopup = 0x80000000

	wmActivate       = 0x0006
	wmNcHitTest      = 0x0084
	wmMouseActivate  = 0x0021
	wmCancelMode     = 0x001F
	wmKillFocus      = 0x0008
	wmQuit           = 0x0012
	wmMove           = 0x0003
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmRButtonDown    = 0x0204
	wmMouseWheel     = 0x020A
	wmMouseHWheel    = 0x020E
	wmCaptureChanged = 0x0215
	wmExitSizeMove   = 0x0232

	wmSkillBarRender   = 0x8053 // WM_APP + 0x53
	wmSkillBarApply    = 0x8054 // WM_APP + 0x54
	wmSkillBarVisible  = 0x8055 // WM_APP + 0x55
	wmSkillBarShutdown = 0x8056 // WM_APP + 0x56
	wmSkillBarDragMove = 0x8057 // WM_APP + 0x57
	wmSkillBarHookLost = 0x8058 // WM_APP + 0x58

	htClient     = 1
	maActivate   = 1
	maNoActivate = 3
	waInactive   = 0
	idcArrow     = 32512

	skillBarMaxSlots         = 48
	skillBarMaxSequenceSteps = 8
	skillBarMaxChordKeys     = 4
	skillBarChordHold        = 20 * time.Millisecond
	// The reference overlay presses a configurable stop key as soon as a
	// left-button skill cell is hit, then releases it 20ms later. Mabinogi can
	// read the physical left button through an input path that is not cancelled
	// by returning a non-zero value from WH_MOUSE_LL, so the short key tap is the
	// part that actually cancels point-to-move.
	skillBarStopKeyHold          = 20 * time.Millisecond
	skillBarStopKeyRetryDelay    = 10 * time.Millisecond
	skillBarStopKeyRetryPause    = 250 * time.Millisecond
	skillBarStopKeyRetryAttempts = 8
	skillBarStopKeyRetryPasses   = 2
	// The reference program's skill-tab path waits 30ms after releasing the tab
	// chord before pressing the skill key (for example Ctrl+[ -> Q).
	skillBarSequenceDelay    = 30 * time.Millisecond
	skillBarDragThreshold    = 6
	skillBarInjectedTag      = 0x5B17BA12
	skillBarMouseButtonLeft  = 0x01
	skillBarMouseButtonRight = 0x02
	skillBarExternalFocusTTL = 1500 * time.Millisecond
	skillBarForegroundSettle = 60 * time.Millisecond
)

const (
	nativeSkillBarForegroundNull uint8 = iota
	nativeSkillBarForegroundSource
	nativeSkillBarForegroundTarget
	nativeSkillBarForegroundThirdParty
)

const (
	nativeSkillBarHookDown uint8 = iota + 1
	nativeSkillBarHookMove
	nativeSkillBarHookUp
)

var (
	procMapVirtualKeyW   = user32.NewProc("MapVirtualKeyW")
	procSendInput        = user32.NewProc("SendInput")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procIsWindowVisible  = user32.NewProc("IsWindowVisible")
	procSetCapture       = user32.NewProc("SetCapture")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")
	procGetCapture       = user32.NewProc("GetCapture")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procLoadCursorW      = user32.NewProc("LoadCursorW")

	skillBarHWND           atomic.Uintptr
	skillBarActive         atomic.Bool
	skillBarVisible        atomic.Bool
	skillBarLocked         atomic.Bool
	skillBarInputEnabled   atomic.Bool
	skillBarClickRight     atomic.Bool
	skillBarClosing        atomic.Bool
	skillBarX              atomic.Int32
	skillBarY              atomic.Int32
	skillBarWidth          atomic.Int32
	skillBarHeight         atomic.Int32
	skillBarStateSeq       atomic.Int64
	skillBarMouseDown      atomic.Int32
	skillBarDragPending    atomic.Bool
	skillBarDragging       atomic.Bool
	skillBarDragLogged     atomic.Bool
	skillBarDragGeneration atomic.Uint64
	skillBarPositionGen    atomic.Uint64
	skillBarDragMoveX      atomic.Int32
	skillBarDragMoveY      atomic.Int32
	skillBarDragMovePosted atomic.Bool
	skillBarSending        atomic.Bool
	skillBarRenderPosted   atomic.Bool
	skillBarApplyPosted    atomic.Bool
	skillBarVisiblePosted  atomic.Bool
	skillBarDesiredVisible atomic.Bool
	skillBarWindowPressBtn atomic.Uint32
	skillBarGestureActive  atomic.Bool
	skillBarGestureOverlap atomic.Bool
	skillBarGestureRestore atomic.Uintptr
	skillBarGestureGame    atomic.Uintptr
	skillBarGestureStopKey atomic.Bool
	skillBarPendingFocus   atomic.Uintptr
	skillBarPendingFocusAt atomic.Int64
	skillBarRecoveryTarget atomic.Uintptr
	skillBarSession        atomic.Uint64
	skillBarWindowSession  atomic.Uint64

	skillBarActivationQueued     atomic.Uint64
	skillBarActivationSucceeded  atomic.Uint64
	skillBarActivationFailed     atomic.Uint64
	skillBarHookDownSwallowed    atomic.Uint64
	skillBarHookUpSwallowed      atomic.Uint64
	skillBarHookEventDropped     atomic.Uint64
	skillBarHookCriticalDropped  atomic.Uint64
	skillBarHookRecoveryVersion  atomic.Uint64
	skillBarHookOwnedButtons     atomic.Uint32
	skillBarHookBarrierRect      atomic.Pointer[nativeRect]
	skillBarHookBarrierEnabled   atomic.Bool
	skillBarHookOperational      atomic.Bool
	skillBarHookCursorX          atomic.Int32
	skillBarHookCursorY          atomic.Int32
	skillBarHookMoveQueued       atomic.Bool
	skillBarHookStopSnapshot     atomic.Pointer[nativeSkillBarStopSnapshot]
	skillBarForegroundGame       atomic.Uintptr
	skillBarStopOutstanding      atomic.Pointer[nativeSkillBarStopRelease]
	skillBarStopKeyDownSent      atomic.Uint64
	skillBarStopKeyUpSent        atomic.Uint64
	skillBarStopKeyFailed        atomic.Uint64
	skillBarDragStarted          atomic.Uint64
	skillBarDragCompleted        atomic.Uint64
	skillBarWindowMu             sync.Mutex
	skillBarLifecycleMu          sync.Mutex
	skillBarFocusMu              sync.Mutex
	skillBarLastExternal         uintptr
	skillBarLastExternalAt       int64
	skillBarWindowDone           chan struct{}
	skillBarActivationCh         = make(chan nativeSkillBarActivationRequest, 64)
	skillBarActivationWorkerOnce sync.Once
	skillBarHookEventCh          = make(chan nativeSkillBarHookEvent, 64)
	skillBarHookWorkerOnce       sync.Once
	skillBarStopReleaseCh        = make(chan *nativeSkillBarStopRelease, 64)
	skillBarStopWorkerOnce       sync.Once
	skillBarDragMu               sync.Mutex
	skillBarDragStart            nativePoint
	skillBarDragRect             nativeRect
	skillBarSettingsMu           sync.RWMutex
	skillBarSettings             nativeSkillBarSettings
	skillBarWndProc              = syscall.NewCallback(skillBarWindowProc)
)

type nativeSkillBarActivationRequest struct {
	Index      int
	TargetHWND uintptr
	SourceHWND uintptr
	Button     uint32
	Session    uint64
}

type nativeSkillBarHookEvent struct {
	Kind            uint8
	Button          uint32
	Point           nativePoint
	Foreground      uintptr
	Session         uint64
	RecoveryVersion uint64
	StopKeySent     bool
}

type nativeSkillBarStopSnapshot struct {
	Code string
	Down nativeSkillBarInput
	Up   nativeSkillBarInput
}

type nativeSkillBarStopRelease struct {
	Input     nativeSkillBarInput
	ReleaseAt time.Time
	Passes    int
	Released  atomic.Bool
	mu        sync.Mutex
}

type nativeSkillBarFocusSnapshot struct {
	PendingFocus   uintptr
	CurrentFocus   uintptr
	SkillBar       uintptr
	RecentExternal uintptr
	RecoveryTarget uintptr
}

type nativeWindowMessage struct {
	HWND     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Point    nativePoint
	LPrivate uint32
}

type nativeSkillBarSlot struct {
	SkillID     uint16     `json:"skillId"`
	KeyCode     string     `json:"keyCode"`
	KeyLabel    string     `json:"keyLabel"`
	KeySequence [][]string `json:"keySequence,omitempty"`
}

type nativeSkillBarSettings struct {
	Active          bool                 `json:"active"`
	Locked          bool                 `json:"locked"`
	InputEnabled    bool                 `json:"inputEnabled"`
	ClickButton     string               `json:"clickButton"`
	StopMovement    bool                 `json:"stopMovement"`
	StopKeyCode     string               `json:"stopKeyCode"`
	StrongIsolation bool                 `json:"strongIsolation,omitempty"` // v1.4 test migration only
	X               int                  `json:"x"`
	Y               int                  `json:"y"`
	Width           int                  `json:"width"`
	Height          int                  `json:"height"`
	Columns         int                  `json:"columns"`
	IconSize        int                  `json:"iconSize"`
	Gap             int                  `json:"gap"`
	Opacity         int                  `json:"opacity"`
	Slots           []nativeSkillBarSlot `json:"slots"`
	PositionSet     bool                 `json:"positionSet"`
	Sequence        int64                `json:"sequence,omitempty"`
}

func defaultNativeSkillBarSettings(cfg config) nativeSkillBarSettings {
	x, y := skillBarDefaultX, skillBarDefaultY
	if cfg.SkillBarPositionSet {
		x, y = cfg.SkillBarX, cfg.SkillBarY
	}
	return nativeSkillBarSettings{
		Locked: true, ClickButton: "left", StopMovement: false, StopKeyCode: "",
		X: x, Y: y, PositionSet: cfg.SkillBarPositionSet,
		Width: skillBarDefaultWidth, Height: skillBarDefaultHeight,
		Columns: 10, IconSize: 48, Gap: 3, Opacity: 100,
	}
}

func initializeSkillBar(cfg config) error {
	skillBarLifecycleMu.Lock()
	defer skillBarLifecycleMu.Unlock()
	if skillBarHWND.Load() != 0 {
		return nil
	}
	// Arm failure detection before the availability check. If the shared hook
	// exits anywhere after this point, its owner thread can atomically mark the
	// SkillBar unavailable instead of racing with a later unconditional true.
	skillBarHookOperational.Store(true)
	if !nativeLowLevelMouseHookAvailable() {
		skillBarHookOperational.Store(false)
		return errors.New("shared low-level mouse hook is unavailable")
	}
	skillBarSession.Add(1)
	skillBarClosing.Store(false)
	skillBarVisible.Store(false)
	settings := defaultNativeSkillBarSettings(cfg)
	skillBarSettingsMu.Lock()
	skillBarSettings = settings
	skillBarSettingsMu.Unlock()
	skillBarLocked.Store(true)
	skillBarClickRight.Store(false)
	skillBarX.Store(int32(settings.X))
	skillBarY.Store(int32(settings.Y))
	skillBarWidth.Store(int32(settings.Width))
	skillBarHeight.Store(int32(settings.Height))
	publishNativeSkillBarHookBarrierRect()
	skillBarMouseDown.Store(-1)
	skillBarDragPending.Store(false)
	skillBarDragging.Store(false)
	skillBarDragLogged.Store(false)
	skillBarDragGeneration.Store(0)
	skillBarPositionGen.Add(1)
	skillBarDragMovePosted.Store(false)
	skillBarActivationQueued.Store(0)
	skillBarActivationSucceeded.Store(0)
	skillBarActivationFailed.Store(0)
	skillBarHookDownSwallowed.Store(0)
	skillBarHookUpSwallowed.Store(0)
	skillBarHookEventDropped.Store(0)
	skillBarHookCriticalDropped.Store(0)
	skillBarHookOwnedButtons.Store(0)
	skillBarHookBarrierEnabled.Store(false)
	skillBarHookMoveQueued.Store(false)
	skillBarDragStarted.Store(0)
	skillBarDragCompleted.Store(0)
	skillBarRenderPosted.Store(false)
	skillBarApplyPosted.Store(false)
	skillBarVisiblePosted.Store(false)
	skillBarDesiredVisible.Store(false)
	skillBarWindowPressBtn.Store(0)
	skillBarGestureActive.Store(false)
	skillBarGestureOverlap.Store(false)
	skillBarGestureRestore.Store(0)
	skillBarGestureGame.Store(0)
	skillBarGestureStopKey.Store(false)
	skillBarPendingFocus.Store(0)
	skillBarPendingFocusAt.Store(0)
	skillBarRecoveryTarget.Store(0)
	skillBarForegroundGame.Store(0)
	skillBarStopOutstanding.Store(nil)
	skillBarStopKeyDownSent.Store(0)
	skillBarStopKeyUpSent.Store(0)
	skillBarStopKeyFailed.Store(0)
	skillBarHookStopSnapshot.Store(nil)
	clearNativeSkillBarExternalForeground()

	if err := startNativeSkillBarWindowThread(settings); err != nil {
		skillBarHookOperational.Store(false)
		return err
	}
	if !nativeLowLevelMouseHookAvailable() || !skillBarHookOperational.Load() {
		stopNativeSkillBarWindowThread()
		return errors.New("shared low-level mouse hook stopped during SkillBar initialization")
	}
	renderNativeSkillBar()
	skillBarActivationWorkerOnce.Do(func() { go runNativeSkillBarActivationQueue() })
	skillBarHookWorkerOnce.Do(func() { go runNativeSkillBarHookEventQueue() })
	skillBarStopWorkerOnce.Do(func() { go runNativeSkillBarStopReleaseQueue() })
	go monitorSkillBarForeground(skillBarHWND.Load())
	return nil
}

func shutdownSkillBar() {
	skillBarLifecycleMu.Lock()
	defer skillBarLifecycleMu.Unlock()
	skillBarClosing.Store(true)
	skillBarSession.Add(1)
	skillBarActive.Store(false)
	skillBarDesiredVisible.Store(false)
	// A successful synthetic key-down must always have a best-effort matching
	// key-up, even if shutdown races the delayed release worker.
	releaseNativeSkillBarOutstandingStopKey()
	skillBarForegroundGame.Store(0)
	skillBarHookStopSnapshot.Store(nil)
	clearNativeSkillBarExternalForeground()
	stopNativeSkillBarWindowThread()
	skillBarVisible.Store(false)
	logger.Printf(
		"native skill bar session summary: queued=%d sent=%d failed=%d hook_down=%d hook_up=%d hook_dropped=%d hook_critical_dropped=%d stop_down=%d stop_up=%d stop_failed=%d drags=%d/%d",
		skillBarActivationQueued.Load(),
		skillBarActivationSucceeded.Load(), skillBarActivationFailed.Load(),
		skillBarHookDownSwallowed.Load(), skillBarHookUpSwallowed.Load(), skillBarHookEventDropped.Load(), skillBarHookCriticalDropped.Load(),
		skillBarStopKeyDownSent.Load(), skillBarStopKeyUpSent.Load(), skillBarStopKeyFailed.Load(),
		skillBarDragCompleted.Load(), skillBarDragStarted.Load(),
	)
}

// The SkillBar owns a dedicated Win32 GUI thread. Its input, movement,
// layered-window rendering and destruction are therefore serialized by one
// message queue instead of competing with the WebView and HTTP goroutines.
func startNativeSkillBarWindowThread(settings nativeSkillBarSettings) error {
	skillBarWindowMu.Lock()
	if skillBarHWND.Load() != 0 {
		skillBarWindowMu.Unlock()
		return nil
	}
	ready := make(chan error, 1)
	done := make(chan struct{})
	skillBarWindowDone = done
	skillBarWindowMu.Unlock()

	session := skillBarSession.Load()
	go runNativeSkillBarWindowThread(settings, session, ready, done)
	return <-ready
}

func runNativeSkillBarWindowThread(settings nativeSkillBarSettings, session uint64, ready chan<- error, done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)

	var module windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &module); err != nil {
		ready <- err
		return
	}
	className, _ := windows.UTF16PtrFromString("DilmeterNativeSkillBarWindow")
	windowName, _ := windows.UTF16PtrFromString("Dilmeter Native Skill Bar")
	arrowCursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	class := trayWndClassEx{
		CbSize:        uint32(unsafe.Sizeof(trayWndClassEx{})),
		LpfnWndProc:   skillBarWndProc,
		HInstance:     uintptr(module),
		HCursor:       arrowCursor,
		LpszClassName: className,
	}
	registered, _, registerErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if registered == 0 && registerErr != nil && !errors.Is(registerErr, syscall.Errno(1410)) { // ERROR_CLASS_ALREADY_EXISTS
		ready <- fmt.Errorf("register native skill bar window: %w", registerErr)
		return
	}
	hwnd, _, createErr := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow|wsExLayered|wsExNoActivate,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsPopup,
		uintptr(int32(settings.X)), uintptr(int32(settings.Y)), uintptr(settings.Width), uintptr(settings.Height),
		0, 0, uintptr(module), 0,
	)
	if hwnd == 0 {
		ready <- fmt.Errorf("create native skill bar window: %w", createErr)
		return
	}
	skillBarWindowSession.Store(session)
	skillBarHWND.Store(hwnd)
	applyNativeSkillBarExtendedStyleNow(hwnd)
	publishNativeSkillBarHookBarrierRectFromWindow(hwnd)
	procShowWindow.Call(hwnd, swHide)
	appliedExStyle, _, _ := procGetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlExStyle))
	logger.Printf(
		"native skill bar window ready: hwnd=%#x thread=%d input=nontransparent-window+low-level-hook-barrier exstyle=%#x transparent=%v",
		hwnd, windows.GetCurrentThreadId(), appliedExStyle, appliedExStyle&wsExTransparent != 0,
	)
	ready <- nil

	var message nativeWindowMessage
	for {
		result, _, messageErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			logger.Println("native skill bar window message loop failed:", messageErr)
			break
		}
		if result == 0 || message.Message == wmQuit {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	// Fail closed even when GetMessage exits abnormally: hide the visible HWND
	// before disabling the low-level barrier.
	procShowWindow.Call(hwnd, swHide)
	skillBarVisible.Store(false)
	skillBarHookBarrierEnabled.Store(false)
	skillBarHWND.CompareAndSwap(hwnd, 0)
	skillBarWindowSession.CompareAndSwap(session, 0)
	releaseNativeSkillBarSurface()
	logger.Printf("native skill bar window stopped: hwnd=%#x", hwnd)
}

func stopNativeSkillBarWindowThread() {
	skillBarWindowMu.Lock()
	done := skillBarWindowDone
	skillBarWindowMu.Unlock()
	if hwnd := skillBarHWND.Load(); hwnd != 0 {
		procPostMessageW.Call(hwnd, wmSkillBarShutdown, uintptr(skillBarWindowSession.Load()), 0)
	}
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		logger.Println("native skill bar window did not stop within 2 seconds")
	}
}

func handleSkillBar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if skillBarHWND.Load() == 0 || skillBarClosing.Load() {
		http.Error(w, "native skill bar is unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeSkillBarState(w)
	case http.MethodPut:
		var request nativeSkillBarSettings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&request); err != nil {
			http.Error(w, "invalid native skill bar settings", http.StatusBadRequest)
			return
		}
		if request.Sequence > 0 && request.Sequence <= skillBarStateSeq.Load() {
			writeSkillBarState(w)
			return
		}
		settings := normalizeNativeSkillBarSettings(request, currentNativeSkillBarSettings())
		if request.Sequence > 0 {
			skillBarStateSeq.Store(request.Sequence)
		}
		applyNativeSkillBarSettings(settings)
		writeSkillBarState(w)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func normalizeNativeSkillBarSettings(request, previous nativeSkillBarSettings) nativeSkillBarSettings {
	if request.ClickButton == "right" {
		request.ClickButton = "right"
	} else {
		request.ClickButton = "left"
	}
	// Match the reference overlay's optional compensation without copying its
	// unsafe W default: only a user-selected, allow-listed key may cancel the
	// point-to-move side effect. Right-button activation keeps the preference but
	// never sends the stop key.
	request.StopKeyCode = strings.TrimSpace(request.StopKeyCode)
	if _, ok := skillBarStopKeyForCode(request.StopKeyCode); !ok {
		request.StopKeyCode = ""
		request.StopMovement = false
	}
	request.StrongIsolation = false
	request.X = max(-32000, min(32000, request.X))
	request.Y = max(-32000, min(32000, request.Y))
	request.Columns = max(1, min(12, request.Columns))
	request.IconSize = max(32, min(80, request.IconSize))
	request.Gap = max(0, min(12, request.Gap))
	request.Opacity = max(25, min(100, request.Opacity))
	if len(request.Slots) > skillBarMaxSlots {
		request.Slots = request.Slots[:skillBarMaxSlots]
	}
	for index := range request.Slots {
		slot := &request.Slots[index]
		slot.KeyCode = strings.TrimSpace(slot.KeyCode)
		if _, ok := skillBarKeyForCode(slot.KeyCode); !ok {
			slot.KeyCode = ""
		}
		slot.KeySequence = normalizeNativeSkillBarKeySequence(slot.KeySequence)
		if len(slot.KeySequence) == 0 && slot.KeyCode != "" {
			slot.KeySequence = [][]string{{slot.KeyCode}}
		}
		if len(slot.KeySequence) == 1 && len(slot.KeySequence[0]) == 1 {
			slot.KeyCode = slot.KeySequence[0][0]
		} else {
			slot.KeyCode = ""
		}
		slot.KeyLabel = sanitizeNativeSkillBarLabel(slot.KeyLabel)
	}
	if len(request.Slots) == 0 && request.Active {
		request.Active = false
	}
	padding := nativeSkillBarPadding(request.Locked)
	columns := max(1, min(request.Columns, max(1, len(request.Slots))))
	rows := max(1, (len(request.Slots)+columns-1)/columns)
	request.Width = padding*2 + columns*request.IconSize + max(0, columns-1)*request.Gap
	request.Height = padding*2 + rows*request.IconSize + max(0, rows-1)*request.Gap
	if request.Width <= 0 || request.Height <= 0 {
		request.Width, request.Height = previous.Width, previous.Height
	}
	request.Sequence = 0
	request.PositionSet = true
	return request
}

func normalizeNativeSkillBarKeySequence(value [][]string) [][]string {
	sequence := make([][]string, 0, min(len(value), skillBarMaxSequenceSteps))
	for _, rawChord := range value {
		if len(sequence) >= skillBarMaxSequenceSteps {
			break
		}
		chord := make([]string, 0, min(len(rawChord), skillBarMaxChordKeys))
		seen := make(map[string]struct{}, skillBarMaxChordKeys)
		for _, rawCode := range rawChord {
			if len(chord) >= skillBarMaxChordKeys {
				break
			}
			code := strings.TrimSpace(rawCode)
			if _, ok := skillBarKeyForCode(code); !ok {
				continue
			}
			if _, duplicate := seen[code]; duplicate {
				continue
			}
			seen[code] = struct{}{}
			chord = append(chord, code)
		}
		if len(chord) > 0 {
			sequence = append(sequence, chord)
		}
	}
	return sequence
}

func sanitizeNativeSkillBarLabel(value string) string {
	value = strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return -1
		}
		return character
	}, strings.TrimSpace(value))
	characters := []rune(value)
	if len(characters) > 48 {
		characters = characters[:48]
	}
	return string(characters)
}

func nativeSkillBarPadding(locked bool) int {
	if locked {
		return 3
	}
	return 6
}

func applyNativeSkillBarSettings(settings nativeSkillBarSettings) {
	// Invalidate any queued drag move before publishing a settings position.
	// The GUI owner thread rejects moves from older generations.
	skillBarPositionGen.Add(1)
	skillBarSettingsMu.Lock()
	skillBarSettings = settings
	skillBarSettingsMu.Unlock()
	skillBarActive.Store(settings.Active)
	skillBarLocked.Store(settings.Locked)
	skillBarInputEnabled.Store(settings.InputEnabled)
	skillBarClickRight.Store(settings.ClickButton == "right")
	skillBarX.Store(int32(settings.X))
	skillBarY.Store(int32(settings.Y))
	skillBarWidth.Store(int32(settings.Width))
	skillBarHeight.Store(int32(settings.Height))
	publishNativeSkillBarStopSnapshot(settings)
	logger.Printf(
		"native skill bar settings applied: active=%v locked=%v input=%v click_button=%s stop_movement=%v stop_key=%q stop_ready=%v rect=(%d,%d %dx%d) slots=%d",
		settings.Active, settings.Locked, settings.InputEnabled, settings.ClickButton,
		settings.StopMovement, settings.StopKeyCode, skillBarHookStopSnapshot.Load() != nil,
		settings.X, settings.Y, settings.Width, settings.Height, len(settings.Slots),
	)
	// The SkillBar itself is interactive UI. Even when input is disabled it
	// must consume clicks so they never become ground clicks in Client.exe.
	applyNativeSkillBarExtendedStyle()
	renderNativeSkillBar()
	if err := updateConfig(func(cfg *config) {
		cfg.SkillBarX = settings.X
		cfg.SkillBarY = settings.Y
		cfg.SkillBarPositionSet = true
	}); err != nil {
		logger.Println("save native skill bar settings position failed:", err)
	}
}

func currentNativeSkillBarSettings() nativeSkillBarSettings {
	skillBarSettingsMu.RLock()
	settings := skillBarSettings
	settings.Slots = cloneNativeSkillBarSlots(settings.Slots)
	skillBarSettingsMu.RUnlock()
	settings.X = int(skillBarX.Load())
	settings.Y = int(skillBarY.Load())
	settings.Width = int(skillBarWidth.Load())
	settings.Height = int(skillBarHeight.Load())
	return settings
}

func cloneNativeSkillBarSlots(slots []nativeSkillBarSlot) []nativeSkillBarSlot {
	cloned := make([]nativeSkillBarSlot, len(slots))
	copy(cloned, slots)
	for index := range cloned {
		cloned[index].KeySequence = make([][]string, len(slots[index].KeySequence))
		for step := range slots[index].KeySequence {
			cloned[index].KeySequence[step] = append([]string(nil), slots[index].KeySequence[step]...)
		}
	}
	return cloned
}

func writeSkillBarState(w http.ResponseWriter) {
	settings := currentNativeSkillBarSettings()
	settings.Active = skillBarActive.Load()
	settings.Locked = skillBarLocked.Load()
	settings.InputEnabled = skillBarInputEnabled.Load()
	_ = json.NewEncoder(w).Encode(settings)
}

func applyNativeSkillBarExtendedStyle() {
	hwnd := skillBarHWND.Load()
	if hwnd == 0 {
		return
	}
	if !skillBarApplyPosted.CompareAndSwap(false, true) {
		return
	}
	if posted, _, _ := procPostMessageW.Call(hwnd, wmSkillBarApply, uintptr(skillBarWindowSession.Load()), 0); posted == 0 {
		skillBarApplyPosted.Store(false)
	}
}

func applyNativeSkillBarExtendedStyleNow(hwnd uintptr) {
	if hwnd == 0 || hwnd != skillBarHWND.Load() {
		return
	}
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlExStyle))
	// Match the reference program's interactive model: keep NOACTIVATE so the
	// game remains the keyboard foreground, but clear TRANSPARENT so the HWND is
	// also the normal hit-test target. The shared WH_MOUSE_LL barrier is a second
	// layer that owns complete button pairs before Client.exe sees them.
	exStyle = nativeSkillBarInteractiveExStyle(exStyle)
	procSetWindowLongW.Call(hwnd, signedWindowLongIndex(gwlExStyle), exStyle)
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpFrameChanged)
}

func nativeSkillBarInteractiveExStyle(exStyle uintptr) uintptr {
	exStyle &^= wsExTransparent
	exStyle |= wsExTopmost | wsExToolWindow | wsExLayered | wsExNoActivate
	return exStyle
}

func skillBarWindowProc(hwnd, message, wParam, lParam uintptr) uintptr {
	if message >= wmSkillBarRender && message <= wmSkillBarShutdown &&
		(uint64(wParam) != skillBarWindowSession.Load() || hwnd != skillBarHWND.Load()) {
		return 0
	}
	if message == wmSkillBarHookLost &&
		(uint64(wParam) != skillBarWindowSession.Load() || hwnd != skillBarHWND.Load()) {
		return 0
	}
	switch message {
	case wmSkillBarRender:
		skillBarRenderPosted.Store(false)
		renderNativeSkillBarNow(hwnd)
		return 0
	case wmSkillBarApply:
		skillBarApplyPosted.Store(false)
		cancelNativeSkillBarWindowGesture(hwnd, "settings changed")
		applyNativeSkillBarExtendedStyleNow(hwnd)
		settings := currentNativeSkillBarSettings()
		desiredRect := nativeRect{
			Left: int32(settings.X), Top: int32(settings.Y),
			Right: int32(settings.X + settings.Width), Bottom: int32(settings.Y + settings.Height),
		}
		// Keep both the old and requested positions protected for the tiny
		// interval in which SetWindowPos moves the HWND.  Consuming a click in
		// the union is safer than ever exposing a visible pixel to Client.exe.
		publishNativeSkillBarHookBarrierTransition(desiredRect)
		moved, _, moveErr := procSetWindowPos.Call(
			hwnd, ^uintptr(0), uintptr(int32(settings.X)), uintptr(int32(settings.Y)),
			uintptr(settings.Width), uintptr(settings.Height), swpNoActivate|swpFrameChanged,
		)
		if moved == 0 {
			logger.Println("native skill bar apply SetWindowPos failed:", moveErr)
		}
		publishNativeSkillBarHookBarrierRectFromWindow(hwnd)
		renderNativeSkillBarNow(hwnd)
		return 0
	case wmSkillBarVisible:
		skillBarVisiblePosted.Store(false)
		if skillBarDesiredVisible.Load() && !skillBarClosing.Load() {
			publishNativeSkillBarHookBarrierRectFromWindow(hwnd)
			// Fail closed: arm the barrier before the first visible frame.
			skillBarHookBarrierEnabled.Store(true)
			procShowWindow.Call(hwnd, swShowNoActivate)
			procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
			skillBarVisible.Store(true)
			renderNativeSkillBarNow(hwnd)
		} else {
			cancelNativeSkillBarWindowGesture(hwnd, "window hidden")
			procShowWindow.Call(hwnd, swHide)
			skillBarVisible.Store(false)
			// Hide first, then disarm. There is never a visible-but-unprotected
			// frame even when Active changes on another goroutine.
			skillBarHookBarrierEnabled.Store(false)
		}
		return 0
	case wmSkillBarShutdown:
		cancelNativeSkillBarWindowGesture(hwnd, "shutdown")
		procShowWindow.Call(hwnd, swHide)
		skillBarVisible.Store(false)
		skillBarHookBarrierEnabled.Store(false)
		procDestroyWindow.Call(hwnd)
		return 0
	case wmSkillBarDragMove:
		skillBarDragMovePosted.Store(false)
		generation := uint64(wParam)
		if !nativeSkillBarDragMoveCurrent(generation) {
			return 0
		}
		// Always consume the latest coalesced target. An older PostMessage may
		// still be queued when button-up synchronously applies the final point;
		// using that message's lParam would move the window backward afterward.
		x, y := int(skillBarDragMoveX.Load()), int(skillBarDragMoveY.Load())
		moved := moveNativeSkillBarWindowNow(hwnd, x, y)
		if moved && !skillBarDragLogged.Swap(true) {
			logger.Printf("native skill bar drag started: target=(%d,%d)", x, y)
		}
		latestX, latestY := int(skillBarDragMoveX.Load()), int(skillBarDragMoveY.Load())
		if moved && (latestX != x || latestY != y) {
			requestNativeSkillBarDragMove(hwnd, latestX, latestY, generation)
		}
		if moved {
			return 1
		}
		return 0
	case wmSkillBarHookLost:
		cancelNativeSkillBarWindowGesture(hwnd, "shared low-level mouse hook stopped")
		releaseNativeSkillBarOutstandingStopKey()
		skillBarDesiredVisible.Store(false)
		procShowWindow.Call(hwnd, swHide)
		skillBarVisible.Store(false)
		skillBarHookBarrierEnabled.Store(false)
		logger.Println("native skill bar hidden fail-closed: shared low-level mouse hook unavailable")
		return 0
	case wmMove:
		updateNativeSkillBarPosition(
			int(int16(uint16(lParam&0xffff))),
			int(int16(uint16((lParam>>16)&0xffff))),
		)
		publishNativeSkillBarHookBarrierRectFromWindow(hwnd)
		return 0
	case wmActivate:
		activationState := uint16(wParam & 0xffff)
		previous := nativeSkillBarPreviousWindowFromActivation(activationState, lParam, hwnd)
		previousIsGame := false
		if previous != 0 {
			rememberNativeSkillBarExternalForeground(previous)
			rememberNativeSkillBarPendingFocus(previous)
			previousIsGame = isMabinogiWindow(previous)
			if !previousIsGame {
				skillBarRecoveryTarget.Store(0)
			}
		} else {
			clearNativeSkillBarPendingFocus()
		}
		logger.Printf(
			"native skill bar WM_ACTIVATE: state=%d previous=%#x accepted=%v game=%v",
			activationState, lParam, previous != 0, previousIsGame,
		)
		// Continue to DefWindowProc below so activation performs the normal
		// keyboard-focus bookkeeping for this native top-level window.
	case wmMouseActivate:
		// This is a fail-safe for unusual accessibility/synthetic input paths.
		// Physical clicks are intercepted before hit testing by the shared hook.
		return nativeSkillBarMouseActivateResult()
	case wmNcHitTest:
		return htClient
	case wmLButtonDown:
		beginNativeSkillBarWindowGesture(hwnd, skillBarMouseButtonLeft, lParam)
		return 0
	case wmLButtonUp:
		completeNativeSkillBarWindowGesture(hwnd, skillBarMouseButtonLeft, lParam)
		return 0
	case wmRButtonDown:
		beginNativeSkillBarWindowGesture(hwnd, skillBarMouseButtonRight, lParam)
		return 0
	case wmRButtonUp:
		completeNativeSkillBarWindowGesture(hwnd, skillBarMouseButtonRight, lParam)
		return 0
	case wmMouseMove:
		moveNativeSkillBarWindowGesture(hwnd)
		return 0
	case wmCaptureChanged, wmCancelMode, wmKillFocus:
		if message == wmKillFocus && !skillBarGestureActive.Load() {
			skillBarRecoveryTarget.Store(0)
		}
		cancelNativeSkillBarWindowGesture(hwnd, fmt.Sprintf("message=%#x", message))
		return 0
	case wmContextMenu, wmMouseWheel, wmMouseHWheel:
		// This is an interactive rectangle. Never forward mouse gestures that
		// landed on it to the game through default processing.
		return 0
	case wmExitSizeMove:
		persistNativeSkillBarWindowPosition(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return result
}

func nativeSkillBarMouseActivateResult() uintptr {
	return maNoActivate
}

func nativeSkillBarPreviousWindowFromActivation(state uint16, previous, skillBar uintptr) uintptr {
	if state == waInactive || previous == 0 || previous == skillBar {
		return 0
	}
	return previous
}

func rememberNativeSkillBarExternalForeground(hwnd uintptr) {
	skillBar := skillBarHWND.Load()
	if hwnd == 0 || hwnd == skillBar {
		return
	}
	skillBarFocusMu.Lock()
	skillBarLastExternal = hwnd
	skillBarLastExternalAt = time.Now().UnixMilli()
	skillBarFocusMu.Unlock()
}

func rememberNativeSkillBarPendingFocus(hwnd uintptr) {
	if hwnd == 0 || hwnd == skillBarHWND.Load() {
		clearNativeSkillBarPendingFocus()
		return
	}
	skillBarPendingFocus.Store(hwnd)
	skillBarPendingFocusAt.Store(time.Now().UnixMilli())
}

func clearNativeSkillBarPendingFocus() {
	skillBarPendingFocus.Store(0)
	skillBarPendingFocusAt.Store(0)
}

func takeNativeSkillBarPendingFocus() uintptr {
	target := skillBarPendingFocus.Swap(0)
	observedAt := skillBarPendingFocusAt.Swap(0)
	if target == 0 || !nativeSkillBarExternalFocusFresh(time.Now().UnixMilli(), observedAt) {
		return 0
	}
	return target
}

func clearNativeSkillBarExternalForeground() {
	skillBarFocusMu.Lock()
	skillBarLastExternal = 0
	skillBarLastExternalAt = 0
	skillBarFocusMu.Unlock()
}

func nativeSkillBarExternalFocusFresh(nowMs, observedAtMs int64) bool {
	return observedAtMs > 0 && nowMs >= observedAtMs &&
		nowMs-observedAtMs <= skillBarExternalFocusTTL.Milliseconds()
}

func recentNativeSkillBarExternalForeground() uintptr {
	skillBarFocusMu.Lock()
	target := skillBarLastExternal
	observedAt := skillBarLastExternalAt
	skillBarFocusMu.Unlock()
	if target == 0 || !nativeSkillBarExternalFocusFresh(time.Now().UnixMilli(), observedAt) {
		return 0
	}
	valid, _, _ := procIsWindow.Call(target)
	visible, _, _ := procIsWindowVisible.Call(target)
	if valid == 0 || visible == 0 || target == skillBarHWND.Load() {
		return 0
	}
	return target
}

func resolveNativeSkillBarFocusTarget(snapshot nativeSkillBarFocusSnapshot) (uintptr, string) {
	if snapshot.CurrentFocus != 0 && snapshot.CurrentFocus != snapshot.SkillBar {
		return snapshot.CurrentFocus, "foreground"
	}
	if snapshot.PendingFocus != 0 && snapshot.PendingFocus != snapshot.SkillBar {
		return snapshot.PendingFocus, "pending-activation"
	}
	if snapshot.RecentExternal != 0 && snapshot.RecentExternal != snapshot.SkillBar {
		return snapshot.RecentExternal, "recent-external"
	}
	if snapshot.RecoveryTarget != 0 && snapshot.RecoveryTarget != snapshot.SkillBar {
		return snapshot.RecoveryTarget, "recovery"
	}
	return 0, "none"
}

// nativeSkillBarLowLevelMouseHandled is called directly by WH_MOUSE_LL. Keep
// it bounded: only atomic state, a GetForegroundWindow snapshot, one prepared
// stop-key SendInput and non-blocking channel writes are allowed here. Window
// movement, logging, activation chords and delayed key-up run on workers.
func nativeSkillBarLowLevelMouseHandled(message uintptr, point nativePoint, flags uint32) bool {
	if !nativeSkillBarPhysicalMouseEvent(flags) {
		return false
	}
	button, kind := nativeSkillBarHookButtonMessage(message)
	if kind == 0 {
		switch message {
		case wmMouseMove:
			if skillBarHookOwnedButtons.Load() == 0 {
				return false
			}
			skillBarHookCursorX.Store(point.X)
			skillBarHookCursorY.Store(point.Y)
			if skillBarHookMoveQueued.CompareAndSwap(false, true) {
				if !queueNativeSkillBarHookEvent(nativeSkillBarHookEvent{Kind: nativeSkillBarHookMove, Session: skillBarSession.Load()}) {
					skillBarHookMoveQueued.Store(false)
				}
			}
			// Preserve normal cursor movement. The swallowed button-down means
			// downstream applications do not see a drag in progress.
			return false
		case wmMouseWheel, wmMouseHWheel:
			return nativeSkillBarHookBarrierContains(
				point, skillBarHookBarrierEnabled.Load(), true, nativeSkillBarHookBarrierRectSnapshot(),
			)
		default:
			return false
		}
	}

	bit := nativeSkillBarHookButtonBit(button)
	if kind == nativeSkillBarHookDown {
		for {
			owned := skillBarHookOwnedButtons.Load()
			hit := nativeSkillBarHookBarrierContains(
				point, skillBarHookBarrierEnabled.Load(), true, nativeSkillBarHookBarrierRectSnapshot(),
			)
			if !nativeSkillBarHookShouldOwnDown(owned, bit, hit) {
				return false
			}
			if owned&bit != 0 {
				return true
			}
			if skillBarHookOwnedButtons.CompareAndSwap(owned, owned|bit) {
				break
			}
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		stopKeySent := button == skillBarMouseButtonLeft &&
			triggerNativeSkillBarStopKey(point, foreground, skillBarHookStopSnapshot.Load())
		queueNativeSkillBarHookEvent(nativeSkillBarHookEvent{
			Kind: nativeSkillBarHookDown, Button: button, Point: point,
			Foreground: foreground, Session: skillBarSession.Load(),
			RecoveryVersion: skillBarHookRecoveryVersion.Load(),
			StopKeySent:     stopKeySent,
		})
		skillBarHookDownSwallowed.Add(1)
		return true
	}

	for {
		owned := skillBarHookOwnedButtons.Load()
		if !nativeSkillBarHookShouldOwnUp(owned, bit) {
			return false
		}
		if skillBarHookOwnedButtons.CompareAndSwap(owned, owned&^bit) {
			break
		}
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	queueNativeSkillBarHookEvent(nativeSkillBarHookEvent{
		Kind: nativeSkillBarHookUp, Button: button, Point: point,
		Foreground: foreground, Session: skillBarSession.Load(),
		RecoveryVersion: skillBarHookRecoveryVersion.Load(),
	})
	skillBarHookUpSwallowed.Add(1)
	return true
}

func nativeSkillBarPhysicalMouseEvent(flags uint32) bool {
	return flags&llmhfInjected == 0
}

func nativeSkillBarHookButtonMessage(message uintptr) (uint32, uint8) {
	switch message {
	case wmLButtonDown:
		return skillBarMouseButtonLeft, nativeSkillBarHookDown
	case wmLButtonUp:
		return skillBarMouseButtonLeft, nativeSkillBarHookUp
	case wmRButtonDown:
		return skillBarMouseButtonRight, nativeSkillBarHookDown
	case wmRButtonUp:
		return skillBarMouseButtonRight, nativeSkillBarHookUp
	default:
		return 0, 0
	}
}

func nativeSkillBarHookButtonBit(button uint32) uint32 {
	if button == skillBarMouseButtonLeft {
		return 1
	}
	if button == skillBarMouseButtonRight {
		return 2
	}
	return 0
}

func nativeSkillBarHookShouldOwnDown(owned, bit uint32, hit bool) bool {
	return bit != 0 && (owned != 0 || hit)
}

func nativeSkillBarHookShouldOwnUp(owned, bit uint32) bool {
	return bit != 0 && owned&bit != 0
}

func nativeSkillBarHookBarrierContains(point nativePoint, active, visible bool, rect nativeRect) bool {
	return active && visible && nativePointInRect(point, rect)
}

func publishNativeSkillBarStopSnapshot(settings nativeSkillBarSettings) {
	skillBarHookStopSnapshot.Store(nativeSkillBarStopSnapshotForSettings(settings))
}

func nativeSkillBarStopSnapshotForSettings(settings nativeSkillBarSettings) *nativeSkillBarStopSnapshot {
	// The CN client can read the physical left button through Raw Input even
	// after WH_MOUSE_LL consumes the matching Windows message. Compensation is
	// therefore tied to every physical left click inside the SkillBar barrier,
	// not to the mouse button selected for skill activation. This also protects
	// padding, gaps, empty cells and unlocked drag mode.
	if !settings.Active || !settings.StopMovement || settings.StopKeyCode == "" {
		return nil
	}
	spec, ok := skillBarStopKeyForCode(settings.StopKeyCode)
	if !ok {
		return nil
	}
	inputs, err := nativeSkillBarInputsForChord([]skillBarKeySpec{spec})
	if err != nil || len(inputs) != 2 {
		return nil
	}
	return &nativeSkillBarStopSnapshot{
		Code: settings.StopKeyCode, Down: inputs[0], Up: inputs[1],
	}
}

func nativeSkillBarStopSnapshotHit(point nativePoint, barrier nativeRect, snapshot *nativeSkillBarStopSnapshot) bool {
	if snapshot == nil {
		return false
	}
	return nativePointInRect(point, barrier)
}

func nativeSkillBarStopShouldTrigger(point nativePoint, foreground, gameForeground uintptr, barrier nativeRect, snapshot *nativeSkillBarStopSnapshot) bool {
	return foreground != 0 && foreground == gameForeground && nativeSkillBarStopSnapshotHit(point, barrier, snapshot)
}

// triggerNativeSkillBarStopKey runs inside WH_MOUSE_LL. It deliberately does
// only one already-prepared SendInput call and a non-blocking queue write. The
// matching key-up is handled by a worker so the hook never sleeps.
func triggerNativeSkillBarStopKey(point nativePoint, foreground uintptr, snapshot *nativeSkillBarStopSnapshot) bool {
	if skillBarClosing.Load() {
		return false
	}
	if !nativeSkillBarStopShouldTrigger(
		point, foreground, skillBarForegroundGame.Load(), nativeSkillBarHookBarrierRectSnapshot(), snapshot,
	) {
		return false
	}
	release := &nativeSkillBarStopRelease{Input: snapshot.Up, ReleaseAt: time.Now().Add(skillBarStopKeyHold)}
	// Publish the release before sending key-down, while holding the same lock
	// used by the release path. Shutdown can therefore never observe no release,
	// exit, and then race with a successful key-down. If shutdown starts after
	// publication it waits here and sends the paired key-up after this call.
	release.mu.Lock()
	skillBarStopOutstanding.Store(release)
	if skillBarClosing.Load() {
		release.Released.Store(true)
		skillBarStopOutstanding.CompareAndSwap(release, nil)
		release.mu.Unlock()
		return false
	}
	if !sendNativeSkillBarSingleInput(&snapshot.Down) {
		release.Released.Store(true)
		skillBarStopOutstanding.CompareAndSwap(release, nil)
		release.mu.Unlock()
		skillBarStopKeyFailed.Add(1)
		return false
	}
	skillBarStopKeyDownSent.Add(1)
	release.mu.Unlock()
	select {
	case skillBarStopReleaseCh <- release:
		return true
	default:
		// Never leave the configured key held if the worker is unexpectedly
		// saturated. Releasing immediately weakens only this one compensation.
		skillBarStopKeyFailed.Add(1)
		releaseNativeSkillBarStopKey(release, 1)
		return true
	}
}

func runNativeSkillBarStopReleaseQueue() {
	for release := range skillBarStopReleaseCh {
		if wait := time.Until(release.ReleaseAt); wait > 0 {
			time.Sleep(wait)
		}
		if releaseNativeSkillBarStopKey(release, skillBarStopKeyRetryAttempts) {
			continue
		}
		release.Passes++
		if release.Passes < skillBarStopKeyRetryPasses {
			release.ReleaseAt = time.Now().Add(skillBarStopKeyRetryPause)
			select {
			case skillBarStopReleaseCh <- release:
			default:
				// The outstanding pointer remains available for the shutdown
				// flush even if an unexpectedly saturated queue rejects a retry.
			}
		}
	}
}

func releaseNativeSkillBarStopKey(release *nativeSkillBarStopRelease, attempts int) bool {
	if release == nil {
		return true
	}
	release.mu.Lock()
	defer release.mu.Unlock()
	if release.Released.Load() {
		return true
	}
	for attempt := 0; attempt < max(1, attempts); attempt++ {
		if sendNativeSkillBarSingleInput(&release.Input) {
			release.Released.Store(true)
			skillBarStopOutstanding.CompareAndSwap(release, nil)
			skillBarStopKeyUpSent.Add(1)
			return true
		}
		if attempt+1 < attempts {
			time.Sleep(skillBarStopKeyRetryDelay)
		}
	}
	skillBarStopKeyFailed.Add(1)
	return false
}

func releaseNativeSkillBarOutstandingStopKey() {
	if release := skillBarStopOutstanding.Load(); release != nil {
		releaseNativeSkillBarStopKey(release, skillBarStopKeyRetryAttempts)
	}
}

func sendNativeSkillBarSingleInput(input *nativeSkillBarInput) bool {
	if input == nil {
		return false
	}
	sent, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(input)), unsafe.Sizeof(*input))
	return sent == 1
}

func queueNativeSkillBarHookEvent(event nativeSkillBarHookEvent) bool {
	select {
	case skillBarHookEventCh <- event:
		return true
	default:
		skillBarHookEventDropped.Add(1)
		if event.Kind == nativeSkillBarHookDown || event.Kind == nativeSkillBarHookUp {
			// A missing edge must poison the entire logical gesture. The version
			// is level-triggered: once the worker drains any queued event it sees
			// the newer version, cancels its state, and skips all older edges.
			skillBarHookCriticalDropped.Add(1)
			skillBarHookRecoveryVersion.Add(1)
		}
		return false
	}
}

func runNativeSkillBarHookEventQueue() {
	lastRecoveryVersion := skillBarHookRecoveryVersion.Load()
	for event := range skillBarHookEventCh {
		if event.Kind == nativeSkillBarHookMove {
			skillBarHookMoveQueued.Store(false)
		}
		currentRecovery := skillBarHookRecoveryVersion.Load()
		if currentRecovery != lastRecoveryVersion {
			cancelNativeSkillBarWindowGesture(skillBarHWND.Load(), "low-level hook edge queue recovered fail-closed")
			lastRecoveryVersion = currentRecovery
		}
		if !nativeSkillBarSessionCurrent(event.Session) {
			continue
		}
		if event.Kind == nativeSkillBarHookDown || event.Kind == nativeSkillBarHookUp {
			if event.RecoveryVersion != currentRecovery {
				continue
			}
		}
		switch event.Kind {
		case nativeSkillBarHookDown:
			beginNativeSkillBarHookGesture(event)
		case nativeSkillBarHookMove:
			moveNativeSkillBarWindowGestureAt(skillBarHWND.Load(), nativePoint{
				X: skillBarHookCursorX.Load(), Y: skillBarHookCursorY.Load(),
			})
		case nativeSkillBarHookUp:
			completeNativeSkillBarHookGesture(event)
		}
	}
}

func beginNativeSkillBarHookGesture(event nativeSkillBarHookEvent) {
	hwnd := skillBarHWND.Load()
	if hwnd == 0 || skillBarClosing.Load() {
		return
	}
	if !nativeSkillBarWindowGestureCanBegin(skillBarGestureActive.Load()) {
		// Once either button owns a barrier gesture, the hook also owns the
		// other button even if it is pressed after the cursor left the bar. Mark
		// the gesture unsafe, but keep swallowing both matching releases.
		skillBarGestureOverlap.Store(true)
		logger.Printf("native skill bar hook ignored overlapping button down: button=%d active_button=%d", event.Button, skillBarWindowPressBtn.Load())
		return
	}
	if !nativeSkillBarHookBarrierContains(
		event.Point, skillBarHookBarrierEnabled.Load(), true, nativeSkillBarHookBarrierRectSnapshot(),
	) {
		return
	}

	foreground := event.Foreground
	if foreground != 0 && foreground != hwnd {
		rememberNativeSkillBarExternalForeground(foreground)
	}
	gameTarget := uintptr(0)
	if foreground != 0 && isMabinogiWindow(foreground) {
		gameTarget = foreground
	}
	rect := nativeSkillBarHookBarrierRectSnapshot()
	skillBarGestureRestore.Store(0)
	skillBarGestureGame.Store(gameTarget)
	skillBarGestureStopKey.Store(event.StopKeySent)
	skillBarWindowPressBtn.Store(event.Button)
	skillBarGestureActive.Store(true)
	skillBarGestureOverlap.Store(false)
	skillBarMouseDown.Store(-1)
	if nativeSkillBarGestureCanArm(skillBarLocked.Load(), event.Button, nativeSkillBarConfiguredButton(), skillBarInputEnabled.Load()) {
		skillBarMouseDown.Store(int32(nativeSkillBarSlotAtPoint(
			int(event.Point.X-rect.Left), int(event.Point.Y-rect.Top),
		)))
	}
	skillBarDragPending.Store(false)
	skillBarDragging.Store(false)
	skillBarDragLogged.Store(false)
	skillBarDragGeneration.Store(0)
	if !skillBarLocked.Load() && event.Button == skillBarMouseButtonLeft {
		skillBarDragMu.Lock()
		skillBarDragStart = event.Point
		skillBarDragRect = rect
		skillBarDragMu.Unlock()
		// Every drag gets a unique generation. A delayed PostMessage from an
		// earlier gesture can therefore never become valid in the next gesture.
		skillBarDragGeneration.Store(skillBarPositionGen.Add(1))
		skillBarDragPending.Store(true)
	}
	logger.Printf(
		"native skill bar hook capture started: button=%d configured_button=%d input=%v slot=%d locked=%v foreground=%#x game=%#x stop_ready=%v stop_key=%v",
		event.Button, nativeSkillBarConfiguredButton(), skillBarInputEnabled.Load(), int(skillBarMouseDown.Load())+1,
		skillBarLocked.Load(), foreground, gameTarget, skillBarHookStopSnapshot.Load() != nil, event.StopKeySent,
	)
}

func completeNativeSkillBarHookGesture(event nativeSkillBarHookEvent) {
	hwnd := skillBarHWND.Load()
	if hwnd == 0 || !skillBarGestureActive.Load() || skillBarWindowPressBtn.Load() != event.Button {
		return
	}
	// Apply the final pointer position even if high-frequency move events were
	// coalesced while the hook queue was busy.
	moveNativeSkillBarWindowGestureAt(hwnd, event.Point)
	dragged := skillBarDragging.Load()
	if dragged {
		x, y := nativeSkillBarDragTarget(event.Point)
		moveNativeSkillBarWindowFinal(hwnd, x, y, skillBarDragGeneration.Load())
	}
	pressed := int(skillBarMouseDown.Load())
	rect := nativeSkillBarHookBarrierRectSnapshot()
	released := nativeSkillBarSlotAtPoint(int(event.Point.X-rect.Left), int(event.Point.Y-rect.Top))
	gameTarget := skillBarGestureGame.Load()
	overlapped := skillBarGestureOverlap.Swap(false)
	foregroundMatches := gameTarget != 0 && event.Foreground == gameTarget && isMabinogiWindow(event.Foreground)
	configuredButton := nativeSkillBarConfiguredButton()
	allOwnedButtonsReleased := skillBarHookOwnedButtons.Load() == 0
	activate := nativeSkillBarWindowGestureActivates(
		event.Button, configuredButton, skillBarInputEnabled.Load(), pressed, released,
		dragged, gameTarget, foregroundMatches,
	) && skillBarLocked.Load() && nativeSkillBarWindowGestureActivationSafe(overlapped, allOwnedButtonsReleased)
	invalidateNativeSkillBarDragGeneration(skillBarDragGeneration.Load())

	skillBarGestureActive.Store(false)
	skillBarWindowPressBtn.Store(0)
	skillBarGestureRestore.Store(0)
	skillBarGestureGame.Store(0)
	stopKeySent := skillBarGestureStopKey.Swap(false)
	skillBarMouseDown.Store(-1)
	skillBarDragPending.Store(false)
	skillBarDragging.Store(false)
	skillBarDragLogged.Store(false)
	skillBarDragGeneration.Store(0)

	if dragged {
		x, y := int(skillBarX.Load()), int(skillBarY.Load())
		skillBarDragCompleted.Add(1)
		logger.Printf("native skill bar hook drag completed: position=(%d,%d)", x, y)
		go persistNativeSkillBarPosition(x, y)
		renderNativeSkillBar()
	}
	logger.Printf(
		"native skill bar hook capture completed: button=%d configured_button=%d input=%v slot=%d drag=%v overlap=%v paired_release=%v foreground=%#x game=%#x stop_key=%v activate=%v",
		event.Button, configuredButton, skillBarInputEnabled.Load(), released+1, dragged, overlapped,
		allOwnedButtonsReleased, event.Foreground, gameTarget, stopKeySent, activate,
	)
	if activate {
		enqueueNativeSkillBarActivationForTarget(pressed, gameTarget, 0, event.Button)
	}
}

func beginNativeSkillBarWindowGesture(hwnd uintptr, button uint32, lParam uintptr) {
	if hwnd == 0 || hwnd != skillBarHWND.Load() || skillBarClosing.Load() {
		return
	}
	if !nativeSkillBarWindowGestureCanBegin(skillBarGestureActive.Load()) {
		// SetCapture keeps both mouse buttons routed to this HWND.  If the user
		// presses the other button before releasing the first one, consume that
		// extra down without replacing the gesture which owns capture/focus.
		skillBarGestureOverlap.Store(true)
		logger.Printf("native skill bar ignored overlapping button down: button=%d active_button=%d", button, skillBarWindowPressBtn.Load())
		return
	}
	skillBarGestureOverlap.Store(false)
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground != 0 && foreground != hwnd {
		rememberNativeSkillBarExternalForeground(foreground)
	}
	recovery := skillBarRecoveryTarget.Load()
	if recovery != 0 && !isMabinogiWindow(recovery) {
		recovery = 0
	}
	restoreTarget, targetSource := resolveNativeSkillBarFocusTarget(nativeSkillBarFocusSnapshot{
		PendingFocus:   takeNativeSkillBarPendingFocus(),
		CurrentFocus:   foreground,
		SkillBar:       hwnd,
		RecentExternal: recentNativeSkillBarExternalForeground(),
		RecoveryTarget: recovery,
	})
	gameTarget := uintptr(0)
	if restoreTarget != 0 && isMabinogiWindow(restoreTarget) {
		gameTarget = restoreTarget
	}

	skillBarGestureRestore.Store(restoreTarget)
	skillBarGestureGame.Store(gameTarget)
	skillBarGestureStopKey.Store(false)
	skillBarWindowPressBtn.Store(button)
	skillBarGestureActive.Store(true)
	skillBarMouseDown.Store(-1)
	if nativeSkillBarGestureCanArm(skillBarLocked.Load(), button, nativeSkillBarConfiguredButton(), skillBarInputEnabled.Load()) {
		skillBarMouseDown.Store(int32(nativeSkillBarSlotAt(lParam)))
	}
	skillBarDragPending.Store(false)
	skillBarDragging.Store(false)
	skillBarDragLogged.Store(false)
	skillBarDragGeneration.Store(0)
	if !skillBarLocked.Load() && button == skillBarMouseButtonLeft {
		var cursor nativePoint
		if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok != 0 {
			rect := nativeSkillBarAtomicRect()
			if got, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); got == 0 {
				rect = nativeSkillBarAtomicRect()
			}
			skillBarDragMu.Lock()
			skillBarDragStart = cursor
			skillBarDragRect = rect
			skillBarDragMu.Unlock()
			skillBarDragGeneration.Store(skillBarPositionGen.Add(1))
			skillBarDragPending.Store(true)
		}
	}
	procSetCapture.Call(hwnd)
	capture, _, _ := procGetCapture.Call()
	if capture != hwnd {
		logger.Printf("native skill bar capture failed: button=%d foreground_before=%#x", button, restoreTarget)
		if gameTarget != 0 {
			// Keep the already validated Client.exe HWND available even when the
			// delayed foreground restore times out or Windows rejects it.
			skillBarRecoveryTarget.Store(gameTarget)
		}
		cancelNativeSkillBarWindowGesture(hwnd, "SetCapture verification failed")
		deferNativeSkillBarForegroundRestore(hwnd, restoreTarget, skillBarSession.Load())
		return
	}
	logger.Printf(
		"native skill bar capture started: button=%d configured_button=%d input=%v slot=%d locked=%v target_source=%s foreground_now=%#x restore_target=%#x game=%#x",
		button, nativeSkillBarConfiguredButton(), skillBarInputEnabled.Load(), int(skillBarMouseDown.Load())+1,
		skillBarLocked.Load(), targetSource, foreground, restoreTarget, gameTarget,
	)
}

func nativeSkillBarWindowGestureCanBegin(active bool) bool {
	return !active
}

func moveNativeSkillBarWindowGesture(hwnd uintptr) {
	var cursor nativePoint
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return
	}
	moveNativeSkillBarWindowGestureAt(hwnd, cursor)
}

func moveNativeSkillBarWindowGestureAt(hwnd uintptr, cursor nativePoint) {
	if !skillBarGestureActive.Load() || skillBarWindowPressBtn.Load() != skillBarMouseButtonLeft ||
		skillBarLocked.Load() || (!skillBarDragPending.Load() && !skillBarDragging.Load()) {
		return
	}
	if skillBarDragPending.Load() && nativeSkillBarDragThresholdExceeded(cursor) {
		skillBarDragPending.Store(false)
		skillBarDragging.Store(true)
		skillBarMouseDown.Store(-1)
		skillBarDragStarted.Add(1)
	}
	if !skillBarDragging.Load() {
		return
	}
	x, y := nativeSkillBarDragTarget(cursor)
	requestNativeSkillBarDragMove(hwnd, x, y, skillBarDragGeneration.Load())
}

func completeNativeSkillBarWindowGesture(hwnd uintptr, button uint32, lParam uintptr) {
	if !skillBarGestureActive.Load() || skillBarWindowPressBtn.Load() != button {
		return
	}
	pressed := int(skillBarMouseDown.Load())
	released := nativeSkillBarSlotAt(lParam)
	dragged := skillBarDragging.Load()
	if dragged {
		var cursor nativePoint
		if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok != 0 {
			x, y := nativeSkillBarDragTarget(cursor)
			moveNativeSkillBarWindowNow(hwnd, x, y)
		}
	}
	restoreTarget := skillBarGestureRestore.Load()
	gameTarget := skillBarGestureGame.Load()
	overlapped := skillBarGestureOverlap.Swap(false)
	mouseButtonsReleased := nativeSkillBarMouseButtonsReleased()
	foregroundAtRelease, _, _ := procGetForegroundWindow.Call()
	configuredButton := nativeSkillBarConfiguredButton()
	activate := nativeSkillBarWindowGestureActivates(
		button, configuredButton, skillBarInputEnabled.Load(), pressed, released,
		dragged, gameTarget, foregroundAtRelease == hwnd,
	) && skillBarLocked.Load() && nativeSkillBarWindowGestureActivationSafe(overlapped, mouseButtonsReleased)
	invalidateNativeSkillBarDragGeneration(skillBarDragGeneration.Load())

	// Copy the complete request above, then mark the gesture finished before
	// ReleaseCapture/SetForegroundWindow synchronously deliver cancellation
	// messages back into this WndProc.
	skillBarGestureActive.Store(false)
	skillBarWindowPressBtn.Store(0)
	skillBarGestureRestore.Store(0)
	skillBarGestureGame.Store(0)
	skillBarGestureStopKey.Store(false)
	skillBarMouseDown.Store(-1)
	skillBarDragPending.Store(false)
	skillBarDragging.Store(false)
	skillBarDragLogged.Store(false)
	skillBarDragGeneration.Store(0)
	procReleaseCapture.Call()

	if dragged {
		x, y := int(skillBarX.Load()), int(skillBarY.Load())
		skillBarDragCompleted.Add(1)
		logger.Printf("native skill bar drag completed: position=(%d,%d)", x, y)
		go persistNativeSkillBarPosition(x, y)
		renderNativeSkillBarNow(hwnd)
	}
	restored := false
	restoreDeferred := false
	if foregroundAtRelease == hwnd {
		if mouseButtonsReleased {
			restored = restoreNativeSkillBarForeground(restoreTarget)
			if !restored && !nativeSkillBarMouseButtonsReleased() {
				restoreDeferred = true
				deferNativeSkillBarForegroundRestore(hwnd, restoreTarget, skillBarSession.Load())
			}
		} else {
			restoreDeferred = true
			deferNativeSkillBarForegroundRestore(hwnd, restoreTarget, skillBarSession.Load())
		}
	}
	if restored {
		skillBarRecoveryTarget.Store(0)
	} else if foregroundAtRelease == hwnd && gameTarget != 0 {
		skillBarRecoveryTarget.Store(gameTarget)
	}
	logger.Printf(
		"native skill bar capture completed: button=%d configured_button=%d input=%v slot=%d drag=%v overlap=%v mouse_buttons_released=%v foreground_release=%#x restore_target=%#x restore_call=%v restore_deferred=%v activate=%v",
		button, configuredButton, skillBarInputEnabled.Load(), released+1, dragged, overlapped, mouseButtonsReleased,
		foregroundAtRelease, restoreTarget, restored, restoreDeferred, activate,
	)
	if activate {
		enqueueNativeSkillBarActivationForTarget(pressed, gameTarget, hwnd, button)
	}
}

func nativeSkillBarWindowGestureActivationSafe(overlapped, mouseButtonsReleased bool) bool {
	return !overlapped && mouseButtonsReleased
}

func nativeSkillBarWindowGestureActivates(
	button, configuredButton uint32,
	inputEnabled bool,
	pressed, released int,
	dragged bool,
	gameTarget uintptr,
	foregroundOwned bool,
) bool {
	return inputEnabled && foregroundOwned && !dragged && gameTarget != 0 &&
		button == configuredButton && pressed >= 0 && pressed == released
}

func cancelNativeSkillBarWindowGesture(hwnd uintptr, reason string) {
	if !skillBarGestureActive.Swap(false) {
		return
	}
	invalidateNativeSkillBarDragGeneration(skillBarDragGeneration.Load())
	restoreTarget := skillBarGestureRestore.Swap(0)
	skillBarGestureOverlap.Store(false)
	skillBarGestureGame.Store(0)
	skillBarGestureStopKey.Store(false)
	skillBarWindowPressBtn.Store(0)
	skillBarMouseDown.Store(-1)
	skillBarDragPending.Store(false)
	skillBarDragging.Store(false)
	skillBarDragLogged.Store(false)
	skillBarDragGeneration.Store(0)
	capture, _, _ := procGetCapture.Call()
	if capture == hwnd {
		procReleaseCapture.Call()
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground != hwnd {
		skillBarRecoveryTarget.Store(0)
	}
	// Cancellation commonly means the user Alt-Tabbed or another window took
	// capture. Never steal focus back to Client.exe while the physical button
	// may still be down; only the normal button-up path restores foreground.
	logger.Printf("native skill bar gesture cancelled: %s previous_target=%#x (focus not restored)", reason, restoreTarget)
}

func restoreNativeSkillBarForeground(target uintptr) bool {
	// This is the lowest-level foreground handoff used by every gesture path.
	// Recheck here to close the gap between an earlier button-up observation and
	// the actual SetForegroundWindow call.
	if target == 0 || !nativeSkillBarMouseButtonsReleased() {
		return false
	}
	restored, _, _ := procSetForegroundWindow.Call(target)
	return restored != 0
}

func deferNativeSkillBarForegroundRestore(hwnd, target uintptr, session uint64) {
	if target == 0 {
		return
	}
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			remaining := time.Until(deadline)
			if remaining <= 0 || !waitForNativeSkillBarMouseButtonsRelease(remaining) {
				logger.Printf("native skill bar deferred foreground restore timed out: target=%#x", target)
				return
			}
			if !nativeSkillBarSessionCurrent(session) {
				return
			}
			foreground, _, _ := procGetForegroundWindow.Call()
			if foreground != hwnd {
				return
			}
			if restoreNativeSkillBarForeground(target) {
				skillBarRecoveryTarget.CompareAndSwap(target, 0)
				return
			}
			if isMabinogiWindow(target) {
				skillBarRecoveryTarget.Store(target)
			}
			if nativeSkillBarMouseButtonsReleased() {
				// Windows rejected the foreground change for a reason unrelated
				// to held mouse buttons. Keep the recovery token for next time.
				return
			}
		}
	}()
}

func nativeSkillBarSlotAt(lParam uintptr) int {
	x := int(int16(uint16(lParam & 0xffff)))
	y := int(int16(uint16((lParam >> 16) & 0xffff)))
	return nativeSkillBarSlotAtPoint(x, y)
}

func nativeSkillBarSlotAtPoint(x, y int) int {
	settings := currentNativeSkillBarSettings()
	if len(settings.Slots) == 0 {
		return -1
	}
	padding := nativeSkillBarPadding(settings.Locked)
	x -= padding
	y -= padding
	if x < 0 || y < 0 {
		return -1
	}
	step := settings.IconSize + settings.Gap
	column, row := x/step, y/step
	if column >= settings.Columns || x%step >= settings.IconSize || y%step >= settings.IconSize {
		return -1
	}
	index := row*settings.Columns + column
	if index < 0 || index >= len(settings.Slots) {
		return -1
	}
	return index
}

func nativeSkillBarGestureCanArm(locked bool, button, configuredButton uint32, inputEnabled bool) bool {
	return locked && inputEnabled && button == configuredButton
}

func nativeSkillBarConfiguredButton() uint32 {
	if skillBarClickRight.Load() {
		return skillBarMouseButtonRight
	}
	return skillBarMouseButtonLeft
}

func nativeSkillBarAtomicRect() nativeRect {
	left, top := skillBarX.Load(), skillBarY.Load()
	return nativeRect{
		Left: left, Top: top,
		Right: left + skillBarWidth.Load(), Bottom: top + skillBarHeight.Load(),
	}
}

func publishNativeSkillBarHookBarrierRect() {
	rect := nativeSkillBarAtomicRect()
	skillBarHookBarrierRect.Store(&rect)
}

func nativeSkillBarRectUnion(first, second nativeRect) nativeRect {
	if first.Right <= first.Left || first.Bottom <= first.Top {
		return second
	}
	if second.Right <= second.Left || second.Bottom <= second.Top {
		return first
	}
	return nativeRect{
		Left:   min(first.Left, second.Left),
		Top:    min(first.Top, second.Top),
		Right:  max(first.Right, second.Right),
		Bottom: max(first.Bottom, second.Bottom),
	}
}

func publishNativeSkillBarHookBarrierTransition(next nativeRect) {
	if !skillBarHookBarrierEnabled.Load() {
		return
	}
	transition := nativeSkillBarRectUnion(nativeSkillBarHookBarrierRectSnapshot(), next)
	skillBarHookBarrierRect.Store(&transition)
}

func publishNativeSkillBarHookBarrierRectFromWindow(hwnd uintptr) bool {
	if hwnd == 0 || hwnd != skillBarHWND.Load() {
		return false
	}
	var rect nativeRect
	if ok, _, rectErr := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		logger.Println("native skill bar GetWindowRect failed:", rectErr)
		return false
	}
	skillBarHookBarrierRect.Store(&rect)
	return true
}

func nativeSkillBarHookBarrierRectSnapshot() nativeRect {
	if rect := skillBarHookBarrierRect.Load(); rect != nil {
		return *rect
	}
	return nativeSkillBarAtomicRect()
}

func nativeSkillBarDragTarget(cursor nativePoint) (int, int) {
	skillBarDragMu.Lock()
	start := skillBarDragStart
	rect := skillBarDragRect
	skillBarDragMu.Unlock()
	x := int(rect.Left + cursor.X - start.X)
	y := int(rect.Top + cursor.Y - start.Y)
	return max(-32000, min(32000, x)), max(-32000, min(32000, y))
}

func nativeSkillBarDragThresholdExceeded(cursor nativePoint) bool {
	skillBarDragMu.Lock()
	start := skillBarDragStart
	skillBarDragMu.Unlock()
	deltaX := int64(cursor.X) - int64(start.X)
	deltaY := int64(cursor.Y) - int64(start.Y)
	if deltaX < 0 {
		deltaX = -deltaX
	}
	if deltaY < 0 {
		deltaY = -deltaY
	}
	return deltaX >= skillBarDragThreshold || deltaY >= skillBarDragThreshold
}

func packNativeSkillBarPosition(x, y int) uintptr {
	x = max(-32000, min(32000, x))
	y = max(-32000, min(32000, y))
	return uintptr(uint16(int16(x))) | uintptr(uint16(int16(y)))<<16
}

func unpackNativeSkillBarPosition(value uintptr) (int, int) {
	return int(int16(uint16(value & 0xffff))), int(int16(uint16((value >> 16) & 0xffff)))
}

func nativeSkillBarDragMoveCurrent(generation uint64) bool {
	return generation != 0 && generation == skillBarPositionGen.Load() &&
		generation == skillBarDragGeneration.Load() && skillBarGestureActive.Load() &&
		skillBarDragging.Load() && !skillBarClosing.Load()
}

func invalidateNativeSkillBarDragGeneration(generation uint64) {
	if generation != 0 {
		skillBarPositionGen.CompareAndSwap(generation, generation+1)
	}
}

func requestNativeSkillBarDragMove(hwnd uintptr, x, y int, generation uint64) bool {
	if hwnd == 0 || hwnd != skillBarHWND.Load() || !nativeSkillBarDragMoveCurrent(generation) {
		return false
	}
	x = max(-32000, min(32000, x))
	y = max(-32000, min(32000, y))
	skillBarDragMoveX.Store(int32(x))
	skillBarDragMoveY.Store(int32(y))
	if !skillBarDragMovePosted.CompareAndSwap(false, true) {
		return true
	}
	posted, _, postErr := procPostMessageW.Call(hwnd, wmSkillBarDragMove, uintptr(generation), packNativeSkillBarPosition(x, y))
	if posted == 0 {
		skillBarDragMovePosted.Store(false)
		logger.Println("native skill bar drag PostMessage failed:", postErr)
		return false
	}
	return true
}

func moveNativeSkillBarWindowFinal(hwnd uintptr, x, y int, generation uint64) bool {
	if hwnd == 0 || hwnd != skillBarHWND.Load() || !nativeSkillBarDragMoveCurrent(generation) {
		return false
	}
	x = max(-32000, min(32000, x))
	y = max(-32000, min(32000, y))
	skillBarDragMoveX.Store(int32(x))
	skillBarDragMoveY.Store(int32(y))
	moved, _, _ := procSendMessageW.Call(hwnd, wmSkillBarDragMove, uintptr(generation), packNativeSkillBarPosition(x, y))
	return moved != 0
}

// moveNativeSkillBarWindowNow must only run on the SkillBar GUI owner thread.
// Both settings and drag moves are serialized there so an older drag cannot
// move the HWND after a newer manually entered position has been applied.
func moveNativeSkillBarWindowNow(hwnd uintptr, x, y int) bool {
	if hwnd == 0 || hwnd != skillBarHWND.Load() {
		return false
	}
	current := nativeSkillBarHookBarrierRectSnapshot()
	desired := nativeRect{
		Left: int32(x), Top: int32(y),
		Right: int32(x) + (current.Right - current.Left), Bottom: int32(y) + (current.Bottom - current.Top),
	}
	publishNativeSkillBarHookBarrierTransition(desired)
	moved, _, moveErr := procSetWindowPos.Call(hwnd, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), 0, 0,
		swpNoSize|swpNoActivate|swpShowWindow)
	if moved == 0 {
		logger.Println("native skill bar drag SetWindowPos failed:", moveErr)
		publishNativeSkillBarHookBarrierRectFromWindow(hwnd)
		return false
	}
	// WM_MOVE normally updates these values synchronously. Store them here as
	// well so the settings page sees X/Y follow the cursor even if Windows
	// coalesces a move notification.
	updateNativeSkillBarPosition(x, y)
	publishNativeSkillBarHookBarrierRectFromWindow(hwnd)
	return true
}

func updateNativeSkillBarPosition(x, y int) {
	x = max(-32000, min(32000, x))
	y = max(-32000, min(32000, y))
	skillBarX.Store(int32(x))
	skillBarY.Store(int32(y))
	skillBarSettingsMu.Lock()
	skillBarSettings.X = x
	skillBarSettings.Y = y
	skillBarSettings.PositionSet = true
	skillBarSettingsMu.Unlock()
}

func enqueueNativeSkillBarActivationForTarget(index int, targetHWND, sourceHWND uintptr, button uint32) {
	if index < 0 {
		return
	}
	select {
	case skillBarActivationCh <- nativeSkillBarActivationRequest{
		Index: index, TargetHWND: targetHWND, SourceHWND: sourceHWND,
		Button: button, Session: skillBarSession.Load(),
	}:
		skillBarActivationQueued.Add(1)
	default:
		skillBarActivationFailed.Add(1)
		logger.Printf("native skill bar activation queue full: slot=%d", index+1)
	}
}

func runNativeSkillBarActivationQueue() {
	for {
		request := <-skillBarActivationCh
		if !nativeSkillBarSessionCurrent(request.Session) {
			logger.Printf("native skill bar discarded stale activation: slot=%d session=%d current=%d", request.Index+1, request.Session, skillBarSession.Load())
			continue
		}
		logger.Printf("native skill bar activation dequeued: slot=%d target=%#x", request.Index+1, request.TargetHWND)
		activateNativeSkillBarSlot(request)
	}
}

func activateNativeSkillBarSlot(request nativeSkillBarActivationRequest) {
	if !nativeSkillBarSessionCurrent(request.Session) {
		return
	}
	index := request.Index
	settings := currentNativeSkillBarSettings()
	if index < 0 || index >= len(settings.Slots) {
		skillBarActivationFailed.Add(1)
		logger.Printf("native skill bar activation rejected: invalid slot=%d slotCount=%d", index+1, len(settings.Slots))
		return
	}
	slot := settings.Slots[index]
	sequence := normalizeNativeSkillBarKeySequence(slot.KeySequence)
	if len(sequence) == 0 && slot.KeyCode != "" {
		sequence = normalizeNativeSkillBarKeySequence([][]string{{slot.KeyCode}})
	}
	// A hotkey is independently usable even when a slot has no skill metadata.
	// This mirrors the reference application and keeps custom/iconless bindings
	// from being rejected before SendInput is reached.
	if len(sequence) == 0 {
		skillBarActivationFailed.Add(1)
		logger.Printf("native skill bar activation rejected: slot=%d has no key sequence", index+1)
		setNativeSkillBarFeedback(index, true)
		return
	}
	if !skillBarSending.CompareAndSwap(false, true) {
		skillBarActivationFailed.Add(1)
		logger.Printf("native skill bar activation rejected: slot=%d another sequence is still running", index+1)
		setNativeSkillBarFeedback(index, true)
		return
	}
	if !waitForNativeSkillBarMouseButtonsRelease(80 * time.Millisecond) {
		skillBarSending.Store(false)
		skillBarActivationFailed.Add(1)
		logger.Printf("native skill bar activation rejected: slot=%d a physical mouse button remained down", index+1)
		if request.TargetHWND != 0 && isMabinogiWindow(request.TargetHWND) {
			skillBarRecoveryTarget.Store(request.TargetHWND)
			deferNativeSkillBarForegroundRestore(request.SourceHWND, request.TargetHWND, request.Session)
		}
		setNativeSkillBarFeedback(index, true)
		return
	}
	if !focusNativeSkillBarGameTarget(request, 150*time.Millisecond) {
		skillBarSending.Store(false)
		skillBarActivationFailed.Add(1)
		foreground, _, _ := procGetForegroundWindow.Call()
		if foreground == request.SourceHWND && request.TargetHWND != 0 && isMabinogiWindow(request.TargetHWND) {
			skillBarRecoveryTarget.Store(request.TargetHWND)
			deferNativeSkillBarForegroundRestore(request.SourceHWND, request.TargetHWND, request.Session)
		}
		logger.Printf(
			"native skill bar activation rejected: slot=%d could not restore Client.exe target=%#x foreground=%#x",
			index+1, request.TargetHWND, foreground,
		)
		setNativeSkillBarFeedback(index, true)
		return
	}
	// Foreground changes synchronously, but DirectInput may need one frame to
	// reacquire the device after Client.exe receives focus again.
	time.Sleep(25 * time.Millisecond)
	if !nativeSkillBarSessionCurrent(request.Session) {
		skillBarSending.Store(false)
		return
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground != request.TargetHWND || !isMabinogiWindow(foreground) {
		skillBarSending.Store(false)
		skillBarActivationFailed.Add(1)
		logger.Printf("native skill bar activation rejected: slot=%d target lost foreground before SendInput", index+1)
		setNativeSkillBarFeedback(index, true)
		return
	}
	setNativeSkillBarFeedback(index, false)
	defer skillBarSending.Store(false)
	if err := sendSkillBarSequenceForRequest(sequence, request); err != nil {
		skillBarActivationFailed.Add(1)
		logger.Println("native skill bar key sequence failed:", err)
		setNativeSkillBarFeedback(index, true)
		return
	}
	skillBarActivationSucceeded.Add(1)
	logger.Printf("native skill bar key sequence sent: slot=%d skill=%d steps=%d target=%#x", index+1, slot.SkillID, len(sequence), request.TargetHWND)
}

func nativeSkillBarMouseButtonReleased(virtualKey uintptr) bool {
	state, _, _ := procGetAsyncKeyState.Call(virtualKey)
	return uint16(state)&0x8000 == 0
}

func nativeSkillBarMouseButtonsReleased() bool {
	return nativeSkillBarMouseButtonReleased(skillBarMouseButtonLeft) &&
		nativeSkillBarMouseButtonReleased(skillBarMouseButtonRight)
}

func waitForNativeSkillBarMouseButtonsRelease(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if nativeSkillBarMouseButtonsReleased() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func classifyNativeSkillBarForeground(foreground, target, source uintptr) uint8 {
	if target != 0 && foreground == target {
		return nativeSkillBarForegroundTarget
	}
	if foreground == 0 {
		return nativeSkillBarForegroundNull
	}
	if source != 0 && foreground == source {
		return nativeSkillBarForegroundSource
	}
	return nativeSkillBarForegroundThirdParty
}

func waitForNativeSkillBarForegroundContext(request nativeSkillBarActivationRequest, timeout time.Duration) (uint8, uintptr, bool) {
	deadline := time.Now().Add(timeout)
	started := time.Now()
	sawTransient := false
	for {
		if !nativeSkillBarSessionCurrent(request.Session) {
			return nativeSkillBarForegroundNull, 0, false
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		state := classifyNativeSkillBarForeground(foreground, request.TargetHWND, request.SourceHWND)
		if state != nativeSkillBarForegroundNull {
			if sawTransient {
				logger.Printf("native skill bar foreground context settled: hwnd=%#x wait=%s", foreground, time.Since(started).Round(time.Millisecond))
			}
			return state, foreground, true
		}
		sawTransient = true
		if time.Now().After(deadline) {
			logger.Printf("native skill bar foreground context remained NULL for %s", timeout)
			return nativeSkillBarForegroundNull, 0, false
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func focusNativeSkillBarGameTarget(request nativeSkillBarActivationRequest, timeout time.Duration) bool {
	target := request.TargetHWND
	if !nativeSkillBarSessionCurrent(request.Session) || target == 0 || !isMabinogiWindow(target) {
		return false
	}
	state, foreground, observed := waitForNativeSkillBarForegroundContext(request, skillBarForegroundSettle)
	if !observed {
		return false
	}
	if state == nativeSkillBarForegroundThirdParty {
		logger.Printf("native skill bar foreground restore cancelled: user switched to hwnd=%#x", foreground)
		return false
	}
	if !nativeSkillBarMouseButtonsReleased() {
		logger.Printf("native skill bar foreground restore cancelled: a physical mouse button is down")
		return false
	}
	if state == nativeSkillBarForegroundTarget {
		if !isMabinogiWindow(foreground) {
			return false
		}
		skillBarRecoveryTarget.CompareAndSwap(target, 0)
		logger.Printf("native skill bar target already foreground: target=%#x", target)
		return true
	}
	if state != nativeSkillBarForegroundSource {
		return false
	}
	requested, _, _ := procSetForegroundWindow.Call(target)
	deadline := time.Now().Add(timeout)
	for {
		if !nativeSkillBarSessionCurrent(request.Session) {
			return false
		}
		foreground, _, _ = procGetForegroundWindow.Call()
		switch classifyNativeSkillBarForeground(foreground, target, request.SourceHWND) {
		case nativeSkillBarForegroundTarget:
			if !isMabinogiWindow(foreground) {
				return false
			}
			skillBarRecoveryTarget.CompareAndSwap(target, 0)
			logger.Printf("native skill bar foreground restored: target=%#x SetForegroundWindow=%v", target, requested != 0)
			return true
		case nativeSkillBarForegroundThirdParty:
			logger.Printf("native skill bar foreground restore cancelled during settle: user switched to hwnd=%#x", foreground)
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sendSkillBarKey(spec skillBarKeySpec) error {
	return sendSkillBarChord([]skillBarKeySpec{spec})
}

func sendSkillBarSequence(sequence [][]string) error {
	foreground, _, _ := procGetForegroundWindow.Call()
	return sendSkillBarSequenceToTarget(sequence, foreground)
}

func sendSkillBarSequenceToTarget(sequence [][]string, target uintptr) error {
	return sendSkillBarSequenceForRequest(sequence, nativeSkillBarActivationRequest{TargetHWND: target})
}

func sendSkillBarSequenceForRequest(sequence [][]string, request nativeSkillBarActivationRequest) error {
	target := request.TargetHWND
	for step, codes := range sequence {
		if request.Session != 0 && !nativeSkillBarSessionCurrent(request.Session) {
			return errors.New("skill bar session ended before skill input")
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		if foreground != target || !isMabinogiWindow(foreground) {
			return errors.New("Mabinogi lost foreground before skill input")
		}
		specs := make([]skillBarKeySpec, 0, len(codes))
		for _, code := range codes {
			spec, ok := skillBarKeyForCode(code)
			if !ok {
				return fmt.Errorf("unsupported skill bar key %q", code)
			}
			specs = append(specs, spec)
		}
		if err := sendSkillBarChord(specs); err != nil {
			return fmt.Errorf("send step %d: %w", step+1, err)
		}
		// Never interrupt a chord before its key-up batch. Cancellation is
		// observed only here, after sendSkillBarChord has released every key.
		if request.Session != 0 && !nativeSkillBarSessionCurrent(request.Session) {
			return errors.New("skill bar session ended after releasing current chord")
		}
		if step+1 < len(sequence) {
			time.Sleep(skillBarSequenceDelay)
		}
	}
	return nil
}

func nativeSkillBarSessionCurrent(session uint64) bool {
	return session != 0 && session == skillBarSession.Load() && !skillBarClosing.Load()
}

func sendSkillBarChord(specs []skillBarKeySpec) error {
	inputs, err := nativeSkillBarInputsForChord(specs)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errors.New("empty skill bar key chord")
	}
	keyCount := len(inputs) / 2
	modifierCount := 0
	for _, spec := range specs {
		if nativeSkillBarModifierKey(spec.VirtualKey) {
			modifierCount++
		}
	}
	presses, releases := inputs[:keyCount], inputs[keyCount:]
	// Match the reference sender precisely: modifiers and target keys use
	// separate SendInput calls so DirectInput can observe the chord transition.
	if modifierCount > 0 {
		if err := sendNativeSkillBarInputs(presses[:modifierCount]); err != nil {
			// SendInput may report a partial write. Always issue matching key-up
			// events for the modifier batch so an error cannot leave Ctrl/Alt/Shift held.
			_ = sendNativeSkillBarInputs(releases[keyCount-modifierCount:])
			return fmt.Errorf("press chord modifiers: %w", err)
		}
	}
	if modifierCount < keyCount {
		if err := sendNativeSkillBarInputs(presses[modifierCount:]); err != nil {
			// Release every key because the failing SendInput call may have inserted
			// only part of the target batch.
			_ = sendNativeSkillBarInputs(releases)
			return fmt.Errorf("press chord targets: %w", err)
		}
	}
	time.Sleep(skillBarChordHold)
	targetReleaseCount := keyCount - modifierCount
	var releaseErr error
	if targetReleaseCount > 0 {
		if err := sendNativeSkillBarInputs(releases[:targetReleaseCount]); err != nil {
			releaseErr = fmt.Errorf("release chord targets: %w", err)
		}
	}
	if modifierCount > 0 {
		if err := sendNativeSkillBarInputs(releases[targetReleaseCount:]); err != nil && releaseErr == nil {
			releaseErr = fmt.Errorf("release chord modifiers: %w", err)
		}
	}
	return releaseErr
}

func sendNativeSkillBarInputs(inputs []nativeSkillBarInput) error {
	if len(inputs) == 0 {
		return errors.New("empty skill bar key input batch")
	}
	sent, _, sendErr := procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if sent != uintptr(len(inputs)) {
		if sendErr != nil && !errors.Is(sendErr, syscall.Errno(0)) {
			return sendErr
		}
		return fmt.Errorf("SendInput sent %d of %d key events", sent, len(inputs))
	}
	return nil
}

func nativeSkillBarInputsForChord(specs []skillBarKeySpec) ([]nativeSkillBarInput, error) {
	// Match the reference sender: modifiers go down before the target key and
	// come up after it, even if the browser recorded the chord in another order.
	ordered := make([]skillBarKeySpec, 0, len(specs))
	for _, spec := range specs {
		if nativeSkillBarModifierKey(spec.VirtualKey) {
			ordered = append(ordered, spec)
		}
	}
	for _, spec := range specs {
		if !nativeSkillBarModifierKey(spec.VirtualKey) {
			ordered = append(ordered, spec)
		}
	}
	inputs := make([]nativeSkillBarInput, len(ordered)*2)
	for index, spec := range ordered {
		scanCode, _, callErr := procMapVirtualKeyW.Call(uintptr(spec.VirtualKey), mapvkVKToVSC)
		if scanCode == 0 {
			if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
				return nil, callErr
			}
			return nil, fmt.Errorf("MapVirtualKeyW returned no scan code for VK %#x", spec.VirtualKey)
		}
		flags := uint32(keyeventfScanCode)
		if spec.Extended {
			flags |= keyeventfExtendedKey
		}
		inputs[index] = nativeSkillBarInput{
			Type: inputKeyboard, Keyboard: nativeSkillBarKeyboardInput{
				ScanCode: uint16(scanCode), Flags: flags, ExtraInfo: skillBarInjectedTag,
			},
		}
		releaseIndex := len(ordered) + (len(ordered) - 1 - index)
		inputs[releaseIndex] = nativeSkillBarInput{
			Type: inputKeyboard, Keyboard: nativeSkillBarKeyboardInput{
				ScanCode: uint16(scanCode), Flags: flags | keyeventfKeyUp, ExtraInfo: skillBarInjectedTag,
			},
		}
	}
	return inputs, nil
}

func nativeSkillBarModifierKey(virtualKey uint16) bool {
	return virtualKey >= 0xA0 && virtualKey <= 0xA5
}

func persistNativeSkillBarWindowPosition(hwnd uintptr) {
	var rect nativeRect
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	x, y := int(rect.Left), int(rect.Top)
	updateNativeSkillBarPosition(x, y)
	persistNativeSkillBarPosition(x, y)
}

func persistNativeSkillBarPosition(x, y int) {
	if err := updateConfig(func(cfg *config) {
		cfg.SkillBarX = x
		cfg.SkillBarY = y
		cfg.SkillBarPositionSet = true
	}); err != nil {
		logger.Println("save native skill bar position failed:", err)
		return
	}
	logger.Printf("native skill bar position persisted: (%d,%d)", x, y)
}

func monitorSkillBarForeground(hwnd uintptr) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if hwnd == 0 || skillBarClosing.Load() || skillBarHWND.Load() != hwnd {
			return
		}
		foreground, _, _ := procGetForegroundWindow.Call()
		gameForeground := isMabinogiWindow(foreground)
		if gameForeground {
			skillBarForegroundGame.Store(foreground)
		}
		if foreground != 0 && foreground != hwnd {
			// Cache every external foreground, not only Client.exe. A recent
			// third-party/main-window token must block reuse of an older game HWND.
			rememberNativeSkillBarExternalForeground(foreground)
			if !gameForeground {
				skillBarRecoveryTarget.Store(0)
			}
		}
		shouldShow := skillBarActive.Load() && !trayPlayerOverlayHidden.Load() &&
			(isCurrentProcessWindow(foreground) || gameForeground)
		requestNativeSkillBarVisibility(shouldShow)
		if shouldShow {
			renderNativeSkillBar()
		}
	}
}

func requestNativeSkillBarVisibility(visible bool) {
	if visible && !skillBarHookOperational.Load() {
		visible = false
	}
	skillBarDesiredVisible.Store(visible)
	hwnd := skillBarHWND.Load()
	if hwnd == 0 || skillBarClosing.Load() {
		return
	}
	if visible == skillBarVisible.Load() && !skillBarVisiblePosted.Load() {
		return
	}
	if !skillBarVisiblePosted.CompareAndSwap(false, true) {
		return
	}
	if posted, _, _ := procPostMessageW.Call(hwnd, wmSkillBarVisible, uintptr(skillBarWindowSession.Load()), 0); posted == 0 {
		skillBarVisiblePosted.Store(false)
	}
}

func handleNativeSkillBarMouseHookUnavailable(reason string) {
	if !skillBarHookOperational.Swap(false) {
		return
	}
	hwnd := skillBarHWND.Load()
	if hwnd == 0 {
		return
	}
	logger.Printf("native skill bar mouse hook unavailable: %s", reason)
	procSendMessageW.Call(hwnd, wmSkillBarHookLost, uintptr(skillBarWindowSession.Load()), 0)
}

func nativeSkillBarCooldowns() map[uint16]nativeSkillOverlayItem {
	skillOverlayState.RLock()
	data := append([]byte(nil), skillOverlayState.data...)
	skillOverlayState.RUnlock()
	var message nativeSkillOverlayMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil
	}
	result := make(map[uint16]nativeSkillOverlayItem, len(message.Items))
	for _, item := range message.Items {
		result[item.SkillID] = item
	}
	return result
}

func nativeSkillBarCooldownText(item nativeSkillOverlayItem, nowMs int64) (string, float64) {
	readyAt := max(item.ReadyAtMs, item.AccumulatedReadyAtMs)
	if readyAt <= nowMs {
		return "", 0
	}
	remaining := readyAt - nowMs
	duration := max(int64(1), readyAt-item.UsedAtMs)
	text := strconv.FormatInt((remaining+999)/1000, 10)
	if remaining < 1_000 {
		// Sub-second countdowns use one decimal place, but never show the
		// misleading boundary values 1.0 or 0.0. Round the remaining time up to
		// the next tenth, cap the first sub-second frame at 0.9, and hold the
		// final visible value at 0.1 until the cooldown is ready.
		tenths := min(int64(9), max(int64(1), (remaining+99)/100))
		text = "0." + strconv.FormatInt(tenths, 10)
	}
	return text, min(1, max(0, float64(remaining)/float64(duration)))
}
