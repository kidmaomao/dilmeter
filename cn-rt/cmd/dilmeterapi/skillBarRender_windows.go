//go:build windows

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	biRGB          = 0
	dibRGBColors   = 0
	ulwAlpha       = 0x00000002
	acSrcOver      = 0x00
	acSrcAlpha     = 0x01
	transparentBK  = 1
	dtCenter       = 0x00000001
	dtRight        = 0x00000002
	dtVCenter      = 0x00000004
	dtBottom       = 0x00000008
	dtSingleLine   = 0x00000020
	dtNoPrefix     = 0x00000800
	defaultGUIFont = 17
)

var (
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procSetBkMode           = gdi32.NewProc("SetBkMode")
	procSetTextColor        = gdi32.NewProc("SetTextColor")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
	procDrawTextW           = user32.NewProc("DrawTextW")

	skillBarSurfaceMu  sync.Mutex
	skillBarRenderMu   sync.Mutex
	skillBarSurface    nativeSkillBarSurface
	skillBarIconMu     sync.Mutex
	skillBarIconCache  = make(map[string]*image.RGBA)
	skillBarFeedbackMu sync.Mutex
	skillBarFeedbacks  = make(map[int]nativeSkillBarFeedback)
)

type nativeBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type nativeBitmapInfo struct {
	Header nativeBitmapInfoHeader
	Colors [1]uint32
}

type nativeSize struct {
	CX, CY int32
}

type nativeBlendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type nativeSkillBarSurface struct {
	width, height int
	dc            uintptr
	bitmap        uintptr
	oldObject     uintptr
	bits          uintptr
}

type nativeSkillBarText struct {
	text         string
	rect         nativeRect
	color        uint32
	flags        uintptr
	fontHeight   int
	fontWeight   int
	outlineColor uint32
	outlineSize  int
}

type nativeSkillBarFeedback struct {
	untilMs int64
	failed  bool
}

func setNativeSkillBarFeedback(index int, failed bool) {
	skillBarFeedbackMu.Lock()
	if !failed {
		// A successful activation must leave the configured icon completely
		// unchanged. It only clears an older failure marker, if one exists.
		_, changed := skillBarFeedbacks[index]
		delete(skillBarFeedbacks, index)
		skillBarFeedbackMu.Unlock()
		if changed {
			renderNativeSkillBar()
		}
		return
	}
	skillBarFeedbacks[index] = nativeSkillBarFeedback{untilMs: time.Now().UnixMilli() + 520, failed: true}
	skillBarFeedbackMu.Unlock()
	renderNativeSkillBar()
}

func currentNativeSkillBarFeedback(index int, nowMs int64) (nativeSkillBarFeedback, bool) {
	skillBarFeedbackMu.Lock()
	feedback, ok := skillBarFeedbacks[index]
	if ok && feedback.untilMs <= nowMs {
		delete(skillBarFeedbacks, index)
		ok = false
	}
	skillBarFeedbackMu.Unlock()
	return feedback, ok
}

func renderNativeSkillBar() {
	hwnd := skillBarHWND.Load()
	if hwnd == 0 || skillBarClosing.Load() {
		return
	}
	if !skillBarRenderPosted.CompareAndSwap(false, true) {
		return
	}
	if posted, _, _ := procPostMessageW.Call(hwnd, wmSkillBarRender, uintptr(skillBarWindowSession.Load()), 0); posted == 0 {
		skillBarRenderPosted.Store(false)
	}
}

