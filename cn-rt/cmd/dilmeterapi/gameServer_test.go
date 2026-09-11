package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGameServerSettingsPersistAcceleratorMode(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
		_, _ = applyGameServer(defaultGameServerID)
		select {
		case <-gameServerChanged:
		default:
		}
	})
	if err := os.WriteFile("config.yaml", []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/game_server",
		strings.NewReader(`{"selected":"yate","acceleratorMode":true}`),
	)
	recorder := httptest.NewRecorder()
	handleGameServer(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %q", recorder.Code, recorder.Body.String())
	}

	var response gameServerSettings
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Selected != "yate" || !response.AcceleratorMode {
		t.Fatalf("unexpected response: %#v", response)
	}

	cfg := loadConfig()
	if cfg.GameServer != "yate" || !cfg.AcceleratorMode {
		t.Fatalf("settings were not persisted: %#v", cfg)
	}

	getRecorder := httptest.NewRecorder()
	handleGameServer(getRecorder, httptest.NewRequest(http.MethodGet, "/api/game_server", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d", getRecorder.Code)
	}
	var loaded gameServerSettings
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Selected != "yate" || !loaded.AcceleratorMode {
		t.Fatalf("GET returned unexpected settings: %#v", loaded)
	}
}
