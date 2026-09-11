package main

import "testing"

func TestSkillBarKeyForCode(t *testing.T) {
	tests := []struct {
		code       string
		virtualKey uint16
		extended   bool
	}{
		{code: "KeyQ", virtualKey: 'Q'},
		{code: "Digit7", virtualKey: '7'},
		{code: "F12", virtualKey: 0x7B},
		{code: "ArrowLeft", virtualKey: 0x25, extended: true},
		{code: "ControlRight", virtualKey: 0xA3, extended: true},
		{code: "Numpad3", virtualKey: 0x63},
		{code: "NumpadEnter", virtualKey: 0x0D, extended: true},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			spec, ok := skillBarKeyForCode(test.code)
			if !ok {
				t.Fatalf("skillBarKeyForCode(%q) was rejected", test.code)
			}
			if spec.VirtualKey != test.virtualKey || spec.Extended != test.extended {
				t.Fatalf("skillBarKeyForCode(%q) = %#v, want VK %#x extended=%v", test.code, spec, test.virtualKey, test.extended)
			}
		})
	}
}

func TestSkillBarKeyForCodeRejectsArbitraryInput(t *testing.T) {
	for _, code := range []string{"", "KeyQQ", "F25", "MediaPlayPause", "../../Enter", "KeyA\nEnter"} {
		if _, ok := skillBarKeyForCode(code); ok {
			t.Fatalf("skillBarKeyForCode(%q) unexpectedly succeeded", code)
		}
	}
}
