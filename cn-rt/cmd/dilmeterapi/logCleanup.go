package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitlab.com/prilus/mabidilmeter/constants"
)

const logCleanupSizeThreshold int64 = 10 * 1024 * 1024 * 1024

var (
	logCleanupMu     sync.Mutex
	logCleanupNow    = time.Now
	recycleLogFiles  = moveFilesToRecycleBin
	errUnsafeLogPath = errors.New("unsafe log path")
)

type managedLogFile struct {
	Path     string
	Name     string
	Size     int64
	Modified time.Time
	Active   bool
}

type logCleanupStatus struct {
	TotalCount         int    `json:"totalCount"`
	TotalBytes         int64  `json:"totalBytes"`
	OldestModifiedAt   string `json:"oldestModifiedAt,omitempty"`
	NewestModifiedAt   string `json:"newestModifiedAt,omitempty"`
	SizeThresholdBytes int64  `json:"sizeThresholdBytes"`
	AgeThresholdMonths int    `json:"ageThresholdMonths"`
	ExceedsSize        bool   `json:"exceedsSize"`
	ExceedsAge         bool   `json:"exceedsAge"`
	ShouldPrompt       bool   `json:"shouldPrompt"`
	DefaultBeforeDate  string `json:"defaultBeforeDate"`
	BeforeDate         string `json:"beforeDate"`
	EligibleCount      int    `json:"eligibleCount"`
	EligibleBytes      int64  `json:"eligibleBytes"`
}

type logCleanupRequest struct {
	Before string `json:"before"`
}

type logCleanupResult struct {
	MovedCount int    `json:"movedCount"`
	MovedBytes int64  `json:"movedBytes"`
	BeforeDate string `json:"beforeDate"`
	Message    string `json:"message"`
}

func handleLogCleanup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	switch r.Method {
	case http.MethodGet:
		now := logCleanupNow()
		before, err := parseLogCleanupDate(r.URL.Query().Get("before"), now)
		if err != nil {
			writeLogCleanupError(w, http.StatusBadRequest, err)
			return
		}
		files, err := scanManagedLogFiles(_logDir)
		if err != nil {
			writeLogCleanupError(w, http.StatusInternalServerError, fmt.Errorf("读取日志目录失败：%w", err))
			return
		}
		writeLogCleanupJSON(w, http.StatusOK, buildLogCleanupStatus(files, now, before))

	case http.MethodPost:
		var request logCleanupRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeLogCleanupError(w, http.StatusBadRequest, errors.New("清理请求格式不正确"))
			return
		}
		now := logCleanupNow()
		before, err := parseLogCleanupDate(request.Before, now)
		if err != nil {
			writeLogCleanupError(w, http.StatusBadRequest, err)
			return
		}

		logCleanupMu.Lock()
		defer logCleanupMu.Unlock()

		files, err := scanManagedLogFiles(_logDir)
		if err != nil {
			writeLogCleanupError(w, http.StatusInternalServerError, fmt.Errorf("读取日志目录失败：%w", err))
			return
		}
		eligible := eligibleLogFiles(files, before)
		paths, err := validateLogCleanupPaths(_logDir, eligible)
		if err != nil {
			writeLogCleanupError(w, http.StatusConflict, errors.New("日志目录在确认后发生了变化，请重新选择"))
			return
		}
		if len(paths) == 0 {
			writeLogCleanupJSON(w, http.StatusOK, logCleanupResult{
				BeforeDate: before.Format("2006-01-02"),
				Message:    "所选日期之前没有可清理的历史日志。",
			})
			return
		}

		if err := recycleLogFiles(paths); err != nil {
			writeLogCleanupError(w, http.StatusInternalServerError, fmt.Errorf("无法将日志移入 Windows 回收站：%w", err))
			return
		}
		var movedBytes int64
		for _, file := range eligible {
			movedBytes += file.Size
		}
		writeLogCleanupJSON(w, http.StatusOK, logCleanupResult{
			MovedCount: len(paths),
			MovedBytes: movedBytes,
			BeforeDate: before.Format("2006-01-02"),
			Message:    "历史日志已移入 Windows 回收站。请手动清空回收站，才能真正释放磁盘空间。",
		})

	default:
		w.Header().Set("Allow", "GET, POST")
		writeLogCleanupError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func scanManagedLogFiles(dir string) ([]managedLogFile, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	activeNames := currentActiveLogNames()
	files := make([]managedLogFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isManagedLogName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, managedLogFile{
			Path:     filepath.Join(dir, entry.Name()),
			Name:     entry.Name(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			Active:   activeNames[strings.ToLower(entry.Name())],
		})
	}
	return files, nil
}

