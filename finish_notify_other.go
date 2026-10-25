//go:build !windows

package main

import (
	"context"
	"errors"
)

func windowsDialog(context.Context, string, string) (string, error) {
	return "", errors.New("windows dialog unavailable")
}
