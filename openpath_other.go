//go:build !windows

package main

import "os/exec"

// setRawCmdLine is only needed on Windows.
func setRawCmdLine(*exec.Cmd, string) {}
