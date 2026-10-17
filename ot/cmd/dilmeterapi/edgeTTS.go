package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const edgeTTSVoice = "zh-CN-XiaoxiaoNeural"
const edgeTTSProbeScript = `import json, sys
result = {"pythonAvailable": True, "ready": False, "executable": sys.executable}
try:
 import edge_tts
 from importlib.metadata import version
 result.update(ready=True, edgeTtsVersion=version("edge-tts"))
except ImportError:
 pass
print(json.dumps(result))
`
const edgeTTSSynthesisScript = `import asyncio, json, sys
import edge_tts
request = json.loads(sys.stdin.buffer.read().decode("utf-8"))
asyncio.run(edge_tts.Communicate(text=request["text"], voice="zh-CN-XiaoxiaoNeural", rate="{:+d}%".format(request["rate"] * 10)).save(request["output"]))
`

type edgeTTSDependencyStatus struct {
	Ready           bool   `json:"ready"`
	PythonAvailable bool   `json:"pythonAvailable"`
	EdgeTTSVersion  string `json:"edgeTtsVersion,omitempty"`
	Source          string `json:"source,omitempty"`
	Message         string `json:"message"`
	ApplicationDir  string `json:"applicationDir"`
	AddonPresent    bool   `json:"addonPresent"`
}

func edgeTTSLocation() edgeTTSDependencyStatus {
	status := edgeTTSDependencyStatus{}
	if executable, err := edgeTTSApplicationExecutable(); err == nil && filepath.IsAbs(executable) {
		status.ApplicationDir = filepath.Dir(executable)
		info, err := os.Stat(filepath.Join(status.ApplicationDir, "addons", "edge-tts", "python", "python.exe"))
		status.AddonPresent = err == nil && !info.IsDir()
	}
	return status
}

type edgeTTSCommand struct {
	Executable string
	Arguments  []string
	Source     string
}

// Keep arguments separate; user text is passed as JSON on stdin, never as
// PowerShell, shell commands, Python source or a generated file name.
func (python edgeTTSCommand) command(ctx context.Context, script string) *exec.Cmd {
	args := append(append([]string{}, python.Arguments...), "-c", script)
	command := exec.CommandContext(ctx, python.Executable, args...)
	hideEdgeTTSCommand(command)
	return command
}

var edgeTTSApplicationExecutable = os.Executable

var edgeTTSCandidates = defaultEdgeTTSCandidates

func defaultEdgeTTSCandidates() []edgeTTSCommand {
	var candidates []edgeTTSCommand
	if executable, err := edgeTTSApplicationExecutable(); err == nil && filepath.IsAbs(executable) {
		// Resolve relative to the app, never the shortcut's working directory or PATH.
		candidates = append(candidates, edgeTTSCommand{
			Executable: filepath.Join(filepath.Dir(executable), "addons", "edge-tts", "python", "python.exe"),
			Source:     "addon",
		})
	}
	return append(candidates, edgeTTSCommand{Executable: "py", Arguments: []string{"-3"}, Source: "system"},
		edgeTTSCommand{Executable: "python", Source: "system"}, edgeTTSCommand{Executable: "python3", Source: "system"})
}

func detectEdgeTTS(ctx context.Context) (edgeTTSCommand, edgeTTSDependencyStatus) {
	status := edgeTTSLocation()
	for _, candidate := range edgeTTSCandidates() {
		if ctx.Err() != nil {
			break
		}
		path, err := exec.LookPath(candidate.Executable)
		if err != nil {
			continue
		}
		candidate.Executable = path
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		output, err := candidate.command(probeCtx, edgeTTSProbeScript).Output()
		cancel()
		var probe struct {
			edgeTTSDependencyStatus
			Executable string `json:"executable"`
		}
		if err != nil || json.Unmarshal(bytes.TrimSpace(output), &probe) != nil {
			continue
		}
		status.PythonAvailable = status.PythonAvailable || probe.PythonAvailable
		if probe.Ready && filepath.IsAbs(probe.Executable) {
			// Launch Python directly so cancellation stops synthesis, not just py.exe.
			candidate.Executable, candidate.Arguments = probe.Executable, nil
			probe.Source = candidate.Source
			probe.ApplicationDir, probe.AddonPresent = status.ApplicationDir, status.AddonPresent
			probe.Message = "已安装，可以制作晓晓语音"
			return candidate, probe.edgeTTSDependencyStatus
		}
	}
	status.Message = "未找到可用语音包，请确认是否已解压至下方软件文件夹"
	if status.AddonPresent {
		status.Message = "语音包安装检查未通过，请重新完整解压后点击自动监测"
	}
	return edgeTTSCommand{}, status
}

