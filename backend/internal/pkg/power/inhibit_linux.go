//go:build linux

package power

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/godbus/dbus/v5"
)

type linuxBackend struct{}

func systemBackend() Backend { return linuxBackend{} }

// Acquire prefers logind's Inhibit over D-Bus (the returned fd holds the lock
// and the kernel drops it if Rum dies) and falls back to a systemd-inhibit
// child running `sleep infinity`. A missing logind/binary is reported, not fatal.
func (linuxBackend) Acquire(reason string) (func(), error) {
	rel, derr := acquireDBus(reason)
	if derr == nil {
		return rel, nil
	}
	rel, cerr := acquireCommand(reason)
	if cerr == nil {
		return rel, nil
	}
	return nil, fmt.Errorf("dbus: %v; systemd-inhibit: %v", derr, cerr)
}

func acquireDBus(reason string) (func(), error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var fd dbus.UnixFD
	obj := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
	if err := obj.Call("org.freedesktop.login1.Manager.Inhibit", 0,
		"sleep", "Rum", reason, "block").Store(&fd); err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "login1-inhibit")
	if f == nil {
		return nil, fmt.Errorf("invalid inhibitor fd")
	}
	var once sync.Once
	return func() { once.Do(func() { _ = f.Close() }) }, nil
}

func acquireCommand(reason string) (func(), error) {
	path, err := exec.LookPath("systemd-inhibit")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, "--what=sleep", "--who=Rum", "--why="+reason, "--mode=block", "sleep", "infinity")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL} // die with Rum
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Wait() }()
	var once sync.Once
	return func() { once.Do(func() { _ = cmd.Process.Kill() }) }, nil
}
