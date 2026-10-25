//go:build windows

package main

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

var procMessageBoxTimeout = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxTimeoutW")

// windowsDialog shows a top-most Yes/No box that times out after 60s. A plain
// message box is used because toast actions need an AppUserModelID and
// protocol activation, neither of which the installer registers. The call blocks
// its own goroutine; if ctx ends first the box is abandoned with the process.
func windowsDialog(ctx context.Context, title, text string) (string, error) {
	if err := procMessageBoxTimeout.Find(); err != nil {
		return "", err
	}
	t, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return "", err
	}
	c, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}
	res := make(chan int, 1)
	go func() {
		r, _, _ := procMessageBoxTimeout.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)),
			uintptr(windowsDialogFlags), 0, uintptr(finishDialogTimeoutMs))
		res <- int(r)
	}()
	select {
	case r := <-res:
		if r == 0 {
			return "", fmt.Errorf("MessageBoxTimeout failed")
		}
		return windowsDialogAction(r), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
