//go:build !windows

package main

func queueNativeReminderSound(nativeReminderSoundRequest) bool { return false }
