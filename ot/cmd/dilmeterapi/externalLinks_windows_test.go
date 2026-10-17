//go:build windows

package main

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestExternalLinksUseDefaultWindowsBrowser(t *testing.T) {
	original := shellExecuteExternalLink
	t.Cleanup(func() { shellExecuteExternalLink = original })
	for _, url := range []string{
		"https://www.noginogi.sbs/article/20260724", "https://cn.noginogi.sbs/article/20260724",
		"https://gear.noginogi.sbs/downloads/", "https://noginogi-downloads.netlify.app/downloads/",
	} {
		called := false
		shellExecuteExternalLink = func(hwnd windows.Handle, verb, file, args, cwd *uint16, show int32) error {
			called = true
			if hwnd != 0 || windows.UTF16PtrToString(verb) != "open" || windows.UTF16PtrToString(file) != url || args != nil || cwd != nil || show != windows.SW_SHOWNORMAL {
				t.Fatal("link must use the OS URL association directly without a hard-coded browser or command arguments")
			}
			return nil
		}
		if err := openExternalLink(url); err != nil || !called {
			t.Fatalf("link was not opened: %s: %v", url, err)
		}
	}
	expected := errors.New("no default handler")
	shellExecuteExternalLink = func(windows.Handle, *uint16, *uint16, *uint16, *uint16, int32) error { return expected }
	if err := openExternalLink("https://www.noginogi.sbs/article/20260724"); !errors.Is(err, expected) {
		t.Fatal("OS failure must reach the WebView promise")
	}
}

func TestExternalLinksRejectFilesCommandsAndOtherDestinations(t *testing.T) {
	original := shellExecuteExternalLink
	t.Cleanup(func() { shellExecuteExternalLink = original })
	shellExecuteExternalLink = func(windows.Handle, *uint16, *uint16, *uint16, *uint16, int32) error {
		t.Fatal("invalid link reached Windows")
		return nil
	}
	for _, url := range []string{
		"", "file:///C:/Windows/System32/cmd.exe", "javascript:alert(1)", "ms-settings:defaultapps",
		"https://www.noginogi.sbs.evil.example/article/20260724", "https://www.noginogi.sbs@evil.example/article/20260724",
		"https://www.noginogi.sbs/article/20260724?redirect=https://example.com", "https://www.noginogi.sbs/article/20260724\x00",
	} {
		if err := openExternalLink(url); err == nil {
			t.Fatalf("unexpectedly accepted %q", url)
		}
	}
}
