package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEdgeTTSHandler(t *testing.T) {
	dir := t.TempDir()
	previousDir, previousSynthesis := customAudioDir, synthesizeEdgeTTS
	t.Cleanup(func() { customAudioDir, synthesizeEdgeTTS = previousDir, previousSynthesis })
	customAudioDir = func() string { return dir }
	synthesizeEdgeTTS = func(_ context.Context, text string, rate int) ([]byte, error) {
		if text != "绿幕即将出现" || rate != 2 {
			t.Fatalf("wrong synthesis input: %q %d", text, rate)
		}
		return []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0, 1, 2, 3}, nil
	}
	request := httptest.NewRequest(http.MethodPost, "/api/edge_tts", strings.NewReader(`{"text":"绿幕即将出现","rate":2}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Dilmeter-Action", "generate-edge-tts")
	recorder := httptest.NewRecorder()
	handleEdgeTTS(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("%d: %s", recorder.Code, recorder.Body.String())
	}
	var result localTTSResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Voice != edgeTTSVoice || result.DisplayName != "晓晓-绿幕即将出现.mp3" {
		t.Fatalf("wrong result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, result.SoundID+".mp3")); err != nil {
		t.Fatal(err)
	}
}

func TestEdgeTTSRejectsInvalidRequests(t *testing.T) {
	previous := synthesizeEdgeTTS
	t.Cleanup(func() { synthesizeEdgeTTS = previous })
	synthesizeEdgeTTS = func(context.Context, string, int) ([]byte, error) {
		t.Fatal("invalid request reached synthesis")
		return nil, nil
	}
	for _, test := range []struct {
		name, remote, action, body string
		code                       int
	}{
		{"remote", "192.168.1.2:123", "generate-edge-tts", `{"text":"提示"}`, 403},
		{"no-action", "127.0.0.1:123", "", `{"text":"提示"}`, 403},
		{"empty", "127.0.0.1:123", "generate-edge-tts", `{"text":" "}`, 400},
		{"long", "127.0.0.1:123", "generate-edge-tts", `{"text":"` + strings.Repeat("字", 121) + `"}`, 400},
		{"rate", "127.0.0.1:123", "generate-edge-tts", `{"text":"提示","rate":6}`, 400},
		{"malformed", "127.0.0.1:123", "generate-edge-tts", `{broken}`, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/edge_tts", strings.NewReader(test.body))
			request.RemoteAddr = test.remote
			request.Header.Set("X-Dilmeter-Action", test.action)
			recorder := httptest.NewRecorder()
			handleEdgeTTS(recorder, request)
			if recorder.Code != test.code {
				t.Fatalf("got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestEdgeTTSFailureDoesNotSaveAudio(t *testing.T) {
	dir := t.TempDir()
	previousDir, previousSynthesis := customAudioDir, synthesizeEdgeTTS
	t.Cleanup(func() { customAudioDir, synthesizeEdgeTTS = previousDir, previousSynthesis })
	customAudioDir = func() string { return dir }
	for _, mode := range []string{"network", "invalid-audio"} {
		synthesizeEdgeTTS = func(context.Context, string, int) ([]byte, error) {
			if mode == "network" {
				return nil, errors.New("network unavailable")
			}
			return []byte("not an MP3"), nil
		}
		request := httptest.NewRequest(http.MethodPost, "/api/edge_tts", strings.NewReader(`{"text":"提示"}`))
		request.RemoteAddr = "127.0.0.1:12345"
		request.Header.Set("X-Dilmeter-Action", "generate-edge-tts")
		recorder := httptest.NewRecorder()
		handleEdgeTTS(recorder, request)
		if recorder.Code != http.StatusBadGateway {
			t.Fatalf("got %d", recorder.Code)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed generation left a selectable audio file")
	}
}

func TestEdgeTTSMissingDependencies(t *testing.T) {
	previous := edgeTTSCandidates
	t.Cleanup(func() { edgeTTSCandidates = previous })
	edgeTTSCandidates = func() []edgeTTSCommand { return []edgeTTSCommand{{Executable: "dilmeter-python-does-not-exist"}} }
	_, status := detectEdgeTTS(context.Background())
	if status.Ready || status.PythonAvailable || !strings.Contains(status.Message, "软件文件夹") {
		t.Fatalf("wrong dependency help: %#v", status)
	}
}

func TestEdgeTTSAddonCandidateLocation(t *testing.T) {
	previous := edgeTTSApplicationExecutable
	t.Cleanup(func() { edgeTTSApplicationExecutable = previous })
	directory := filepath.Join(t.TempDir(), "中文 软件目录")
	edgeTTSApplicationExecutable = func() (string, error) { return filepath.Join(directory, "DilmeterCN.exe"), nil }
	candidates := defaultEdgeTTSCandidates()
	if len(candidates) != 4 || candidates[0].Source != "addon" ||
		candidates[0].Executable != filepath.Join(directory, "addons", "edge-tts", "python", "python.exe") {
		t.Fatalf("addon must be beside the app and ahead of system Python: %#v", candidates)
	}
	edgeTTSApplicationExecutable = func() (string, error) { return "", errors.New("unavailable") }
	for _, candidate := range defaultEdgeTTSCandidates() {
		if candidate.Source != "system" {
			t.Fatal("must not search the working directory when the app path is unavailable")
		}
	}
}

// Opt-in integration test uses the unpacked deliverable, not the developer's Python.
// Run with DILMETER_EDGE_TTS_ADDON_TEST_ROOT pointing to the ZIP extraction directory.
func TestEdgeTTSAddonIntegration(t *testing.T) {
	root := os.Getenv("DILMETER_EDGE_TTS_ADDON_TEST_ROOT")
	if root == "" {
		t.Skip("set DILMETER_EDGE_TTS_ADDON_TEST_ROOT to test the portable addon")
	}
	root = filepath.Clean(root)
	previous := edgeTTSApplicationExecutable
	t.Cleanup(func() { edgeTTSApplicationExecutable = previous })
	edgeTTSApplicationExecutable = func() (string, error) { return filepath.Join(root, "Dilmeter.exe"), nil }
	t.Setenv("PATH", "")
	t.Setenv("PYTHONHOME", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("PYTHONPATH", filepath.Join(t.TempDir(), "nonexistent"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command, status := detectEdgeTTS(ctx)
	if !status.Ready || !status.AddonPresent || status.ApplicationDir != root || status.Source != "addon" || !strings.HasPrefix(command.Executable, root+string(filepath.Separator)) {
		t.Fatalf("portable addon failed without system Python: %#v, %#v", command, status)
	}
	if os.Getenv("DILMETER_EDGE_TTS_NETWORK_TEST") != "1" {
		return
	}
	data, err := generateEdgeTTS(ctx, "绿幕即将出现", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ext, ok := detectCustomAudioExtension(data); !ok || ext != ".mp3" || len(data) < 1000 {
		t.Fatalf("addon returned invalid audio: %d bytes", len(data))
	}
	t.Logf("portable addon generated %d bytes of MP3 with Python absent from PATH", len(data))
}
