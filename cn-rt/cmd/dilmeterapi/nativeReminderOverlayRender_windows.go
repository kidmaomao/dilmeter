//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"math"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	dtWordBreak                = 0x00000010
	dtEndEllipsis              = 0x00008000
	nativeFontBold             = 700
	nativeReminderMaxDimension = 8192
	nativeReminderMaxPixels    = 16 * 1024 * 1024
)

var (
	procGetDpiForWindow = user32.NewProc("GetDpiForWindow")
	procCreateFontW     = gdi32.NewProc("CreateFontW")

	nativeReminderClassOnce sync.Once
	nativeReminderClassErr  error
	nativeReminderWndProc   = syscall.NewCallback(nativeReminderWindowProc)
	buffReminderSurface     nativeReminderSurface
	debuffReminderSurface   nativeReminderSurface
	skillReminderSurface    nativeReminderSurface
	nativeReminderIconMu    sync.Mutex
	nativeReminderIconCache = make(map[string]*image.RGBA)
	nativeReminderFontMu    sync.Mutex
	nativeReminderFontCache = make(map[string]uintptr)
)

type nativeReminderSurface struct {
	mu            sync.Mutex
	width, height int
	dc            uintptr
	bitmap        uintptr
	oldObject     uintptr
	bits          uintptr
}

type nativeReminderText struct {
	text       string
	rect       nativeRect
	color      uint32
	flags      uintptr
	fontHeight int
	fontWeight int
}

func createNativeReminderOverlayWindow(title string, x, y, width, height int) (uintptr, error) {
	var module windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &module); err != nil {
		return 0, err
	}
	className, _ := windows.UTF16PtrFromString("DilmeterNativeReminderOverlayWindow")
	nativeReminderClassOnce.Do(func() {
		class := trayWndClassEx{
			CbSize: uint32(unsafe.Sizeof(trayWndClassEx{})), LpfnWndProc: nativeReminderWndProc,
			HInstance: uintptr(module), LpszClassName: className,
		}
		registered, _, registerErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if registered == 0 && registerErr != nil && !errors.Is(registerErr, syscall.Errno(1410)) {
			nativeReminderClassErr = fmt.Errorf("register native reminder window: %w", registerErr)
		}
	})
	if nativeReminderClassErr != nil {
		return 0, nativeReminderClassErr
	}
	windowName, _ := windows.UTF16PtrFromString(title)
	hwnd, _, createErr := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow|wsExLayered|wsExNoActivate|wsExTransparent,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)), wsPopup,
		uintptr(int32(x)), uintptr(int32(y)), uintptr(width), uintptr(height), 0, 0, uintptr(module), 0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("create native reminder window: %w", createErr)
	}
	return hwnd, nil
}

