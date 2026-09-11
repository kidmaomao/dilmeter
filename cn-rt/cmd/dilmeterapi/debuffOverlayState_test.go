package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebuffOverlayStateRoundTrip(t *testing.T) {
	body := `{"type":"debuff-state","at":123,"items":[{"ccId":882,"state":"missing"}],"settings":{"iconSize":36}}`
	put := httptest.NewRequest(http.MethodPut, "/api/debuff_overlay/state", strings.NewReader(body))
	putResult := httptest.NewRecorder()
	handleDebuffOverlayState(putResult, put)
	if putResult.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want %d", putResult.Code, http.StatusNoContent)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/debuff_overlay/state", nil)
	getResult := httptest.NewRecorder()
	handleDebuffOverlayState(getResult, get)
	if getResult.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResult.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(getResult.Body.String()); got != body {
		t.Fatalf("GET body = %s, want %s", got, body)
	}
}

func TestDebuffOverlayStateRejectsInvalidJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/api/debuff_overlay/state", strings.NewReader("not-json"))
	result := httptest.NewRecorder()
	handleDebuffOverlayState(result, request)
	if result.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", result.Code, http.StatusBadRequest)
	}
}
