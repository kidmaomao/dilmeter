package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"gitlab.com/prilus/mabidilmeter/lib/util"
	"golang.org/x/net/websocket"
)

const (
	_port             = 8030
	embeddedStaticDir = "static_v130_release"
)

var _logDir = filepath.Join(appDataDir(), "logs")

//go:generate ./gen_static.sh
//go:embed static_v130_release
var staticFiles embed.FS

var logger = util.NewLogger("dilmeterapi")
var packetLogFilename = ""
var BuildVariant = "release"
var AppName = "DilmeterCN"
var AppVersion = "1.4.1"
var resourcePackSessionVersion = time.Now().Unix()

func main() {
	if runUpdateHelperMode() {
		return
	}
	// main ctx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := util.LogInit(filepath.Join(_logDir, fmt.Sprintf("log_%v.txt", constants.SERVER_START_AT_STR))); err != nil {
		// warning...
		logger.Println("LogInit failed:", err)
	}

	logger.Printf("* dilmatulgi build time: %s", BuildTime)
	if executable, err := os.Executable(); err == nil {
		logger.Printf(
			"runtime identity: version=%s variant=%s pid=%d exe=%q data=%q",
			AppVersion, BuildVariant, os.Getpid(), filepath.Clean(executable), filepath.Clean(appDataDir()),
		)
	}

	main2(ctx)
}

func run(ctx context.Context, cfg config) (*packet.GameServerPacketReader, *eventPublisher) {
	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:    ctx,
		LogDir: _logDir,
	})
	if err != nil {
		messagebox(fmt.Sprintf("NewGameServerPacketReader failed: %v", err))
		logger.Fatalln("NewGameServerPacketReader failed:", err)
	}

	pub := newEventPublisher(ctx, r)
	startNativeReminderRuntime(ctx, pub)

	// packet writer (for debug)
	go func() {
		ch := make(chan []event.IEvent, 10000)
		defer close(ch)

		pub.addClient(ctx, ch)
		if err := startPacketWriter(ctx, ch); err != nil {
			logger.Println("startPacketWriter failed:", err)
			return
		}
	}()

	startWebsocketServer(ctx, cfg, func(ws *websocket.Conn) {
		logger.Printf("Client connected from %s", ws.RemoteAddr())

		wsCtx, wsCtxCancel := context.WithCancel(ws.Request().Context())

		// 생각보다 websocket이 send queue 비워지는게 느리다
		ch := make(chan []event.IEvent, 10000)
		defer wsCtxCancel()
		defer close(ch)

		go pub.addClient(wsCtx, ch)

		packetReceiveLoop := func() {
			for {
				select {
				case <-wsCtx.Done():
					logger.Printf("Client disconnected from %s", ws.RemoteAddr())
					return

				default:
					_ = 1
				}

				var event string
				err := websocket.JSON.Receive(ws, &event)
				if err != nil {
					logger.Printf("Receive failed: %s; closing connection...", err.Error())
					if err = ws.Close(); err != nil {
						logger.Println("Error closing connection:", err.Error())
					}

					wsCtxCancel()
					break
				} else {
					// discard...
					logger.Println("Received:", event)
				}
			}
		}

		go packetReceiveLoop()

		for {
			select {
			case <-wsCtx.Done():
				logger.Printf("Client disconnected from %s", ws.RemoteAddr())
				return

			case events := <-ch:
				err := websocket.JSON.Send(ws, events)
				if err != nil {
					logger.Printf("Can't send: %s", err.Error())
					return
				}
			}
		}
	})

	return r, pub
}