func nativeReminderWindowProc(hwnd, message, wParam, lParam uintptr) uintptr {
	if message == wmMouseActivate {
		return maNoActivate
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return result
}

func renderNativeBuffReminder(hwnd uintptr) bool {
	if hwnd == 0 || buffOverlayClosing.Load() {
		return false
	}
	buffOverlayState.RLock()
	data := append([]byte(nil), buffOverlayState.data...)
	buffOverlayState.RUnlock()
	var message nativeBuffOverlayMessage
	if json.Unmarshal(data, &message) != nil {
		return false
	}
	enabled := message.Settings.OverlayEnabled || message.Settings.Opacity == 0
	if !enabled {
		return false
	}
	now := float64(time.Now().UnixMilli()) / 1000
	items := make([]nativeBuffOverlayItem, 0, len(message.Items))
	for _, item := range message.Items {
		if !item.Active || item.ExpiresAt == nil || *item.ExpiresAt > now {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return false
	}
	scalePercent := nativeReminderScalePercent(hwnd, message.Settings.DPIPercent)
	iconSize := scaleOverlayCeil(max(16, message.Settings.IconSize), scalePercent)
	gap := scaleOverlayCeil(overlayGap, scalePercent)
	padding := scaleOverlayCeil(overlayPadding, scalePercent)
	countdownHeight := scaleOverlayCeil(overlayCountdown, scalePercent)
	width := padding*2 + len(items)*iconSize + max(0, len(items)-1)*gap
	height := padding*2 + iconSize + countdownHeight
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	texts := make([]nativeReminderText, 0, len(items))
	for index, item := range items {
		left := padding + index*(iconSize+gap)
		rect := image.Rect(left, padding, left+iconSize, padding+iconSize)
		border := color.RGBA{R: 126, G: 225, B: 246, A: 245}
		glow := color.RGBA{R: 77, G: 219, B: 255, A: 120}
		glowRadius := max(2, padding-1)
		if !item.Active {
			border = color.RGBA{R: 115, G: 124, B: 128, A: 210}
			glow = color.RGBA{R: 78, G: 91, B: 94, A: 75}
		} else if item.ExpiresAt != nil && item.FlashEnabled && *item.ExpiresAt-now <= float64(item.FlashThresholdSeconds) {
			pulse := nativeReminderPulse(time.Now().UnixMilli(), 580)
			border = color.RGBA{R: 255, G: byte(76 + 86*pulse), B: 58, A: 255}
			glow = color.RGBA{R: 255, G: 62, B: 38, A: byte(115 + 110*pulse)}
			glowRadius = min(padding-1, max(3, int(4+3*pulse)))
		}
		drawNativeReminderGlow(canvas, rect, glow, glowRadius)
		drawNativeReminderIcon(canvas, rect, "condition-icons", uint64(item.CCID), border)
		if !item.Active {
			fillNativeSkillBarRect(canvas, rect.Inset(2), color.RGBA{R: 36, G: 43, B: 45, A: 142})
		}
		if item.Active && item.ExpiresAt != nil {
			remaining := max(0.0, *item.ExpiresAt-now)
			total := max(0.001, *item.ExpiresAt-float64(item.AppliedAt))
			shade := int(float64(iconSize) * min(1.0, remaining/total))
			fillNativeSkillBarRect(canvas, image.Rect(rect.Min.X+1, rect.Min.Y+1, rect.Max.X-1, min(rect.Max.Y-1, rect.Min.Y+shade)), color.RGBA{A: 104})
			labelRect := image.Rect(rect.Min.X, rect.Max.Y, rect.Max.X, height-padding)
			fillNativeSkillBarRect(canvas, labelRect, color.RGBA{R: 9, G: 15, B: 17, A: 220})
			texts = append(texts, nativeReminderText{text: strconv.Itoa(int(math.Ceil(remaining))), rect: imageRectToNative(labelRect), color: nativeColorRef(255, 255, 255), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(10, countdownHeight-3), fontWeight: nativeFontBold})
		}
		if !nativeReminderOverlaysAreLocked() {
			drawNativeUnlockedFrame(canvas, rect)
		}
	}
	x, y := nativeWindowPosition(hwnd)
	updateNativeReminderLayer(hwnd, x, y, nativeReminderOpacity(message.Settings.Opacity), canvas, texts, &buffReminderSurface)
	buffOverlayCount.Store(int32(len(items)))
	buffOverlaySize.Store(int32(message.Settings.IconSize))
	buffOverlayScale.Store(int32(scalePercent))
	return true
}

func renderNativeDebuffReminder(hwnd uintptr) bool {
	if hwnd == 0 || debuffOverlayClosing.Load() {
		return false
	}
	debuffOverlayState.RLock()
	data := append([]byte(nil), debuffOverlayState.data...)
	debuffOverlayState.RUnlock()
	var message nativeDebuffOverlayMessage
	if json.Unmarshal(data, &message) != nil || !message.Settings.OverlayEnabled {
		return false
	}
	now := float64(time.Now().UnixMilli()) / 1000
	items := make([]nativeDebuffOverlayItem, 0, len(message.Items))
	for _, item := range message.Items {
		if item.State == "missing" || (item.ExpiresAt != nil && *item.ExpiresAt > now) {
			items = append(items, item)
		}
	}
	showDebuffs := message.Boss != nil && len(items) > 0
	showHealth := message.TargetHealth != nil && message.TargetHealth.MaximumHealth > 0 &&
		!math.IsNaN(message.TargetHealth.MaximumHealth) && !math.IsInf(message.TargetHealth.MaximumHealth, 0) &&
		!math.IsNaN(message.TargetHealth.CurrentHealth) && !math.IsInf(message.TargetHealth.CurrentHealth, 0)
	if !showDebuffs && !showHealth {
		return false
	}
	if !showDebuffs {
		items = nil
	}
	scalePercent := nativeReminderScalePercent(hwnd, message.Settings.DPIPercent)
	iconSize := scaleOverlayCeil(max(16, message.Settings.IconSize), scalePercent)
	gap := scaleOverlayCeil(overlayGap, scalePercent)
	padding := scaleOverlayCeil(overlayPadding, scalePercent)
	headerHeight := scaleOverlayCeil(debuffBossHeaderHeight, scalePercent)
	headerGap := scaleOverlayCeil(debuffBossHeaderGap, scalePercent)
	healthHeight := scaleOverlayCeil(debuffHealthBarHeight, scalePercent)
	healthGap := scaleOverlayCeil(debuffHealthBarGap, scalePercent)
	countdownHeight := scaleOverlayCeil(overlayCountdown, scalePercent)
	iconsWidth := len(items)*iconSize + max(0, len(items)-1)*gap
	contentWidth := max(scaleOverlayCeil(debuffBossHeaderWidth, scalePercent), iconsWidth)
	width := padding*2 + contentWidth
	height := padding * 2
	if showHealth {
		height += healthHeight
	}
	if showDebuffs {
		if showHealth {
			height += healthGap
		}
		height += headerHeight + headerGap + iconSize + countdownHeight
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	texts := make([]nativeReminderText, 0, len(items)+3)
	top := padding
	if showHealth {
		healthRect := image.Rect(padding, top, padding+contentWidth, top+healthHeight)
		fillNativeSkillBarRect(canvas, healthRect, color.RGBA{R: 35, G: 35, B: 37, A: 242})
		ratio := math.Max(0, math.Min(1, message.TargetHealth.CurrentHealth/message.TargetHealth.MaximumHealth))
		fillRight := healthRect.Min.X + int(math.Round(float64(healthRect.Dx())*ratio))
		if fillRight > healthRect.Min.X {
			fillNativeSkillBarRect(canvas, image.Rect(healthRect.Min.X, healthRect.Min.Y, fillRight, healthRect.Max.Y), color.RGBA{R: 190, G: 19, B: 25, A: 247})
		}
		drawNativeSkillBarBorder(canvas, healthRect, color.RGBA{R: 15, G: 15, B: 16, A: 255}, max(1, scaleOverlayCeil(1, scalePercent)))
		textInset := max(7, scaleOverlayCeil(10, scalePercent))
		percentWidth := max(scaleOverlayCeil(92, scalePercent), contentWidth/5)
		texts = append(texts,
			nativeReminderText{text: message.TargetHealth.Name, rect: imageRectToNative(image.Rect(healthRect.Min.X+textInset, healthRect.Min.Y, healthRect.Max.X-percentWidth, healthRect.Max.Y)), color: nativeColorRef(255, 255, 255), flags: dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(13, healthHeight/2), fontWeight: nativeFontBold},
			nativeReminderText{text: fmt.Sprintf("%.2f %%", ratio*100), rect: imageRectToNative(image.Rect(healthRect.Max.X-percentWidth, healthRect.Min.Y, healthRect.Max.X-textInset, healthRect.Max.Y)), color: nativeColorRef(255, 255, 255), flags: dtRight | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(13, healthHeight/2), fontWeight: nativeFontBold},
		)
		if !nativeReminderOverlaysAreLocked() {
			drawNativeUnlockedFrame(canvas, healthRect)
		}
		top += healthHeight
		if showDebuffs {
			top += healthGap
		}
	}
	if showDebuffs {
		headerRect := image.Rect(padding, top, padding+contentWidth, top+headerHeight)
		fillNativeSkillBarRect(canvas, headerRect, color.RGBA{R: 43, G: 22, B: 20, A: 225})
		drawNativeSkillBarBorder(canvas, headerRect, color.RGBA{R: 255, G: 174, B: 91, A: 245}, 1)
		texts = append(texts, nativeReminderText{text: message.Boss.Name, rect: imageRectToNative(headerRect), color: nativeColorRef(255, 246, 210), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(11, headerHeight-5), fontWeight: nativeFontBold})
		top += headerHeight + headerGap
	}
	for index, item := range items {
		left := padding + index*(iconSize+gap)
		rect := image.Rect(left, top, left+iconSize, top+iconSize)
		border := color.RGBA{R: 255, G: 83, B: 78, A: 255}
		pulse := nativeReminderPulse(time.Now().UnixMilli()+int64(index*73), 580)
		glow := color.RGBA{R: 255, G: 44, B: 38, A: byte(120 + 110*pulse)}
		if item.State == "expiring" {
			border = color.RGBA{R: 255, G: byte(165 + 70*pulse), B: 68, A: 255}
			glow = color.RGBA{R: 255, G: 174, B: 45, A: byte(105 + 120*pulse)}
		}
		drawNativeReminderGlow(canvas, rect, glow, min(padding-1, max(3, int(4+3*pulse))))
		drawNativeReminderIcon(canvas, rect, "condition-icons", uint64(item.CCID), border)
		label := "缺失"
		if item.State == "expiring" && item.ExpiresAt != nil {
			label = strconv.Itoa(int(math.Ceil(max(0.0, *item.ExpiresAt-now))))
		}
		labelRect := image.Rect(rect.Min.X, rect.Max.Y, rect.Max.X, height-padding)
		fillNativeSkillBarRect(canvas, labelRect, color.RGBA{R: 23, G: 10, B: 9, A: 225})
		texts = append(texts, nativeReminderText{text: label, rect: imageRectToNative(labelRect), color: nativeColorRef(255, 240, 220), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(10, countdownHeight-3), fontWeight: nativeFontBold})
		if !nativeReminderOverlaysAreLocked() {
			drawNativeUnlockedFrame(canvas, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, labelRect.Max.Y))
		}
	}
	x, y := nativeWindowPosition(hwnd)
	updateNativeReminderLayer(hwnd, x, y, nativeReminderOpacity(message.Settings.Opacity), canvas, texts, &debuffReminderSurface)
	debuffOverlayCount.Store(int32(len(items)))
	debuffOverlaySize.Store(int32(message.Settings.IconSize))
	debuffOverlayScale.Store(int32(scalePercent))
	debuffOverlayHasBoss.Store(showDebuffs)
	debuffOverlayHasHealth.Store(showHealth)
	return true
}

func renderNativeSkillReminder(hwnd uintptr) bool {
	if hwnd == 0 || skillOverlayClosing.Load() {
		setNativeReminderHitRegions(nil)
		return false
	}
	skillOverlayState.RLock()
	data := append([]byte(nil), skillOverlayState.data...)
	skillOverlayState.RUnlock()
	var message nativeSkillOverlayMessage
	if json.Unmarshal(data, &message) != nil {
		setNativeReminderHitRegions(nil)
		return false
	}
	applyNativeReminderDragOverride(&message)
	nowMs := time.Now().UnixMilli()
	if !nativeSkillOverlayMessageVisible(message, nowMs) {
		setNativeReminderHitRegions(nil)
		return false
	}
	scalePercent := nativeReminderScalePercent(hwnd, message.Settings.DPIPercent)
	dpiScale := float64(scalePercent) / 100
	iconSize := max(24, int(math.Ceil(float64(max(24, message.Settings.IconSize))*dpiScale)))
	visibleSkills := make([]nativeSkillOverlayItem, 0, len(message.Items))
	for _, item := range message.Items {
		if nativeSkillReminderItemVisible(item, nowMs) {
			visibleSkills = append(visibleSkills, item)
		}
	}
	mechanics := make([]nativeBossMechanicOverlayItem, 0, len(message.Mechanics))
	for _, item := range message.Mechanics {
		if item.EndsAtMs > nowMs {
			mechanics = append(mechanics, item)
		}
	}
	stacks := make([]nativeBuffStackOverlayItem, 0, len(message.StackAlerts))
	for _, item := range message.StackAlerts {
		if item.Persistent || item.EndsAtMs > nowMs {
			stacks = append(stacks, item)
		}
	}
	aim := message.AimReminder
	if aim != nil && !aim.Active && !aim.AlwaysVisible {
		aim = nil
	}
	bounds := image.Rectangle{}
	addBounds := func(rect image.Rectangle) {
		if bounds.Empty() {
			bounds = rect
		} else {
			bounds = bounds.Union(rect)
		}
	}
	for _, item := range visibleSkills {
		addBounds(image.Rect(item.X-70, item.Y-70, item.X+iconSize+70, item.Y+iconSize+70))
	}
	if aim != nil {
		factor := dpiScale * float64(max(50, min(200, aim.ScalePercent))) / 100
		addBounds(image.Rect(aim.X-48, aim.Y-48, aim.X+int(math.Ceil(292*factor))+48, aim.Y+int(math.Ceil(64*factor))+48))
	}
	for _, item := range mechanics {
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		addBounds(image.Rect(item.X-48, item.Y-48, item.X+int(math.Ceil(118*factor))+48, item.Y+int(math.Ceil(118*factor))+48))
	}
	for _, item := range stacks {
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		addBounds(image.Rect(item.X-32, item.Y-32, item.X+int(math.Ceil(220*factor))+32, item.Y+int(math.Ceil(104*factor))+32))
	}
	if bounds.Empty() {
		setNativeReminderHitRegions(nil)
		return false
	}
	bounds = bounds.Inset(-4)
	width, height := max(1, bounds.Dx()), max(1, bounds.Dy())
	if width > nativeReminderMaxDimension || height > nativeReminderMaxDimension || int64(width)*int64(height) > nativeReminderMaxPixels {
		setNativeReminderHitRegions(nil)
		return false
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	texts := make([]nativeReminderText, 0, len(visibleSkills)*2+8)
	hitRegions := make([]nativeReminderHitRegion, 0, len(visibleSkills)+len(mechanics)+len(stacks)+1)
	offset := image.Pt(-bounds.Min.X, -bounds.Min.Y)
	for _, item := range visibleSkills {
		texts = append(texts, drawNativeSkillReminderItem(canvas, item, offset, iconSize, nowMs)...)
		hitRegions = append(hitRegions, nativeReminderRegionForSkill(item, iconSize))
	}
	if aim != nil {
		texts = append(texts, drawNativeAimReminder(canvas, *aim, offset, dpiScale, nowMs)...)
		factor := dpiScale * float64(max(50, min(200, aim.ScalePercent))) / 100
		hitRegions = append(hitRegions, nativeReminderHitRegion{Kind: "aim", ID: "aim", X: aim.X, Y: aim.Y,
			Rect: nativeRect{Left: int32(aim.X), Top: int32(aim.Y), Right: int32(aim.X + max(146, int(math.Ceil(292*factor)))), Bottom: int32(aim.Y + max(32, int(math.Ceil(64*factor))))}})
	}
	for _, item := range mechanics {
		texts = append(texts, drawNativeMechanicReminder(canvas, item, offset, dpiScale, nowMs)...)
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		size := max(59, int(math.Ceil(118*factor)))
		hitRegions = append(hitRegions, nativeReminderHitRegion{Kind: "mechanic", ID: item.Key, X: item.X, Y: item.Y,
			Rect: nativeRect{Left: int32(item.X), Top: int32(item.Y), Right: int32(item.X + size), Bottom: int32(item.Y + size)}})
	}
	for _, item := range stacks {
		texts = append(texts, drawNativeStackReminder(canvas, item, offset, dpiScale, nowMs)...)
		factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
		width, height := max(110, int(math.Ceil(220*factor))), max(48, int(math.Ceil(96*factor)))
		kind, id := nativeStackReminderIdentity(item)
		hitRegions = append(hitRegions, nativeReminderHitRegion{Kind: kind, ID: id, X: item.X, Y: item.Y,
			Rect: nativeRect{Left: int32(item.X), Top: int32(item.Y), Right: int32(item.X + width), Bottom: int32(item.Y + height)}})
	}
	setNativeReminderHitRegions(hitRegions)
	updateNativeReminderLayer(hwnd, bounds.Min.X, bounds.Min.Y, nativeReminderOpacity(message.Settings.Opacity), canvas, texts, &skillReminderSurface)
	skillOverlayX.Store(int32(bounds.Min.X))
	skillOverlayY.Store(int32(bounds.Min.Y))
	skillOverlayWidth.Store(int32(width))
	skillOverlayHeight.Store(int32(height))
	return true
}

func nativeSkillReminderItemVisible(item nativeSkillOverlayItem, nowMs int64) bool {
	if item.BarOnly {
		return false
	}
	if item.ProgressObserved != nil {
		progress, threshold := 0.0, 95.0
		if item.ProgressPercent != nil {
			progress = *item.ProgressPercent
		}
		if item.ProgressThresholdPercent != nil {
			threshold = *item.ProgressThresholdPercent
		}
		return item.AlwaysVisible || (*item.ProgressObserved && progress >= threshold && progress < 100)
	}
	return item.AlwaysVisible || (item.ReadyAtMs > 0 && nowMs >= item.ReadyAtMs && nowMs-item.ReadyAtMs < 2600)
}

func drawNativeSkillReminderItem(canvas *image.RGBA, item nativeSkillOverlayItem, offset image.Point, iconSize int, nowMs int64) []nativeReminderText {
	left, top := item.X+offset.X, item.Y+offset.Y
	rect := image.Rect(left, top, left+iconSize, top+iconSize)
	border := color.RGBA{R: 113, G: 221, B: 244, A: 245}
	readyElapsed := int64(-1)
	if item.ReadyAtMs > 0 && nowMs >= item.ReadyAtMs {
		readyElapsed = nowMs - item.ReadyAtMs
		border = color.RGBA{R: 255, G: 231, B: 103, A: 255}
	}
	if item.CooldownPhase == "accumulating" {
		border = color.RGBA{R: 255, G: 178, B: 67, A: 255}
	}
	glow := color.RGBA{R: 72, G: 217, B: 255, A: 145}
	if item.CooldownPhase == "accumulating" {
		glow = color.RGBA{R: 255, G: 142, B: 42, A: 175}
	}
	renderRect := rect
	if readyElapsed >= 0 && readyElapsed < 2600 {
		phase := float64(readyElapsed) / 2600
		scale := nativeReadyBurstScale(phase)
		grow := int(float64(iconSize) * (scale - 1) / 2)
		renderRect = rect.Inset(-grow)
		pulse := nativeReminderPulse(nowMs, 360)
		glow = color.RGBA{R: 255, G: byte(196 + 55*pulse), B: 78, A: byte(170 + 80*pulse)}
		drawNativeReadyParticles(canvas, rect, phase)
	}
	drawNativeReminderGlow(canvas, renderRect, glow, max(5, iconSize/5))
	drawNativeReminderIcon(canvas, renderRect, "skill-icons", uint64(item.SkillID), border)
	texts := make([]nativeReminderText, 0, 2)
	label := ""
	if item.ProgressObserved != nil {
		progress := 0.0
		if item.ProgressPercent != nil {
			progress = min(100.0, max(0.0, *item.ProgressPercent))
		}
		mask := int(float64(renderRect.Dy()-2) * (100 - progress) / 100)
		fillNativeSkillBarRect(canvas, image.Rect(renderRect.Min.X+1, renderRect.Min.Y+1, renderRect.Max.X-1, renderRect.Min.Y+1+mask), color.RGBA{R: 100, G: 102, B: 108, A: 220})
		if item.ProgressObserved != nil && *item.ProgressObserved {
			label = fmt.Sprintf("%.0f%%", progress)
		} else {
			label = "--"
		}
	} else if item.UsedAtMs > 0 && item.ReadyAtMs > nowMs {
		remaining := float64(item.ReadyAtMs-nowMs) / 1000
		total := max(int64(1), item.ReadyAtMs-item.UsedAtMs)
		fraction := min(1.0, max(0.0, float64(item.ReadyAtMs-nowMs)/float64(total)))
		drawNativeCooldownSweep(canvas, renderRect.Inset(1), fraction, color.RGBA{R: 4, G: 7, B: 9, A: 178})
		if item.CooldownPhase == "accumulating" && item.CumulativeCooldownSeconds != nil {
			label = fmt.Sprintf("累计 %.1f/%.1fs", item.AccumulatedCooldownSeconds, *item.CumulativeCooldownSeconds)
		} else if remaining >= 10 {
			label = fmt.Sprintf("%ds", int(math.Ceil(remaining)))
		} else {
			label = fmt.Sprintf("%.1fs", math.Ceil(remaining*10)/10)
		}
	}
	if item.PetSkill {
		texts = append(texts, nativeReminderText{text: "宠", rect: imageRectToNative(image.Rect(renderRect.Min.X, renderRect.Min.Y, renderRect.Min.X+max(14, iconSize/3), renderRect.Min.Y+max(14, iconSize/3))), color: nativeColorRef(255, 242, 151), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(9, iconSize/5), fontWeight: nativeFontBold})
	}
	if label != "" {
		labelRect := image.Rect(rect.Min.X-12, rect.Max.Y+2, rect.Max.X+12, rect.Max.Y+22)
		fillNativeSkillBarRect(canvas, labelRect, color.RGBA{R: 7, G: 12, B: 14, A: 220})
		texts = append(texts, nativeReminderText{text: label, rect: imageRectToNative(labelRect), color: nativeColorRef(255, 255, 255), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(10, iconSize/5), fontWeight: nativeFontBold})
	}
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, image.Rect(rect.Min.X-2, rect.Min.Y-2, rect.Max.X+2, rect.Max.Y+24))
	}
	return texts
}

func drawNativeAimReminder(canvas *image.RGBA, item nativeAimReminderOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	width, height := max(146, int(math.Ceil(292*factor))), max(32, int(math.Ceil(64*factor)))
	left, top := item.X+offset.X, item.Y+offset.Y
	rect := image.Rect(left, top, left+width, top+height)
	progress := 0.0
	if item.Active && item.ReadyAtMs > item.StartedAtMs {
		progress = nativeMagnumAimDisplayProgress(
			item.StartedAtMs,
			item.ReadyAtMs,
			nowMs,
			item.CalibrationPercent,
		)
	}
	pulse := nativeReminderPulse(nowMs, 520)
	glow := color.RGBA{R: 54, G: 213, B: 248, A: 145}
	border := color.RGBA{R: 96, G: 211, B: 235, A: 245}
	if progress >= .85 {
		glow = color.RGBA{R: 255, G: 214, B: 74, A: byte(155 + 95*pulse)}
		border = color.RGBA{R: 255, G: 230, B: 116, A: 255}
	}
	if progress >= 1 {
		glow = color.RGBA{R: 255, G: 82, B: 44, A: byte(175 + 75*pulse)}
		border = color.RGBA{R: 255, G: 245, B: 205, A: 255}
	}
	drawNativeReminderGlow(canvas, rect, glow, max(6, int(12*factor)))
	if progress >= .78 {
		drawNativeAimRays(canvas, rect, progress, nowMs)
	}
	fillNativeSkillBarRect(canvas, rect, color.RGBA{R: 8, G: 17, B: 20, A: 225})
	drawNativeSkillBarBorder(canvas, rect, border, max(1, int(2*factor)))
	icon := max(24, int(math.Ceil(44*factor)))
	iconRect := image.Rect(left+3, top+(height-icon)/2, left+3+icon, top+(height-icon)/2+icon)
	drawNativeReminderIcon(canvas, iconRect, "skill-icons", 21002, color.RGBA{R: 114, G: 225, B: 245, A: 255})
	bodyLeft := iconRect.Max.X + max(4, int(7*factor))
	trackTop := top + height/2 - 2
	trackRight := rect.Max.X - 5
	track := image.Rect(bodyLeft, trackTop, trackRight, trackTop+max(8, int(14*factor)))
	fillNativeSkillBarRect(canvas, track, color.RGBA{R: 23, G: 34, B: 37, A: 235})
	drawNativeSkillBarBorder(canvas, track, color.RGBA{R: 105, G: 142, B: 150, A: 235}, 1)
	fillWidth := int(float64(max(0, track.Dx()-2)) * progress)
	fillNativeSkillBarRect(canvas, image.Rect(track.Min.X+1, track.Min.Y+1, track.Min.X+1+fillWidth, track.Max.Y-1), color.RGBA{R: 82, G: 218, B: 241, A: 245})
	markerX := track.Min.X + int(float64(track.Dx())*0.85)
	fillNativeSkillBarRect(canvas, image.Rect(markerX, track.Min.Y-2, markerX+2, track.Max.Y+2), color.RGBA{R: 255, G: 221, B: 91, A: 255})
	status := "等待穿心锁定目标"
	if item.Active {
		names := strings.Join(item.BuffNames, " + ")
		if names == "" {
			names = "无临时瞄速 Buff"
		}
		status = fmt.Sprintf("%s · ×%.1f", names, max(1.0, item.SpeedMultiplier))
	}
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	return []nativeReminderText{
		{text: "穿心", rect: imageRectToNative(image.Rect(bodyLeft, top, trackRight, track.Min.Y)), color: nativeColorRef(240, 252, 255), flags: dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(11, int(12*factor)), fontWeight: nativeFontBold},
		{text: fmt.Sprintf("%d%%", int(math.Floor(progress*100))), rect: imageRectToNative(image.Rect(trackRight-max(42, int(48*factor)), top, trackRight, track.Min.Y)), color: nativeColorRef(255, 218, 91), flags: dtRight | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(10, int(11*factor)), fontWeight: nativeFontBold},
		{text: status, rect: imageRectToNative(image.Rect(bodyLeft, track.Max.Y, trackRight, rect.Max.Y)), color: nativeColorRef(194, 218, 222), flags: dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(8, int(9*factor)), fontWeight: nativeFontBold},
	}
}

func drawNativeMechanicReminder(canvas *image.RGBA, item nativeBossMechanicOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	size := max(59, int(math.Ceil(118*factor)))
	left, top := item.X+offset.X, item.Y+offset.Y
	rect := image.Rect(left, top, left+size, top+size)
	remaining := max(0.0, float64(item.EndsAtMs-nowMs)/1000)
	danger := (item.Key == "miel-orb" && remaining <= 6.5) || (item.Key == "miel-laser" && remaining <= 1)
	border := color.RGBA{R: 255, G: 207, B: 105, A: 255}
	pulse := nativeReminderPulse(nowMs, 620)
	glow := color.RGBA{R: 255, G: 177, B: 62, A: byte(115 + 80*pulse)}
	if danger {
		border = color.RGBA{R: 255, G: 65, B: 46, A: 255}
		pulse = nativeReminderPulse(nowMs, 340)
		glow = color.RGBA{R: 255, G: 42, B: 28, A: byte(160 + 95*pulse)}
	}
	drawNativeReminderGlow(canvas, rect, glow, max(6, int(12*factor)))
	fillNativeSkillBarRect(canvas, rect, color.RGBA{R: 24, G: 13, B: 8, A: 190})
	drawNativeSkillBarBorder(canvas, rect, border, 2)
	if danger && pulse > .55 {
		drawNativeSkillBarBorder(canvas, rect.Inset(-3), color.RGBA{R: 255, G: 238, B: 215, A: byte(90 + 100*pulse)}, 1)
	}
	total := max(int64(1), item.EndsAtMs-item.StartedAtMs)
	barWidth := int(float64(size-12) * float64(item.EndsAtMs-nowMs) / float64(total))
	fillNativeSkillBarRect(canvas, image.Rect(left+6, rect.Max.Y-8, left+6+max(0, barWidth), rect.Max.Y-5), border)
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	return []nativeReminderText{{text: item.Name, rect: imageRectToNative(image.Rect(left+4, top+4, rect.Max.X-4, top+max(22, int(30*factor)))), color: nativeColorRef(255, 248, 225), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(9, int(11*factor)), fontWeight: nativeFontBold}, {text: fmt.Sprintf("%.1f", remaining), rect: imageRectToNative(image.Rect(left+4, top+max(20, int(26*factor)), rect.Max.X-4, rect.Max.Y-10)), color: nativeColorRef(255, 255, 255), flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(20, int(46*factor)), fontWeight: 900}}
}

func drawNativeStackReminder(canvas *image.RGBA, item nativeBuffStackOverlayItem, offset image.Point, dpiScale float64, nowMs int64) []nativeReminderText {
	factor := dpiScale * float64(max(50, min(200, item.ScalePercent))) / 100
	width := max(110, int(math.Ceil(220*factor)))
	height := max(48, int(math.Ceil(96*factor)))
	left, top := item.X+offset.X, item.Y+offset.Y
	rect := image.Rect(left, top, left+width, top+height)
	age := max(int64(0), nowMs-item.StartedAtMs)
	remaining := max(int64(0), item.EndsAtMs-nowMs)
	pulse := nativeReminderPulse(nowMs, 720)
	glowAlpha := byte(125 + 75*pulse)
	if age < 420 {
		glowAlpha = byte(180 + 75*nativeReminderPulse(nowMs, 240))
	}
	if !item.Persistent && remaining < 600 {
		glowAlpha = byte(float64(glowAlpha) * float64(remaining) / 600)
	}
	drawNativeReminderGlow(canvas, rect, color.RGBA{R: 70, G: 218, B: 255, A: glowAlpha}, max(5, int(9*factor)))
	fillNativeSkillBarRect(canvas, rect, color.RGBA{R: 8, G: 18, B: 26, A: 220})
	drawNativeSkillBarBorder(canvas, rect, color.RGBA{R: 96, G: 211, B: 246, A: 245}, 1)
	if !nativeReminderOverlaysAreLocked() {
		drawNativeUnlockedFrame(canvas, rect)
	}
	if item.QuantityText != "" {
		unitLeft := rect.Max.X - max(36, int(62*factor))
		return []nativeReminderText{
			{text: item.Name, rect: imageRectToNative(image.Rect(left+8, top+5, rect.Max.X-8, top+max(24, int(30*factor)))), color: nativeColorRef(207, 243, 255), flags: dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(10, int(14*factor)), fontWeight: nativeFontBold},
			{text: item.QuantityText, rect: imageRectToNative(image.Rect(left+8, top+max(24, int(28*factor)), unitLeft-4, rect.Max.Y-5)), color: nativeColorRef(255, 255, 255), flags: dtRight | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(20, int(42*factor)), fontWeight: 900},
			{text: item.QuantityUnit, rect: imageRectToNative(image.Rect(unitLeft, top+int(56*factor), rect.Max.X-8, rect.Max.Y-5)), color: nativeColorRef(207, 243, 255), flags: dtRight | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(10, int(18*factor)), fontWeight: nativeFontBold},
		}
	}
	return []nativeReminderText{{text: item.Name, rect: imageRectToNative(image.Rect(left+8, top+5, rect.Max.X-8, top+max(24, int(30*factor)))), color: nativeColorRef(207, 243, 255), flags: dtVCenter | dtSingleLine | dtNoPrefix | dtEndEllipsis, fontHeight: max(10, int(14*factor)), fontWeight: nativeFontBold}, {text: nativeStackReminderValue(item), rect: imageRectToNative(image.Rect(left+8, top+max(24, int(28*factor)), rect.Max.X-8, rect.Max.Y-5)), color: nativeColorRef(255, 255, 255), flags: dtRight | dtVCenter | dtSingleLine | dtNoPrefix, fontHeight: max(20, int(42*factor)), fontWeight: 900}}
}

func nativeReminderPulse(nowMs, periodMs int64) float64 {
	if periodMs <= 0 {
		return 1
	}
	return .5 + .5*math.Sin(float64(nowMs%periodMs)*2*math.Pi/float64(periodMs))
}

func nativeReadyBurstScale(phase float64) float64 {
	phase = min(1.0, max(0.0, phase))
	switch {
	case phase < .18:
		return .55 + phase/.18*.67
	case phase < .52:
		return 1.22 - (phase-.18)/.34*.20
	case phase < .78:
		return 1.02 + (phase-.52)/.26*.34
	default:
		return 1.36 - (phase-.78)/.22*.36
	}
}

func drawNativeReminderGlow(canvas *image.RGBA, rect image.Rectangle, glow color.RGBA, radius int) {
	radius = max(1, radius)
	for distance := radius; distance >= 1; distance-- {
		alpha := int(glow.A) * (radius - distance + 1) / (radius * 2)
		stroke := glow
		stroke.A = byte(max(1, alpha))
		drawNativeSkillBarBorder(canvas, rect.Inset(-distance), stroke, 1)
	}
}

func drawNativeUnlockedFrame(canvas *image.RGBA, rect image.Rectangle) {
	stroke := color.RGBA{R: 92, G: 237, B: 255, A: 250}
	drawNativeSkillBarBorder(canvas, rect.Inset(-2), stroke, 1)
	corner := max(4, min(9, min(rect.Dx(), rect.Dy())/5))
	fillNativeSkillBarRect(canvas, image.Rect(rect.Min.X-3, rect.Min.Y-3, rect.Min.X+corner, rect.Min.Y-1), stroke)
	fillNativeSkillBarRect(canvas, image.Rect(rect.Min.X-3, rect.Min.Y-3, rect.Min.X-1, rect.Min.Y+corner), stroke)
}

func drawNativeCooldownSweep(canvas *image.RGBA, rect image.Rectangle, fraction float64, shade color.RGBA) {
	rect = rect.Intersect(canvas.Bounds())
	if rect.Empty() || fraction <= 0 {
		return
	}
	fraction = min(1.0, fraction)
	cx, cy := float64(rect.Min.X+rect.Max.X-1)/2, float64(rect.Min.Y+rect.Max.Y-1)/2
	limit := fraction * 2 * math.Pi
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			angle := math.Atan2(float64(y)-cy, float64(x)-cx) + math.Pi/2
			if angle < 0 {
				angle += 2 * math.Pi
			}
			if angle <= limit {
				blendNativeReminderPixel(canvas, x, y, shade)
			}
		}
	}
}

