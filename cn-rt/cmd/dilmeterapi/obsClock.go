package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/util"
)

func obsEventClock(id event.EventId, scope string, observedAt time.Time) *event.ObsClockStamp {
	if scope == "" || observedAt.IsZero() {
		return nil
	}
	switch id {
	case event.EventIdEntityAppear, event.EventIdEntityDisappear, event.EventIdLocalEntity, event.EventIdSkillAction, event.EventIdSkillState:
		return &event.ObsClockStamp{Session: util.ObsClockSession, AtMs: util.ObsClockAt(observedAt)}
	}
	return nil
}

var obsClockNonce = regexp.MustCompile(`^[a-zA-Z0-9-]{16,80}$`)

// A fresh nonce and no-store response let Star measure round-trip uncertainty.
// This read-only endpoint neither captures packets nor changes the system clock.
func handleObsClock(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "local access only", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nonce := r.URL.Query().Get("nonce")
	if !obsClockNonce.MatchString(nonce) {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		Version int    `json:"version"`
		Session string `json:"session"`
		NowMs   int64  `json:"nowMs"`
		Nonce   string `json:"nonce"`
	}{1, util.ObsClockSession, util.ObsClockNow(), nonce})
}
