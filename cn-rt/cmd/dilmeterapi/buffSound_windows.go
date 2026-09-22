//go:build windows

package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmm                   = windows.NewLazySystemDLL("winmm.dll")
	procMCISendStringW      = winmm.NewProc("mciSendStringW")
	procPlaySoundW          = winmm.NewProc("PlaySoundW")
	mciCommandSender        = sendMCICommand
	mciCommandResultSender  = sendMCICommandResult
	buffSoundPlaybackMu     sync.Mutex
	bossSoundPlaybackMu     sync.Mutex
	buffSoundExtractedMu    sync.Mutex
	buffSoundExtractedPaths = map[string]string{}
)

type buffSoundRequest struct {
	Kind    string `json:"kind"`
	Volume  int    `json:"volume"`
	SoundID string `json:"soundId,omitempty"`
}

func handleBuffSound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request buffSoundRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid sound request", http.StatusBadRequest)
		return
	}
	if request.Volume < 0 {
		request.Volume = 0
	}
	if request.Volume > 100 {
		request.Volume = 100
	}
	if err := playBuffSound(strings.ToLower(strings.TrimSpace(request.Kind)), request.Volume, strings.TrimSpace(request.SoundID)); err != nil {
		logger.Println("buff sound playback failed:", err)
		status := http.StatusInternalServerError
		if errors.Is(err, errInvalidCustomSoundID) {
			status = http.StatusBadRequest
		} else if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func playBuffSound(kind string, volume int, customSoundID ...string) error {
	return playBuffSoundOnChannel(kind, volume, false, customSoundID...)
}

func playBossReminderSound(kind string, volume int, customSoundID ...string) error {
	return playBuffSoundOnChannel(kind, volume, true, customSoundID...)
}

func playBuffSoundOnChannel(kind string, volume int, bossChannel bool, customSoundID ...string) error {
	if traySoundMuted.Load() || volume <= 0 {
		return nil
	}
	filename := ""
	path := ""
	switch kind {
	case "electronic":
		filename = "buff-ending-electronic.wav"
	case "healer-angel":
		filename = "healer-angel.wav"
	case "healer-music":
		filename = "healer-music-xiaoxiao.mp3"
	case "healer-health":
		filename = "healer-health-xiaoxiao.mp3"
	case "healer-buff":
		filename = "healer-buff-xiaoxiao.mp3"
	case "skill-ready":
		filename = "skill-ready-pop.mp3"
	case "voice":
		filename = "buff-ending-xiaoxiao.mp3"
	case "boss-miel-orb":
		filename = "boss-miel-orb.wav"
	case "boss-miel-laser":
		filename = "boss-miel-laser.wav"
	case "custom":
		soundID := ""
		if len(customSoundID) > 0 {
			soundID = customSoundID[0]
		}
		var err error
		path, err = resolveCustomAudio(soundID)
		if err != nil {
			return err
		}
		filename = filepath.Base(path)
	default:
		return fmt.Errorf("unsupported sound kind")
	}
	playbackMu := &buffSoundPlaybackMu
	alias := "dilmeter_buff_alert"
	if bossChannel {
		playbackMu = &bossSoundPlaybackMu
		alias = "dilmeter_boss_alert"
	}
	playbackMu.Lock()
	defer playbackMu.Unlock()
	// MCI device aliases are scoped to the Windows thread that opened them.
	// A Go goroutine may otherwise resume on another OS thread between open,
	// status and play, which makes the alias disappear and returns MCI error
	// 263. Keep the entire native playback transaction on one OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if path == "" {
		var err error
		path, err = extractBuffSound(filename, volume)
		if err != nil {
			return err
		}
	}
	if kind == "electronic" {
		value, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		const sndNodefault = 0x0002
		const sndFilename = 0x00020000
		if result, _, _ := procPlaySoundW.Call(uintptr(unsafe.Pointer(value)), 0, sndFilename|sndNodefault); result == 0 {
			return fmt.Errorf("PlaySound failed")
		}
		return nil
	}
	isMPEG := strings.EqualFold(filepath.Ext(filename), ".mp3")
	openCommand := fmt.Sprintf("open \"%s\" alias %s", strings.ReplaceAll(path, "\"", ""), alias)
	if isMPEG {
		openCommand = fmt.Sprintf("open \"%s\" type mpegvideo alias %s", strings.ReplaceAll(path, "\"", ""), alias)
	}
	if err := playMCIFileAlias(openCommand, isMPEG, kind, volume, alias); err != nil {
		return err
	}
	return nil
}

// playMCIFile performs close -> open -> configure -> play as one transaction.
// If a device/power transition invalidates the alias (263), the entire
// transaction is repeated once; repeating only the play command cannot recover
// an alias that the media driver has discarded.
func playMCIFile(openCommand string, isMPEG bool, kind string, volume int) error {
	return playMCIFileAlias(openCommand, isMPEG, kind, volume, "dilmeter_buff_alert")
}

