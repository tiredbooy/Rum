//go:build darwin

package power

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type caffeinateBackend struct{}

func systemBackend() Backend { return caffeinateBackend{} }

// Acquire runs `caffeinate -i -w <pid>`: -i prevents idle system sleep (not
// display sleep) and -w makes caffeinate exit on its own if Rum dies.
func (caffeinateBackend) Acquire(string) (func(), error) {
	path, err := exec.LookPath("caffeinate")
	if err != nil {
		return nil, fmt.Errorf("caffeinate not found: %w", err)
	}
	cmd := exec.Command(path, "-i", "-w", fmt.Sprint(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Wait() }()
	var once sync.Once
	return func() { once.Do(func() { _ = cmd.Process.Kill() }) }, nil
}
