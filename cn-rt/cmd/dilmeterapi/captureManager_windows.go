//go:build windows

package main

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"gitlab.com/prilus/mabidilmeter/lib/pcaputil"
)

const captureDetectionInterval = 2 * time.Second

// startCaptureManager keeps the desktop application alive independently of the
// game. It waits for Client.exe, attaches when a game connection appears, and
// returns to the waiting state after the game disconnects.
func startCaptureManager(ctx context.Context, reader *packet.GameServerPacketReader, acceleratorMode bool, diagnosticMode bool) {
	go func() {
		capturing := false
		captureFollowsConfiguredConnections := false
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
			case change := <-gameServerChanged:
				reader.CloseNic()
				capturing = false
				captureFollowsConfiguredConnections = false
				acceleratorMode = change.AcceleratorMode
				modeMessage := ""
				if acceleratorMode {
					modeMessage = "，加速器兼容模式已开启"
				}
				setAppStatus(appStatus{
					State:   statusWaiting,
					Message: fmt.Sprintf("已应用%s%s，请在游戏内切换一次地图以开始捕捉。", change.Option.Name, modeMessage),
				})
				continue
			case <-ticker.C:
			}

			gameRunning := pcaputil.HasGameConnectionWithMode(acceleratorMode)
			if capturing {
				if gameRunning && (captureFollowsConfiguredConnections || pcaputil.HasActiveCaptureConnection()) {
					continue
				}

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
					Message: "游戏已关闭，监测器正在等待下次启动。",
				})
				continue
			}

			if !gameRunning {
				continue
			}

			setAppStatus(appStatus{
				State:       statusDetecting,
				Message:     "已发现游戏，正在识别网络连接…",
				GameRunning: true,
			})

			var nicName string
			var err error
			if diagnosticMode {
				nicName, err = pcaputil.FindNicWithDiagnosticMode(acceleratorMode)
			} else {
				nicName, err = pcaputil.FindNicWithMode(acceleratorMode)
			}
			if err != nil {
				logger.Println("automatic capture detection failed:", err)
				if attempted, started := tryAlternateCapture(reader); attempted {
					capturing = started
					continue
				}
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
