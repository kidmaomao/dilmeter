package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const appDirectoryName = "DilmeterOT"

var BuildTime = "LOCAL BUILD"
var configMu sync.Mutex
var portableDataOnce sync.Once
var portableDataPath string

type config struct {
	Key                      string   `yaml:"key"`
	IV                       string   `yaml:"iv"`
	Nic                      string   `yaml:"nic"`
	AutoConnect              bool     `yaml:"auto_connect"`
	AllowRemote              bool     `yaml:"allow_remote"`
	GameServerNetwork        string   `yaml:"game_server_network"`
	GameServerPorts          []string `yaml:"game_server_ports"`
	AcceleratorMode          bool     `yaml:"accelerator_mode"`
	BuffOverlayLocked        bool     `yaml:"buff_overlay_locked"`
	BuffOverlayX             int      `yaml:"buff_overlay_x"`
	BuffOverlayY             int      `yaml:"buff_overlay_y"`
	BuffOverlayPositionSet   bool     `yaml:"buff_overlay_position_set"`
	DebuffOverlayX           int      `yaml:"debuff_overlay_x"`
	DebuffOverlayY           int      `yaml:"debuff_overlay_y"`
	DebuffOverlayPositionSet bool     `yaml:"debuff_overlay_position_set"`
	SkillOverlayX            int      `yaml:"skill_overlay_x"`
	SkillOverlayY            int      `yaml:"skill_overlay_y"`
	SkillOverlayPositionSet  bool     `yaml:"skill_overlay_position_set"`
	SkillBarX                int      `yaml:"skill_bar_x"`
	SkillBarY                int      `yaml:"skill_bar_y"`
	SkillBarPositionSet      bool     `yaml:"skill_bar_position_set"`
}

func findConfigPath() string {
	// cwd
	const configFileName = "config.yaml"
	if _, err := os.Stat(configFileName); err == nil {
		return configFileName
	}

	// executable directory
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		configPath := filepath.Join(exeDir, configFileName)

		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	return filepath.Join(appDataDir(), configFileName)
}

func appDataDir() string {
	portableDataOnce.Do(func() {
		base := "."
		if exe, err := os.Executable(); err == nil && exe != "" {
			base = filepath.Dir(exe)
			if strings.HasSuffix(strings.ToLower(filepath.Base(exe)), ".test.exe") {
				portableDataPath = filepath.Join(os.TempDir(), appDirectoryName+"-test")
				_ = os.MkdirAll(portableDataPath, 0755)
				return
			}
		} else if cwd, err := os.Getwd(); err == nil && cwd != "" {
			base = cwd
		}
		portableDataPath = filepath.Join(base, "data")
		if err := os.MkdirAll(portableDataPath, 0755); err != nil {
			portableDataPath = base
			return
		}
		migrateLegacyAppData(portableDataPath)
	})
	return portableDataPath
}

func migrateLegacyAppData(target string) {
	marker := filepath.Join(target, ".migration-from-appdata-v1")
	if _, err := os.Stat(marker); err == nil {
		return
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return
	}
	source := filepath.Join(base, appDirectoryName)
	if samePath(source, target) {
		return
	}
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		_ = os.WriteFile(marker, []byte("no legacy directory\n"), 0644)
		return
	}
	if err := copyMissingTree(source, target); err == nil {
		_ = os.WriteFile(marker, []byte("copied\n"), 0644)
	}
}

func copyMissingTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		if _, err := os.Stat(destination); err == nil {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			_ = in.Close()
			if os.IsExist(err) {
				return nil
			}
			return err
		}
		_, copyErr := io.Copy(out, in)
		inCloseErr := in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if inCloseErr != nil {
			return inCloseErr
		}
		return closeErr
	})
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func loadConfig() config {
	configMu.Lock()
	defer configMu.Unlock()
	return loadConfigUnlocked()
}

func loadConfigUnlocked() config {
	var cfg config

	data, err := os.ReadFile(findConfigPath())
	if err != nil {
		return cfg
	}

	yaml.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg config) error {
	configMu.Lock()
	defer configMu.Unlock()
	return saveConfigUnlocked(cfg)
}

func saveConfigUnlocked(cfg config) error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(findConfigPath(), data, 0644)
}

func updateConfig(update func(*config)) error {
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadConfigUnlocked()
	update(&cfg)
	return saveConfigUnlocked(cfg)
}
