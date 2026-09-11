//go:build windows && dilmeter_rt

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func winDivertDLLPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法确定程序目录：%w", err)
	}
	baseDir := filepath.Dir(executable)
	dllPath := filepath.Join(baseDir, "WinDivert.dll")
	driverPath := filepath.Join(baseDir, "WinDivert64.sys")
	if _, err := os.Stat(dllPath); err != nil {
		return "", fmt.Errorf("缺少 WinDivert.dll，请先完整解压实验版压缩包")
	}
	if _, err := os.Stat(driverPath); err != nil {
		return "", fmt.Errorf("缺少 WinDivert64.sys，请先完整解压实验版压缩包")
	}
	return dllPath, nil
}