func renderNativeSkillBarNow(hwnd uintptr) {
	skillBarRenderMu.Lock()
	defer skillBarRenderMu.Unlock()
	if hwnd == 0 || hwnd != skillBarHWND.Load() || skillBarClosing.Load() || skillBarDragging.Load() {
		return
	}
	settings := currentNativeSkillBarSettings()
	if settings.Width <= 0 || settings.Height <= 0 {
		return
	}
	canvas := image.NewRGBA(image.Rect(0, 0, settings.Width, settings.Height))
	texts := make([]nativeSkillBarText, 0, len(settings.Slots))
	if settings.Locked {
		// Keep the complete rectangle in layered-window hit testing. Alpha 1 can
		// round down to zero after global opacity is applied, reopening invisible
		// click-through gaps; alpha 8 remains visually imperceptible but hittable.
		fillNativeSkillBarRect(canvas, canvas.Bounds(), color.RGBA{A: 8})
	} else {
		fillNativeSkillBarRect(canvas, canvas.Bounds(), color.RGBA{R: 12, G: 45, B: 54, A: 205})
		drawNativeSkillBarBorder(canvas, canvas.Bounds(), color.RGBA{R: 116, G: 226, B: 246, A: 245}, 1)
	}
	cooldowns := nativeSkillBarCooldowns()
	nowMs := time.Now().UnixMilli()
	padding := nativeSkillBarPadding(settings.Locked)
	for index, slot := range settings.Slots {
		column := index % settings.Columns
		row := index / settings.Columns
		left := padding + column*(settings.IconSize+settings.Gap)
		top := padding + row*(settings.IconSize+settings.Gap)
		rect := image.Rect(left, top, left+settings.IconSize, top+settings.IconSize)
		feedback, hasFeedback := currentNativeSkillBarFeedback(index, nowMs)
		texts = append(texts, drawNativeSkillBarSlot(canvas, rect, slot, cooldowns[slot.SkillID], nowMs, feedback, hasFeedback)...)
	}
	updateNativeSkillBarLayer(hwnd, settings, canvas, texts)
}

func drawNativeSkillBarSlot(canvas *image.RGBA, rect image.Rectangle, slot nativeSkillBarSlot, cooldown nativeSkillOverlayItem, nowMs int64, feedback nativeSkillBarFeedback, hasFeedback bool) []nativeSkillBarText {
	border := color.RGBA{R: 167, G: 196, B: 201, A: 225}
	if slot.SkillID == 0 {
		border = color.RGBA{R: 91, G: 114, B: 119, A: 155}
	}
	hasFailureFeedback := hasFeedback && feedback.failed
	if hasFailureFeedback {
		border = color.RGBA{R: 255, G: 84, B: 84, A: 255}
	}
	fillNativeSkillBarRect(canvas, rect, border)
	inner := rect.Inset(1)
	fillNativeSkillBarRect(canvas, inner, color.RGBA{R: 20, G: 28, B: 30, A: 235})
	texts := make([]nativeSkillBarText, 0, 1)
	if slot.SkillID == 0 {
		texts = append(texts, nativeSkillBarText{
			text: "+", rect: imageRectToNative(inner), color: nativeColorRef(132, 157, 162),
			flags: dtCenter | dtVCenter | dtSingleLine | dtNoPrefix,
		})
		return texts
	}
	if icon := nativeSkillBarIcon(slot.SkillID, inner.Dx(), inner.Dy()); icon != nil {
		draw.Draw(canvas, inner, icon, icon.Bounds().Min, draw.Over)
	}
	if countdown, fraction := nativeSkillBarCooldownText(cooldown, nowMs); countdown != "" {
		shadeHeight := max(1, int(float64(inner.Dy())*fraction))
		fillNativeSkillBarRect(canvas, image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, min(inner.Max.Y, inner.Min.Y+shadeHeight)), color.RGBA{A: 165})
		fontHeight := max(13, min(24, inner.Dy()*2/5))
		texts = append(texts, nativeSkillBarText{
			text: countdown, rect: imageRectToNative(inner), color: nativeColorRef(255, 255, 255),
			flags:      dtCenter | dtVCenter | dtSingleLine | dtNoPrefix,
			fontHeight: fontHeight, fontWeight: nativeFontBold,
			outlineColor: nativeColorRef(0, 0, 0), outlineSize: nativeSkillBarCooldownOutlineSize(inner.Dy()),
		})
	}
	if hasFailureFeedback {
		drawNativeSkillBarBorder(canvas, rect.Inset(2), border, 2)
	}
	return texts
}

