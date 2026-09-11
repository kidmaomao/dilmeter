package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
