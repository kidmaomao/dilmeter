package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

var recordDPSEvents atomic.Bool

func init() {
	recordDPSEvents.Store(true)
}

func handleDPSRecording(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPut {
		var request struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid DPS recording setting", http.StatusBadRequest)
			return
		}
		recordDPSEvents.Store(request.Enabled)
	} else if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"enabled": recordDPSEvents.Load()})
}

func shouldPersistEvent(value event.IEvent) bool {
	return recordDPSEvents.Load() || value.GetEventId() != event.EventIdDamage
}
