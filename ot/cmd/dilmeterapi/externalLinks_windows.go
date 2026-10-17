//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

var shellExecuteExternalLink = windows.ShellExecute

// Only the built-in help/download pages may cross the WebView-to-OS bridge.
// Pass the URL directly to the registered Windows handler, never to a shell.
func openExternalLink(rawURL string) error {
	switch rawURL {
	case "https://www.noginogi.sbs/article/20260724",
		"https://cn.noginogi.sbs/article/20260724",
		"https://gear.noginogi.sbs/downloads/",
		"https://noginogi-downloads.netlify.app/downloads/":
	default:
		return errors.New("unsupported external link")
	}
	verb, _ := windows.UTF16PtrFromString("open")
	url, err := windows.UTF16PtrFromString(rawURL)
	if err != nil {
		return err
	}
	if err := shellExecuteExternalLink(0, verb, url, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("open default browser: %w", err)
	}
	return nil
}