func blendNativeReminderPixel(canvas *image.RGBA, x, y int, overlay color.RGBA) {
	if !image.Pt(x, y).In(canvas.Bounds()) || overlay.A == 0 {
		return
	}
	offset := canvas.PixOffset(x, y)
	alpha := int(overlay.A)
	inverse := 255 - alpha
	canvas.Pix[offset] = byte((int(overlay.R)*alpha + int(canvas.Pix[offset])*inverse) / 255)
	canvas.Pix[offset+1] = byte((int(overlay.G)*alpha + int(canvas.Pix[offset+1])*inverse) / 255)
	canvas.Pix[offset+2] = byte((int(overlay.B)*alpha + int(canvas.Pix[offset+2])*inverse) / 255)
	canvas.Pix[offset+3] = byte(max(int(canvas.Pix[offset+3]), alpha))
}

func drawNativeReadyParticles(canvas *image.RGBA, rect image.Rectangle, phase float64) {
	centerX, centerY := float64(rect.Min.X+rect.Max.X)/2, float64(rect.Min.Y+rect.Max.Y)/2
	distance := float64(rect.Dx()) * (.35 + phase*1.05)
	alpha := byte(255 * max(0.0, 1-phase))
	for index := 0; index < 12; index++ {
		angle := float64(index)*2*math.Pi/12 + phase*.7
		x := int(centerX + math.Cos(angle)*distance)
		y := int(centerY + math.Sin(angle)*distance)
		size := 1 + index%3
		particle := color.RGBA{R: 255, G: byte(190 + index%4*16), B: 72, A: alpha}
		fillNativeSkillBarRect(canvas, image.Rect(x-size, y-size, x+size+1, y+size+1), particle)
	}
}

