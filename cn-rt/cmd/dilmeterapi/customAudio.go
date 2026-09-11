package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxCustomAudioBytes = 10 << 20
	// Leave room for the multipart boundary and file metadata while still
	// bounding the entire request body.
	maxCustomUploadBytes = maxCustomAudioBytes + (1 << 20)
)

var (
	errInvalidCustomSoundID = errors.New("invalid custom sound id")
	customAudioDir          = func() string { return filepath.Join(appDataDir(), "custom-audio") }
)

type customAudioUploadResponse struct {
	SoundID     string `json:"soundId"`
	DisplayName string `json:"displayName"`
}

func handleCustomAudioUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRequest(r) {
		http.Error(w, "custom audio upload is only available locally", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCustomUploadBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "audio file is too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid multipart upload", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "audio file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxCustomAudioBytes+1))
	if err != nil {
		http.Error(w, "read audio file failed", http.StatusBadRequest)
		return
	}
	if len(data) > maxCustomAudioBytes {
		http.Error(w, "audio file is too large", http.StatusRequestEntityTooLarge)
		return
	}
	extension, ok := detectCustomAudioExtension(data)
	if !ok {
		http.Error(w, "only MP3 and WAV audio are supported", http.StatusUnsupportedMediaType)
		return
	}
	if extension == ".wav" {
		data, err = normalizeWaveToPCM16(data)
		if err != nil {
			http.Error(w, "unsupported or invalid WAV audio: "+err.Error(), http.StatusUnsupportedMediaType)
			return
		}
	}

	digest := sha256.Sum256(data)
	soundID := hex.EncodeToString(digest[:])
	dir := customAudioDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		logger.Println("custom audio directory failed:", err)
		http.Error(w, "save custom audio failed", http.StatusInternalServerError)
		return
	}
	destination := filepath.Join(dir, soundID+extension)
	if err := writeCustomAudioFile(destination, data); err != nil {
		logger.Println("custom audio save failed:", err)
		http.Error(w, "save custom audio failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(customAudioUploadResponse{
		SoundID:     soundID,
		DisplayName: safeAudioDisplayName(header.Filename),
	})
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(r.RemoteAddr), "[]")
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func detectCustomAudioExtension(data []byte) (string, bool) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return ".wav", true
	}
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		return ".mp3", true
	}
	if len(data) >= 4 && data[0] == 0xff && data[1]&0xe0 == 0xe0 {
		// Reject reserved MPEG version/layer values and invalid bitrate indices.
		if data[1]&0x18 != 0x08 && data[1]&0x06 != 0 && data[2]&0xf0 != 0 && data[2]&0xf0 != 0xf0 {
			return ".mp3", true
		}
	}
	return "", false
}

func safeAudioDisplayName(filename string) string {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	if name == "." || name == "/" || name == "" {
		return "自定义音效"
	}
	return name
}

func writeCustomAudioFile(destination string, data []byte) error {
	if info, err := os.Lstat(destination); err == nil && info.Mode().IsRegular() {
		return nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".audio-upload-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		if info, statErr := os.Lstat(destination); statErr == nil && info.Mode().IsRegular() {
			return nil
		}
		return err
	}
	return nil
}

func resolveCustomAudio(soundID string) (string, error) {
	if !isValidCustomSoundID(soundID) {
		return "", errInvalidCustomSoundID
	}
	for _, extension := range []string{".mp3", ".wav"} {
		candidate := filepath.Join(customAudioDir(), soundID+extension)
		info, err := os.Lstat(candidate)
		if err == nil && info.Mode().IsRegular() {
			if extension == ".wav" {
				data, readErr := os.ReadFile(candidate)
				if readErr != nil {
					return "", fmt.Errorf("read custom WAV: %w", readErr)
				}
				normalized, normalizeErr := normalizeWaveToPCM16(data)
				if normalizeErr != nil {
					return "", fmt.Errorf("normalize custom WAV: %w", normalizeErr)
				}
				if !bytes.Equal(data, normalized) {
					if writeErr := os.WriteFile(candidate, normalized, 0600); writeErr != nil {
						return "", fmt.Errorf("save normalized custom WAV: %w", writeErr)
					}
				}
			}
			return candidate, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read custom sound: %w", err)
		}
	}
	return "", os.ErrNotExist
}

func isValidCustomSoundID(soundID string) bool {
	if len(soundID) != sha256.Size*2 {
		return false
	}
	for _, char := range soundID {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
