//go:build windows && 386

package main

type nativeSkillBarKeyboardInput struct {
	VirtualKey uint16
	ScanCode   uint16
	Flags      uint32
	Time       uint32
	ExtraInfo  uintptr
}

// INPUT uses a 24-byte union on 32-bit Windows.
type nativeSkillBarInput struct {
	Type         uint32
	Keyboard     nativeSkillBarKeyboardInput
	_unionRemain [8]byte
}
