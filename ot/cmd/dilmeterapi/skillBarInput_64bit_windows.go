//go:build windows && (amd64 || arm64)

package main

type nativeSkillBarKeyboardInput struct {
	VirtualKey uint16
	ScanCode   uint16
	Flags      uint32
	Time       uint32
	ExtraInfo  uintptr
}

// INPUT uses an eight-byte aligned 32-byte union on 64-bit Windows.
type nativeSkillBarInput struct {
	Type         uint32
	_            uint32
	Keyboard     nativeSkillBarKeyboardInput
	_unionRemain [8]byte
}
