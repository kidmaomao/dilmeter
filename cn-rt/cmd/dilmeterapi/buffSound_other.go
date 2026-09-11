//go:build !windows

package main

import (
	"net/http"
)

func handleBuffSound(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "buff audio is only available on Windows", http.StatusNotImplemented)
}