func drawNativeAimRays(canvas *image.RGBA, rect image.Rectangle, progress float64, nowMs int64) {
	pulse := nativeReminderPulse(nowMs, 420)
	stroke := color.RGBA{R: 255, G: 230, B: 118, A: byte(100 + 120*pulse)}
	cx, cy := (rect.Min.X+rect.Max.X)/2, (rect.Min.Y+rect.Max.Y)/2
	length := max(8, rect.Dy()/3)
	if progress >= 1 {
		stroke = color.RGBA{R: 255, G: 102, B: 58, A: byte(130 + 120*pulse)}
		length += rect.Dy() / 6
	}
	fillNativeSkillBarRect(canvas, image.Rect(cx-1, rect.Min.Y-length, cx+2, rect.Min.Y-3), stroke)
	fillNativeSkillBarRect(canvas, image.Rect(cx-1, rect.Max.Y+3, cx+2, rect.Max.Y+length), stroke)
	fillNativeSkillBarRect(canvas, image.Rect(rect.Min.X-length, cy-1, rect.Min.X-3, cy+2), stroke)
	fillNativeSkillBarRect(canvas, image.Rect(rect.Max.X+3, cy-1, rect.Max.X+length, cy+2), stroke)
}

func drawNativeReminderIcon(canvas *image.RGBA, rect image.Rectangle, directory string, id uint64, border color.RGBA) {
	fillNativeSkillBarRect(canvas, rect, border)
	inner := rect.Inset(1)
	fillNativeSkillBarRect(canvas, inner, color.RGBA{R: 18, G: 26, B: 29, A: 238})
	if icon := nativeReminderIcon(directory, id, inner.Dx(), inner.Dy()); icon != nil {
		draw.Draw(canvas, inner, icon, icon.Bounds().Min, draw.Over)
	}
}

