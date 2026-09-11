package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestDPSRecordingDisablesOnlyPersistedDamage(t *testing.T) {
	recordDPSEvents.Store(true)
	t.Cleanup(func() { recordDPSEvents.Store(true) })

	request := httptest.NewRequest(http.MethodPut, "/api/dps_recording", strings.NewReader(`{"enabled":false}`))
	response := httptest.NewRecorder()
	handleDPSRecording(response, request)
	if response.Code != http.StatusOK || recordDPSEvents.Load() {
		t.Fatalf("DPS recording switch was not disabled: status=%d body=%s", response.Code, response.Body.String())
	}
	if shouldPersistEvent(&event.EventDamage{EventBase: event.EventBase{EventId: event.EventIdDamage}}) {
		t.Fatal("damage event was persisted while DPS recording was disabled")
	}
	if !shouldPersistEvent(&event.EventSkillAction{EventBase: event.EventBase{EventId: event.EventIdSkillAction}}) {
		t.Fatal("reminder event was suppressed with DPS recording")
	}
}
