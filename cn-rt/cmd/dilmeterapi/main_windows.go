//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	webview2 "github.com/jchv/go-webview2"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"golang.org/x/sys/windows"
)

func main2(ctx context.Context) {
	// WebView2 and the Win32 message loop must be created on the main OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instanceHandle, alreadyRunning, instanceErr := acquireDilmeterSingleInstance()
	if instanceHandle != 0 {
		defer windows.CloseHandle(instanceHandle)
	}
	if alreadyRunning {
		if !tryActivateExistingDilmeter() {
			messagebox("Dilmeter 已在运行或正在启动，请稍候再看任务栏右下角。")
		}
		return
	}
	if instanceErr != nil {
		// Different elevation levels can prevent access to an existing named
		// mutex. The local-port preflight below remains a safe fallback.
		logger.Println("single-instance mutex unavailable:", instanceErr)
	}
	resetMainWindowActivationState()

	cfg := loadConfig()
	diagnosticMode := strings.EqualFold(BuildVariant, "diagnostic")
	if diagnosticMode {
		logger.Println("running diagnostic capture build")
	}
	cfg.GameServer = normalizeGameServerID(cfg.GameServer)
	selectedServer, err := applyGameServer(cfg.GameServer)
	if err != nil {
		messagebox(fmt.Sprintf("无法载入服务器设置：%v", err))
		return
	}
	logger.Printf("selected game server: %s (%s)", selectedServer.Name, selectedServer.Network)
	// The desktop edition never exposes raw combat logs on the LAN. Sharing is
	// done through an explicitly generated image instead.
	if cfg.AllowRemote {
		logger.Println("desktop mode disables allow_remote for safety")
		cfg.AllowRemote = false
	}
	listenAddress := fmt.Sprintf("127.0.0.1:%d", _port)
	if err := checkTCPListenAvailable(listenAddress); err != nil {
		if tryActivateExistingDilmeter() {
			return
		}
		logger.Printf("startup port preflight failed for %s: %v", listenAddress, err)
		messagebox("Dilmeter 无法启动：本机端口 8030 已被其他程序占用。\n\n如果任务栏右下角已有 Dilmeter 图标，请直接打开该实例；否则请关闭占用 8030 端口的程序后重试。")
		return
	}

	versionChanged := consumeVersionLaunchMarker()
	if versionChanged {
		logger.Printf("startup stabilization: version_changed=%v", versionChanged)
		// On the first manual launch after an update, allow any WebView2 process
		// left by the previous version to release the shared profile first.
		if err := waitForWebViewProfileRelease(filepath.Join(appDataDir(), "webview2"), 15*time.Second); err != nil {
			logger.Println("startup WebView2 profile wait timed out; continuing startup:", err)
		}
	}

	if cfg.Key != "" && cfg.IV != "" {
		if err := packet.InitKoreaDamageBlock(cfg.Key, cfg.IV); err != nil {
			logger.Println("config: InitKoreaDamageBlock failed:", err)
		} else {
			logger.Println("config: loaded key/iv from config")
		}
	}

	reader, _ := run(ctx, cfg)
	if replayFile := commandLineReplayFile(); replayFile != "" {
		if err := reader.OpenFile(replayFile); err != nil {
			setAppStatus(appStatus{State: statusError, Message: fmt.Sprintf("无法打开抓包文件：%v", err)})
		} else {
			setAppStatus(appStatus{State: statusReplay, Message: "正在回放抓包文件。", Capturing: true})
		}
	} else {
		startCaptureManager(ctx, reader, cfg.AcceleratorMode, diagnosticMode)
	}

	dataPath := filepath.Join(appDataDir(), "webview2")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		logger.Println("create WebView2 data directory failed:", err)
	}
	// Reminder scheduling must continue when the main WebView is minimized or
	// covered by the fullscreen game. WebView2 otherwise throttles JavaScript
	// timers and custom sounds may be requested only after their alert window.
	ensureBackgroundWebViewTimers()

	windowTitle := fmt.Sprintf("%s v%s", AppName, AppVersion)
	if diagnosticMode {
		windowTitle += "-diagnostic"
	} else if !strings.EqualFold(BuildVariant, "release") {
		windowTitle += " · " + BuildVariant
	}
	view := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:                  false,
		DataPath:               dataPath,
		AutoFocus:              true,
		KeepAliveWhenMinimized: true,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  1440,
			Height: 900,
			Center: true,
			IconId: 2,
		},
	})
	if view == nil {
		messagebox("无法创建桌面窗口。请安装 Microsoft Edge WebView2 Runtime 后重试。")
		return
	}
	defer view.Destroy()
	mainHWND := uintptr(view.Window())
	setMainWindowHandle(mainHWND)
	defer setMainWindowHandle(0)
	procShowWindow.Call(mainHWND, swHide)
	var showMainWindowOnce sync.Once
	showMainWindow := func(reason string) {
		showMainWindowOnce.Do(func() {
			queued := markMainWindowReadyForActivation()
			view.Dispatch(func() {
				logger.Printf("main window shown: %s (activation_queued=%v)", reason, queued)
				showAndActivateMainWindow(mainHWND)
			})
		})
	}
	var documentReady atomic.Bool
	var recoveryRequested atomic.Bool
	var readyFallback *time.Timer
	if err := view.Bind("__dilmeterMainReady", func() {
		if documentReady.Swap(true) {
			return
		}
		if readyFallback != nil {
			readyFallback.Stop()
		}
		showMainWindow("document ready")
	}); err != nil {
		logger.Println("bind main-window ready callback failed:", err)
	}
	// The Vue entrypoint calls __dilmeterMainReady only after mount("#app")
	// returns. Do not infer readiness from DOMContentLoaded or incidental shell
	// content here: both can precede module execution and reproduce the blank
	// post-update window on slower machines.
	readyFallback = time.AfterFunc(8*time.Second, func() {
		if documentReady.Load() {
			return
		}
		if recoveryRequested.CompareAndSwap(false, true) {
			logger.Println("main-window ready callback timed out; retrying navigation without cache")
			view.Dispatch(func() {
				view.Navigate(fmt.Sprintf("http://127.0.0.1:%d/?startup-recovery=%d", _port, time.Now().UnixNano()))
			})
		}
		time.AfterFunc(8*time.Second, func() {
			if documentReady.Load() {
				return
			}
			logger.Println("main-window recovery failed; showing diagnostic error")
			view.Dispatch(func() {
				view.SetHtml(`<html><body style="background:#111;color:#fff;font:16px 'Microsoft YaHei',sans-serif;padding:28px"><h2>Dilmeter 界面加载失败</h2><p>本地服务已经启动，但界面未能加载。请关闭软件后重新打开；若仍失败，请附上 data/logs 中最新日志。</p></body></html>`)
				showMainWindow("startup recovery failed")
			})
		})
	})
	defer readyFallback.Stop()
	if err := initializeTray(uintptr(view.Window())); err != nil {
		logger.Println("initialize tray failed:", err)
	} else {
		defer shutdownTray()
	}
	// A single dedicated low-level mouse router serves reminder dragging and the
	// SkillBar click barrier. Keeping one hook avoids ordering gaps where one
	// global hook consumes an event before the other can pair its button-up.
	if err := initializeNativeReminderOverlayInput(); err != nil {
		logger.Println("initialize reminder overlay drag input failed:", err)
	} else {
		defer shutdownNativeReminderOverlayInput()
	}
	if err := initializeSkillBar(cfg); err != nil {
		logger.Println("initialize native skill bar failed:", err)
	} else {
		defer shutdownSkillBar()
	}

	// Reminder visuals use the same transparent WebView windows as v1.3.8.
	// The HTML/CSS renderer is important here: ready bursts, glows, cooldown
	// sweeps and warning pulses cannot be reproduced faithfully by the small
	// native GDI fallback renderer.
	buffOverlay := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:       false,
		DataPath:    dataPath,
		AutoFocus:   false,
		Transparent: true,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle + " Buff Overlay",
			Width:  520,
			Height: 100,
			Center: false,
			IconId: 2,
		},
	})
	if buffOverlay != nil {
		defer buffOverlay.Destroy()
		initializeBuffOverlay(buffOverlay, cfg)
		defer shutdownBuffOverlay()
		buffOverlay.Navigate(fmt.Sprintf("http://127.0.0.1:%d/?buffOverlay=1", _port))
	}

	debuffOverlay := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:       false,
		DataPath:    dataPath,
		AutoFocus:   false,
		Transparent: true,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle + " Debuff Overlay",
			Width:  520,
			Height: 100,
			Center: false,
			IconId: 2,
		},
	})
	if debuffOverlay != nil {
		defer debuffOverlay.Destroy()
		initializeDebuffOverlay(debuffOverlay, cfg)
		defer shutdownDebuffOverlay()
		debuffOverlay.Navigate(fmt.Sprintf("http://127.0.0.1:%d/?debuffOverlay=1", _port))
	}

	skillOverlay := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:       false,
		DataPath:    dataPath,
		AutoFocus:   false,
		Transparent: true,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle + " Skill Overlay",
			Width:  skillOverlayDefaultWidth,
			Height: skillOverlayDefaultHeight,
			Center: false,
			IconId: 2,
		},
	})
	if skillOverlay != nil {
		defer func() {
			skillOverlayClosing.Store(true)
			skillOverlayActive.Store(false)
			skillOverlay.Destroy()
		}()
		initializeSkillOverlay(skillOverlay, cfg)
		skillOverlay.Navigate(fmt.Sprintf("http://127.0.0.1:%d/?skillOverlay=1", _port))
	}
	stopBackgroundTicks := startNativeWebViewTicks(ctx, view, buffOverlay, debuffOverlay, skillOverlay)
	defer stopBackgroundTicks()

	view.Navigate(fmt.Sprintf("http://127.0.0.1:%d", _port))
	view.Run()
}

