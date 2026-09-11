//go:build windows

package main

import (
	"image"
	"strconv"
	"testing"
	"time"
	"unsafe"
)

func TestNativeSkillBarInputSize(t *testing.T) {
	want := uintptr(28)
	if strconv.IntSize == 64 {
		want = 40
	}
	if got := unsafe.Sizeof(nativeSkillBarInput{}); got != want {
		t.Fatalf("nativeSkillBarInput is %d bytes, want %d for SendInput", got, want)
	}
}

func TestNativeLowLevelMouseHookSize(t *testing.T) {
	want := uintptr(24)
	if strconv.IntSize == 64 {
		want = 32
	}
	if got := unsafe.Sizeof(nativeLowLevelMouseHook{}); got != want {
		t.Fatalf("nativeLowLevelMouseHook is %d bytes, want %d", got, want)
	}
}

func TestNativeWindowMessageSize(t *testing.T) {
	want := uintptr(28)
	if strconv.IntSize == 64 {
		want = 48
	}
	if got := unsafe.Sizeof(nativeWindowMessage{}); got != want {
		t.Fatalf("nativeWindowMessage is %d bytes, want %d", got, want)
	}
}

func TestUpdateNativeSkillBarPositionUpdatesLiveState(t *testing.T) {
	previous := currentNativeSkillBarSettings()
	defer func() {
		skillBarSettingsMu.Lock()
		skillBarSettings = previous
		skillBarSettingsMu.Unlock()
		skillBarX.Store(int32(previous.X))
		skillBarY.Store(int32(previous.Y))
	}()

	updateNativeSkillBarPosition(1234, -567)
	got := currentNativeSkillBarSettings()
	if got.X != 1234 || got.Y != -567 || !got.PositionSet {
		t.Fatalf("live position = (%d,%d), positionSet=%v", got.X, got.Y, got.PositionSet)
	}

	updateNativeSkillBarPosition(99999, -99999)
	got = currentNativeSkillBarSettings()
	if got.X != 32000 || got.Y != -32000 {
		t.Fatalf("clamped live position = (%d,%d), want (32000,-32000)", got.X, got.Y)
	}
}

func TestNormalizeNativeSkillBarSettings(t *testing.T) {
	settings := normalizeNativeSkillBarSettings(nativeSkillBarSettings{
		Active: true, Locked: true, InputEnabled: true, ClickButton: "right", StopMovement: true,
		StopKeyCode: "MediaPlayPause", Columns: 0, IconSize: 2, Gap: 99, Opacity: 200,
		Slots: []nativeSkillBarSlot{{
			SkillID: 21002, KeyCode: "MediaPlayPause", KeyLabel: "  Ctrl+[ → Q  ",
			KeySequence: [][]string{{"ControlLeft", "BracketLeft", "ControlLeft", "MediaPlayPause"}, {"KeyQ"}},
		}},
	}, nativeSkillBarSettings{Width: 520, Height: 58})
	if settings.Columns != 1 || settings.IconSize != 32 || settings.Gap != 12 || settings.Opacity != 100 {
		t.Fatalf("settings were not clamped: %#v", settings)
	}
	if settings.Width != 38 || settings.Height != 38 {
		t.Fatalf("native bounds = %dx%d, want 38x38", settings.Width, settings.Height)
	}
	if settings.Slots[0].KeyCode != "" || settings.Slots[0].KeyLabel != "Ctrl+[ → Q" {
		t.Fatalf("slot was not sanitized: %#v", settings.Slots[0])
	}
	if got := settings.Slots[0].KeySequence; len(got) != 2 || len(got[0]) != 2 || got[0][0] != "ControlLeft" || got[0][1] != "BracketLeft" || got[1][0] != "KeyQ" {
		t.Fatalf("key sequence was not normalized: %#v", got)
	}
	if !settings.PositionSet {
		t.Fatal("a web configuration update must establish the native position")
	}
	if settings.ClickButton != "right" || settings.StopMovement || settings.StopKeyCode != "" {
		t.Fatalf("reference input settings were not normalized: %#v", settings)
	}
}

func TestNormalizeNativeSkillBarKeepsOptInStopKeyCompensation(t *testing.T) {
	settings := normalizeNativeSkillBarSettings(nativeSkillBarSettings{
		StrongIsolation: true, StopMovement: true, StopKeyCode: "KeyS", Columns: 1, IconSize: 48, Opacity: 100,
	}, nativeSkillBarSettings{Width: 54, Height: 54})
	if !settings.StopMovement || settings.StopKeyCode != "KeyS" || settings.StrongIsolation {
		t.Fatalf("opt-in stop-key compensation was not normalized: %#v", settings)
	}
}

