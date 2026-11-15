//go:build windows

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"gitlab.com/prilus/mabidilmeter/lib/pcaputil"
)

const captureDetectionInterval = 2 * time.Second

// startCaptureManager keeps the desktop application alive independently of the
// game. It waits for Client.exe, attaches when a game connection appears, and
// returns to the waiting state after the game disconnects.
func startCaptureManager(ctx context.Context, reader *packet.GameServerPacketReader, acceleratorMode bool) {
	go func() {
		capturing := false
		captureFollowsConfiguredConnections := false
		connectionMonitor := captureConnectionMonitor{}
		setAppStatus(appStatus{
			State:   statusWaiting,
			Message: "等待洛奇启动。仍可加载和查看历史战斗日志。",
		})

		ticker := time.NewTicker(captureDetectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				reader.CloseNic()
				return
			case settings := <-gameServerChanged:
				reader.CloseNic()
				capturing = false
				captureFollowsConfiguredConnections = false
				connectionMonitor = captureConnectionMonitor{}
				acceleratorMode = settings.AcceleratorMode
				modeMessage := ""
				if acceleratorMode {
					modeMessage = "，加速器兼容模式已开启"
				}
				setAppStatus(appStatus{
					State:   statusWaiting,
					Message: fmt.Sprintf("已应用服务器 %s（端口 %s）%s，请在游戏内切换一次地图以开始捕捉。", settings.Network, strings.Join(settings.Ports, "、"), modeMessage),
				})
				continue
			case <-ticker.C:
			}

			connections, checkErr := pcaputil.CheckCaptureConnections(acceleratorMode)
			now := time.Now()
			connectionMonitor.reportError(now, checkErr)
			gameRunning := connections.GameConnected
			if capturing {
				connected := gameRunning && (captureFollowsConfiguredConnections || connections.ActiveConnected)
				if !connectionMonitor.shouldClose(now, reader.LivePayloadCount(), connected, checkErr) {
					continue
				}

				logger.Printf("capture disconnect confirmed: checks=%d idle=%s game_connected=%t active_connected=%t",
					connectionMonitor.missingChecks, now.Sub(connectionMonitor.lastPayloadAt).Round(time.Millisecond), gameRunning, connections.ActiveConnected)
				if gameRunning {
					logger.Println("game connection changed; rebinding capture for the new channel")
				} else {
					logger.Println("Client.exe connection closed; returning to waiting state")
				}
				reader.CloseNic()
				capturing = false
				captureFollowsConfiguredConnections = false
				setAppStatus(appStatus{
					State:   statusWaiting,
					Message: "游戏连接已断开，监测器正在等待重新连接。",
				})
				continue
			}

			if checkErr != nil || !gameRunning {
				continue
			}

			setAppStatus(appStatus{
				State:       statusDetecting,
				Message:     "已发现游戏，正在识别网络连接…",
				GameRunning: true,
			})

			nicName, err := pcaputil.FindNicWithMode(acceleratorMode)
			if err != nil {
				logger.Println("automatic capture detection failed:", err)
				setAppStatus(appStatus{
					State:       statusDetecting,
					Message:     fmt.Sprintf("已发现游戏，但暂未捕获到数据：%v。将自动重试。", err),
					GameRunning: true,
				})
				continue
			}

			captureFilter := pcaputil.SelectedCaptureFilter()
			if captureFilter != "" {
				err = reader.OpenNicWithFilter(nicName, captureFilter)
			} else {
				captureFilter, captureFollowsConfiguredConnections = constants.GameServerRoamingFilter()
				err = reader.OpenNicWithFilter(nicName, captureFilter)
			}
			if err != nil {
				captureFollowsConfiguredConnections = false
				logger.Println("automatic OpenNic failed:", err)
				setAppStatus(appStatus{
					State:       statusError,
					Message:     fmt.Sprintf("无法打开抓包设备：%v。请检查 Npcap 与权限。", err),
					GameRunning: true,
				})
				continue
			}

			capturing = true
			connectionMonitor = captureConnectionMonitor{lastPayloadAt: time.Now(), lastPayloadCount: reader.LivePayloadCount()}
			setAppStatus(appStatus{
				State:       statusCapturing,
				Message:     "正在监测洛奇战斗数据。",
				GameRunning: true,
				Capturing:   true,
				Interface:   nicName,
			})
		}
	}()
}