func nativeReminderIcon(directory string, id uint64, width, height int) *image.RGBA {
	if id == 0 || width <= 0 || height <= 0 {
		return nil
	}
	key := fmt.Sprintf("%s:%d:%dx%d", directory, id, width, height)
	nativeReminderIconMu.Lock()
	cached := nativeReminderIconCache[key]
	nativeReminderIconMu.Unlock()
	if cached != nil {
		return cached
	}
	data, err := staticFiles.ReadFile(fmt.Sprintf("%s/%s/%d.png", embeddedStaticDir, directory, id))
	if err != nil {
		return nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := decoded.Bounds()
	for y := 0; y < height; y++ {
		sy := bounds.Min.Y + y*bounds.Dy()/height
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/width
			result.Set(x, y, decoded.At(sx, sy))
		}
	}
	nativeReminderIconMu.Lock()
	if len(nativeReminderIconCache) >= 512 {
		nativeReminderIconCache = make(map[string]*image.RGBA)
	}
	nativeReminderIconCache[key] = result
	nativeReminderIconMu.Unlock()
	return result
}

func nativeReminderScalePercent(hwnd uintptr, configured int) int {
	if configured >= 50 {
		return min(500, configured)
	}
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		return 100
	}
	return max(50, min(500, int(dpi)*100/96))
}
func nativeReminderOpacity(value int) int {
	if value < 20 || value > 100 {
		return 100
	}
	return value
}
func nativeWindowPosition(hwnd uintptr) (int, int) {
	var rect nativeRect
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return 0, 0
	}
	return int(rect.Left), int(rect.Top)
}