func nativeSkillBarCooldownOutlineSize(iconHeight int) int {
	if iconHeight >= 40 {
		return 2
	}
	return 1
}

func nativeSkillBarIcon(skillID uint16, width, height int) *image.RGBA {
	if skillID == 0 || width <= 0 || height <= 0 {
		return nil
	}
	key := fmt.Sprintf("%d:%dx%d", skillID, width, height)
	skillBarIconMu.Lock()
	if cached := skillBarIconCache[key]; cached != nil {
		skillBarIconMu.Unlock()
		return cached
	}
	skillBarIconMu.Unlock()
	data, err := staticFiles.ReadFile(fmt.Sprintf("%s/skill-icons/%d.png", embeddedStaticDir, skillID))
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
		sourceY := bounds.Min.Y + y*bounds.Dy()/height
		for x := 0; x < width; x++ {
			sourceX := bounds.Min.X + x*bounds.Dx()/width
			result.Set(x, y, decoded.At(sourceX, sourceY))
		}
	}
	skillBarIconMu.Lock()
	if len(skillBarIconCache) >= 256 {
		skillBarIconCache = make(map[string]*image.RGBA)
	}
	skillBarIconCache[key] = result
	skillBarIconMu.Unlock()
	return result
}

func fillNativeSkillBarRect(destination *image.RGBA, rect image.Rectangle, fill color.RGBA) {
	rect = rect.Intersect(destination.Bounds())
	if rect.Empty() {
		return
	}
	draw.Draw(destination, rect, &image.Uniform{C: fill}, image.Point{}, draw.Over)
}

func drawNativeSkillBarBorder(destination *image.RGBA, rect image.Rectangle, stroke color.RGBA, thickness int) {
	for offset := 0; offset < thickness; offset++ {
		current := rect.Inset(offset)
		if current.Empty() {
			return
		}
		fillNativeSkillBarRect(destination, image.Rect(current.Min.X, current.Min.Y, current.Max.X, current.Min.Y+1), stroke)
		fillNativeSkillBarRect(destination, image.Rect(current.Min.X, current.Max.Y-1, current.Max.X, current.Max.Y), stroke)
		fillNativeSkillBarRect(destination, image.Rect(current.Min.X, current.Min.Y, current.Min.X+1, current.Max.Y), stroke)
		fillNativeSkillBarRect(destination, image.Rect(current.Max.X-1, current.Min.Y, current.Max.X, current.Max.Y), stroke)
	}
}

func imageRectToNative(rect image.Rectangle) nativeRect {
	return nativeRect{Left: int32(rect.Min.X), Top: int32(rect.Min.Y), Right: int32(rect.Max.X), Bottom: int32(rect.Max.Y)}
}

func nativeColorRef(red, green, blue byte) uint32 {
	return uint32(red) | uint32(green)<<8 | uint32(blue)<<16
}

func updateNativeSkillBarLayer(hwnd uintptr, settings nativeSkillBarSettings, canvas *image.RGBA, texts []nativeSkillBarText) {
	skillBarSurfaceMu.Lock()
	defer skillBarSurfaceMu.Unlock()
	if skillBarClosing.Load() || skillBarHWND.Load() != hwnd {
		return
	}
	if !ensureNativeSkillBarSurfaceLocked(settings.Width, settings.Height) {
		return
	}
	destination := unsafe.Slice((*byte)(unsafe.Pointer(skillBarSurface.bits)), settings.Width*settings.Height*4)
	for y := 0; y < settings.Height; y++ {
		for x := 0; x < settings.Width; x++ {
			sourceOffset := canvas.PixOffset(x, y)
			destinationOffset := (y*settings.Width + x) * 4
			destination[destinationOffset] = canvas.Pix[sourceOffset+2]
			destination[destinationOffset+1] = canvas.Pix[sourceOffset+1]
			destination[destinationOffset+2] = canvas.Pix[sourceOffset]
			destination[destinationOffset+3] = canvas.Pix[sourceOffset+3]
		}
	}
	procSetBkMode.Call(skillBarSurface.dc, transparentBK)
	for _, annotation := range texts {
		drawNativeSkillBarText(skillBarSurface.dc, annotation)
	}
	size := nativeSize{CX: int32(settings.Width), CY: int32(settings.Height)}
	sourcePoint := nativePoint{}
	blend := nativeBlendFunction{BlendOp: acSrcOver, SourceConstantAlpha: byte(settings.Opacity * 255 / 100), AlphaFormat: acSrcAlpha}
	procUpdateLayeredWindow.Call(
		hwnd, 0,
		0, uintptr(unsafe.Pointer(&size)),
		skillBarSurface.dc, uintptr(unsafe.Pointer(&sourcePoint)),
		0, uintptr(unsafe.Pointer(&blend)), ulwAlpha,
	)
}

