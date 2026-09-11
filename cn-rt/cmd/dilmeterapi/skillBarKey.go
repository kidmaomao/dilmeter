package main

import (
	"strconv"
	"strings"
)

type skillBarKeySpec struct {
	VirtualKey uint16
	Extended   bool
}

// skillBarKeyForCode converts the layout-independent KeyboardEvent.code used
// by the settings UI into the Win32 virtual key that MapVirtualKeyW expects.
// Keeping this list explicit also prevents the local HTTP endpoint from being
// used as an arbitrary input injector.
func skillBarKeyForCode(rawCode string) (skillBarKeySpec, bool) {
	code := strings.TrimSpace(rawCode)
	if len(code) == 4 && strings.HasPrefix(code, "Key") {
		letter := code[3]
		if letter >= 'A' && letter <= 'Z' {
			return skillBarKeySpec{VirtualKey: uint16(letter)}, true
		}
	}
	if len(code) == 6 && strings.HasPrefix(code, "Digit") {
		digit := code[5]
		if digit >= '0' && digit <= '9' {
			return skillBarKeySpec{VirtualKey: uint16(digit)}, true
		}
	}
	if strings.HasPrefix(code, "F") {
		if number, err := strconv.Atoi(code[1:]); err == nil && number >= 1 && number <= 24 {
			return skillBarKeySpec{VirtualKey: uint16(0x70 + number - 1)}, true
		}
	}

	keys := map[string]skillBarKeySpec{
		"Escape":         {VirtualKey: 0x1B},
		"Tab":            {VirtualKey: 0x09},
		"CapsLock":       {VirtualKey: 0x14},
		"Space":          {VirtualKey: 0x20},
		"Enter":          {VirtualKey: 0x0D},
		"Backspace":      {VirtualKey: 0x08},
		"ShiftLeft":      {VirtualKey: 0xA0},
		"ShiftRight":     {VirtualKey: 0xA1},
		"ControlLeft":    {VirtualKey: 0xA2},
		"ControlRight":   {VirtualKey: 0xA3, Extended: true},
		"AltLeft":        {VirtualKey: 0xA4},
		"AltRight":       {VirtualKey: 0xA5, Extended: true},
		"Insert":         {VirtualKey: 0x2D, Extended: true},
		"Delete":         {VirtualKey: 0x2E, Extended: true},
		"Home":           {VirtualKey: 0x24, Extended: true},
		"End":            {VirtualKey: 0x23, Extended: true},
		"PageUp":         {VirtualKey: 0x21, Extended: true},
		"PageDown":       {VirtualKey: 0x22, Extended: true},
		"ArrowLeft":      {VirtualKey: 0x25, Extended: true},
		"ArrowUp":        {VirtualKey: 0x26, Extended: true},
		"ArrowRight":     {VirtualKey: 0x27, Extended: true},
		"ArrowDown":      {VirtualKey: 0x28, Extended: true},
		"Minus":          {VirtualKey: 0xBD},
		"Equal":          {VirtualKey: 0xBB},
		"BracketLeft":    {VirtualKey: 0xDB},
		"BracketRight":   {VirtualKey: 0xDD},
		"Backslash":      {VirtualKey: 0xDC},
		"Semicolon":      {VirtualKey: 0xBA},
		"Quote":          {VirtualKey: 0xDE},
		"Backquote":      {VirtualKey: 0xC0},
		"Comma":          {VirtualKey: 0xBC},
		"Period":         {VirtualKey: 0xBE},
		"Slash":          {VirtualKey: 0xBF},
		"NumpadAdd":      {VirtualKey: 0x6B},
		"NumpadSubtract": {VirtualKey: 0x6D},
		"NumpadMultiply": {VirtualKey: 0x6A},
		"NumpadDivide":   {VirtualKey: 0x6F, Extended: true},
		"NumpadDecimal":  {VirtualKey: 0x6E},
		"NumpadEnter":    {VirtualKey: 0x0D, Extended: true},
	}
	if spec, ok := keys[code]; ok {
		return spec, true
	}
	if len(code) == 7 && strings.HasPrefix(code, "Numpad") {
		digit := code[6]
		if digit >= '0' && digit <= '9' {
			return skillBarKeySpec{VirtualKey: uint16(0x60 + digit - '0')}, true
		}
	}
	return skillBarKeySpec{}, false
}

// skillBarStopKeyForCode is intentionally narrower than the normal skill-key
// allow-list. A stop key is tapped on every compensated left click, so a
// modifier could remain logically active if another program interferes with
// key-up and a toggle key such as CapsLock would change global keyboard state.
func skillBarStopKeyForCode(rawCode string) (skillBarKeySpec, bool) {
	code := strings.TrimSpace(rawCode)
	switch code {
	case "ShiftLeft", "ShiftRight", "ControlLeft", "ControlRight", "AltLeft", "AltRight", "CapsLock":
		return skillBarKeySpec{}, false
	default:
		return skillBarKeyForCode(code)
	}
}
