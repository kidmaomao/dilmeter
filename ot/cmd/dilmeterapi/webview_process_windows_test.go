//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestQueryCurrentProcessCommandLine(t *testing.T) {
	commandLine, err := queryProcessCommandLine(uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(commandLine), ".test") {
		t.Fatalf("current process command line was not returned: %q", commandLine)
	}
}

func TestCommandLineUsesWebViewProfile(t *testing.T) {
	target := normalizeCommandLinePath(`C:\Games\Dilmeter\data\webview2`)
	targetWithSuffix := normalizeCommandLinePath(`C:\Games\Dilmeter\data\webview2\EBWebView`)
	commandLine := `msedgewebview2.exe --user-data-dir="C:\Games\Dilmeter\data\webview2\EBWebView" --type=renderer`
	if !commandLineUsesWebViewProfile(commandLine, target, targetWithSuffix) {
		t.Fatal("matching WebView2 profile was not detected")
	}
	if commandLineUsesWebViewProfile(`msedgewebview2.exe --user-data-dir="C:\Other\EBWebView"`, target, targetWithSuffix) {
		t.Fatal("unrelated WebView2 profile was matched")
	}
	if commandLineUsesWebViewProfile(`msedgewebview2.exe --user-data-dir="C:\Games\Dilmeter\data\webview20\EBWebView"`, target, targetWithSuffix) {
		t.Fatal("profile path prefix was incorrectly matched")
	}
	if !commandLineUsesWebViewProfile(`msedgewebview2.exe --user-data-dir "C:\Games\Dilmeter\data\webview2\EBWebView" --type=renderer`, target, targetWithSuffix) {
		t.Fatal("separate quoted WebView2 profile argument was not detected")
	}
	if commandLineUsesWebViewProfile(`msedgewebview2.exe --other="C:\Games\Dilmeter\data\webview2\EBWebView"`, target, targetWithSuffix) {
		t.Fatal("profile path outside --user-data-dir was incorrectly matched")
	}
}

func TestWaitForWebViewProfileReleaseRequiresStableClearState(t *testing.T) {
	originalFind := findWebViewProfileProcesses
	originalInterval := webViewProfilePollInterval
	t.Cleanup(func() {
		findWebViewProfileProcesses = originalFind
		webViewProfilePollInterval = originalInterval
	})
	responses := [][]uint32{{101}, nil, {102}, nil, nil}
	calls := 0
	findWebViewProfileProcesses = func(string) ([]uint32, error) {
		response := responses[calls]
		calls++
		return response, nil
	}
	webViewProfilePollInterval = time.Millisecond
	if err := waitForWebViewProfileRelease(`C:\profile`, time.Second); err != nil {
		t.Fatal(err)
	}
	if calls != len(responses) {
		t.Fatalf("profile release checks = %d, want %d", calls, len(responses))
	}
}
