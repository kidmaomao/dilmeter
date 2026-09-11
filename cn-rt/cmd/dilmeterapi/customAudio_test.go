package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleCustomAudioUpload(t *testing.T) {
	dir := t.TempDir()
	previousDir := customAudioDir
	customAudioDir = func() string { return dir }
	t.Cleanup(func() { customAudioDir = previousDir })

	data := []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0, 1, 2, 3}
	request := newCustomAudioUploadRequest(t, `C:\Music\提醒.mp3`, data)
	request.RemoteAddr = "127.0.0.1:43120"
	recorder := httptest.NewRecorder()

	handleCustomAudioUpload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response customAudioUploadResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	digest := sha256.Sum256(data)
	expectedID := hex.EncodeToString(digest[:])
	if response.SoundID != expectedID {
		t.Fatalf("unexpected sound id: %q", response.SoundID)
	}
	if response.DisplayName != "提醒.mp3" {
		t.Fatalf("unexpected display name: %q", response.DisplayName)
	}
	saved, err := os.ReadFile(filepath.Join(dir, expectedID+".mp3"))
	if err != nil {
		t.Fatalf("read saved audio: %v", err)
	}
	if !bytes.Equal(saved, data) {
		t.Fatal("saved audio differs from upload")
	}
}

func TestHandleCustomAudioUploadRejectsRemoteRequest(t *testing.T) {
	request := newCustomAudioUploadRequest(t, "alert.wav", minimalWaveHeader())
	request.RemoteAddr = "192.0.2.10:43120"
	recorder := httptest.NewRecorder()

	handleCustomAudioUpload(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandleCustomAudioUploadRejectsInvalidContent(t *testing.T) {
	request := newCustomAudioUploadRequest(t, "fake.mp3", []byte("not audio"))
	request.RemoteAddr = "[::1]:43120"
	recorder := httptest.NewRecorder()

	handleCustomAudioUpload(recorder, request)

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNormalizeFloatWaveToCanonicalPCM16(t *testing.T) {
	input := makeFloatWave([]float32{-1, -0.5, 0, 0.5, 1})
	output, err := normalizeWaveToPCM16(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 44+10 || string(output[:4]) != "RIFF" || string(output[8:12]) != "WAVE" {
		t.Fatalf("unexpected normalized WAV header/size: %d bytes", len(output))
	}
	if got := binary.LittleEndian.Uint16(output[20:22]); got != 1 {
		t.Fatalf("format tag = %d, want PCM", got)
	}
	if got := binary.LittleEndian.Uint16(output[34:36]); got != 16 {
		t.Fatalf("bit depth = %d, want 16", got)
	}
	want := []int16{-32768, -16384, 0, 16384, 32767}
	for index, expected := range want {
		got := int16(binary.LittleEndian.Uint16(output[44+index*2 : 46+index*2]))
		if got != expected {
			t.Fatalf("sample %d = %d, want %d", index, got, expected)
		}
	}
}

func TestHandleCustomAudioUploadNormalizesWaveBeforeSaving(t *testing.T) {
	dir := t.TempDir()
	previousDir := customAudioDir
	customAudioDir = func() string { return dir }
	t.Cleanup(func() { customAudioDir = previousDir })

	request := newCustomAudioUploadRequest(t, "tts.wav", makeFloatWave([]float32{0.25}))
	request.RemoteAddr = "127.0.0.1:43120"
	recorder := httptest.NewRecorder()
	handleCustomAudioUpload(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response customAudioUploadResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(dir, response.SoundID+".wav"))
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(saved[20:22]) != 1 || binary.LittleEndian.Uint16(saved[34:36]) != 16 {
		t.Fatal("uploaded WAV was not normalized to PCM16")
	}
}

func TestResolveCustomAudioRejectsPathInput(t *testing.T) {
	if _, err := resolveCustomAudio(`..\outside.mp3`); err != errInvalidCustomSoundID {
		t.Fatalf("expected invalid id error, got %v", err)
	}
	if _, err := resolveCustomAudio(string(bytes.Repeat([]byte{'a'}, 64)) + ".mp3"); err != errInvalidCustomSoundID {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}

func newCustomAudioUploadRequest(t *testing.T, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart body: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/buff_sound/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func minimalWaveHeader() []byte {
	return []byte{'R', 'I', 'F', 'F', 4, 0, 0, 0, 'W', 'A', 'V', 'E'}
}

func makeFloatWave(samples []float32) []byte {
	dataSize := len(samples) * 4
	output := make([]byte, 44+dataSize)
	copy(output[0:4], "RIFF")
	binary.LittleEndian.PutUint32(output[4:8], uint32(len(output)-8))
	copy(output[8:12], "WAVE")
	copy(output[12:16], "fmt ")
	binary.LittleEndian.PutUint32(output[16:20], 16)
	binary.LittleEndian.PutUint16(output[20:22], 3)
	binary.LittleEndian.PutUint16(output[22:24], 1)
	binary.LittleEndian.PutUint32(output[24:28], 44100)
	binary.LittleEndian.PutUint32(output[28:32], 44100*4)
	binary.LittleEndian.PutUint16(output[32:34], 4)
	binary.LittleEndian.PutUint16(output[34:36], 32)
	copy(output[36:40], "data")
	binary.LittleEndian.PutUint32(output[40:44], uint32(dataSize))
	for index, sample := range samples {
		binary.LittleEndian.PutUint32(output[44+index*4:48+index*4], math.Float32bits(sample))
	}
	return output
}