func updateNativeReminderLayer(hwnd uintptr, x, y, opacity int, canvas *image.RGBA, texts []nativeReminderText, surface *nativeReminderSurface) {
	if hwnd == 0 || canvas.Bounds().Empty() {
		return
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()
	width, height := canvas.Bounds().Dx(), canvas.Bounds().Dy()
	if !ensureNativeReminderSurfaceLocked(surface, width, height) {
		return
	}
	destination := unsafe.Slice((*byte)(unsafe.Pointer(surface.bits)), width*height*4)
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			so := canvas.PixOffset(px, py)
			do := (py*width + px) * 4
			destination[do] = canvas.Pix[so+2]
			destination[do+1] = canvas.Pix[so+1]
			destination[do+2] = canvas.Pix[so]
			destination[do+3] = canvas.Pix[so+3]
		}
	}
	procSetBkMode.Call(surface.dc, transparentBK)
	for _, annotation := range texts {
		drawNativeReminderText(surface, width, height, annotation)
	}
	destinationPoint := nativePoint{X: int32(x), Y: int32(y)}
	size := nativeSize{CX: int32(width), CY: int32(height)}
	sourcePoint := nativePoint{}
	blend := nativeBlendFunction{BlendOp: acSrcOver, SourceConstantAlpha: byte(nativeReminderOpacity(opacity) * 255 / 100), AlphaFormat: acSrcAlpha}
	procUpdateLayeredWindow.Call(hwnd, 0, uintptr(unsafe.Pointer(&destinationPoint)), uintptr(unsafe.Pointer(&size)), surface.dc, uintptr(unsafe.Pointer(&sourcePoint)), 0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
}

