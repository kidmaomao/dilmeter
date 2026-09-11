package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const maxLocalTTSTextRunes = 120

type localTTSVoice struct {
	Name        string `json:"name"`
	Culture     string `json:"culture"`
	Gender      string `json:"gender"`
	Description string `json:"description"`
}

type localTTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
	Rate  int    `json:"rate"`
}

type localTTSResponse struct {
	SoundID     string `json:"soundId"`
	DisplayName string `json:"displayName"`
	Voice       string `json:"voice"`
}

func handleLocalTTS(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "local TTS is only available locally", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		voices, err := listPlatformTTSVoices(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("读取本地语音失败：%v", err), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"voices": voices, "offline": true})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		var request localTTSRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid local TTS request", http.StatusBadRequest)
			return
		}
		request.Text = sanitizeLocalTTSText(request.Text)
		request.Voice = sanitizeLocalTTSVoiceName(request.Voice)
		if request.Text == "" {
			http.Error(w, "请输入要朗读的文字", http.StatusBadRequest)
			return
		}
		if len([]rune(request.Text)) > maxLocalTTSTextRunes {
			http.Error(w, fmt.Sprintf("本地 TTS 最多支持 %d 个字符", maxLocalTTSTextRunes), http.StatusBadRequest)
			return
		}
		if request.Rate < -5 {
			request.Rate = -5
		}
		if request.Rate > 5 {
			request.Rate = 5
		}
		data, selectedVoice, err := synthesizePlatformTTS(r.Context(), request.Text, request.Voice, request.Rate)
		if err != nil {
			http.Error(w, fmt.Sprintf("生成本地 TTS 失败：%v", err), http.StatusInternalServerError)
			return
		}
		if len(data) == 0 || len(data) > maxCustomAudioBytes {
			http.Error(w, "生成的 TTS 音频大小异常", http.StatusInternalServerError)
			return
		}
		if extension, ok := detectCustomAudioExtension(data); !ok || extension != ".wav" {
			http.Error(w, "本地 TTS 未生成有效 WAV", http.StatusInternalServerError)
			return
		}
		data, err = normalizeWaveToPCM16(data)
		if err != nil {
			http.Error(w, fmt.Sprintf("本地 TTS 音频转换失败：%v", err), http.StatusInternalServerError)
			return
		}
		digest := sha256.Sum256(data)
		soundID := hex.EncodeToString(digest[:])
		if err := os.MkdirAll(customAudioDir(), 0700); err != nil {
			http.Error(w, "创建 TTS 音频目录失败", http.StatusInternalServerError)
			return
		}
		if err := writeCustomAudioFile(filepath.Join(customAudioDir(), soundID+".wav"), data); err != nil {
			http.Error(w, "保存 TTS 音频失败", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(localTTSResponse{
			SoundID:     soundID,
			DisplayName: localTTSDisplayName(request.Text),
			Voice:       selectedVoice,
		})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func sanitizeLocalTTSText(value string) string {
	value = strings.Map(func(char rune) rune {
		if char == '\r' || char == '\n' || char == '\t' {
			return ' '
		}
		if unicode.IsControl(char) {
			return -1
		}
		return char
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func sanitizeLocalTTSVoiceName(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 180 || strings.ContainsAny(value, "\r\n\x00") {
		return ""
	}
	return value
}

func localTTSDisplayName(text string) string {
	runes := []rune(text)
	if len(runes) > 18 {
		runes = append(runes[:18], '…')
	}
	return "本地TTS-" + string(runes) + ".wav"
}

func readLimitedTTSWave(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCustomAudioBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCustomAudioBytes {
		return nil, errors.New("生成的 TTS 超过 10MB")
	}
	return data, nil
}
