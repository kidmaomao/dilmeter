package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// Local QA only: serve the real handlers under continuous live refresh.
func TestReminderPreviewBrowserHarness(t *testing.T) {
	if os.Getenv("DILMETER_REMINDER_PREVIEW_QA") != "1" {
		t.Skip("local browser harness")
	}
	preserveBurstPreviewState(t)
	listener, err := net.Listen("tcp", "127.0.0.1:8061")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/reminder_preview", handleReminderPreview)
	mux.HandleFunc("/api/burst_preview", handleBurstReminderPreview)
	mux.HandleFunc("/api/boss_mechanic_preview", handleBossMechanicPreview)
	mux.HandleFunc("/api/skill_overlay/state", handleSkillOverlayState)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	server := &http.Server{Handler: mux}
	defer server.Close()
	go server.Serve(listener)
	t.Log("Reminder preview QA listening on 127.0.0.1:8061")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	done := time.NewTimer(10 * time.Minute)
	defer done.Stop()
	for {
		select {
		case now := <-ticker.C:
			raw, _ := json.Marshal(nativeSkillOverlayMessage{Type: "skill-cooldown-state", AtMs: now.UnixMilli(),
				Settings: nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100, IconSize: 48}})
			setSkillOverlayState(raw)
		case <-done.C:
			return
		}
	}
}
