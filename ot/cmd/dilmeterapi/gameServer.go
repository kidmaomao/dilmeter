package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"gitlab.com/prilus/mabidilmeter/constants"
)

const defaultGameServerNetwork = "211.147.76.0/24"

var defaultGameServerPorts = []string{"11020", "11021", "11023"}

type gameServerSettings struct {
	Network         string   `json:"network"`
	Ports           []string `json:"ports"`
	AcceleratorMode bool     `json:"acceleratorMode"`
	Message         string   `json:"message,omitempty"`
}

var gameServerChanged = make(chan gameServerSettings, 1)

func normalizeGameServerNetwork(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("IP 或网段不能为空")
	}

	if !strings.Contains(value, "/") {
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() == nil {
			return "", fmt.Errorf("请输入有效的 IPv4 地址或 CIDR 网段")
		}
		return ip.To4().String() + "/32", nil
	}

	ip, subnet, err := net.ParseCIDR(value)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("请输入有效的 IPv4 CIDR 网段，例如 211.147.76.0/24")
	}
	return subnet.String(), nil
}

func normalizeGameServerPorts(values []string) ([]string, error) {
	var normalized []string
	seen := make(map[string]struct{})
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			switch r {
			case ',', '，', ';', '；', ' ', '\t', '\r', '\n':
				return true
			default:
				return false
			}
		}) {
			portNumber, err := strconv.Atoi(part)
			if err != nil || portNumber < 1 || portNumber > 65535 {
				return nil, fmt.Errorf("端口 %q 无效，请输入 1 到 65535 之间的数字", part)
			}
			port := strconv.Itoa(portNumber)
			if _, ok := seen[port]; ok {
				continue
			}
			seen[port] = struct{}{}
			normalized = append(normalized, port)
		}
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("请至少填写一个端口")
	}
	return normalized, nil
}

func configuredGameServer(cfg config) gameServerSettings {
	network := cfg.GameServerNetwork
	if strings.TrimSpace(network) == "" {
		network = defaultGameServerNetwork
	}
	ports := cfg.GameServerPorts
	if len(ports) == 0 {
		ports = defaultGameServerPorts
	}

	normalizedNetwork, networkErr := normalizeGameServerNetwork(network)
	normalizedPorts, portsErr := normalizeGameServerPorts(ports)
	if networkErr != nil || portsErr != nil {
		return gameServerSettings{
			Network: defaultGameServerNetwork,
			Ports:   append([]string(nil), defaultGameServerPorts...),
		}
	}
	return gameServerSettings{Network: normalizedNetwork, Ports: normalizedPorts}
}

func applyGameServer(settings gameServerSettings) (gameServerSettings, error) {
	network, err := normalizeGameServerNetwork(settings.Network)
	if err != nil {
		return gameServerSettings{}, err
	}
	ports, err := normalizeGameServerPorts(settings.Ports)
	if err != nil {
		return gameServerSettings{}, err
	}
	if err := constants.ConfigureGameServer(network, ports); err != nil {
		return gameServerSettings{}, err
	}
	return gameServerSettings{Network: network, Ports: ports}, nil
}

func handleGameServer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	switch r.Method {
	case http.MethodGet:
		cfg := loadConfig()
		settings := configuredGameServer(cfg)
		settings.AcceleratorMode = cfg.AcceleratorMode
		_ = json.NewEncoder(w).Encode(settings)
	case http.MethodPut:
		var request gameServerSettings
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "无效的服务器设置", http.StatusBadRequest)
			return
		}
		settings, err := applyGameServer(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := updateConfig(func(cfg *config) {
			cfg.GameServerNetwork = settings.Network
			cfg.GameServerPorts = append([]string(nil), settings.Ports...)
			cfg.AcceleratorMode = request.AcceleratorMode
		}); err != nil {
			http.Error(w, "无法保存服务器设置", http.StatusInternalServerError)
			return
		}
		settings.AcceleratorMode = request.AcceleratorMode

		select {
		case <-gameServerChanged:
		default:
		}
		gameServerChanged <- settings
		modeMessage := ""
		if settings.AcceleratorMode {
			modeMessage = "，已开启加速器兼容模式"
		}
		settings.Message = fmt.Sprintf(
			"已应用服务器 %s，端口 %s%s。请在游戏内切换一次地图以开始捕捉。",
			settings.Network,
			strings.Join(settings.Ports, "、"),
			modeMessage,
		)
		_ = json.NewEncoder(w).Encode(settings)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
