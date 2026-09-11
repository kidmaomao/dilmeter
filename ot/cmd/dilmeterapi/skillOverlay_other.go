//go:build !windows

package main

import "net/http"

func setNativeSkillOverlayActive(_ bool) {}

func handleSkillOverlay(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}

func handleSkillOverlayPosition(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}
