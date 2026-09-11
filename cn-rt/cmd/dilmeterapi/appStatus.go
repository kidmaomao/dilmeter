package main

import (
	"sync"
	"time"
)

const (
	statusStarting  = "starting"
	statusWaiting   = "waiting_game"
	statusDetecting = "detecting_game"
	statusCapturing = "capturing"
	statusReplay    = "replay"
	statusError     = "error"
)

type appStatus struct {
	State       string `json:"state"`
	Message     string `json:"message"`
	GameRunning bool   `json:"gameRunning"`
	Capturing   bool   `json:"capturing"`
	Interface   string `json:"interface,omitempty"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type appStatusStore struct {
	sync.RWMutex
	value appStatus
}

var runtimeStatus = appStatusStore{
	value: appStatus{
		State:     statusStarting,
		Message:   "正在初始化桌面监测器…",
		UpdatedAt: time.Now().Unix(),
	},
}

func setAppStatus(next appStatus) {
	next.UpdatedAt = time.Now().Unix()
	runtimeStatus.Lock()
	runtimeStatus.value = next
	runtimeStatus.Unlock()
}

func getAppStatus() appStatus {
	runtimeStatus.RLock()
	defer runtimeStatus.RUnlock()
	return runtimeStatus.value
}