var synthesizeEdgeTTS = generateEdgeTTS

func handleEdgeTTS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if !isLoopbackRequest(r) {
		http.Error(w, "语音制作仅支持本机访问", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("check") == "location" {
			_ = json.NewEncoder(w).Encode(edgeTTSLocation())
			return
		}
		_, status := detectEdgeTTS(r.Context())
		_ = json.NewEncoder(w).Encode(status)
	case http.MethodPost:
		if r.Header.Get("X-Dilmeter-Action") != "generate-edge-tts" {
			http.Error(w, "missing speech generation action", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		var request localTTSRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "语音制作请求无效", http.StatusBadRequest)
			return
		}
		request.Text = sanitizeLocalTTSText(request.Text)
		if length := len([]rune(request.Text)); length == 0 || length > maxLocalTTSTextRunes {
			http.Error(w, "请输入 1–120 个字符的提醒文字", http.StatusBadRequest)
			return
		}
		if request.Rate < -5 || request.Rate > 5 {
			http.Error(w, "语速应在 -5 至 5 之间", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		data, err := synthesizeEdgeTTS(ctx, request.Text, request.Rate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		extension, valid := detectCustomAudioExtension(data)
		if !valid || extension != ".mp3" || len(data) > maxCustomAudioBytes {
			http.Error(w, "晓晓服务未返回有效 MP3，请稍后重试", http.StatusBadGateway)
			return
		}
		digest := sha256.Sum256(data)
		soundID := hex.EncodeToString(digest[:])
		if err := os.MkdirAll(customAudioDir(), 0700); err != nil {
			http.Error(w, "创建语音目录失败", http.StatusInternalServerError)
			return
		}
		if err := writeCustomAudioFile(filepath.Join(customAudioDir(), soundID+".mp3"), data); err != nil {
			http.Error(w, "保存晓晓语音失败", http.StatusInternalServerError)
			return
		}
		name := []rune(request.Text)
		if len(name) > 18 {
			name = append(name[:18], '…')
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(localTTSResponse{SoundID: soundID, DisplayName: "晓晓-" + string(name) + ".mp3", Voice: edgeTTSVoice})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func generateEdgeTTS(ctx context.Context, text string, rate int) ([]byte, error) {
	python, status := detectEdgeTTS(ctx)
	if !status.Ready {
		return nil, errors.New(status.Message)
	}
	directory, err := os.MkdirTemp("", "dilmeter-edge-tts-*")
	if err != nil {
		return nil, fmt.Errorf("创建语音临时目录失败：%w", err)
	}
	defer os.RemoveAll(directory)
	outputPath := filepath.Join(directory, "speech.mp3")
	input, _ := json.Marshal(map[string]any{"text": text, "rate": rate, "output": outputPath})
	command := python.command(ctx, edgeTTSSynthesisScript)
	command.Stdin = bytes.NewReader(input)
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("晓晓语音生成超时或已取消，请检查网络后重试")
		}
		return nil, errors.New("晓晓在线语音生成失败：请检查网络和系统时间，更新语音附加包或 edge-tts 后重试；也可切换 Windows 本地语音")
	}
	data, err := readLimitedTTSWave(outputPath) // size-bounded reader; format checked by handler
	if err != nil {
		return nil, fmt.Errorf("读取晓晓语音失败：%s", strings.TrimSpace(err.Error()))
	}
	return data, nil
}
