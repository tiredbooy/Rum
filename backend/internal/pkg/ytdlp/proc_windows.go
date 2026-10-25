//go:build windows

package ytdlp

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// setProcAttr hides the console window yt-dlp would otherwise flash.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree ends yt-dlp and any ffmpeg child it started.
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if kill.Run() == nil {
		return nil
	}
	return cmd.Process.Kill()
}
