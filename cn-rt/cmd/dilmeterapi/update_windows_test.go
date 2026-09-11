//go:build windows

package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSafeUpdateRelativePathRejectsTraversal(t *testing.T) {
	for _, value := range []string{"../evil.exe", "folder/../../evil.exe", `C:\\evil.exe`, `/root/evil.exe`} {
		if _, err := safeUpdateRelativePath(value); err == nil {
			t.Fatalf("unsafe update path %q was accepted", value)
		}
	}
	if got, err := safeUpdateRelativePath(`assets\\icon.png`); err != nil || got != "assets/icon.png" {
		t.Fatalf("safe path normalization = %q, %v", got, err)
	}
}

func TestChooseUpdateExecutable(t *testing.T) {
	for _, appName := range []string{"DilmeterCN", "DilmeterRT", "DilmeterOT"} {
		files := []string{"使用说明.md", appName + "-v1.3.2.exe"}
		if got, err := chooseUpdateExecutable(files, appName); err != nil || got != appName+"-v1.3.2.exe" {
			t.Fatalf("%s versioned executable = %q, %v", appName, got, err)
		}
		files = append(files, appName+".exe")
		if got, err := chooseUpdateExecutable(files, appName); err != nil || got != appName+".exe" {
			t.Fatalf("%s stable executable must win = %q, %v", appName, got, err)
		}
	}
}

func TestInstallStagedUpdateKeepsCurrentExecutableNameAndUserFiles(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	target := filepath.Join(root, "target")
	backup := filepath.Join(root, "backup")
	for _, directory := range []string{stage, target, backup} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(filePath, contents string) {
		if err := os.WriteFile(filePath, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(stage, "DilmeterCN-v1.3.2.exe"), "new executable")
	write(filepath.Join(stage, "使用说明.md"), "new help")
	write(filepath.Join(target, "DilmeterCN-v1.3.0.exe"), "old executable")
	write(filepath.Join(target, "user-config.json"), "keep me")
	files := []string{"DilmeterCN-v1.3.2.exe", "使用说明.md"}
	if err := installStagedUpdate(stage, target, backup, files, files[0], "DilmeterCN-v1.3.0.exe"); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(filepath.Join(target, "DilmeterCN-v1.3.0.exe"))
	if string(updated) != "new executable" {
		t.Fatalf("current executable was not replaced: %q", updated)
	}
	if _, err := os.Stat(filepath.Join(target, "DilmeterCN-v1.3.2.exe")); !os.IsNotExist(err) {
		t.Fatal("versioned archive executable should be installed under the current executable name")
	}
	userConfig, _ := os.ReadFile(filepath.Join(target, "user-config.json"))
	if string(userConfig) != "keep me" {
		t.Fatal("unrelated user file was modified")
	}
	rollbackInstalledUpdate(target, backup, files, files[0], "DilmeterCN-v1.3.0.exe")
	restored, _ := os.ReadFile(filepath.Join(target, "DilmeterCN-v1.3.0.exe"))
	if string(restored) != "old executable" {
		t.Fatalf("installation rollback did not restore the old executable: %q", restored)
	}
	if _, err := os.Stat(filepath.Join(target, "使用说明.md")); !os.IsNotExist(err) {
		t.Fatal("installation rollback should remove newly added files")
	}
}

func TestVerifiedUpdaterHasNoAutomaticRelaunchPath(t *testing.T) {
	for _, fileName := range []string{"update_windows.go", "main_windows.go", "instance_windows.go"} {
		source, err := os.ReadFile(fileName)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "--post-update") {
			t.Fatalf("%s still accepts the retired automatic relaunch argument", fileName)
		}
	}

	source, err := os.ReadFile("update_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	bodyStart := strings.Index(string(source), "func applyVerifiedUpdate")
	if bodyStart < 0 {
		t.Fatal("applyVerifiedUpdate was not found")
	}
	body := string(source[bodyStart:])
	if bodyEnd := strings.Index(body, "\nfunc waitForParentProcess"); bodyEnd >= 0 {
		body = body[:bodyEnd]
	}
	for _, forbidden := range []string{"exec.Command(", "os.StartProcess(", "ShellExecute"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("verified update completion must not launch the application: found %q", forbidden)
		}
	}
}

func TestWaitForTCPPortRelease(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	go func() {
		time.Sleep(60 * time.Millisecond)
		_ = listener.Close()
	}()
	if err := waitForTCPPortRelease(address, time.Second); err != nil {
		t.Fatalf("wait for released port failed: %v", err)
	}

	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := waitForTCPPortRelease(held.Addr().String(), 80*time.Millisecond); err == nil {
		t.Fatal("held port unexpectedly reported as released")
	}
}

func TestUpdateShutdownRequestsGracefulCloseBeforeFallback(t *testing.T) {
	originalDelay := updateShutdownDelay
	originalFallback := updateShutdownFallbackDelay
	originalClose := requestUpdateAppClose
	originalExit := forceUpdateAppExit
	t.Cleanup(func() {
		updateShutdownDelay = originalDelay
		updateShutdownFallbackDelay = originalFallback
		requestUpdateAppClose = originalClose
		forceUpdateAppExit = originalExit
	})

	closeCalled := make(chan struct{}, 1)
	exitCalled := make(chan struct{}, 1)
	updateShutdownDelay = time.Millisecond
	updateShutdownFallbackDelay = time.Millisecond
	requestUpdateAppClose = func() bool {
		closeCalled <- struct{}{}
		return true
	}
	forceUpdateAppExit = func(code int) {
		if code != 0 {
			t.Errorf("forced exit code = %d", code)
		}
		exitCalled <- struct{}{}
	}
	scheduleApplicationExitForUpdate()
	select {
	case <-closeCalled:
	case <-time.After(time.Second):
		t.Fatal("graceful close was not requested")
	}
	select {
	case <-exitCalled:
	case <-time.After(time.Second):
		t.Fatal("fallback exit was not scheduled")
	}
}
