//go:build !windows

package main

import "net/http"

func handleSkillBar(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "desktop skill bar is only available on Windows", http.StatusNotImplemented)
}
