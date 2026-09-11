//go:build !windows

package main

import "sync/atomic"

var nativeReminderLocked atomic.Bool

func init() { nativeReminderLocked.Store(true) }

func initializeNativeReminderOverlayInput() error {
	return nil
}
func shutdownNativeReminderOverlayInput()         {}
func nativeReminderOverlaysAreLocked() bool       { return nativeReminderLocked.Load() }
func setNativeReminderOverlaysLocked(locked bool) { nativeReminderLocked.Store(locked) }
func refreshWebViewReminderHitRegions()           {}
