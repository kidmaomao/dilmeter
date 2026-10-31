package main

import (
	"fmt"
	"net/http"
	"time"
)

// Called with skillOverlayState locked. A slow WebView retains just the latest
// complete snapshot, never a queue of obsolete player positions.
func publishSkillOverlaySnapshotLocked(data []byte) {
	for ch := range skillOverlayState.listeners {
		select {
		case <-ch:
		default:
		}
		ch <- data // immutable marshalled bytes, not the reused state buffer
	}
}

func handleSkillOverlayStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unavailable", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ch := make(chan []byte, 1)
	skillOverlayState.Lock()
	if skillOverlayState.listeners == nil {
		skillOverlayState.listeners = make(map[chan []byte]bool)
	}
	skillOverlayState.listeners[ch] = true
	ch <- append([]byte(nil), skillOverlayState.data...)
	skillOverlayState.Unlock()
	defer func() { skillOverlayState.Lock(); delete(skillOverlayState.listeners, ch); skillOverlayState.Unlock() }()
	keepalive := time.NewTicker(3 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, "event: ping\ndata: {}\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
