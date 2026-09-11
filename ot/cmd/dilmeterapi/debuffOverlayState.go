package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

const emptyDebuffOverlayState = `{"type":"debuff-state","at":0,"boss":null,"targetHealth":null,"items":[],"settings":{"iconSize":30,"overlayEnabled":true}}`

var debuffOverlayState = struct {
	sync.RWMutex
	data []byte
}{data: []byte(emptyDebuffOverlayState)}

func setDebuffOverlayState(data []byte) bool {
	if len(data) == 0 || !json.Valid(data) {
		return false
	}
	debuffOverlayState.Lock()
	debuffOverlayState.data = append(debuffOverlayState.data[:0], data...)
	debuffOverlayState.Unlock()
	var message nativeDebuffOverlayMessage
	if json.Unmarshal(data, &message) == nil {
		setNativeDebuffOverlayActive(message.Settings.OverlayEnabled && (message.TargetHealth != nil || (message.Boss != nil && len(message.Items) > 0)))
	}
	return true
}

func handleDebuffOverlayState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		debuffOverlayState.RLock()
		data := append([]byte(nil), debuffOverlayState.data...)
		debuffOverlayState.RUnlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPut:
		data, err := io.ReadAll(io.LimitReader(r.Body, 256*1024))
		if err != nil || !setDebuffOverlayState(data) {
			http.Error(w, "invalid overlay state", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
