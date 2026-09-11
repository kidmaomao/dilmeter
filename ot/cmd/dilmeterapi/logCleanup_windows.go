//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 0x1
	fofSilent               = 0x0004
	fofNoConfirmation       = 0x0010
	fofAllowUndo            = 0x0040
	fofNoErrorUI            = 0x0400
	fofxRecycleOnDelete     = 0x00080000
	fofxEarlyFailure        = 0x00100000
)

var (
	logCleanupOle32                 = windows.NewLazySystemDLL("ole32.dll")
	logCleanupShell32               = windows.NewLazySystemDLL("shell32.dll")
	procLogCleanupCoInitializeEx    = logCleanupOle32.NewProc("CoInitializeEx")
	procLogCleanupCoUninitialize    = logCleanupOle32.NewProc("CoUninitialize")
	procLogCleanupCoCreateInstance  = logCleanupOle32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = logCleanupShell32.NewProc("SHCreateItemFromParsingName")

	clsidFileOperation = windows.GUID{Data1: 0x3AD05575, Data2: 0x8857, Data3: 0x4850, Data4: [8]byte{0x92, 0x77, 0x11, 0xB8, 0x5B, 0xDB, 0x8E, 0x09}}
	iidIFileOperation  = windows.GUID{Data1: 0x947AAB5F, Data2: 0x0A5C, Data3: 0x4C13, Data4: [8]byte{0xB4, 0xD6, 0x4B, 0xF7, 0x83, 0x6F, 0xC9, 0xF8}}
	iidIShellItem      = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

func moveFilesToRecycleBin(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procLogCleanupCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hresultFailed(hr) {
		return fmt.Errorf("初始化 Windows 回收站接口失败（0x%08X）", uint32(hr))
	}
	defer procLogCleanupCoUninitialize.Call()

	var operation uintptr
	hr, _, _ = procLogCleanupCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOperation)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOperation)),
		uintptr(unsafe.Pointer(&operation)),
	)
	if hresultFailed(hr) || operation == 0 {
		return fmt.Errorf("创建 Windows 回收站操作失败（0x%08X）", uint32(hr))
	}
	defer comRelease(operation)

	flags := uintptr(fofSilent | fofNoConfirmation | fofAllowUndo | fofNoErrorUI | fofxRecycleOnDelete | fofxEarlyFailure)
	if hr = comCall(operation, 5, flags); hresultFailed(hr) {
		return fmt.Errorf("设置 Windows 回收站操作失败（0x%08X）", uint32(hr))
	}

	for _, path := range paths {
		pathPtr, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		var item uintptr
		hr, _, _ = procSHCreateItemFromParsingName.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			0,
			uintptr(unsafe.Pointer(&iidIShellItem)),
			uintptr(unsafe.Pointer(&item)),
		)
		if hresultFailed(hr) || item == 0 {
			return fmt.Errorf("无法读取待回收日志 %q（0x%08X）", path, uint32(hr))
		}
		hr = comCall(operation, 18, item, 0)
		comRelease(item)
		runtime.KeepAlive(pathPtr)
		if hresultFailed(hr) {
			return fmt.Errorf("无法安排日志回收 %q（0x%08X）", path, uint32(hr))
		}
	}

	if hr = comCall(operation, 21); hresultFailed(hr) {
		return fmt.Errorf("Windows 回收站操作失败（0x%08X）", uint32(hr))
	}
	var aborted int32
	if hr = comCall(operation, 22, uintptr(unsafe.Pointer(&aborted))); hresultFailed(hr) {
		return fmt.Errorf("无法确认 Windows 回收站操作结果（0x%08X）", uint32(hr))
	}
	if aborted != 0 {
		return errors.New("Windows 已取消回收操作，没有执行永久删除")
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return fmt.Errorf("日志仍在原位置：%s", path)
			}
			return fmt.Errorf("无法确认日志已进入回收站：%w", err)
		}
	}
	return nil
}

func hresultFailed(value uintptr) bool {
	return int32(value) < 0
}

func comCall(object uintptr, methodIndex uintptr, args ...uintptr) uintptr {
	vtable := *(*uintptr)(unsafe.Pointer(object))
	method := *(*uintptr)(unsafe.Pointer(vtable + methodIndex*unsafe.Sizeof(uintptr(0))))
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, object)
	callArgs = append(callArgs, args...)
	result, _, _ := syscall.SyscallN(method, callArgs...)
	return result
}

func comRelease(object uintptr) {
	if object != 0 {
		_ = comCall(object, 2)
	}
}
