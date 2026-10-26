//go:build windows

package main

// Window messages and drawing primitives shared by read-only reminder overlays.
// This file contains no key mapping, skill activation or simulated input.
import (
	"image"
	"image/color"
	"image/draw"
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
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
)

const (
	wsPopup         = 0x80000000
	wmMouseActivate = 0x0021
	wmQuit          = 0x0012
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	maNoActivate    = 3
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
type nativeWindowMessage struct {
	HWND     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Point    nativePoint
	LPrivate uint32
}

func fillNativeOverlayRect(destination *image.RGBA, rect image.Rectangle, fill color.RGBA) {
	rect = rect.Intersect(destination.Bounds())
	if rect.Empty() {
		return
	}
	draw.Draw(destination, rect, &image.Uniform{C: fill}, image.Point{}, draw.Over)
}
func drawNativeOverlayBorder(destination *image.RGBA, rect image.Rectangle, stroke color.RGBA, thickness int) {
	for offset := 0; offset < thickness; offset++ {
		current := rect.Inset(offset)
		if current.Empty() {
			return
		}
		fillNativeOverlayRect(destination, image.Rect(current.Min.X, current.Min.Y, current.Max.X, current.Min.Y+1), stroke)
		fillNativeOverlayRect(destination, image.Rect(current.Min.X, current.Max.Y-1, current.Max.X, current.Max.Y), stroke)
		fillNativeOverlayRect(destination, image.Rect(current.Min.X, current.Min.Y, current.Min.X+1, current.Max.Y), stroke)
		fillNativeOverlayRect(destination, image.Rect(current.Max.X-1, current.Min.Y, current.Max.X, current.Max.Y), stroke)
	}
}
func imageRectToNative(rect image.Rectangle) nativeRect {
	return nativeRect{Left: int32(rect.Min.X), Top: int32(rect.Min.Y), Right: int32(rect.Max.X), Bottom: int32(rect.Max.Y)}
}
func nativeColorRef(red, green, blue byte) uint32 {
	return uint32(red) | uint32(green)<<8 | uint32(blue)<<16
}