func drawNativeReminderText(surface *nativeReminderSurface, width, height int, annotation nativeReminderText) {
	rect := annotation.rect
	left, top := max(0, int(rect.Left)), max(0, int(rect.Top))
	right, bottom := min(width, int(rect.Right)), min(height, int(rect.Bottom))
	if annotation.text == "" || right <= left || bottom <= top {
		return
	}
	destination := unsafe.Slice((*byte)(unsafe.Pointer(surface.bits)), width*height*4)
	before := make([]byte, (right-left)*(bottom-top)*3)
	index := 0
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			offset := (y*width + x) * 4
			copy(before[index:index+3], destination[offset:offset+3])
			index += 3
		}
	}
	font := nativeReminderFont(max(8, annotation.fontHeight), annotation.fontWeight)
	old := uintptr(0)
	if font != 0 {
		old, _, _ = procSelectObject.Call(surface.dc, font)
	}
	text, _ := windows.UTF16PtrFromString(annotation.text)
	procSetTextColor.Call(surface.dc, uintptr(annotation.color))
	procDrawTextW.Call(surface.dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), annotation.flags)
	if old != 0 {
		procSelectObject.Call(surface.dc, old)
	}
	index = 0
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			offset := (y*width + x) * 4
			if destination[offset] != before[index] || destination[offset+1] != before[index+1] || destination[offset+2] != before[index+2] {
				destination[offset+3] = 255
			}
			index += 3
		}
	}
}

