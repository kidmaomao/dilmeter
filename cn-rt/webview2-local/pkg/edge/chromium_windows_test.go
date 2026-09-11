//go:build windows

package edge

import (
	"testing"

	"github.com/jchv/go-webview2/internal/w32"
)

func TestValidControllerBounds(t *testing.T) {
	tests := []struct {
		name   string
		bounds w32.Rect
		valid  bool
	}{
		{name: "normal", bounds: w32.Rect{Left: 0, Top: 0, Right: 1440, Bottom: 900}, valid: true},
		{name: "zero", bounds: w32.Rect{}, valid: false},
		{name: "zero width", bounds: w32.Rect{Bottom: 100}, valid: false},
		{name: "zero height", bounds: w32.Rect{Right: 100}, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validControllerBounds(test.bounds); got != test.valid {
				t.Fatalf("validControllerBounds(%+v) = %v, want %v", test.bounds, got, test.valid)
			}
		})
	}
}

func TestEvalBeforeControllerReadyIsSafe(t *testing.T) {
	(&Chromium{}).Eval(`window.test = true`)
	var browser *Chromium
	browser.Eval(`window.test = true`)
}
