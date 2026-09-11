//go:build windows

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

func revealDownloadedUpdate(path string) {
	_ = exec.Command("explorer.exe", "/select,", filepath.Clean(path)).Start()
}

const updateHelperMode = "--apply-verified-update"

var (
	updateShutdownDelay         = 1400 * time.Millisecond
	updateShutdownFallbackDelay = 7 * time.Second
	updateParentWaitTimeout     = 60 * time.Second
	updatePortWaitTimeout       = 12 * time.Second
	updateWebViewCleanupTimeout = 15 * time.Second
	requestUpdateAppClose       = requestMainWindowClose
	forceUpdateAppExit          = os.Exit
)

type updateHelperConfig struct {
	zipPath       string
	targetDir     string
	currentExe    string
	appName       string
	expectedHash  string
	parentProcess uint32
}

func runUpdateHelperMode() bool {
	config, active, err := parseUpdateHelperArgs(os.Args)
	if !active {
		return false
	}
	if err == nil {
		err = applyVerifiedUpdate(config)
	}
	if err != nil {
		messagebox(fmt.Sprintf("自动更新失败：%v\n\n请重新打开原路径程序，或下载 ZIP 手动更新。", err))
	}
	return true
}

func startVerifiedUpdateInstall(zipPath string, manifest updateManifest) error {
	currentExe, err := os.Executable()
	if err != nil {
		return err
	}
	currentExe, err = filepath.Abs(currentExe)
	if err != nil {
		return err
	}
	helperDir, err := os.MkdirTemp("", "dilmeter-update-helper-*")
	if err != nil {
		return err
	}
	helperPath := filepath.Join(helperDir, "DilmeterUpdateHelper.exe")
	if err := copyRegularFile(currentExe, helperPath, 0755); err != nil {
		_ = os.RemoveAll(helperDir)
		return err
	}
	command := exec.Command(
		helperPath,
		updateHelperMode,
		"--zip", zipPath,
		"--target", filepath.Dir(currentExe),
		"--current-exe", currentExe,
		"--app", AppName,
		"--sha256", strings.ToLower(strings.TrimSpace(manifest.SHA256)),
		"--parent", strconv.Itoa(os.Getpid()),
	)
	command.Dir = filepath.Dir(currentExe)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(helperDir)
		return err
	}
	scheduleApplicationExitForUpdate()
	return nil
}

func scheduleApplicationExitForUpdate() {
	go func() {
		time.Sleep(updateShutdownDelay)
		if !requestUpdateAppClose() {
			logger.Println("update shutdown: main window was not ready for WM_CLOSE")
		}
		time.Sleep(updateShutdownFallbackDelay)
		logger.Println("update shutdown: graceful close timed out; forcing exit")
		forceUpdateAppExit(0)
	}()
}