func drawNativeSkillBarText(dc uintptr, annotation nativeSkillBarText) {
	if dc == 0 || annotation.text == "" {
		return
	}
	font, _, _ := procGetStockObject.Call(defaultGUIFont)
	if annotation.fontHeight > 0 {
		font = nativeReminderFont(annotation.fontHeight, annotation.fontWeight)
	}
	oldFont := uintptr(0)
	if font != 0 {
		oldFont, _, _ = procSelectObject.Call(dc, font)
	}
	text, _ := windows.UTF16PtrFromString(annotation.text)
	if annotation.outlineSize > 0 {
		procSetTextColor.Call(dc, uintptr(annotation.outlineColor))
		for offsetY := -annotation.outlineSize; offsetY <= annotation.outlineSize; offsetY++ {
			for offsetX := -annotation.outlineSize; offsetX <= annotation.outlineSize; offsetX++ {
				if offsetX == 0 && offsetY == 0 {
					continue
				}
				rect := offsetNativeSkillBarTextRect(annotation.rect, offsetX, offsetY)
				procDrawTextW.Call(dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), annotation.flags)
			}
		}
	}
	rect := annotation.rect
	procSetTextColor.Call(dc, uintptr(annotation.color))
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), annotation.flags)
	if oldFont != 0 {
		procSelectObject.Call(dc, oldFont)
	}
}

func offsetNativeSkillBarTextRect(rect nativeRect, x, y int) nativeRect {
	rect.Left += int32(x)
	rect.Right += int32(x)
	rect.Top += int32(y)
	rect.Bottom += int32(y)
	return rect
}

func ensureNativeSkillBarSurfaceLocked(width, height int) bool {
	if skillBarSurface.dc != 0 && skillBarSurface.width == width && skillBarSurface.height == height {
		return true
	}
	releaseNativeSkillBarSurfaceLocked()
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return false
	}
	info := nativeBitmapInfo{Header: nativeBitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(nativeBitmapInfoHeader{})), Width: int32(width), Height: -int32(height),
		Planes: 1, BitCount: 32, Compression: biRGB, SizeImage: uint32(width * height * 4),
	}}
	var bits uintptr
	bitmap, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == 0 {
		procDeleteDC.Call(dc)
		return false
	}
	oldObject, _, _ := procSelectObject.Call(dc, bitmap)
	skillBarSurface = nativeSkillBarSurface{width: width, height: height, dc: dc, bitmap: bitmap, oldObject: oldObject, bits: bits}
	return true
}

func releaseNativeSkillBarSurface() {
	skillBarSurfaceMu.Lock()
	releaseNativeSkillBarSurfaceLocked()
	skillBarSurfaceMu.Unlock()
}

func releaseNativeSkillBarSurfaceLocked() {
	if skillBarSurface.dc != 0 && skillBarSurface.oldObject != 0 {
		procSelectObject.Call(skillBarSurface.dc, skillBarSurface.oldObject)
	}
	if skillBarSurface.bitmap != 0 {
		procDeleteObject.Call(skillBarSurface.bitmap)
	}
	if skillBarSurface.dc != 0 {
		procDeleteDC.Call(skillBarSurface.dc)
	}
	skillBarSurface = nativeSkillBarSurface{}
}