func ensureNativeReminderSurfaceLocked(surface *nativeReminderSurface, width, height int) bool {
	if surface.dc != 0 && surface.width == width && surface.height == height {
		return true
	}
	releaseNativeReminderSurfaceLocked(surface)
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return false
	}
	info := nativeBitmapInfo{Header: nativeBitmapInfoHeader{Size: uint32(unsafe.Sizeof(nativeBitmapInfoHeader{})), Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32, Compression: biRGB, SizeImage: uint32(width * height * 4)}}
	var bits uintptr
	bitmap, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == 0 {
		procDeleteDC.Call(dc)
		return false
	}
	old, _, _ := procSelectObject.Call(dc, bitmap)
	surface.width = width
	surface.height = height
	surface.dc = dc
	surface.bitmap = bitmap
	surface.oldObject = old
	surface.bits = bits
	return true
}
func releaseNativeReminderSurface(surface *nativeReminderSurface) {
	surface.mu.Lock()
	releaseNativeReminderSurfaceLocked(surface)
	surface.mu.Unlock()
}
func releaseNativeReminderSurfaceLocked(surface *nativeReminderSurface) {
	if surface.dc != 0 && surface.oldObject != 0 {
		procSelectObject.Call(surface.dc, surface.oldObject)
	}
	if surface.bitmap != 0 {
		procDeleteObject.Call(surface.bitmap)
	}
	if surface.dc != 0 {
		procDeleteDC.Call(surface.dc)
	}
	surface.width = 0
	surface.height = 0
	surface.dc = 0
	surface.bitmap = 0
	surface.oldObject = 0
	surface.bits = 0
}

func nativeReminderFont(height, weight int) uintptr {
	if weight <= 0 {
		weight = 400
	}
	key := fmt.Sprintf("%d:%d", height, weight)
	nativeReminderFontMu.Lock()
	defer nativeReminderFontMu.Unlock()
	if font := nativeReminderFontCache[key]; font != 0 {
		return font
	}
	face, _ := windows.UTF16PtrFromString("Microsoft YaHei UI")
	font, _, _ := procCreateFontW.Call(uintptr(int32(-height)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(face)))
	if font != 0 {
		nativeReminderFontCache[key] = font
	}
	return font
}
