package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// Optional local UI harness: exercises the actual preview/state handlers while
// real-time state replacement continues every 200 ms. No capture or user data.
func TestBurstPreviewBrowserHarness(t *testing.T) {
	if os.Getenv("DILMETER_BURST_PREVIEW_QA") != "1" {
		t.Skip("local browser harness")
	}
	preserveBurstPreviewState(t)
	listener, err := net.Listen("tcp", "127.0.0.1:8058")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/burst_preview", handleBurstReminderPreview)
	mux.HandleFunc("/api/skill_overlay/state", handleSkillOverlayState)
	server := &http.Server{Handler: mux}
	defer server.Close()
	go server.Serve(listener)
	t.Log("Preview QA API listening on 127.0.0.1:8058")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	done := time.NewTimer(5 * time.Minute)
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
