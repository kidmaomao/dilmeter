//go:build !windows

package main

import (
	"net/http"
)

func setNativeBuffOverlayActive(_ bool)   {}
func setNativeDebuffOverlayActive(_ bool) {}

func handleBuffOverlay(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}

func handleBuffOverlayDrag(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}

func handleBuffOverlayPosition(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}

func handleBuffOverlayResetPosition(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}

func handleDebuffOverlay(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}

func handleDebuffOverlayPosition(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop overlay is only available on Windows", http.StatusNotImplemented)
}
