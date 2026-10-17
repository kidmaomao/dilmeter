package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func preserveBossPreviewState(t *testing.T) {
	t.Helper()
	preserveBurstPreviewState(t)
	skillOverlayState.Lock()
	old := skillOverlayState.bossPreviews
	skillOverlayState.bossPreviews = nil
	skillOverlayState.data = []byte(emptySkillOverlayState)
	skillOverlayState.Unlock()
	t.Cleanup(func() {
		skillOverlayState.Lock()
		skillOverlayState.bossPreviews = old
		skillOverlayState.Unlock()
	})
}

func TestBossMechanicPreviewsSurviveLiveFramesUntilTheirDeadline(t *testing.T) {
	for _, tc := range []struct {
		key     string
		seconds float64
	}{
		{"miel-orb", 20}, {"miel-laser", 5}, {greenCurtainKey, 10}, {greenCurtainKey, 20.5},
	} {
		t.Run(tc.key, func(t *testing.T) {
			preserveBossPreviewState(t)
			now := int64(100000)
			end := now + int64(tc.seconds*1000)
			setBossMechanicPreview(bossMechanicPreviewRequest{Key: tc.key, DurationSeconds: tc.seconds, X: 400, Y: 300, ScalePercent: 125}, now)
			live := nativeSkillOverlayMessage{Type: "skill-cooldown-state", Settings: nativeSkillOverlaySettings{Opacity: 100}, Items: []nativeSkillOverlayItem{{SkillID: 21002}}}
			data, _ := json.Marshal(live)
			// Reproduce the background refresh that previously erased the one-shot
			// renderer frame, including immediately before and at natural expiry.
			for at := now; at < end; at += 200 {
				setSkillOverlayStateAt(data, at)
				message := readBurstPreviewState(t)
				if len(message.Mechanics) != 1 || message.Mechanics[0].Key != tc.key || message.Mechanics[0].EndsAtMs != end || message.Mechanics[0].X != 400 {
					t.Fatalf("preview lost or restarted at %d: %+v", at, message.Mechanics)
				}
				if !nativeSkillOverlayMessageVisible(message, at) || len(message.Items) != 1 {
					t.Fatal("preview visibility or unrelated live state lost")
				}
			}
			setSkillOverlayStateAt(data, end-1)
			if !nativeSkillOverlayMessageVisible(readBurstPreviewState(t), end-1) {
				t.Fatal("expired early")
			}
			setSkillOverlayStateAt(data, end)
			if message := readBurstPreviewState(t); len(message.Mechanics) != 0 || nativeSkillOverlayMessageVisible(message, end) {
				t.Fatal("preview failed to expire")
			}
		})
	}
}

func TestBossMechanicPreviewRepeatedClickAndLiveEncounter(t *testing.T) {
	preserveBossPreviewState(t)
	setBossMechanicPreview(bossMechanicPreviewRequest{Key: greenCurtainKey, DurationSeconds: 10}, 100000)
	setBossMechanicPreview(bossMechanicPreviewRequest{Key: greenCurtainKey, DurationSeconds: 20.5}, 102000)
	setBossMechanicPreview(bossMechanicPreviewRequest{Key: "miel-orb", DurationSeconds: 20}, 102000)
	live := nativeSkillOverlayMessage{Mechanics: []nativeBossMechanicOverlayItem{
		{Key: greenCurtainKey, Name: "live green curtain", EndsAtMs: 140000},
		{Key: "miel-laser", Name: "live laser", EndsAtMs: 140000},
	}}
	data, _ := json.Marshal(live)
	setSkillOverlayStateAt(data, 103000)
	message := readBurstPreviewState(t)
	if len(message.Mechanics) != 3 {
		t.Fatalf("lost or duplicated mechanics: %+v", message.Mechanics)
	}
	for _, item := range message.Mechanics {
		if item.Key == greenCurtainKey && item.EndsAtMs != 122500 {
			t.Fatal("repeat click did not replace its deadline")
		}
	}
	setSkillOverlayStateAt(data, 122500)
	message = readBurstPreviewState(t)
	if len(message.Mechanics) != 2 || message.Mechanics[0].Name != "live green curtain" || message.Mechanics[0].PreviewExpiresAtMs != 0 {
		t.Fatalf("preview changed the live encounter: %+v", message.Mechanics)
	}
}

func TestBossMechanicPreviewHTTPValidationAndDisabledRule(t *testing.T) {
	preserveBossPreviewState(t)
	for _, body := range []string{`{}`, `{"key":"unknown","durationSeconds":10}`, `{"key":"miel-orb","durationSeconds":0}`, `{"key":"miel-orb","durationSeconds":121}`, `{"key":"miel-green-curtain","durationSeconds":63}`} {
		response := httptest.NewRecorder()
		handleBossMechanicPreview(response, httptest.NewRequest("POST", "/api/boss_mechanic_preview", strings.NewReader(body)))
		if response.Code != 400 {
			t.Fatalf("invalid request accepted: %s", body)
		}
	}
	response := httptest.NewRecorder()
	handleBossMechanicPreview(response, httptest.NewRequest("GET", "/api/boss_mechanic_preview", nil))
	if response.Code != 405 {
		t.Fatal("GET should not start a preview")
	}
	response = httptest.NewRecorder()
	handleBossMechanicPreview(response, httptest.NewRequest("POST", "/api/boss_mechanic_preview", strings.NewReader(`{"key":"miel-green-curtain","durationSeconds":20.5,"x":420,"y":280,"scalePercent":100}`)))
	message := readBurstPreviewState(t)
	if response.Code != 204 || len(message.Mechanics) != 1 || message.Mechanics[0].EndsAtMs-message.Mechanics[0].StartedAtMs != 20500 {
		t.Fatalf("explicit preview must work without enabling a live rule: %d %+v", response.Code, message)
	}
}
