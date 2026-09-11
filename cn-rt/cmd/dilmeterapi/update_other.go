//go:build !windows

package main

func revealDownloadedUpdate(_ string) {}

func runUpdateHelperMode() bool { return false }

func startVerifiedUpdateInstall(_ string, _ updateManifest) error {
	return nil
}
