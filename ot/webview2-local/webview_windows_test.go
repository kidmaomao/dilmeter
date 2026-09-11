//go:build windows

package webview2

import (
	"strings"
	"testing"

	"github.com/jchv/go-webview2/internal/w32"
)

type windowStateBrowser struct {
	resizeCount int
	showCount   int
	hideCount   int
	evalScripts []string
}

func (b *windowStateBrowser) Embed(uintptr) bool                       { return true }
func (b *windowStateBrowser) Resize()                                  { b.resizeCount++ }
func (b *windowStateBrowser) Show() error                              { b.showCount++; return nil }
func (b *windowStateBrowser) Hide() error                              { b.hideCount++; return nil }
func (b *windowStateBrowser) Navigate(string)                          {}
func (b *windowStateBrowser) NavigateToString(string)                  {}
func (b *windowStateBrowser) Init(string)                              {}
func (b *windowStateBrowser) Eval(script string)                       { b.evalScripts = append(b.evalScripts, script) }
func (b *windowStateBrowser) NotifyParentWindowPositionChanged() error { return nil }
func (b *windowStateBrowser) Focus()                                   {}

func TestHandleWindowSizeSkipsZeroBoundsWhenMinimized(t *testing.T) {
	browser := &windowStateBrowser{}
	view := &webview{browser: browser}

	view.handleWindowSize(w32.SizeMinimized)
	if browser.hideCount != 1 || browser.resizeCount != 0 || browser.showCount != 0 {
		t.Fatalf("minimize calls: hide=%d resize=%d show=%d", browser.hideCount, browser.resizeCount, browser.showCount)
	}

	view.handleWindowSize(w32.SizeRestored)
	if browser.hideCount != 1 || browser.resizeCount != 1 || browser.showCount != 1 {
		t.Fatalf("restore calls: hide=%d resize=%d show=%d", browser.hideCount, browser.resizeCount, browser.showCount)
	}
}

func TestHandleWindowSizeKeepsMonitoringRendererActiveWhenMinimized(t *testing.T) {
	browser := &windowStateBrowser{}
	view := &webview{browser: browser, keepAliveWhenMinimized: true}

	view.handleWindowSize(w32.SizeMinimized)
	if browser.hideCount != 0 || browser.resizeCount != 0 || browser.showCount != 0 {
		t.Fatalf("keep-alive minimize calls: hide=%d resize=%d show=%d", browser.hideCount, browser.resizeCount, browser.showCount)
	}
	if len(browser.evalScripts) != 1 || !strings.Contains(browser.evalScripts[0], "minimized:true") {
		t.Fatalf("missing minimized lifecycle event: %#v", browser.evalScripts)
	}

	view.handleWindowSize(w32.SizeRestored)
	if browser.resizeCount != 1 || browser.showCount != 1 {
		t.Fatalf("keep-alive restore calls: resize=%d show=%d", browser.resizeCount, browser.showCount)
	}
	if len(browser.evalScripts) != 2 || !strings.Contains(browser.evalScripts[1], "minimized:false") {
		t.Fatalf("missing restored lifecycle event: %#v", browser.evalScripts)
	}
}

func TestKeepAliveRestoreEventRequiresPriorMinimize(t *testing.T) {
	browser := &windowStateBrowser{}
	view := &webview{browser: browser, keepAliveWhenMinimized: true}

	view.handleWindowSize(w32.SizeRestored)
	if len(browser.evalScripts) != 0 {
		t.Fatalf("ordinary resize emitted restore lifecycle event: %#v", browser.evalScripts)
	}
}

func TestActivationStateUsesLowWord(t *testing.T) {
	const minimizedInactive = uintptr(1 << 16)
	if got := lowWord(minimizedInactive); got != w32.WAInactive {
		t.Fatalf("activation state = %d, want inactive", got)
	}
	if got := lowWord(minimizedInactive | w32.WAActive); got != w32.WAActive {
		t.Fatalf("activation state = %d, want active", got)
	}
}