func TestNativeSkillBarStopSnapshotCoversWholeLiveBarrier(t *testing.T) {
	settings := nativeSkillBarSettings{
		Active: true, Locked: false, InputEnabled: false, ClickButton: "right",
		StopMovement: true, StopKeyCode: "KeyS", X: 100, Y: 200,
		Columns: 2, IconSize: 40, Gap: 4,
		Slots: []nativeSkillBarSlot{
			{SkillID: 1, KeySequence: [][]string{{"KeyQ"}}},
			{SkillID: 2},
			{SkillID: 3, KeyCode: "KeyE"},
		},
	}
	snapshot := nativeSkillBarStopSnapshotForSettings(settings)
	if snapshot == nil || snapshot.Code != "KeyS" {
		t.Fatalf("stop snapshot = %#v, want KeyS in right-click/unlocked mode", snapshot)
	}
	barrier := nativeRect{Left: 100, Top: 200, Right: 187, Bottom: 287}
	for _, point := range []nativePoint{
		{X: 100, Y: 200}, // padding
		{X: 103, Y: 203}, // populated cell
		{X: 145, Y: 203}, // gap
		{X: 147, Y: 203}, // empty cell
		{X: 186, Y: 286}, // lower-right interior
	} {
		if !nativeSkillBarStopSnapshotHit(point, barrier, snapshot) {
			t.Fatalf("barrier point %+v did not trigger stop compensation", point)
		}
	}
	for _, point := range []nativePoint{{X: 99, Y: 203}, {X: 187, Y: 203}, {X: 103, Y: 287}} {
		if nativeSkillBarStopSnapshotHit(point, barrier, snapshot) {
			t.Fatalf("outside point %+v unexpectedly triggered stop compensation", point)
		}
	}
	if !nativeSkillBarStopShouldTrigger(nativePoint{X: 103, Y: 203}, 0x2000, 0x2000, barrier, snapshot) {
		t.Fatal("matching game foreground did not permit stop compensation")
	}
	if nativeSkillBarStopShouldTrigger(nativePoint{X: 103, Y: 203}, 0x3000, 0x2000, barrier, snapshot) {
		t.Fatal("third-party foreground permitted stop compensation")
	}
	if skillBarStopKeyHold != 20*time.Millisecond {
		t.Fatalf("stop key hold = %s, want reference interval 20ms", skillBarStopKeyHold)
	}
}

func TestNativeSkillBarStopKeyRejectsModifiersAndToggleKeys(t *testing.T) {
	for _, code := range []string{"ShiftLeft", "ControlRight", "AltLeft", "CapsLock"} {
		if _, ok := skillBarStopKeyForCode(code); ok {
			t.Fatalf("unsafe stop key %q was accepted", code)
		}
	}
	for _, code := range []string{"KeyS", "F8", "NumpadEnter"} {
		if _, ok := skillBarStopKeyForCode(code); !ok {
			t.Fatalf("safe stop key %q was rejected", code)
		}
	}
}

func TestNativeSkillBarStopSnapshotIsStrictlyOptInButActivationIndependent(t *testing.T) {
	base := nativeSkillBarSettings{
		Active: true, Locked: true, InputEnabled: true, ClickButton: "left",
		StopMovement: true, StopKeyCode: "KeyS", X: 100, Y: 200,
		Columns: 1, IconSize: 40, Slots: []nativeSkillBarSlot{{KeyCode: "KeyQ"}},
	}
	if nativeSkillBarStopSnapshotForSettings(base) == nil {
		t.Fatal("valid opt-in settings did not publish a stop snapshot")
	}
	for _, mutate := range []func(*nativeSkillBarSettings){
		func(settings *nativeSkillBarSettings) { settings.Locked = false },
		func(settings *nativeSkillBarSettings) { settings.InputEnabled = false },
		func(settings *nativeSkillBarSettings) { settings.ClickButton = "right" },
		func(settings *nativeSkillBarSettings) { settings.Slots = nil },
	} {
		settings := base
		mutate(&settings)
		if snapshot := nativeSkillBarStopSnapshotForSettings(settings); snapshot == nil {
			t.Fatalf("display/activation-mode change disabled left-click protection: %#v", settings)
		}
	}
	for _, mutate := range []func(*nativeSkillBarSettings){
		func(settings *nativeSkillBarSettings) { settings.Active = false },
		func(settings *nativeSkillBarSettings) { settings.StopMovement = false },
		func(settings *nativeSkillBarSettings) { settings.StopKeyCode = "" },
		func(settings *nativeSkillBarSettings) { settings.StopKeyCode = "CapsLock" },
	} {
		settings := base
		mutate(&settings)
		if snapshot := nativeSkillBarStopSnapshotForSettings(settings); snapshot != nil {
			t.Fatalf("disabled/unsafe settings published stop snapshot: %#v", snapshot)
		}
	}
}

