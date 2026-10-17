package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// Opt-in UI verification against the real handlers with live empty frames.
func TestBossMechanicPreviewBrowserHarness(t *testing.T) {
	if os.Getenv("DILMETER_BOSS_PREVIEW_QA") != "1" {
		t.Skip("local browser harness")
	}
	preserveBossPreviewState(t)
	listener, err := net.Listen("tcp", "127.0.0.1:8059")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/boss_mechanic_preview", handleBossMechanicPreview)
	mux.HandleFunc("/api/skill_overlay/state", handleSkillOverlayState)
	mux.HandleFunc("/api/skill_overlay", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	server := &http.Server{Handler: mux}
	defer server.Close()
	go server.Serve(listener)
	t.Log("Boss preview QA listening on 127.0.0.1:8059; live refresh every 200 ms")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	done := time.NewTimer(4 * time.Minute)
	defer done.Stop()
	for {
		select {
		case now := <-ticker.C:
			data, _ := json.Marshal(nativeSkillOverlayMessage{Type: "skill-cooldown-state", AtMs: now.UnixMilli(), Settings: nativeSkillOverlaySettings{OverlayEnabled: true, Opacity: 100, IconSize: 48}})
			setSkillOverlayState(data)
		case <-done.C:
			return
		}
	}
}
