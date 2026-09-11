//go:build windows

package main

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	trayIconID        = 1
	trayCallbackMsg   = 0x8001 // WM_APP + 1
	trayCommandPlayer = 1001
	trayCommandDebuff = 1002
	trayCommandMute   = 1003
	trayCommandExit   = 1004

	nimAdd     = 0x00000000
	nimDelete  = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmNull          = 0x0000
	wmContextMenu   = 0x007B
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205

	mfString    = 0x0000
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	imageIcon     = 1
	lrDefaultSize = 0x0040
	lrShared      = 0x8000
	swRestore     = 9
)

var (
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procLoadImageW          = user32.NewProc("LoadImageW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")

	trayHWND                uintptr
	trayMainHWND            uintptr
	trayIconData            trayNotifyIconData
	trayWndProcCallback     = syscall.NewCallback(trayWindowProc)
	trayPlayerOverlayHidden atomic.Bool
	trayDebuffOverlayHidden atomic.Bool
	traySoundMuted          atomic.Bool
)

type trayWndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type trayNotifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

func initializeTray(mainHWND uintptr) error {
	if trayHWND != 0 {
		return nil
	}
	trayMainHWND = mainHWND
	var module windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &module); err != nil {
		return err
	}
	className, _ := windows.UTF16PtrFromString("DilmeterCNTrayWindow")
	windowName, _ := windows.UTF16PtrFromString("DilmeterCN Tray")
	class := trayWndClassEx{
		CbSize:        uint32(unsafe.Sizeof(trayWndClassEx{})),
		LpfnWndProc:   trayWndProcCallback,
		HInstance:     uintptr(module),
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		0, 0, uintptr(module), 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("cannot create tray window")
	}
	trayHWND = hwnd
	icon, _, _ := procLoadImageW.Call(uintptr(module), 2, imageIcon, 0, 0, lrDefaultSize|lrShared)
	if icon == 0 {
		shutdownTray()
		return fmt.Errorf("cannot load tray icon")
	}
	trayIconData = trayNotifyIconData{
		CbSize:           uint32(unsafe.Sizeof(trayNotifyIconData{})),
		HWnd:             trayHWND,
		UID:              trayIconID,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: trayCallbackMsg,
		HIcon:            icon,
	}
	tip, _ := windows.UTF16FromString(fmt.Sprintf("%s v%s", AppName, AppVersion))
	copy(trayIconData.SzTip[:], tip)
	if result, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayIconData))); result == 0 {
		shutdownTray()
		return fmt.Errorf("cannot add tray icon")
	}
	return nil
}

func shutdownTray() {
	if trayIconData.HWnd != 0 {
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayIconData)))
		trayIconData = trayNotifyIconData{}
	}
	if trayHWND != 0 {
		procDestroyWindow.Call(trayHWND)
		trayHWND = 0
	}
}

func trayWindowProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case trayCallbackMsg:
		switch lParam {
		case wmRButtonUp, wmContextMenu:
			showTrayMenu(hwnd)
			return 0
		case wmLButtonDblClk:
			procShowWindow.Call(trayMainHWND, swRestore)
			procSetForegroundWindow.Call(trayMainHWND)
			return 0
		}
	case wmDestroy:
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return result
}

func showTrayMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	appendTrayMenuItem(menu, trayCommandPlayer, "关闭 Buff/技能 覆盖", trayPlayerOverlayHidden.Load())
	appendTrayMenuItem(menu, trayCommandDebuff, "关闭 Debuff 覆盖", trayDebuffOverlayHidden.Load())
	appendTrayMenuItem(menu, trayCommandMute, "静音", traySoundMuted.Load())
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	appendTrayMenuItem(menu, trayCommandExit, "关闭程序", false)
	var cursor nativePoint
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return
	}
	procSetForegroundWindow.Call(hwnd)
	command, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmReturnCmd,
		uintptr(cursor.X), uintptr(cursor.Y), 0,
		hwnd, 0,
	)
	handleTrayCommand(uint32(command))
	procPostMessageW.Call(hwnd, wmNull, 0, 0)
}

func appendTrayMenuItem(menu uintptr, command uint32, label string, checked bool) {
	flags := uintptr(mfString)
	if checked {
		flags |= mfChecked
	}
	text, _ := windows.UTF16PtrFromString(label)
	procAppendMenuW.Call(menu, flags, uintptr(command), uintptr(unsafe.Pointer(text)))
}

func handleTrayCommand(command uint32) {
	switch command {
	case trayCommandPlayer:
		hidden := !trayPlayerOverlayHidden.Load()
		trayPlayerOverlayHidden.Store(hidden)
		if hidden {
			procShowWindow.Call(buffOverlayHWND, swHide)
			procShowWindow.Call(skillOverlayHWND, swHide)
			requestNativeSkillBarVisibility(false)
		}
	case trayCommandDebuff:
		hidden := !trayDebuffOverlayHidden.Load()
		trayDebuffOverlayHidden.Store(hidden)
		if hidden {
			procShowWindow.Call(debuffOverlayHWND, swHide)
		}
	case trayCommandMute:
		traySoundMuted.Store(!traySoundMuted.Load())
	case trayCommandExit:
		procPostMessageW.Call(trayMainHWND, wmClose, 0, 0)
	}
}
