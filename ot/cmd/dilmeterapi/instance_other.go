//go:build !windows

package main

import (
	"net/http"
)

func handleAppActivate(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not supported", http.StatusNotImplemented)
}