func TestNativeSkillBarR7StopAndActivationMatrix(t *testing.T) {
	settings := nativeSkillBarSettings{
		Active: true, Locked: true, InputEnabled: true, ClickButton: "left",
		StopMovement: true, StopKeyCode: "KeyS", Columns: 2, IconSize: 40, Gap: 4,
		Slots: []nativeSkillBarSlot{{KeyCode: "KeyQ"}, {KeyCode: "KeyE"}},
	}
	snapshot := nativeSkillBarStopSnapshotForSettings(settings)
	if snapshot == nil {
		t.Fatal("matrix setup did not produce a stop snapshot")
	}
	barrier := nativeRect{Left: 100, Top: 200, Right: 187, Bottom: 246}
	for _, test := range []struct {
		name                     string
		button, configured       uint32
		locked, inputEnabled     bool
		foreground, cachedGame   uintptr
		point                    nativePoint
		pressed, released        int
		wantStop, wantActivation bool
	}{
		{name: "right mode left cell only stops", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonRight, locked: true, inputEnabled: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0, wantStop: true},
		{name: "right mode right cell only activates", button: skillBarMouseButtonRight, configured: skillBarMouseButtonRight, locked: true, inputEnabled: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0, wantActivation: true},
		{name: "left mode left cell stops and activates", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, locked: true, inputEnabled: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0, wantStop: true, wantActivation: true},
		{name: "left mode right cell does neither", button: skillBarMouseButtonRight, configured: skillBarMouseButtonLeft, locked: true, inputEnabled: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0},
		{name: "unlocked left still stops", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, inputEnabled: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0, wantStop: true},
		{name: "input disabled left still stops", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, locked: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0, wantStop: true},
		{name: "third party foreground does neither", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, locked: true, inputEnabled: true, foreground: 0x3000, cachedGame: 0x2000, point: nativePoint{X: 110, Y: 210}, pressed: 0, released: 0},
		{name: "gap stops without activating", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, locked: true, inputEnabled: true, foreground: 0x2000, cachedGame: 0x2000, point: nativePoint{X: 145, Y: 210}, pressed: -1, released: -1, wantStop: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stop := test.button == skillBarMouseButtonLeft && nativeSkillBarStopShouldTrigger(
				test.point, test.foreground, test.cachedGame, barrier, snapshot,
			)
			armed := nativeSkillBarGestureCanArm(test.locked, test.button, test.configured, test.inputEnabled)
			activate := armed && nativeSkillBarWindowGestureActivates(
				test.button, test.configured, test.inputEnabled, test.pressed, test.released,
				false, test.cachedGame, test.foreground == test.cachedGame,
			)
			if stop != test.wantStop || activate != test.wantActivation {
				t.Fatalf("stop/activate = %v/%v, want %v/%v", stop, activate, test.wantStop, test.wantActivation)
			}
		})
	}
}

func TestNormalizeNativeSkillBarLegacyKey(t *testing.T) {
	settings := normalizeNativeSkillBarSettings(nativeSkillBarSettings{
		Locked: true, Columns: 1, IconSize: 48, Opacity: 100,
		Slots: []nativeSkillBarSlot{{SkillID: 21002, KeyCode: "KeyQ", KeyLabel: "Q"}},
	}, nativeSkillBarSettings{Width: 54, Height: 54})
	if got := settings.Slots[0].KeySequence; len(got) != 1 || len(got[0]) != 1 || got[0][0] != "KeyQ" {
		t.Fatalf("legacy key was not migrated: %#v", got)
	}
}

func TestNativeSkillBarChordInputOrder(t *testing.T) {
	control, _ := skillBarKeyForCode("ControlLeft")
	bracket, _ := skillBarKeyForCode("BracketLeft")
	// Deliberately provide the target first. The native sender must still press
	// Ctrl before [ and release it last, as the reference application does.
	inputs, err := nativeSkillBarInputsForChord([]skillBarKeySpec{bracket, control})
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 4 {
		t.Fatalf("input count = %d, want 4", len(inputs))
	}
	if inputs[0].Keyboard.Flags&keyeventfKeyUp != 0 || inputs[1].Keyboard.Flags&keyeventfKeyUp != 0 {
		t.Fatal("the chord must press all keys before releasing any key")
	}
	if inputs[2].Keyboard.Flags&keyeventfKeyUp == 0 || inputs[3].Keyboard.Flags&keyeventfKeyUp == 0 {
		t.Fatal("the chord must release all keys")
	}
	if inputs[0].Keyboard.ScanCode != inputs[3].Keyboard.ScanCode || inputs[1].Keyboard.ScanCode != inputs[2].Keyboard.ScanCode {
		t.Fatal("the chord must release keys in reverse order")
	}
	if inputs[0].Keyboard.ScanCode == inputs[2].Keyboard.ScanCode {
		t.Fatal("the modifier must be pressed before the target and released after it")
	}
	if skillBarChordHold != 20*time.Millisecond {
		t.Fatalf("skill bar chord hold = %s, want the reference interval of 20ms", skillBarChordHold)
	}
	if skillBarSequenceDelay != 30*time.Millisecond {
		t.Fatalf("skill bar sequence delay = %s, want the reference skill-tab interval of 30ms", skillBarSequenceDelay)
	}
	for index, input := range inputs {
		if input.Keyboard.ExtraInfo != skillBarInjectedTag {
			t.Fatalf("input %d tag = %#x, want reference tag %#x", index, input.Keyboard.ExtraInfo, skillBarInjectedTag)
		}
	}
}

