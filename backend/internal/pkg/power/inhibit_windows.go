//go:build windows

package power

import (
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var procSetThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

type executionStateBackend struct{}

func systemBackend() Backend { return executionStateBackend{} }

// Acquire calls SetThreadExecutionState(ES_CONTINUOUS|ES_SYSTEM_REQUIRED) on a
// dedicated locked OS thread. The state belongs to the calling thread, so the
// same thread must clear it; it is cleared (and the thread freed) on release.
func (executionStateBackend) Acquire(string) (func(), error) {
	stop := make(chan struct{})
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		// Never unlocked: the thread exits with this goroutine, dropping any state.
		r, _, err := procSetThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired))
		if r == 0 {
			ready <- err
			return
		}
		ready <- nil
		<-stop
		procSetThreadExecutionState.Call(uintptr(esContinuous))
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { close(stop) }) }, nil
}