func isManagedLogName(name string) bool {
	lower := strings.ToLower(name)
	return (strings.HasPrefix(lower, "log_") && strings.HasSuffix(lower, ".txt")) ||
		(strings.HasPrefix(lower, "packet_log_") && strings.HasSuffix(lower, ".ndjson")) ||
		(strings.HasPrefix(lower, "packet_log_") && strings.HasSuffix(lower, ".ndjson.checkpoints")) ||
		(strings.HasPrefix(lower, "packet_capture_") && strings.HasSuffix(lower, ".pcapng"))
}

func currentActiveLogNames() map[string]bool {
	return map[string]bool{
		strings.ToLower(fmt.Sprintf("log_%v.txt", constants.SERVER_START_AT_STR)):                       true,
		strings.ToLower(fmt.Sprintf("packet_log_%v.ndjson", constants.SERVER_START_AT_STR)):             true,
		strings.ToLower(fmt.Sprintf("packet_log_%v.ndjson.checkpoints", constants.SERVER_START_AT_STR)): true,
		strings.ToLower(fmt.Sprintf("packet_capture_%v.pcapng", constants.SERVER_START_AT)):             true,
	}
}

func parseLogCleanupDate(value string, now time.Time) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		value = now.AddDate(0, -2, 0).Format("2006-01-02")
	}
	date, err := time.ParseInLocation("2006-01-02", value, now.Location())
	if err != nil {
		return time.Time{}, errors.New("请选择有效的截止日期")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if date.After(today) {
		return time.Time{}, errors.New("截止日期不能晚于今天")
	}
	return date, nil
}

func buildLogCleanupStatus(files []managedLogFile, now, before time.Time) logCleanupStatus {
	status := logCleanupStatus{
		SizeThresholdBytes: logCleanupSizeThreshold,
		AgeThresholdMonths: 2,
		DefaultBeforeDate:  now.AddDate(0, -2, 0).Format("2006-01-02"),
		BeforeDate:         before.Format("2006-01-02"),
	}
	ageThreshold := now.AddDate(0, -2, 0)
	var oldest time.Time
	var newest time.Time
	for _, file := range files {
		status.TotalCount++
		status.TotalBytes += file.Size
		if oldest.IsZero() || file.Modified.Before(oldest) {
			oldest = file.Modified
		}
		if newest.IsZero() || file.Modified.After(newest) {
			newest = file.Modified
		}
		if file.Modified.Before(ageThreshold) {
			status.ExceedsAge = true
		}
	}
	if !oldest.IsZero() {
		status.OldestModifiedAt = oldest.Format(time.RFC3339)
		status.NewestModifiedAt = newest.Format(time.RFC3339)
	}
	for _, file := range eligibleLogFiles(files, before) {
		status.EligibleCount++
		status.EligibleBytes += file.Size
	}
	status.ExceedsSize = status.TotalBytes > logCleanupSizeThreshold
	status.ShouldPrompt = status.ExceedsSize || status.ExceedsAge
	return status
}

func eligibleLogFiles(files []managedLogFile, before time.Time) []managedLogFile {
	eligible := make([]managedLogFile, 0, len(files))
	for _, file := range files {
		if !file.Active && file.Modified.Before(before) {
			eligible = append(eligible, file)
		}
	}
	return eligible
}

func validateLogCleanupPaths(dir string, files []managedLogFile) ([]string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path, err := filepath.Abs(file.Path)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !isManagedLogName(info.Name()) {
			return nil, errUnsafeLogPath
		}
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, errUnsafeLogPath
		}
		relative, err := filepath.Rel(root, resolvedPath)
		if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) || filepath.Dir(relative) != "." {
			return nil, errUnsafeLogPath
		}
		if currentActiveLogNames()[strings.ToLower(info.Name())] {
			return nil, errUnsafeLogPath
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func writeLogCleanupError(w http.ResponseWriter, status int, err error) {
	writeLogCleanupJSON(w, status, map[string]string{"error": err.Error()})
}

func writeLogCleanupJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logger.Println("log cleanup response failed:", err)
	}
}
