//go:build windows && dilmeter_rt

package main

import (
	"fmt"
	"strings"

	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func tryAlternateCapture(reader *packet.GameServerPacketReader) (attempted bool, started bool) {
	setAppStatus(appStatus{
		State:       statusDetecting,
		Message:     "Npcap 未捕获到游戏正文，正在启用 DilmeterRT 路由模式后端…",
		GameRunning: true,
	})
	filter, err := constants.WinDivertGameServerFilter()
	if err == nil {
		var dllPath string
		dllPath, err = winDivertDLLPath()
		if err == nil {
			err = reader.OpenWinDivert(dllPath, filter)
		}
	}
	if err != nil {
		setRouteCaptureError(err)
		return true, false
	}

	setAppStatus(appStatus{
		State:       statusCapturing,
		Message:     "正在通过 DilmeterRT 路由模式后端监测洛奇战斗数据。请切换一次地图。",
		GameRunning: true,
		Capturing:   true,
		Interface:   "WinDivert/WFP（路由模式）",
	})
	return true, true
}

func setRouteCaptureError(err error) {
	logger.Println("DilmeterRT WinDivert/WFP capture failed:", err)
	message := fmt.Sprintf("DilmeterRT 路由模式后端启动失败：%v。", err)
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "access is denied") || strings.Contains(lower, "access denied") || strings.Contains(lower, "拒绝访问") {
		message = "DilmeterRT 路由模式后端需要管理员权限。请关闭软件，然后右键选择“以管理员身份运行”。"
	}
	setAppStatus(appStatus{
		State:       statusError,
		Message:     message + " 将自动重试。",
		GameRunning: true,
	})
}
