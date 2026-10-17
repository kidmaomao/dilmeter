//go:build !windows

package main

import "os/exec"

func hideEdgeTTSCommand(_ *exec.Cmd) {}