func startWebsocketServer(ctx context.Context, cfg config, newClientCb func(*websocket.Conn)) {
	remote, err := url.Parse("https://mabires.pril.cc")
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.ModifyResponse = func(r *http.Response) error {
		r.Header.Set("Access-Control-Allow-Origin", "*")
		return nil
	}
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.URL.Path = r.URL.Path[4:]
		r.Host = remote.Host
	}

	var staticFS = fs.FS(staticFiles)
	htmlContent, err := fs.Sub(staticFS, embeddedStaticDir)
	if err != nil {
		logger.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", websocket.Handler(newClientCb))
	mux.HandleFunc("/api/packet_log", httpHandlerPacketLog)
	mux.HandleFunc("/api/battle_records", handleBattleRecords)
	mux.HandleFunc("/api/log_cleanup", handleLogCleanup)
	mux.HandleFunc("/api/dps_recording", handleDPSRecording)
	mux.HandleFunc("/api/build_time", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(BuildTime))
	})
	mux.HandleFunc("/api/app_info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":         AppName,
			"version":      AppVersion,
			"buildVariant": BuildVariant,
			"buildTime":    BuildTime,
		})
	})
	mux.HandleFunc("/api/activate", handleAppActivate)
	mux.HandleFunc("/api/update", handleUpdate)
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(getAppStatus()); err != nil {
			logger.Println("status response failed:", err)
		}
	})
	mux.HandleFunc("/api/game_server", handleGameServer)
	mux.HandleFunc("/api/buff_overlay", handleBuffOverlay)
	mux.HandleFunc("/api/buff_overlay/drag", handleBuffOverlayDrag)
	mux.HandleFunc("/api/buff_overlay/position", handleBuffOverlayPosition)
	mux.HandleFunc("/api/buff_overlay/reset-position", handleBuffOverlayResetPosition)
	mux.HandleFunc("/api/buff_overlay/state", handleBuffOverlayState)
	mux.HandleFunc("/api/debuff_overlay", handleDebuffOverlay)
	mux.HandleFunc("/api/debuff_overlay/position", handleDebuffOverlayPosition)
	mux.HandleFunc("/api/debuff_overlay/state", handleDebuffOverlayState)
	mux.HandleFunc("/api/skill_overlay", handleSkillOverlay)
	mux.HandleFunc("/api/skill_overlay/position", handleSkillOverlayPosition)
	mux.HandleFunc("/api/skill_overlay/state", handleSkillOverlayState)
	mux.HandleFunc("/api/skill_bar", handleSkillBar)
	mux.HandleFunc("/api/buff_sound", handleBuffSound)
	mux.HandleFunc("/api/reminder_runtime/settings", handleNativeReminderSettings)
	mux.HandleFunc("/api/reminder_overlay/drag_state", handleNativeReminderDragState)
	mux.HandleFunc("/api/buff_sound/upload", handleCustomAudioUpload)
	mux.HandleFunc("/api/local_tts", handleLocalTTS)
	mux.Handle("/res/", localFirstResourceHandler(htmlContent, proxy))
	mux.Handle("/local-res/", localResourcePackHandler(htmlContent))

	mux.Handle("/", http.FileServer(http.FS(htmlContent)))

	bindAddress := "127.0.0.1"
	if cfg.AllowRemote {
		bindAddress = "0.0.0.0"
	}

	address := fmt.Sprintf("%s:%d", bindAddress, _port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		messagebox("Dilmeter 无法启动：本机端口 8030 已被占用。\n\n如果任务栏右下角已有 Dilmeter 图标，请直接打开该实例；否则请关闭占用端口的程序后重试。")
		logger.Fatalln("HTTP listen failed:", err)
	}

	server := &http.Server{Handler: mux}
	logger.Printf("Server listening on %s", address)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			messagebox(fmt.Sprintf("HTTP server failed: %v", err))
			logger.Println("HTTP server failed:", err)
		}
	}()
}