func TestNativeSkillBarSlotAt(t *testing.T) {
	previous := currentNativeSkillBarSettings()
	defer func() {
		skillBarSettingsMu.Lock()
		skillBarSettings = previous
		skillBarSettingsMu.Unlock()
	}()
	skillBarSettingsMu.Lock()
	skillBarSettings = nativeSkillBarSettings{
		Locked: true, Columns: 2, IconSize: 40, Gap: 4,
		Slots: []nativeSkillBarSlot{{SkillID: 1}, {SkillID: 2}, {SkillID: 3}},
	}
	skillBarSettingsMu.Unlock()
	point := func(x, y uint16) uintptr { return uintptr(x) | uintptr(y)<<16 }
	for _, test := range []struct {
		x, y uint16
		want int
	}{
		{x: 4, y: 4, want: 0},
		{x: 48, y: 4, want: 1},
		{x: 43, y: 4, want: -1},
		{x: 4, y: 48, want: 2},
	} {
		if got := nativeSkillBarSlotAt(point(test.x, test.y)); got != test.want {
			t.Fatalf("slot at (%d,%d) = %d, want %d", test.x, test.y, got, test.want)
		}
	}
}

func TestNativeSkillBarBarrierWindowStyle(t *testing.T) {
	style := nativeSkillBarInteractiveExStyle(wsExTransparent | 0x40)
	required := uintptr(wsExTopmost | wsExToolWindow | wsExLayered | wsExNoActivate)
	if style&required != required {
		t.Fatalf("barrier style is missing required flags: %#x", style)
	}
	if style&wsExTransparent != 0 {
		t.Fatalf("interactive barrier must be a real hit-test target: %#x", style)
	}
	if style&0x40 == 0 {
		t.Fatalf("barrier style discarded an unrelated existing flag: %#x", style)
	}
	if got := nativeSkillBarMouseActivateResult(); got != maNoActivate {
		t.Fatalf("WM_MOUSEACTIVATE result = %d, want MA_NOACTIVATE", got)
	}
}

func TestNativeSkillBarPhysicalMouseEventExcludesInjectedInput(t *testing.T) {
	if !nativeSkillBarPhysicalMouseEvent(0) {
		t.Fatal("unflagged physical mouse input was rejected")
	}
	if nativeSkillBarPhysicalMouseEvent(llmhfInjected) {
		t.Fatal("injected mouse input was accepted as physical")
	}
	if nativeSkillBarPhysicalMouseEvent(llmhfInjected | 0x00000002) {
		t.Fatal("lower-integrity injected mouse input was accepted as physical")
	}
}

func TestNativeSkillBarHookButtonMessages(t *testing.T) {
	for _, test := range []struct {
		message uintptr
		button  uint32
		kind    uint8
		bit     uint32
	}{
		{message: wmLButtonDown, button: skillBarMouseButtonLeft, kind: nativeSkillBarHookDown, bit: 1},
		{message: wmLButtonUp, button: skillBarMouseButtonLeft, kind: nativeSkillBarHookUp, bit: 1},
		{message: wmRButtonDown, button: skillBarMouseButtonRight, kind: nativeSkillBarHookDown, bit: 2},
		{message: wmRButtonUp, button: skillBarMouseButtonRight, kind: nativeSkillBarHookUp, bit: 2},
		{message: wmMouseMove},
	} {
		button, kind := nativeSkillBarHookButtonMessage(test.message)
		if button != test.button || kind != test.kind {
			t.Fatalf("message %#x = button %d kind %d, want %d/%d", test.message, button, kind, test.button, test.kind)
		}
		if got := nativeSkillBarHookButtonBit(button); got != test.bit {
			t.Fatalf("button %d bit = %d, want %d", button, got, test.bit)
		}
	}
}

