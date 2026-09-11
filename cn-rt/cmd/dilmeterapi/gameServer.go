package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gitlab.com/prilus/mabidilmeter/constants"
)

const defaultGameServerID = "irusha"

type gameServerOption struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Network string   `json:"network"`
	Ports   []string `json:"ports"`
}

type gameServerSettings struct {
	Selected        string             `json:"selected"`
	Options         []gameServerOption `json:"options"`
	AcceleratorMode bool               `json:"acceleratorMode"`
	Message         string             `json:"message,omitempty"`
}

type gameServerChange struct {
	Option          gameServerOption
	AcceleratorMode bool
}

var gameServerOptions = []gameServerOption{
	{ID: "irusha", Name: "CN服伊鲁夏", Network: "211.147.76.0/24", Ports: []string{"11020", "11021", "11023"}},
	{ID: "yate", Name: "CN服亚特", Network: "61.164.61.0/24", Ports: []string{"11020", "11021", "11023"}},
}

var gameServerChanged = make(chan gameServerChange, 1)

func normalizeGameServerID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, option := range gameServerOptions {
		if option.ID == id {
			return id
		}
	}
	return defaultGameServerID
}

func findGameServer(id string) (gameServerOption, bool) {
	for _, option := range gameServerOptions {
		if option.ID == id {
			return option, true
		}
	}
	return gameServerOption{}, false
}

func applyGameServer(id string) (gameServerOption, error) {
	option, ok := findGameServer(normalizeGameServerID(id))
	if !ok {
		return gameServerOption{}, fmt.Errorf("unknown game server %q", id)
	}
	if err := constants.ConfigureGameServer(option.Network, option.Ports); err != nil {
		return gameServerOption{}, err
	}
	return option, nil
}

func handleGameServer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	switch r.Method {
	case http.MethodGet:
		cfg := loadConfig()
		json.NewEncoder(w).Encode(gameServerSettings{
			Selected:        normalizeGameServerID(cfg.GameServer),
			Options:         gameServerOptions,
			AcceleratorMode: cfg.AcceleratorMode,
		})
	case http.MethodPut:
		var request struct {
			Selected        string `json:"selected"`
			AcceleratorMode bool   `json:"acceleratorMode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "无效的服务器设置", http.StatusBadRequest)
			return
		}
		option, ok := findGameServer(strings.ToLower(strings.TrimSpace(request.Selected)))
		if !ok {
			http.Error(w, "不支持的服务器", http.StatusBadRequest)
			return
		}
		if err := constants.ConfigureGameServer(option.Network, option.Ports); err != nil {
			http.Error(w, "无法应用服务器设置", http.StatusInternalServerError)
			return
		}
		if err := updateConfig(func(cfg *config) {
			cfg.GameServer = option.ID
			cfg.AcceleratorMode = request.AcceleratorMode
		}); err != nil {
			http.Error(w, "无法保存服务器设置", http.StatusInternalServerError)
			return
		}

		select {
		case <-gameServerChanged:
		default:
		}
		gameServerChanged <- gameServerChange{
			Option:          option,
			AcceleratorMode: request.AcceleratorMode,
		}
		modeMessage := ""
		if request.AcceleratorMode {
			modeMessage = "，已开启加速器兼容模式"
		}
		json.NewEncoder(w).Encode(gameServerSettings{
			Selected:        option.ID,
			Options:         gameServerOptions,
			AcceleratorMode: request.AcceleratorMode,
			Message:         fmt.Sprintf("已应用%s%s，请在游戏内切换一次地图以开始捕捉。", option.Name, modeMessage),
		})
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
