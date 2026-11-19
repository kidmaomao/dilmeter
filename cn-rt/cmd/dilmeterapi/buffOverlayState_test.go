package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuffOverlayStateRoundTrip(t *testing.T) {
	body := `{"type":"buff-state","at":123,"items":[{"ccId":680},{"ccId":63}]}`
	put := httptest.NewRequest(http.MethodPut, "/api/buff_overlay/state", strings.NewReader(body))
	putResult := httptest.NewRecorder()
	handleBuffOverlayState(putResult, put)
	if putResult.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want %d", putResult.Code, http.StatusNoContent)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/buff_overlay/state", nil)
	getResult := httptest.NewRecorder()
	handleBuffOverlayState(getResult, get)
	if getResult.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResult.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(getResult.Body.String()); got != body {
		t.Fatalf("GET body = %s, want %s", got, body)
	}
}

func TestBuffOverlayStateRejectsInvalidJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/api/buff_overlay/state", strings.NewReader("not-json"))
	result := httptest.NewRecorder()
	handleBuffOverlayState(result, request)
	if result.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", result.Code, http.StatusBadRequest)
	}
}

func TestNativeBuffWindowKeepsSavedOrderAcrossStateChanges(t *testing.T) {
	for _, tc := range []struct {
		rules string
		want  []uint32
	}{
		{`{"680":{"overlayEnabled":true},"192":{"overlayEnabled":true}}`, []uint32{192, 680}},
		{`{"680":{"overlayEnabled":true,"order":0},"192":{"overlayEnabled":true,"order":2},"17":{"order":1}}`, []uint32{680, 192}},
	} {
		var settings nativeReminderSettings
		if err := json.Unmarshal([]byte(tc.rules), &settings.Buff.Rules); err != nil {
			t.Fatal(err)
		}
		var sounds []nativeReminderSoundRequest
		runtime := newNativeReminderRuntimeForTest(settings, &sounds)
		runtime.localID = "self"
		player := runtime.ensureEntity("self")
		for _, active := range []bool{false, true, false} {
			if active {
				player.Conditions[192] = nativeReminderCondition{CCID: 192, At: 100}
			} else {
				delete(player.Conditions, 192)
			}
			runtime.evaluateBuffs(time.Unix(101, 0))
			rec := httptest.NewRecorder()
			handleBuffOverlayState(rec, httptest.NewRequest(http.MethodGet, "/api/buff_overlay/state", nil))
			var message nativeBuffOverlayMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &message); err != nil {
				t.Fatal(err)
			}
			var ids []uint32
			for _, item := range message.Items {
				ids = append(ids, item.CCID)
				if item.CCID == 192 && item.Active != active {
					t.Fatalf("Buff 192 active = %t, want %t", item.Active, active)
				}
			}
			if !slices.Equal(ids, tc.want) {
				t.Fatalf("Buff order = %v, want %v", ids, tc.want)
			}
		}
	}
}
