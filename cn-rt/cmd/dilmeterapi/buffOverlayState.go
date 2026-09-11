package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

const emptyBuffOverlayState = `{"type":"buff-state","at":0,"items":[]}`

var buffOverlayState = struct {
	sync.RWMutex
	data []byte
}{data: []byte(emptyBuffOverlayState)}

func setBuffOverlayState(data []byte) bool {
	if len(data) == 0 || !json.Valid(data) {
		return false
	}
	buffOverlayState.Lock()
	buffOverlayState.data = append(buffOverlayState.data[:0], data...)
	buffOverlayState.Unlock()
	var message nativeBuffOverlayMessage
	if json.Unmarshal(data, &message) == nil {
		enabled := message.Settings.OverlayEnabled || message.Settings.Opacity == 0
		setNativeBuffOverlayActive(enabled && len(message.Items) > 0)
	}
	return true
}

// handleBuffOverlayState bridges the main UI and native reminder window. The
// native runtime also publishes here while the main WebView is suspended.
func handleBuffOverlayState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		buffOverlayState.RLock()
		data := append([]byte(nil), buffOverlayState.data...)
		buffOverlayState.RUnlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPut:
		data, err := io.ReadAll(io.LimitReader(r.Body, 256*1024))
		if err != nil || !setBuffOverlayState(data) {
			http.Error(w, "invalid overlay state", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
