package main

import (
	"context"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	quitPauseLabel  = "Quit and pause"
	quitCancelLabel = "Cancel"
)

// quitPrompt is the question shown when downloads are running at quit time.
func quitPrompt(n int) string {
	if n == 1 {
		return "1 download is running. Quit and pause it?"
	}
	return fmt.Sprintf("%d downloads are running. Quit and pause them?", n)
}

// confirmQuitWithDownloads asks whether to quit while n downloads are running.
// It is the single prompt for window close and tray quit; a failed dialog
// returns false so a download is never paused by accident.
func confirmQuitWithDownloads(ctx context.Context, n int) bool {
	res, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         DefaultConfirmConfig.Title,
		Message:       quitPrompt(n),
		Buttons:       []string{quitPauseLabel, quitCancelLabel},
		DefaultButton: quitPauseLabel,
		CancelButton:  quitCancelLabel,
	})
	if err != nil {
		log.Println("Quit dialog failed:", err)
		return false
	}
	return res == quitPauseLabel
}