// localResourcePackHandler keeps CN/RT builds pinned to their bundled CN data.
// The OT build may override any file below /local-res/ with a matching file in
// data/resource-pack, allowing a Prilus region pack to be installed without
// replacing or repacking the executable.
func localResourcePackHandler(localFS fs.FS) http.Handler {
	embedded := http.FileServer(http.FS(localFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AppName == "DilmeterOT" {
			relativeURLPath := strings.TrimPrefix(r.URL.Path, "/local-res/")
			root := filepath.Join(appDataDir(), "resource-pack")
			if regionData, err := os.ReadFile(filepath.Join(root, "active-region.txt")); err == nil {
				region := strings.ToLower(strings.TrimSpace(string(regionData)))
				relativeURLPath = resourcePackRelativeURLPath(relativeURLPath, region)
			}
			cleanRelativePath := filepath.Clean(filepath.FromSlash(relativeURLPath))
			candidate := filepath.Join(root, cleanRelativePath)
			if relative, err := filepath.Rel(root, candidate); err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					w.Header().Set("Cache-Control", "no-store")
					if strings.HasSuffix(strings.ToLower(filepath.Base(candidate)), "_resourceversion.json") && info.Size() <= 64*1024 {
						var version struct {
							CreatedAt int64 `json:"CreatedAt"`
						}
						if data, err := os.ReadFile(candidate); err == nil && json.Unmarshal(data, &version) == nil {
							version.CreatedAt = max(version.CreatedAt, resourcePackSessionVersion)
							w.Header().Set("Content-Type", "application/json; charset=utf-8")
							_ = json.NewEncoder(w).Encode(version)
							return
						}
					}
					http.ServeFile(w, r, candidate)
					return
				}
			}
		}
		embedded.ServeHTTP(w, r)
	})
}

func resourcePackRelativeURLPath(requestPath, region string) string {
	if !validResourcePackRegion(region) {
		return requestPath
	}
	requestPath = strings.Replace(requestPath, "resourcedata/cn/cn_", "resourcedata/"+region+"/"+region+"_", 1)
	return strings.Replace(requestPath, "resourceversion/cn/cn_", "resourceversion/"+region+"/"+region+"_", 1)
}

func validResourcePackRegion(region string) bool {
	switch region {
	case "kr", "krt", "cn", "jp", "tw", "us":
		return true
	}
	return false
}

// localFirstResourceHandler keeps CN condition and skill icons available
// offline. Resources not present in the bundle continue to use the upstream
// service for forward compatibility.
func localFirstResourceHandler(localFS fs.FS, upstream http.Handler) http.Handler {
	localFiles := http.FileServer(http.FS(localFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localPath, ok := bundledResourceIconPath(r.URL.Path)
		if ok {
			if _, err := fs.Stat(localFS, strings.TrimPrefix(localPath, "/")); err == nil {
				localRequest := r.Clone(r.Context())
				localURL := *r.URL
				localURL.Path = localPath
				localRequest.URL = &localURL
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				localFiles.ServeHTTP(w, localRequest)
				return
			}
		}
		upstream.ServeHTTP(w, r)
	})
}

func bundledResourceIconPath(requestPath string) (string, bool) {
	types := []struct {
		prefix    string
		localRoot string
	}{
		{"/res/characterconditionimage/cn/", "/condition-icons/"},
		{"/res/skillimage/cn/", "/skill-icons/"},
	}
	for _, iconType := range types {
		if !strings.HasPrefix(requestPath, iconType.prefix) {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(requestPath, iconType.prefix), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] != parts[0]+".png" {
			return "", false
		}
		for _, char := range parts[0] {
			if char < '0' || char > '9' {
				return "", false
			}
		}
		return iconType.localRoot + parts[0] + ".png", true
	}
	return "", false
}

func startPacketWriter(ctx context.Context, ch <-chan []event.IEvent) error {
	logPath := _logDir
	if err := os.MkdirAll(logPath, os.ModePerm); err != nil {
		logger.Println("Failed to create log directory:", err)
		return err
	}

	packetLogFilename = fmt.Sprintf("packet_log_%v.ndjson", constants.SERVER_START_AT_STR)
	packetLogFilePath := filepath.Join(logPath, packetLogFilename)

	fd, err := os.OpenFile(packetLogFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logger.Println("packetWriter open file failed:", err)
		return err
	}
	defer fd.Close()

	flushTicker := time.NewTicker(5 * time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case events := <-ch:
			for _, e := range events {
				if !shouldPersistEvent(e) {
					continue
				}
				if eventId := e.GetEventId(); eventId < 0 {
					// system message는 굳이 저장할 필요 없을듯
					continue
				}

				b, err := json.Marshal(e)
				if err != nil {
					// ?
					continue
				}

				b = append(b, '\n')

				_, err = fd.Write(b)
				if err != nil {
					logger.Println("packetWriter write failed:", err)
					return err
				}
			}

		case <-flushTicker.C:
			err := fd.Sync()
			if err != nil {
				// ignore
				_ = 1
			}
		}
	}
}
