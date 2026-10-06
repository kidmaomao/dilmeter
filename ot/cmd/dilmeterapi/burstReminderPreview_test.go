package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func preserveBurstPreviewState(t *testing.T) {
	t.Helper()
	skillOverlayState.Lock()
	oldData, oldPreview := append([]byte(nil), skillOverlayState.data...), skillOverlayState.burstPreview
	skillOverlayState.burstPreview = nil
	skillOverlayState.Unlock()
	t.Cleanup(func() {
		skillOverlayState.Lock()
		skillOverlayState.data, skillOverlayState.burstPreview = oldData, oldPreview
		skillOverlayState.Unlock()
	})
}

func readBurstPreviewState(t *testing.T) nativeSkillOverlayMessage {
	t.Helper()
	request := httptest.NewRequest("GET", "/api/skill_overlay/state", nil)
	response := httptest.NewRecorder()
	handleSkillOverlayState(response, request)
	var message nativeSkillOverlayMessage
	if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
		t.Fatal(err)
	}
	return message
}

func TestBurstPreviewSurvivesLiveRefreshForEightSeconds(t *testing.T) {
	preserveBurstPreviewState(t)
	for _, skill := range []uint16{59005, 58014} {
		for _, phase := range []string{"cast", "effect", "ready", "cooldown"} {
			t.Run(phase+string(rune(skill)), func(t *testing.T) {
				now := int64(100000)
				live := nativeSkillOverlayMessage{Type: "skill-cooldown-state", Settings: nativeSkillOverlaySettings{OverlayEnabled: false, Opacity: 100}, Items: []nativeSkillOverlayItem{{SkillID: 21002}}}
				data, _ := json.Marshal(live)
				setSkillOverlayStateAt(data, now)
				setBurstReminderPreview(nativeBossMechanicOverlayItem{SkillID: skill, Phase: phase, ActorName: "预览队友", EndsAtMs: now + 10, ScalePercent: 100}, now)
				for _, elapsed := range []int64{0, 200, 999, 3000, 7999} {
					live.AtMs = now + elapsed
					data, _ = json.Marshal(live)
					setSkillOverlayStateAt(data, now+elapsed)
					message := readBurstPreviewState(t)
					if len(message.Mechanics) != 1 || message.Mechanics[0].EndsAtMs != now+8000 || message.Mechanics[0].Phase != phase {
						t.Fatalf("preview replaced after %d ms: %+v", elapsed, message.Mechanics)
					}
					if len(message.Items) != 1 || message.Items[0].SkillID != 21002 {
						t.Fatal("preview replaced live state")
					}
					if !nativeSkillOverlayMessageVisible(message, now+elapsed) {
						t.Fatal("explicit preview hidden by normal reminder settings")
					}
				}
				setSkillOverlayStateAt(data, now+8000)
				message := readBurstPreviewState(t)
				if len(message.Mechanics) != 0 || nativeSkillOverlayMessageVisible(message, now+8000) {
					t.Fatal("preview exceeded eight seconds")
				}
			})
		}
	}
}

func TestBurstPreviewNextClickReplacesInsteadOfExtendingOldPreview(t *testing.T) {
	preserveBurstPreviewState(t)
	setBurstReminderPreview(nativeBossMechanicOverlayItem{SkillID: 59005, Phase: "cast", ScalePercent: 100}, 100000)
	setBurstReminderPreview(nativeBossMechanicOverlayItem{SkillID: 59005, Phase: "cooldown", Compact: true, ScalePercent: 100}, 102000)
	message := readBurstPreviewState(t)
	if len(message.Mechanics) != 1 || !message.Mechanics[0].Compact || message.Mechanics[0].EndsAtMs != 110000 {
		t.Fatalf("latest preview: %+v", message.Mechanics)
	}
	for _, body := range []string{`{}`, `{"skillId":123,"phase":"cast"}`, `{"skillId":59005,"phase":"invalid"}`} {
		response := httptest.NewRecorder()
		handleBurstReminderPreview(response, httptest.NewRequest("POST", "/api/burst_preview", strings.NewReader(body)))
		if response.Code != 400 {
			t.Fatalf("invalid preview accepted: %s", body)
		}
	}
}