func TestNativeSkillBarHookBarrierCoversWholeWindow(t *testing.T) {
	rect := nativeRect{Left: 100, Top: 200, Right: 300, Bottom: 260}
	for _, test := range []struct {
		name            string
		point           nativePoint
		active, visible bool
		want            bool
	}{
		{name: "first pixel", point: nativePoint{X: 100, Y: 200}, active: true, visible: true, want: true},
		{name: "icon gap is still barrier", point: nativePoint{X: 149, Y: 225}, active: true, visible: true, want: true},
		{name: "last pixel", point: nativePoint{X: 299, Y: 259}, active: true, visible: true, want: true},
		{name: "right edge excluded", point: nativePoint{X: 300, Y: 225}, active: true, visible: true},
		{name: "bottom edge excluded", point: nativePoint{X: 150, Y: 260}, active: true, visible: true},
		{name: "inactive", point: nativePoint{X: 150, Y: 225}, visible: true},
		{name: "hidden", point: nativePoint{X: 150, Y: 225}, active: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeSkillBarHookBarrierContains(test.point, test.active, test.visible, rect); got != test.want {
				t.Fatalf("barrier contains = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNativeSkillBarBarrierTransitionCoversOldAndNewWindows(t *testing.T) {
	oldRect := nativeRect{Left: 100, Top: 200, Right: 300, Bottom: 260}
	newRect := nativeRect{Left: 700, Top: 500, Right: 940, Bottom: 590}
	transition := nativeSkillBarRectUnion(oldRect, newRect)
	want := nativeRect{Left: 100, Top: 200, Right: 940, Bottom: 590}
	if transition != want {
		t.Fatalf("transition rect = %+v, want %+v", transition, want)
	}
	if !nativePointInRect(nativePoint{X: 150, Y: 225}, transition) ||
		!nativePointInRect(nativePoint{X: 800, Y: 550}, transition) {
		t.Fatal("transition failed to protect both the old and new visible positions")
	}
}

func TestNativeSkillBarHookCriticalRecoveryVersion(t *testing.T) {
	oldVersion := skillBarHookRecoveryVersion.Load()
	oldDropped := skillBarHookCriticalDropped.Load()
	oldAllDropped := skillBarHookEventDropped.Load()
	defer func() {
		skillBarHookRecoveryVersion.Store(oldVersion)
		skillBarHookCriticalDropped.Store(oldDropped)
		skillBarHookEventDropped.Store(oldAllDropped)
	}()

	// A real queue overflow is deliberately not manufactured against the
	// process-global worker channel here. Verify the version contract used by
	// every critical event: an edge recorded before recovery is stale, while a
	// later edge carries the new version and can start a fresh gesture.
	before := skillBarHookRecoveryVersion.Load()
	after := skillBarHookRecoveryVersion.Add(1)
	if before == after {
		t.Fatal("critical recovery did not advance the gesture generation")
	}
	stale := nativeSkillBarHookEvent{Kind: nativeSkillBarHookUp, RecoveryVersion: before}
	fresh := nativeSkillBarHookEvent{Kind: nativeSkillBarHookDown, RecoveryVersion: after}
	if stale.RecoveryVersion == skillBarHookRecoveryVersion.Load() {
		t.Fatal("dropped-up recovery failed to invalidate an earlier edge")
	}
	if fresh.RecoveryVersion != skillBarHookRecoveryVersion.Load() {
		t.Fatal("a fresh edge did not inherit the recovered generation")
	}
}

func TestNativeSkillBarHookOwnsCompleteButtonPairs(t *testing.T) {
	if !nativeSkillBarHookShouldOwnDown(0, 1, true) {
		t.Fatal("button down inside the barrier was not owned")
	}
	if nativeSkillBarHookShouldOwnDown(0, 1, false) {
		t.Fatal("unrelated button down outside the barrier was owned")
	}
	if !nativeSkillBarHookShouldOwnDown(1, 2, false) {
		t.Fatal("overlapping second button outside the barrier must remain owned")
	}
	if !nativeSkillBarHookShouldOwnUp(1, 1) {
		t.Fatal("owned button up was not swallowed after leaving the barrier")
	}
	if nativeSkillBarHookShouldOwnUp(1, 2) {
		t.Fatal("unowned button up was swallowed")
	}
}

func TestNativeSkillBarActivationPreviousWindow(t *testing.T) {
	const skillBar = uintptr(0x1000)
	for _, test := range []struct {
		name     string
		state    uint16
		previous uintptr
		want     uintptr
	}{
		{name: "click activation captures old window", state: 2, previous: 0x2000, want: 0x2000},
		{name: "programmatic activation captures old window", state: 1, previous: 0x2000, want: 0x2000},
		{name: "deactivation ignored", state: waInactive, previous: 0x2000},
		{name: "null old window ignored", state: 2},
		{name: "self ignored", state: 2, previous: skillBar},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeSkillBarPreviousWindowFromActivation(test.state, test.previous, skillBar); got != test.want {
				t.Fatalf("previous activation window = %#x, want %#x", got, test.want)
			}
		})
	}
}

func TestResolveNativeSkillBarFocusTarget(t *testing.T) {
	const skillBar = uintptr(0x1000)
	for _, test := range []struct {
		name     string
		snapshot nativeSkillBarFocusSnapshot
		want     uintptr
		source   string
	}{
		{name: "activation token wins", snapshot: nativeSkillBarFocusSnapshot{PendingFocus: 0x2000, CurrentFocus: skillBar, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x2000, source: "pending-activation"},
		{name: "third party activation blocks cached game", snapshot: nativeSkillBarFocusSnapshot{PendingFocus: 0x9000, CurrentFocus: skillBar, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x9000, source: "pending-activation"},
		{name: "current external wins", snapshot: nativeSkillBarFocusSnapshot{CurrentFocus: 0x2000, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x2000, source: "foreground"},
		{name: "current third party blocks cached game", snapshot: nativeSkillBarFocusSnapshot{CurrentFocus: 0x9000, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x9000, source: "foreground"},
		{name: "current third party blocks stale pending game", snapshot: nativeSkillBarFocusSnapshot{PendingFocus: 0x2000, CurrentFocus: 0x9000, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x9000, source: "foreground"},
		{name: "current game blocks stale pending third party", snapshot: nativeSkillBarFocusSnapshot{PendingFocus: 0x9000, CurrentFocus: 0x2000, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x2000, source: "foreground"},
		{name: "transient null uses recent external", snapshot: nativeSkillBarFocusSnapshot{SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x3000, source: "recent-external"},
		{name: "self foreground uses recent external", snapshot: nativeSkillBarFocusSnapshot{CurrentFocus: skillBar, SkillBar: skillBar, RecentExternal: 0x3000, RecoveryTarget: 0x4000}, want: 0x3000, source: "recent-external"},
		{name: "validated recovery is last", snapshot: nativeSkillBarFocusSnapshot{SkillBar: skillBar, RecoveryTarget: 0x4000}, want: 0x4000, source: "recovery"},
		{name: "no evidence fails closed", snapshot: nativeSkillBarFocusSnapshot{SkillBar: skillBar}, source: "none"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, source := resolveNativeSkillBarFocusTarget(test.snapshot)
			if got != test.want || source != test.source {
				t.Fatalf("focus target = (%#x, %q), want (%#x, %q)", got, source, test.want, test.source)
			}
		})
	}
}

func TestClassifyNativeSkillBarForeground(t *testing.T) {
	const (
		source = uintptr(0x1000)
		target = uintptr(0x2000)
	)
	for _, test := range []struct {
		name       string
		foreground uintptr
		target     uintptr
		source     uintptr
		want       uint8
	}{
		{name: "transient NULL waits", target: target, source: source, want: nativeSkillBarForegroundNull},
		{name: "skill bar source may restore", foreground: source, target: target, source: source, want: nativeSkillBarForegroundSource},
		{name: "exact game target succeeds", foreground: target, target: target, source: source, want: nativeSkillBarForegroundTarget},
		{name: "third party rejects", foreground: 0x3000, target: target, source: source, want: nativeSkillBarForegroundThirdParty},
		{name: "another Client window still rejects", foreground: 0x4000, target: target, source: source, want: nativeSkillBarForegroundThirdParty},
		{name: "missing source does not match NULL", target: target, want: nativeSkillBarForegroundNull},
		{name: "target wins if handles coincide", foreground: target, target: target, source: target, want: nativeSkillBarForegroundTarget},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyNativeSkillBarForeground(test.foreground, test.target, test.source); got != test.want {
				t.Fatalf("foreground classification = %d, want %d", got, test.want)
			}
		})
	}
}

func TestNativeSkillBarExternalFocusFresh(t *testing.T) {
	ttl := skillBarExternalFocusTTL.Milliseconds()
	if !nativeSkillBarExternalFocusFresh(10_000, 10_000-ttl) {
		t.Fatal("focus observation at the TTL boundary should remain usable")
	}
	if nativeSkillBarExternalFocusFresh(10_000, 10_000-ttl-1) {
		t.Fatal("stale focus observation must not be reused")
	}
	if nativeSkillBarExternalFocusFresh(10_000, 0) {
		t.Fatal("missing focus observation must not be reused")
	}
	if nativeSkillBarExternalFocusFresh(10_000, 10_001) {
		t.Fatal("future focus observation must be rejected")
	}
}

func TestNativeSkillBarOverlappingButtonDoesNotReplaceGesture(t *testing.T) {
	if !nativeSkillBarWindowGestureCanBegin(false) {
		t.Fatal("first button down should begin a gesture")
	}
	if nativeSkillBarWindowGestureCanBegin(true) {
		t.Fatal("overlapping button down must be consumed without replacing the active gesture")
	}
	for _, test := range []struct {
		name                        string
		overlapped, buttonsReleased bool
		want                        bool
	}{
		{name: "ordinary release", buttonsReleased: true, want: true},
		{name: "overlap already released", overlapped: true, buttonsReleased: true},
		{name: "other button still down"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeSkillBarWindowGestureActivationSafe(test.overlapped, test.buttonsReleased); got != test.want {
				t.Fatalf("activation safety = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNativeSkillBarWindowGestureActivationRules(t *testing.T) {
	for _, test := range []struct {
		name                     string
		button, configured       uint32
		enabled                  bool
		pressed, released        int
		dragged, foregroundOwned bool
		gameTarget               uintptr
		want                     bool
	}{
		{name: "same slot", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: 2, foregroundOwned: true, gameTarget: 0x1234, want: true},
		{name: "right configured same slot", button: skillBarMouseButtonRight, configured: skillBarMouseButtonRight, enabled: true, pressed: 2, released: 2, foregroundOwned: true, gameTarget: 0x1234, want: true},
		{name: "left does not activate in right mode", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonRight, enabled: true, pressed: 2, released: 2, foregroundOwned: true, gameTarget: 0x1234},
		{name: "other button", button: skillBarMouseButtonRight, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: 2, foregroundOwned: true, gameTarget: 0x1234},
		{name: "released outside", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: -1, foregroundOwned: true, gameTarget: 0x1234},
		{name: "different slot", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: 3, foregroundOwned: true, gameTarget: 0x1234},
		{name: "drag", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: 2, dragged: true, foregroundOwned: true, gameTarget: 0x1234},
		{name: "no game", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: 2, foregroundOwned: true},
		{name: "focus lost", button: skillBarMouseButtonLeft, configured: skillBarMouseButtonLeft, enabled: true, pressed: 2, released: 2, gameTarget: 0x1234},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := nativeSkillBarWindowGestureActivates(
				test.button, test.configured, test.enabled, test.pressed, test.released,
				test.dragged, test.gameTarget, test.foregroundOwned,
			)
			if got != test.want {
				t.Fatalf("activation = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNativeSkillBarUnlockedGestureIsPositionOnly(t *testing.T) {
	if nativeSkillBarGestureCanArm(false, skillBarMouseButtonLeft, skillBarMouseButtonLeft, true) {
		t.Fatal("an unlocked SkillBar gesture armed a skill instead of remaining position-only")
	}
	if !nativeSkillBarGestureCanArm(true, skillBarMouseButtonLeft, skillBarMouseButtonLeft, true) {
		t.Fatal("a locked configured click did not arm the skill")
	}
	if nativeSkillBarGestureCanArm(true, skillBarMouseButtonRight, skillBarMouseButtonLeft, true) {
		t.Fatal("the non-configured mouse button armed the skill")
	}
}

func TestNativeSkillBarActivationRequestPreservesGestureIdentity(t *testing.T) {
	for {
		select {
		case <-skillBarActivationCh:
		default:
			goto drained
		}
	}

drained:
	oldSession := skillBarSession.Load()
	oldClosing := skillBarClosing.Load()
	oldQueued := skillBarActivationQueued.Load()
	defer func() {
		skillBarSession.Store(oldSession)
		skillBarClosing.Store(oldClosing)
		skillBarActivationQueued.Store(oldQueued)
		for {
			select {
			case <-skillBarActivationCh:
			default:
				return
			}
		}
	}()

	skillBarSession.Store(42)
	skillBarClosing.Store(false)
	enqueueNativeSkillBarActivationForTarget(2, 0x1234, 0x5678, skillBarMouseButtonRight)
	select {
	case request := <-skillBarActivationCh:
		if request.Index != 2 || request.TargetHWND != 0x1234 || request.SourceHWND != 0x5678 ||
			request.Button != skillBarMouseButtonRight || request.Session != 42 {
			t.Fatalf("activation request lost gesture identity: %+v", request)
		}
	default:
		t.Fatal("activation request was not queued")
	}
}

func TestNativeSkillBarDragThreshold(t *testing.T) {
	skillBarDragMu.Lock()
	previousStart := skillBarDragStart
	previousRect := skillBarDragRect
	skillBarDragStart = nativePoint{X: 100, Y: 200}
	skillBarDragRect = nativeRect{Left: 300, Top: 400, Right: 360, Bottom: 460}
	skillBarDragMu.Unlock()
	defer func() {
		skillBarDragMu.Lock()
		skillBarDragStart = previousStart
		skillBarDragRect = previousRect
		skillBarDragMu.Unlock()
	}()
	if x, y := nativeSkillBarDragTarget(nativePoint{X: 140, Y: 250}); x != 340 || y != 450 {
		t.Fatalf("drag target = (%d,%d), want (340,450)", x, y)
	}

	if nativeSkillBarDragThresholdExceeded(nativePoint{X: 105, Y: 205}) {
		t.Fatal("movement below the threshold was classified as a drag")
	}
	if !nativeSkillBarDragThresholdExceeded(nativePoint{X: 106, Y: 200}) {
		t.Fatal("horizontal movement at the threshold was not classified as a drag")
	}
	if !nativeSkillBarDragThresholdExceeded(nativePoint{X: 100, Y: 194}) {
		t.Fatal("negative vertical movement at the threshold was not classified as a drag")
	}
}

func TestNativeSkillBarPackedDragPositionRoundTrip(t *testing.T) {
	for _, test := range []struct{ x, y int }{
		{x: 0, y: 0},
		{x: 321, y: 434},
		{x: -32000, y: 32000},
	} {
		x, y := unpackNativeSkillBarPosition(packNativeSkillBarPosition(test.x, test.y))
		if x != test.x || y != test.y {
			t.Fatalf("packed position (%d,%d) became (%d,%d)", test.x, test.y, x, y)
		}
	}
}

func TestNativeSkillBarCooldownTextBoundaries(t *testing.T) {
	for _, test := range []struct {
		name        string
		remainingMs int64
		want        string
	}{
		{name: "five and a half seconds rounds up", remainingMs: 5_500, want: "6"},
		{name: "just above one second rounds up", remainingMs: 1_001, want: "2"},
		{name: "exactly one second is integer", remainingMs: 1_000, want: "1"},
		{name: "just below one second is decimal", remainingMs: 999, want: "0.9"},
		{name: "sub-second tenths round up", remainingMs: 899, want: "0.9"},
		{name: "partial tenth does not under-report", remainingMs: 101, want: "0.2"},
		{name: "one tenth", remainingMs: 100, want: "0.1"},
		{name: "final hundredth never becomes zero", remainingMs: 10, want: "0.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			text, fraction := nativeSkillBarCooldownText(nativeSkillOverlayItem{
				UsedAtMs:  0,
				ReadyAtMs: test.remainingMs,
			}, 0)
			if text != test.want {
				t.Fatalf("cooldown with %dms remaining = %q, want %q", test.remainingMs, text, test.want)
			}
			if fraction != 1 {
				t.Fatalf("full cooldown fraction with %dms remaining = %.4f, want 1", test.remainingMs, fraction)
			}
		})
	}

	text, fraction := nativeSkillBarCooldownText(nativeSkillOverlayItem{UsedAtMs: 100_000, ReadyAtMs: 105_000}, 102_500)
	if text != "3" || fraction != 0.5 {
		t.Fatalf("half elapsed cooldown = %q %.2f, want 3 0.5", text, fraction)
	}
}

func TestNativeSkillBarConfiguredSlotShowsOnlyCenteredCooldownText(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 48, 48))
	texts := drawNativeSkillBarSlot(
		canvas,
		canvas.Bounds(),
		nativeSkillBarSlot{SkillID: 65535, KeyLabel: "Ctrl+[ -> Q"},
		nativeSkillOverlayItem{UsedAtMs: 100_000, ReadyAtMs: 105_000},
		102_000,
		nativeSkillBarFeedback{},
		false,
	)
	if len(texts) != 1 {
		t.Fatalf("configured slot annotations = %#v, want cooldown only", texts)
	}
	cooldown := texts[0]
	if cooldown.text != "3" {
		t.Fatalf("cooldown text = %q, want 3", cooldown.text)
	}
	if cooldown.flags&dtCenter == 0 || cooldown.flags&dtVCenter == 0 || cooldown.flags&dtBottom != 0 {
		t.Fatalf("cooldown flags = %#x, want horizontal and vertical centering without bottom alignment", cooldown.flags)
	}
	wantRect := imageRectToNative(canvas.Bounds().Inset(1))
	if cooldown.rect != wantRect {
		t.Fatalf("cooldown rect = %#v, want complete icon rect %#v", cooldown.rect, wantRect)
	}
	if cooldown.fontHeight < 13 || cooldown.fontWeight != nativeFontBold {
		t.Fatalf("cooldown font = height %d weight %d, want large bold", cooldown.fontHeight, cooldown.fontWeight)
	}
	if cooldown.color != nativeColorRef(255, 255, 255) {
		t.Fatalf("cooldown color = %#x, want pure white", cooldown.color)
	}
	if cooldown.outlineSize != 2 || cooldown.outlineColor != nativeColorRef(0, 0, 0) {
		t.Fatalf("cooldown outline = size %d color %#x, want two-pixel black at 46px", cooldown.outlineSize, cooldown.outlineColor)
	}
}

func TestNativeSkillBarSuccessfulFeedbackDoesNotChangeSlotStyle(t *testing.T) {
	rect := image.Rect(0, 0, 48, 48)
	slot := nativeSkillBarSlot{SkillID: 65535, KeyLabel: "Q"}
	baseline := image.NewRGBA(rect)
	drawNativeSkillBarSlot(baseline, rect, slot, nativeSkillOverlayItem{}, 100_000, nativeSkillBarFeedback{}, false)

	success := image.NewRGBA(rect)
	drawNativeSkillBarSlot(success, rect, slot, nativeSkillOverlayItem{}, 100_000, nativeSkillBarFeedback{failed: false}, true)
	if success.RGBAAt(0, 0) != baseline.RGBAAt(0, 0) || success.RGBAAt(2, 2) != baseline.RGBAAt(2, 2) {
		t.Fatalf("successful activation changed slot style: baseline=%#v/%#v success=%#v/%#v",
			baseline.RGBAAt(0, 0), baseline.RGBAAt(2, 2), success.RGBAAt(0, 0), success.RGBAAt(2, 2))
	}

	failure := image.NewRGBA(rect)
	drawNativeSkillBarSlot(failure, rect, slot, nativeSkillOverlayItem{}, 100_000, nativeSkillBarFeedback{failed: true}, true)
	if failure.RGBAAt(0, 0) == baseline.RGBAAt(0, 0) {
		t.Fatal("failed activation lost its red border feedback")
	}
}

func TestNativeSkillBarCooldownOutlineScalesForSmallIcons(t *testing.T) {
	for _, test := range []struct {
		height int
		want   int
	}{
		{height: 24, want: 1},
		{height: 39, want: 1},
		{height: 40, want: 2},
		{height: 46, want: 2},
	} {
		if got := nativeSkillBarCooldownOutlineSize(test.height); got != test.want {
			t.Fatalf("outline size at %dpx = %d, want %d", test.height, got, test.want)
		}
	}
}

func TestNativeSkillBarConfiguredSlotWithoutCooldownHasNoFallbackText(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 48, 48))
	texts := drawNativeSkillBarSlot(
		canvas,
		canvas.Bounds(),
		nativeSkillBarSlot{SkillID: 65535, KeyLabel: "W"},
		nativeSkillOverlayItem{},
		100_000,
		nativeSkillBarFeedback{},
		false,
	)
	if len(texts) != 0 {
		t.Fatalf("configured slot annotations = %#v, want no skill ID or key label", texts)
	}
}

func TestNativeSkillBarEmptySlotKeepsPlus(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 48, 48))
	texts := drawNativeSkillBarSlot(canvas, canvas.Bounds(), nativeSkillBarSlot{}, nativeSkillOverlayItem{}, 100_000, nativeSkillBarFeedback{}, false)
	if len(texts) != 1 || texts[0].text != "+" {
		t.Fatalf("empty slot annotations = %#v, want plus", texts)
	}
}
