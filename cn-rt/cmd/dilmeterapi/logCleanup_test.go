package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanManagedLogFilesOnlyIncludesKnownDirectRegularFiles(t *testing.T) {
	dir := t.TempDir()
	managedNames := []string{
		"log_2026-05-01_10-00-00.txt",
		"packet_log_2026-05-01_10-00-00.ndjson",
		"packet_capture_1780000000.pcapng",
	}
	for _, name := range managedNames {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "archive")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "log_2025-01-01_00-00-00.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := scanManagedLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(managedNames) {
		t.Fatalf("managed file count = %d, want %d", len(files), len(managedNames))
	}
}

func TestBuildLogCleanupStatusThresholds(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, time.August, 21, 12, 0, 0, 0, location)
	before := time.Date(2026, time.June, 21, 0, 0, 0, 0, location)

	status := buildLogCleanupStatus([]managedLogFile{{
		Name:     "log_old.txt",
		Size:     1,
		Modified: now.AddDate(0, -2, -1),
	}}, now, before)
	if !status.ExceedsAge || !status.ShouldPrompt {
		t.Fatalf("old log did not trigger prompt: %+v", status)
	}

	status = buildLogCleanupStatus([]managedLogFile{{
		Name:     "log_large.txt",
		Size:     logCleanupSizeThreshold + 1,
		Modified: now,
	}}, now, before)
	if !status.ExceedsSize || !status.ShouldPrompt {
		t.Fatalf("large log set did not trigger prompt: %+v", status)
	}

	status = buildLogCleanupStatus([]managedLogFile{{
		Name:     "log_boundary.txt",
		Size:     logCleanupSizeThreshold,
		Modified: now.AddDate(0, -2, 0),
	}}, now, before)
	if status.ExceedsSize || status.ExceedsAge || status.ShouldPrompt {
		t.Fatalf("exact thresholds must not count as exceeded: %+v", status)
	}
}

func TestEligibleLogFilesUsesStrictDateAndSkipsActive(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	before := time.Date(2026, time.July, 1, 0, 0, 0, 0, location)
	files := []managedLogFile{
		{Name: "old", Modified: before.Add(-time.Second)},
		{Name: "on-boundary", Modified: before},
		{Name: "current", Modified: before.Add(-time.Hour), Active: true},
	}
	eligible := eligibleLogFiles(files, before)
	if len(eligible) != 1 || eligible[0].Name != "old" {
		t.Fatalf("eligible logs = %+v, want only old", eligible)
	}
}

func TestHandleLogCleanupMovesOnlySelectedHistoryToRecycler(t *testing.T) {
	dir := t.TempDir()
	location := time.FixedZone("test", 8*60*60)
	fixedNow := time.Date(2026, time.August, 21, 12, 0, 0, 0, location)
	oldPath := filepath.Join(dir, "packet_log_2026-06-01_12-00-00.ndjson")
	boundaryPath := filepath.Join(dir, "log_2026-07-01_00-00-00.txt")
	for _, path := range []string{oldPath, boundaryPath} {
		if err := os.WriteFile(path, []byte("test log"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(oldPath, fixedNow, time.Date(2026, time.June, 30, 23, 59, 59, 0, location)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(boundaryPath, fixedNow, time.Date(2026, time.July, 1, 0, 0, 0, 0, location)); err != nil {
		t.Fatal(err)
	}

	previousDir := _logDir
	previousNow := logCleanupNow
	previousRecycler := recycleLogFiles
	_logDir = dir
	logCleanupNow = func() time.Time { return fixedNow }
	var recycled []string
	recycleLogFiles = func(paths []string) error {
		recycled = append(recycled, paths...)
		return nil
	}
	t.Cleanup(func() {
		_logDir = previousDir
		logCleanupNow = previousNow
		recycleLogFiles = previousRecycler
	})

	body := bytes.NewBufferString(`{"before":"2026-07-01"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/log_cleanup", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handleLogCleanup(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if len(recycled) != 1 || !sameTestPath(recycled[0], oldPath) {
		t.Fatalf("recycled paths = %v, want only %s", recycled, oldPath)
	}
	var result logCleanupResult
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.MovedCount != 1 || result.MovedBytes != int64(len("test log")) {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func sameTestPath(left, right string) bool {
	leftAbsolute, _ := filepath.Abs(left)
	rightAbsolute, _ := filepath.Abs(right)
	return filepath.Clean(leftAbsolute) == filepath.Clean(rightAbsolute)
}