func ensureBackgroundWebViewTimers() {
	const required = "--disable-background-timer-throttling --disable-renderer-backgrounding --disable-backgrounding-occluded-windows --disable-features=IntensiveWakeUpThrottling"
	existing := strings.TrimSpace(os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"))
	for _, argument := range strings.Fields(required) {
		if !strings.Contains(existing, argument) {
			existing = strings.TrimSpace(existing + " " + argument)
		}
	}
	if err := os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", existing); err != nil {
		logger.Println("enable background WebView timers failed:", err)
	}
}

// startNativeWebViewTicks is a host-side clock for reminder scheduling and
// overlay countdowns. Chromium begins intensive JavaScript timer throttling
// after a page has been hidden for several minutes; a Go ticker is independent
// of that lifecycle and dispatching script explicitly wakes each controller.
func startNativeWebViewTicks(ctx context.Context, views ...webview2.WebView) func() {
	if len(views) == 0 || views[0] == nil {
		return func() {}
	}
	evaluators := make([]func(string), 0, len(views))
	for _, current := range views {
		if current == nil {
			continue
		}
		view := current
		evaluators = append(evaluators, view.Eval)
	}
	return startNativeTickLoop(ctx, 200*time.Millisecond, views[0].Dispatch, evaluators...)
}

// startNativeTickLoop deliberately dispatches only through the WebView whose
// Run method owns the Windows message loop. The local WebView2 wrapper keeps a
// separate Dispatch queue per WebView instance, so dispatching on an overlay
// whose Run method is never called would leak queued callbacks indefinitely.
func startNativeTickLoop(ctx context.Context, interval time.Duration, dispatch func(func()), evaluators ...func(string)) func() {
	tickCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-tickCtx.Done():
				return
			case <-ticker.C:
				// ExecuteScript is asynchronous. A renderer suspended by an older
				// WebView2 runtime can therefore receive a burst of queued calls when
				// it resumes. Coalesce using the renderer's current clock so stale
				// host timestamps never replay hundreds of publish/poll cycles.
				const script = `(function(){const now=Date.now();const last=Number(window.__dilmeterNativeTickAt)||0;if(now-last<150)return;window.__dilmeterNativeTickAt=now;window.dispatchEvent(new CustomEvent("dilmeter-native-tick",{detail:{nowMs:now}}));})()`
				dispatch(func() {
					for _, evaluate := range evaluators {
						evaluate(script)
					}
				})
			}
		}
	}()
	return cancel
}

func commandLineReplayFile() string {
	if len(os.Args) < 2 {
		return ""
	}
	file := strings.TrimSpace(os.Args[1])
	if file == "" {
		return ""
	}
	if _, err := os.Stat(file); err != nil {
		return ""
	}
	return file
}
