package packet

import (
	"errors"
	"testing"
	"time"
)

type captureOptionsFake struct {
	calls         []string
	failImmediate bool
}

func (f *captureOptionsFake) SetSnapLen(n int) error { f.calls = append(f.calls, "snap"); return nil }
func (f *captureOptionsFake) SetPromisc(v bool) error {
	f.calls = append(f.calls, "promisc")
	return nil
}
func (f *captureOptionsFake) SetTimeout(v time.Duration) error {
	f.calls = append(f.calls, "timeout")
	return nil
}
func (f *captureOptionsFake) SetBufferSize(v int) error {
	f.calls = append(f.calls, "buffer")
	return nil
}
func (f *captureOptionsFake) SetImmediateMode(v bool) error {
	if !v {
		panic("disabled immediate mode")
	}
	f.calls = append(f.calls, "immediate")
	if f.failImmediate {
		return errors.New("old driver")
	}
	return nil
}
func TestImmediateCaptureConfiguration(t *testing.T) {
	f := &captureOptionsFake{}
	if err := configureImmediateCapture(f, 10*time.Millisecond); err != nil || len(f.calls) != 5 || f.calls[4] != "immediate" {
		t.Fatal(f, err)
	}
	f.failImmediate = true
	if configureImmediateCapture(f, 10*time.Millisecond) == nil {
		t.Fatal("old-driver failure must reach fallback")
	}
}