func parseUpdateHelperArgs(args []string) (updateHelperConfig, bool, error) {
	if len(args) < 2 || args[1] != updateHelperMode {
		return updateHelperConfig{}, false, nil
	}
	set := flag.NewFlagSet("dilmeter-update-helper", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	config := updateHelperConfig{}
	parent := 0
	set.StringVar(&config.zipPath, "zip", "", "verified update ZIP")
	set.StringVar(&config.targetDir, "target", "", "installation folder")
	set.StringVar(&config.currentExe, "current-exe", "", "current executable")
	set.StringVar(&config.appName, "app", "", "application name")
	set.StringVar(&config.expectedHash, "sha256", "", "verified ZIP hash")
	set.IntVar(&parent, "parent", 0, "parent process id")
	if err := set.Parse(args[2:]); err != nil {
		return config, true, err
	}
	config.parentProcess = uint32(parent)
	if config.zipPath == "" || config.targetDir == "" || config.currentExe == "" || config.appName == "" || len(config.expectedHash) != 64 || parent <= 0 {
		return config, true, errors.New("更新参数不完整")
	}
	return config, true, nil
}

func applyVerifiedUpdate(config updateHelperConfig) error {
	if err := waitForParentProcess(config.parentProcess, updateParentWaitTimeout); err != nil {
		return err
	}
	if err := waitForTCPPortRelease(fmt.Sprintf("127.0.0.1:%d", _port), updatePortWaitTimeout); err != nil {
		return err
	}
	// WebView2 child processes can outlive the main window and keep the shared
	// profile locked. Wait for the actual profile owners instead of assuming a
	// fixed delay is long enough on every machine.
	webViewDataPath := filepath.Join(config.targetDir, "data", "webview2")
	if err := waitForWebViewProfileRelease(webViewDataPath, updateWebViewCleanupTimeout); err != nil {
		return err
	}
	if err := verifyFileSHA256(config.zipPath, config.expectedHash); err != nil {
		return err
	}
	targetDir, err := filepath.Abs(config.targetDir)
	if err != nil {
		return err
	}
	currentExe, err := filepath.Abs(config.currentExe)
	if err != nil {
		return err
	}
	if !pathWithinRoot(targetDir, currentExe) || !strings.EqualFold(filepath.Ext(currentExe), ".exe") {
		return errors.New("当前程序路径不在安装目录内")
	}
	stageDir, err := os.MkdirTemp(targetDir, ".dilmeter-update-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)
	files, err := extractVerifiedUpdateZip(config.zipPath, stageDir)
	if err != nil {
		return err
	}
	newExeRelative, err := chooseUpdateExecutable(files, config.appName)
	if err != nil {
		return err
	}
	backupDir, err := os.MkdirTemp(targetDir, ".dilmeter-update-backup-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(backupDir)
	if err := installStagedUpdate(stageDir, targetDir, backupDir, files, newExeRelative, filepath.Base(currentExe)); err != nil {
		return err
	}
	// Do not restart the application from the helper. WebView2 child processes
	// can linger after the old host exits and reopening the shared profile here
	// has caused a blank main window on some machines. The verified update is
	// installed in place; the user starts Dilmeter again when convenient.
	_ = os.Remove(config.zipPath)
	return nil
}

func waitForParentProcess(processID uint32, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, processID)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return fmt.Errorf("wait for old Dilmeter process: %w", err)
	}
	defer windows.CloseHandle(handle)
	waitMilliseconds := uint32(timeout / time.Millisecond)
	result, err := windows.WaitForSingleObject(handle, waitMilliseconds)
	if err != nil {
		return fmt.Errorf("wait for old Dilmeter process: %w", err)
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("old Dilmeter process did not exit within %s", timeout)
	}
	return nil
}

func waitForTCPPortRelease(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := checkTCPListenAvailable(address); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("local service %s was not released within %s", address, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func verifyFileSHA256(filePath, expected string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected) {
		return errors.New("更新 ZIP 的 SHA-256 校验失败")
	}
	return nil
}

func extractVerifiedUpdateZip(zipPath, stageDir string) ([]string, error) {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	if len(archive.File) == 0 || len(archive.File) > 2000 {
		return nil, errors.New("更新 ZIP 文件数量异常")
	}
	const maxExpandedBytes uint64 = 1 << 30
	var expandedBytes uint64
	files := make([]string, 0, len(archive.File))
	for _, entry := range archive.File {
		entryName, err := decodedUpdateZipEntryName(entry)
		if err != nil {
			return nil, err
		}
		relative, err := safeUpdateRelativePath(entryName)
		if err != nil {
			return nil, err
		}
		if relative == "" {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("更新 ZIP 不允许符号链接：%s", entry.Name)
		}
		expandedBytes += entry.UncompressedSize64
		if expandedBytes > maxExpandedBytes {
			return nil, errors.New("更新 ZIP 解压后超过 1GB")
		}
		destination := filepath.Join(stageDir, filepath.FromSlash(relative))
		if !pathWithinRoot(stageDir, destination) {
			return nil, fmt.Errorf("更新 ZIP 路径越界：%s", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return nil, err
		}
		source, err := entry.Open()
		if err != nil {
			return nil, err
		}
		destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm())
		if err != nil {
			source.Close()
			return nil, err
		}
		_, copyErr := io.Copy(destinationFile, io.LimitReader(source, int64(entry.UncompressedSize64)+1))
		closeDestinationErr := destinationFile.Close()
		closeSourceErr := source.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeDestinationErr != nil {
			return nil, closeDestinationErr
		}
		if closeSourceErr != nil {
			return nil, closeSourceErr
		}
		files = append(files, relative)
	}
	return files, nil
}

// decodedUpdateZipEntryName keeps standards-compliant UTF-8 names untouched,
// while accepting legacy Windows ZIPs whose non-UTF-8 names were written in
// the Simplified Chinese ANSI code page (CP936/GBK). archive/zip deliberately
// exposes those bytes verbatim, so they must be decoded before path checks and
// filesystem operations.
func decodedUpdateZipEntryName(entry *zip.File) (string, error) {
	if entry == nil {
		return "", errors.New("更新 ZIP 包含无效文件条目")
	}
	if !entry.NonUTF8 || utf8.ValidString(entry.Name) {
		return entry.Name, nil
	}

	rawName := []byte(entry.Name)
	if len(rawName) == 0 {
		return "", nil
	}
	const (
		codePageGBK       = 936
		mbErrInvalidChars = 0x00000008
	)
	required, err := windows.MultiByteToWideChar(codePageGBK, mbErrInvalidChars, &rawName[0], int32(len(rawName)), nil, 0)
	if err != nil || required <= 0 {
		return "", fmt.Errorf("更新 ZIP 文件名不是有效的 UTF-8 或 GBK 编码")
	}
	decoded := make([]uint16, required)
	written, err := windows.MultiByteToWideChar(codePageGBK, mbErrInvalidChars, &rawName[0], int32(len(rawName)), &decoded[0], int32(len(decoded)))
	if err != nil || written != required {
		return "", fmt.Errorf("更新 ZIP 文件名 GBK 解码失败")
	}
	name := syscall.UTF16ToString(decoded[:written])
	if name == "" || strings.ContainsRune(name, '\x00') {
		return "", errors.New("更新 ZIP 包含无效文件名")
	}
	return name, nil
}

func safeUpdateRelativePath(value string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "" {
		return "", nil
	}
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(strings.Split(cleaned, "/")[0], ":") {
		return "", fmt.Errorf("更新 ZIP 包含不安全路径：%s", value)
	}
	return cleaned, nil
}

func chooseUpdateExecutable(files []string, appName string) (string, error) {
	exactName := strings.ToLower(appName + ".exe")
	versionedPrefix := strings.ToLower(appName + "-v")
	var candidate string
	for _, relative := range files {
		base := strings.ToLower(filepath.Base(relative))
		if base == exactName {
			return relative, nil
		}
		if strings.HasPrefix(base, versionedPrefix) && strings.HasSuffix(base, ".exe") && candidate == "" {
			candidate = relative
		}
	}
	if candidate == "" {
		return "", fmt.Errorf("更新 ZIP 中未找到 %s 可执行文件", appName)
	}
	return candidate, nil
}

type updateInstallMove struct {
	destination string
	backup      string
}

func installStagedUpdate(stageDir, targetDir, backupDir string, files []string, newExeRelative, currentExeName string) error {
	moves := make([]updateInstallMove, 0, len(files))
	rollback := func() {
		for index := len(moves) - 1; index >= 0; index-- {
			move := moves[index]
			_ = os.Remove(move.destination)
			if move.backup != "" {
				_ = os.MkdirAll(filepath.Dir(move.destination), 0755)
				_ = os.Rename(move.backup, move.destination)
			}
		}
	}
	for _, relative := range files {
		source := filepath.Join(stageDir, filepath.FromSlash(relative))
		info, err := os.Stat(source)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		destinationRelative := relative
		if relative == newExeRelative {
			destinationRelative = currentExeName
		}
		destination := filepath.Join(targetDir, filepath.FromSlash(destinationRelative))
		if !pathWithinRoot(targetDir, destination) {
			rollback()
			return fmt.Errorf("更新目标路径越界：%s", relative)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			rollback()
			return err
		}
		backup := ""
		if existing, statErr := os.Stat(destination); statErr == nil && existing.Mode().IsRegular() {
			backup = filepath.Join(backupDir, filepath.FromSlash(destinationRelative))
			if err := os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
				rollback()
				return err
			}
			if err := os.Rename(destination, backup); err != nil {
				rollback()
				return err
			}
		}
		if err := os.Rename(source, destination); err != nil {
			if backup != "" {
				_ = os.Rename(backup, destination)
			}
			rollback()
			return err
		}
		moves = append(moves, updateInstallMove{destination: destination, backup: backup})
	}
	return nil
}

func rollbackInstalledUpdate(targetDir, backupDir string, files []string, newExeRelative, currentExeName string) {
	for _, relative := range files {
		destinationRelative := relative
		if relative == newExeRelative {
			destinationRelative = currentExeName
		}
		_ = os.Remove(filepath.Join(targetDir, filepath.FromSlash(destinationRelative)))
	}
	_ = filepath.WalkDir(backupDir, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(backupDir, filePath)
		if err != nil {
			return err
		}
		destination := filepath.Join(targetDir, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		return os.Rename(filePath, destination)
	})
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func copyRegularFile(source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
