//go:build windows

package main

import "testing"

func TestTrayCommandsToggleIndependentState(t *testing.T) {
	trayPlayerOverlayHidden.Store(false)
	trayDebuffOverlayHidden.Store(false)
	traySoundMuted.Store(false)
	defer func() {
		trayPlayerOverlayHidden.Store(false)
		trayDebuffOverlayHidden.Store(false)
		traySoundMuted.Store(false)
	}()

	handleTrayCommand(trayCommandPlayer)
	if !trayPlayerOverlayHidden.Load() || trayDebuffOverlayHidden.Load() || traySoundMuted.Load() {
		t.Fatal("Buff/skill tray command changed the wrong state")
	}
	handleTrayCommand(trayCommandDebuff)
	if !trayDebuffOverlayHidden.Load() {
		t.Fatal("Debuff tray command did not hide its overlay")
	}
	handleTrayCommand(trayCommandMute)
	if !traySoundMuted.Load() {
		t.Fatal("mute tray command did not enable mute")
	}
	handleTrayCommand(trayCommandPlayer)
	if trayPlayerOverlayHidden.Load() {
		t.Fatal("second Buff/skill command did not restore the overlay")
	}
}

func TestTrayMuteSuppressesNativeSound(t *testing.T) {
	traySoundMuted.Store(true)
	defer traySoundMuted.Store(false)
	if err := playBuffSound("unsupported", 100); err != nil {
		t.Fatalf("muted sound should be ignored before decoding: %v", err)
	}
}
