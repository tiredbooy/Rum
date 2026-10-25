//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// setRawCmdLine overrides Go's argv quoting with an exact command line.
func setRawCmdLine(cmd *exec.Cmd, line string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
}
