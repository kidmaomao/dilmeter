//go:build windows && dilmeter_rt

// Package windivert provides the small, read-only subset of the WinDivert API
// used by the experimental desktop capture backend.
package windivert

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	layerNetwork = 0
	flagSniff    = 0x0001
	flagRecvOnly = 0x0004
	shutdownBoth = 0x0003
)

var errInvalidHandle = errors.New("WinDivertOpen returned an invalid handle")

// Handle is a receive-only WinDivert NETWORK-layer handle.
type Handle struct {
	dll          *windows.DLL
	handle       windows.Handle
	procRecv     *windows.Proc
	procShutdown *windows.Proc
	procClose    *windows.Proc
	closeOnce    sync.Once
	closeErr     error
}

// Open loads the official WinDivert DLL at dllPath and opens a passive packet
// sniffer. The caller must run elevated because WinDivert installs/opens a WFP
// kernel driver on first use.
func Open(dllPath string, filter string) (*Handle, error) {
	if dllPath == "" {
		return nil, errors.New("WinDivert DLL path is empty")
	}
	if filter == "" {
		return nil, errors.New("WinDivert filter is empty")
	}

	dll, err := windows.LoadDLL(dllPath)
	if err != nil {
		return nil, fmt.Errorf("load WinDivert.dll: %w", err)
	}

	openProc, err := dll.FindProc("WinDivertOpen")
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("find WinDivertOpen: %w", err)
	}
	recvProc, err := dll.FindProc("WinDivertRecv")
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("find WinDivertRecv: %w", err)
	}
	shutdownProc, err := dll.FindProc("WinDivertShutdown")
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("find WinDivertShutdown: %w", err)
	}
	closeProc, err := dll.FindProc("WinDivertClose")
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("find WinDivertClose: %w", err)
	}

	filterBytes, err := windows.BytePtrFromString(filter)
	if err != nil {
		_ = dll.Release()
		return nil, fmt.Errorf("encode WinDivert filter: %w", err)
	}

	result, _, callErr := openProc.Call(
		uintptr(unsafe.Pointer(filterBytes)),
		uintptr(layerNetwork),
		uintptr(0),
		uintptr(flagSniff|flagRecvOnly),
	)
	if result == uintptr(windows.InvalidHandle) {
		_ = dll.Release()
		if callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			return nil, fmt.Errorf("WinDivertOpen: %w", callErr)
		}
		return nil, errInvalidHandle
	}

	return &Handle{
		dll:          dll,
		handle:       windows.Handle(result),
		procRecv:     recvProc,
		procShutdown: shutdownProc,
		procClose:    closeProc,
	}, nil
}

// Recv waits for one raw IP packet. WinDivert NETWORK-layer packets do not
// include an Ethernet header.
func (h *Handle) Recv(packet []byte) (int, error) {
	if h == nil || h.handle == windows.InvalidHandle || h.handle == 0 {
		return 0, errors.New("WinDivert handle is closed")
	}
	if len(packet) == 0 {
		return 0, errors.New("receive buffer is empty")
	}

	var received uint32
	// WINDIVERT_ADDRESS is 80 bytes in WinDivert 2.2.x. It is intentionally
	// opaque here because the reader only needs the raw packet bytes.
	var address [10]uint64
	result, _, callErr := h.procRecv.Call(
		uintptr(h.handle),
		uintptr(unsafe.Pointer(&packet[0])),
		uintptr(len(packet)),
		uintptr(unsafe.Pointer(&received)),
		uintptr(unsafe.Pointer(&address[0])),
	)
	if result == 0 {
		if callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			return 0, callErr
		}
		return 0, errors.New("WinDivertRecv failed")
	}
	return int(received), nil
}

// Close interrupts a pending receive and closes the driver handle. The DLL is
// intentionally kept loaded until process exit so a blocked native call can
// never race with FreeLibrary.
func (h *Handle) Close() error {
	if h == nil {
		return nil
	}
	h.closeOnce.Do(func() {
		if h.handle == windows.InvalidHandle || h.handle == 0 {
			return
		}
		_, _, _ = h.procShutdown.Call(uintptr(h.handle), uintptr(shutdownBoth))
		result, _, callErr := h.procClose.Call(uintptr(h.handle))
		if result == 0 && callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			h.closeErr = callErr
		}
		h.handle = windows.InvalidHandle
	})
	return h.closeErr
}