func playMCIFileAlias(openCommand string, isMPEG bool, kind string, volume int, alias string) error {
	for attempt := 0; attempt < 2; attempt++ {
		_ = mciCommandSender("close " + alias)
		if attempt > 0 {
			time.Sleep(25 * time.Millisecond)
			logger.Printf("alert audio MCI device reset on thread %d; reopening", windows.GetCurrentThreadId())
		}
		if err := mciCommandSender(openCommand); err != nil {
			if attempt == 0 && isMCIErrorCode(err, 263) {
				continue
			}
			return fmt.Errorf("open alert audio: %w", err)
		}
		// Waiting for completion is deliberate. Some systems accept an asynchronous
		// MCI command but never route the sound before the request finishes.
		_ = mciCommandSender(fmt.Sprintf("setaudio %s volume to %d", alias, volume*10))
		err := playOpenedMCIAlias(isMPEG, kind, alias)
		_ = mciCommandSender("close " + alias)
		if attempt == 0 && isMCIErrorCode(err, 263) {
			continue
		}
		return err
	}
	return fmt.Errorf("play alert audio: MCI device remained unavailable")
}

func playOpenedMCI(isMPEG bool, kind string) error {
	return playOpenedMCIAlias(isMPEG, kind, "dilmeter_buff_alert")
}

func playOpenedMCIAlias(isMPEG bool, kind string, alias string) error {
	if isMPEG {
		// The legacy MPEG MCI driver rejects the WAIT flag on some Windows 11
		// systems. Start playback asynchronously, then keep the device open for
		// the reported media length so it cannot be closed before audio is routed.
		_ = mciCommandSender("set " + alias + " time format milliseconds")
		lengthText, _ := mciCommandResultSender("status " + alias + " length")
		lengthMs, _ := strconv.Atoi(strings.TrimSpace(lengthText))
		if lengthMs < 300 || lengthMs > 30000 {
			lengthMs = 1500
			if kind == "voice" {
				lengthMs = 3000
			}
		}
		if err := mciCommandSender("play " + alias + " from 0"); err != nil {
			return fmt.Errorf("play alert audio: %w", err)
		}
		time.Sleep(time.Duration(lengthMs+150) * time.Millisecond)
		return nil
	}
	if err := mciCommandSender("play " + alias + " from 0 wait"); err != nil {
		return fmt.Errorf("play alert audio: %w", err)
	}
	return nil
}

func isMCIErrorCode(err error, code uintptr) bool {
	var mciErr *mciCommandError
	return errors.As(err, &mciErr) && mciErr.Code == code
}

func extractBuffSound(filename string, volume int) (string, error) {
	cacheKey := fmt.Sprintf("%s:%d", filename, volume)
	buffSoundExtractedMu.Lock()
	if cached := buffSoundExtractedPaths[cacheKey]; cached != "" {
		if _, err := os.Stat(cached); err == nil {
			buffSoundExtractedMu.Unlock()
			return cached, nil
		}
	}
	buffSoundExtractedMu.Unlock()
	data, err := staticFiles.ReadFile(filepath.ToSlash(filepath.Join(embeddedStaticDir, "audio", filename)))
	if err != nil {
		return "", fmt.Errorf("read embedded alert audio: %w", err)
	}
	if strings.HasSuffix(strings.ToLower(filename), ".wav") && volume < 100 {
		data, err = scalePCM16Wave(data, volume)
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(appDataDir(), "audio-cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create alert audio cache: %w", err)
	}
	extension := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, extension)
	path := filepath.Join(dir, fmt.Sprintf("%s-v%d%s", base, volume, extension))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write alert audio cache: %w", err)
	}
	buffSoundExtractedMu.Lock()
	buffSoundExtractedPaths[cacheKey] = path
	buffSoundExtractedMu.Unlock()
	return path, nil
}

func scalePCM16Wave(data []byte, volume int) ([]byte, error) {
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("unsupported WAV file")
	}
	result := append([]byte(nil), data...)
	formatPCM16 := false
	dataStart, dataEnd := 0, 0
	for offset := 12; offset+8 <= len(result); {
		chunkSize := int(binary.LittleEndian.Uint32(result[offset+4 : offset+8]))
		start := offset + 8
		end := start + chunkSize
		if end > len(result) {
			return nil, fmt.Errorf("invalid WAV chunk")
		}
		switch string(result[offset : offset+4]) {
		case "fmt ":
			if chunkSize >= 16 {
				formatPCM16 = binary.LittleEndian.Uint16(result[start:start+2]) == 1 &&
					binary.LittleEndian.Uint16(result[start+14:start+16]) == 16
			}
		case "data":
			dataStart, dataEnd = start, end
		}
		offset = end + chunkSize%2
	}
	if !formatPCM16 || dataStart == 0 {
		return nil, fmt.Errorf("alert WAV must be 16-bit PCM")
	}
	for offset := dataStart; offset+1 < dataEnd; offset += 2 {
		sample := int16(binary.LittleEndian.Uint16(result[offset : offset+2]))
		scaled := int32(sample) * int32(volume) / 100
		binary.LittleEndian.PutUint16(result[offset:offset+2], uint16(int16(scaled)))
	}
	return result, nil
}

func sendMCICommand(command string) error {
	_, err := sendMCICommandResult(command)
	return err
}

func sendMCICommandResult(command string) (string, error) {
	value, err := syscall.UTF16PtrFromString(command)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, 256)
	result, _, _ := procMCISendStringW.Call(
		uintptr(unsafe.Pointer(value)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		0,
	)
	if result != 0 {
		return "", &mciCommandError{Code: result, Command: mciCommandVerb(command)}
	}
	return windows.UTF16ToString(buffer), nil
}

type mciCommandError struct {
	Code    uintptr
	Command string
}

func (e *mciCommandError) Error() string {
	return fmt.Sprintf("MCI error %d (%s)", e.Code, e.Command)
}

func mciCommandVerb(command string) string {
	if fields := strings.Fields(command); len(fields) > 0 {
		return fields[0]
	}
	return "command"
}
