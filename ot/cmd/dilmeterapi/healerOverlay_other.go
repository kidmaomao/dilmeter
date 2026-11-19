//go:build !windows

package main

// The browser uses its own display scale when automatic DPI is requested without a native window.
func healerOverlayDPI(configured int) int { return configured }
